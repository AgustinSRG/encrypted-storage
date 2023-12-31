// Tool to encrypt large files in blocks
// This allows the file to be encrypted,
// without having to decrypt it fully each time a part of it it's requested.
// This is specially important for video files.
// ---
// Main file structure:
// Header:
//   - File size in bytes (uint64 big endian) (8 bytes)
//   - Block size in bytes (uint64 big endian) (8 bytes)
// Block Index:
//   - For each block, in order:
//		- Start pointer: First byte in the file where the block starts (uint64 big endian) (8 bytes)
//      - Block length (Up to the block size defined in the header, can be less) (uint64 big endian) (8 bytes)
// Blocks (rest of the file)
// Every block is encrypted

package encrypted_storage

import (
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"os"
)

//////////////////////////
//     WRITE STREAM    //
/////////////////////////

// Status of a write stream
type FileBlockEncryptWriteStream struct {
	f *os.File // File descriptor

	file_size   int64 // File size in bytes
	block_size  int64 // Block size in bytes
	block_count int64 // Block count

	key []byte // Encryption key

	current_write_index int64 // Current block being written
	current_write_pt    int64 // Position of the file to write the next block

	buf []byte // Write buffer
}

// Creates a write stream
// file - Path of the file to create
// perm - File mode
func CreateFileBlockEncryptWriteStream(file string, perm fs.FileMode) (*FileBlockEncryptWriteStream, error) {
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)

	if err != nil {
		return nil, err
	}

	i := FileBlockEncryptWriteStream{
		f: f,
	}

	return &i, nil
}

// Initializes the file
// Must be called before any writes
// file_size - Size of the original file to encrypt
// block_size - Block size in bytes
// key - Encryption key
func (file *FileBlockEncryptWriteStream) Initialize(file_size int64, block_size int64, key []byte) error {
	blockCount := file_size / block_size

	if file_size%block_size != 0 {
		blockCount++
	}

	file.file_size = file_size
	file.block_count = blockCount
	file.block_size = block_size
	file.key = key

	// Set the size of the file
	err := file.f.Truncate(16 + (16)*blockCount)
	if err != nil {
		return err
	}

	// Rewind to the start of the file
	_, err = file.f.Seek(0, 0)

	if err != nil {
		return err
	}

	// Write size
	b := make([]byte, 8)

	binary.BigEndian.PutUint64(b, uint64(file_size))
	_, err = file.f.Write(b)
	if err != nil {
		return err
	}

	// Write block size
	binary.BigEndian.PutUint64(b, uint64(block_size))
	_, err = file.f.Write(b)
	if err != nil {
		return err
	}

	// Write default block values
	binary.BigEndian.PutUint64(b, 0)
	for i := int64(0); i < blockCount; i++ {
		_, err = file.f.Write(b)
		if err != nil {
			return err
		}

		_, err = file.f.Write(b)
		if err != nil {
			return err
		}
	}

	file.current_write_index = 0
	file.current_write_pt = 16 + (16)*blockCount
	file.buf = make([]byte, 0)

	return nil
}

// Writes data
// data - Chunk of data to write
func (file *FileBlockEncryptWriteStream) Write(data []byte) error {
	if file.current_write_index >= file.block_count {
		return errors.New("Exceeded file size limit")
	}

	file.buf = append(file.buf, data...)

	for int64(len(file.buf)) >= file.block_size {
		if file.current_write_index >= file.block_count {
			return errors.New("Exceeded file size limit")
		}

		blockData := file.buf[:file.block_size]
		file.buf = file.buf[file.block_size:]

		content, err := EncryptFileContents(blockData, AES256_ZIP, file.key)

		if err != nil {
			return err
		}

		// Save metadata
		_, err = file.f.Seek(16+file.current_write_index*16, 0)

		if err != nil {
			return err
		}

		b := make([]byte, 8)

		// Write start pointer
		binary.BigEndian.PutUint64(b, uint64(file.current_write_pt))
		_, err = file.f.Write(b)
		if err != nil {
			return err
		}

		// Write length
		binary.BigEndian.PutUint64(b, uint64(len(content)))
		_, err = file.f.Write(b)
		if err != nil {
			return err
		}

		// Write data

		_, err = file.f.Seek(file.current_write_pt, 0)

		if err != nil {
			return err
		}

		_, err = file.f.Write(content)
		if err != nil {
			return err
		}

		file.current_write_index++
		file.current_write_pt += int64(len(content))
	}

	return nil
}

// Closes the file, writing any pending data in the buffer
func (file *FileBlockEncryptWriteStream) Close() error {
	if len(file.buf) > 0 {
		if file.current_write_index >= file.block_count {
			return errors.New("Exceeded file size limit")
		}

		content, err := EncryptFileContents(file.buf, AES256_ZIP, file.key)

		if err != nil {
			return err
		}

		// Save metadata
		_, err = file.f.Seek(16+file.current_write_index*16, 0)

		if err != nil {
			return err
		}

		b := make([]byte, 8)

		// Write start pointer
		binary.BigEndian.PutUint64(b, uint64(file.current_write_pt))
		_, err = file.f.Write(b)
		if err != nil {
			return err
		}

		// Write length
		binary.BigEndian.PutUint64(b, uint64(len(content)))
		_, err = file.f.Write(b)
		if err != nil {
			return err
		}

		// Write data

		_, err = file.f.Seek(file.current_write_pt, 0)

		if err != nil {
			return err
		}

		_, err = file.f.Write(content)
		if err != nil {
			return err
		}

		file.current_write_index++
		file.current_write_pt += int64(len(content))
	}

	file.f.Close()

	return nil
}

//////////////////////////
//     READ STREAM     //
/////////////////////////

// Status of a read stream
type FileBlockEncryptReadStream struct {
	f *os.File // File descriptor

	file_size   int64 // Original (unencrypted) file size in bytes
	block_size  int64 // Block size in bytes
	block_count int64 // Total number of blocks

	key []byte // Decryption key

	cur_pos int64 // Current position of the read cursor

	cur_block      int64  // Current block the cursor is reading
	cur_block_data []byte // Buffer to store the current block (Decrypted)
}

// Creates a read stream
// file - Path to the file
// key - Decryption key
// perm - File mode
func CreateFileBlockEncryptReadStream(file string, key []byte, perm fs.FileMode) (*FileBlockEncryptReadStream, error) {
	f, err := os.OpenFile(file, os.O_RDONLY, perm)

	if err != nil {
		return nil, err
	}

	i := FileBlockEncryptReadStream{
		f: f,
	}

	b := make([]byte, 8)

	// Read original file size

	_, err = f.Read(b)

	if err != nil {
		f.Close()
		return nil, err
	}

	i.file_size = int64(binary.BigEndian.Uint64(b))

	// Read block size

	_, err = f.Read(b)

	if err != nil {
		f.Close()
		return nil, err
	}

	i.block_size = int64(binary.BigEndian.Uint64(b))

	i.key = key

	i.block_count = i.file_size / i.block_size

	if i.file_size%i.block_size != 0 {
		i.block_count++
	}

	i.cur_block = -1
	i.cur_pos = 0

	return &i, nil
}

// Returns the file size
func (file *FileBlockEncryptReadStream) FileSize() int64 {
	return file.file_size
}

// Returns the block size
func (file *FileBlockEncryptReadStream) BlockSize() int64 {
	return file.block_size
}

// Returns the block count
func (file *FileBlockEncryptReadStream) BlockCount() int64 {
	return file.block_count
}

// Returns the cursor position
func (file *FileBlockEncryptReadStream) Cursor() int64 {
	return file.cur_pos
}

// Fetches a block and decrypt its contents, making it the current block
// block_num - Block number
func (file *FileBlockEncryptReadStream) fetch_block(block_num int64) error {
	if block_num < 0 || block_num >= file.block_count {
		return errors.New("Block index out of bounds")
	}

	// Read block metadata

	_, err := file.f.Seek(16+block_num*16, 0)

	if err != nil {
		return err
	}

	ptBytes := make([]byte, 8)
	lenBytes := make([]byte, 8)

	_, err = file.f.Read(ptBytes)

	if err != nil {
		return err
	}

	_, err = file.f.Read(lenBytes)

	if err != nil {
		return err
	}

	pt := int64(binary.BigEndian.Uint64(ptBytes))

	_, err = file.f.Seek(pt, 0)

	if err != nil {
		return err
	}

	l := int64(binary.BigEndian.Uint64(lenBytes))

	// Read encrypted data

	data := make([]byte, l)

	_, err = file.f.Read(data)

	if err != nil {
		return err
	}

	// Decrypt block data

	data, err = DecryptFileContents(data, file.key)

	if err != nil {
		return err
	}

	// Assign current block
	file.cur_block = block_num
	file.cur_block_data = data

	return nil
}

// Reads from stream, returns the amount of bytes obtained
// Normally reads until the buffer is full, unless the file ends
// buf - Buffer to fill
// Returns the number of bytes read
func (file *FileBlockEncryptReadStream) Read(buf []byte) (int, error) {
	if file.cur_pos >= file.file_size {
		return 0, io.EOF
	}

	filedLength := 0

	for filedLength < len(buf) && file.cur_pos < file.file_size {
		blockIndex := file.cur_pos / file.block_size
		blockOffset := int(file.cur_pos % file.block_size)

		if blockIndex != file.cur_block {
			err := file.fetch_block(blockIndex)

			if err != nil {
				return 0, err
			}
		}

		blockLen := len(file.cur_block_data)
		bytesToCopy := blockLen - blockOffset
		bytesCanFit := len(buf) - filedLength

		if bytesToCopy > bytesCanFit {
			bytesToCopy = bytesCanFit
		}

		// Copy data into the buffer
		copy(buf[filedLength:filedLength+bytesToCopy], file.cur_block_data[blockOffset:blockOffset+bytesToCopy])

		filedLength += bytesToCopy

		// Seek
		file.cur_pos += int64(bytesToCopy)
	}

	return filedLength, nil
}

// Moves the cursor
// pos - Position for the cursor to move
// whence - Position interpretation method. Can be 0 = absolute, 1 = Relative to current position, 2 = Relative to file end
// Returns the new cursor position (absolute)
func (file *FileBlockEncryptReadStream) Seek(pos int64, whence int) (int64, error) {
	switch whence {
	case 1:
		pos = file.cur_pos + pos
	case 2:
		pos = file.file_size - pos
	}

	if pos < 0 || pos > file.file_size {
		return file.cur_pos, errors.New("Cursor position out of bounds")
	}

	file.cur_pos = pos

	return file.cur_pos, nil
}

// Closes the read stream
func (file *FileBlockEncryptReadStream) Close() {
	file.f.Close()
}

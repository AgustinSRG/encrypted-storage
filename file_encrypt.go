// File encryption and decryption

package encrypted_storage

import (
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
)

type FileEncryptionMethod uint16

const (
	AES256_ZIP  FileEncryptionMethod = 1 // Compress data, then encrypt it with AES-256-CBC
	AES256_FLAT FileEncryptionMethod = 2 // Just encrypt the data with AES-256-CBC
)

// Encrypts file contents
// data - File data
// method - algorithm to use
// key - Encryption key
// Returns the cipher text, or an error
func EncryptFileContents(data []byte, method FileEncryptionMethod, key []byte) (ret_cipher_text []byte, ret_error error) {
	if len(data) == 0 {
		return make([]byte, 0), nil
	}

	result := make([]byte, 2)

	binary.BigEndian.PutUint16(result, uint16(method)) // Include method

	if method == AES256_ZIP {
		// Compress the data
		var b bytes.Buffer
		w := zlib.NewWriter(&b)
		_, err := w.Write(data)

		if err != nil {
			return nil, err
		}

		err = w.Close()

		if err != nil {
			return nil, err
		}

		final_data := b.Bytes()

		// Include pre-encryption size to the header
		header := make([]byte, 20)
		binary.BigEndian.PutUint32(header[:4], uint32(len(final_data)))

		// Pad data
		final_data = add_padding(final_data, 16)

		// Generate IV
		iv := make([]byte, 16)
		_, err = rand.Read(iv)

		if err != nil {
			return nil, err
		}

		// Include IV into the header
		copy(header[4:20], iv)

		// Encrypt
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		cipher_text := make([]byte, len(final_data))
		mode := cipher.NewCBCEncrypter(block, iv)
		mode.CryptBlocks(cipher_text, final_data)

		// Include in result

		result = append(result, header...)
		result = append(result, cipher_text...)
	} else if method == AES256_FLAT {
		// Include pre-encryption size to the header
		header := make([]byte, 20)
		binary.BigEndian.PutUint32(header[:4], uint32(len(data)))

		// Pad data
		final_data := add_padding(data, 16)

		// Generate IV
		iv := make([]byte, 16)
		_, err := rand.Read(iv)

		if err != nil {
			return nil, err
		}

		// Include IV into the header
		copy(header[4:20], iv)

		// Encrypt
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		cipher_text := make([]byte, len(final_data))
		mode := cipher.NewCBCEncrypter(block, iv)
		mode.CryptBlocks(cipher_text, final_data)

		// Include in result

		result = append(result, header...)
		result = append(result, cipher_text...)

	} else {
		return nil, errors.New("Invalid method")
	}

	return result, nil
}

// Decrypts file contents
// data - Cipher text
// key - Decryption key
// Returns the original file data, or an error
func DecryptFileContents(data []byte, key []byte) (ret_pain_text []byte, ret_error error) {
	if len(data) < 2 {
		if len(data) == 0 {
			return make([]byte, 0), nil
		} else {
			return nil, errors.New("Invalid data provided")
		}
	}

	method := FileEncryptionMethod(binary.BigEndian.Uint16(data[:2]))

	if method == AES256_ZIP {
		if len(data) < 23 {
			return nil, errors.New("Invalid data provided")
		}

		// Read params
		pre_encoded_data_length := int(binary.BigEndian.Uint32(data[2:6]))
		iv := data[6:22]
		cipher_text := data[22:]

		if pre_encoded_data_length < 0 || pre_encoded_data_length > len(cipher_text) {
			return nil, errors.New("Invalid method")
		}

		// Decrypt
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		mode := cipher.NewCBCDecrypter(block, iv)
		plaintext := make([]byte, len(cipher_text))
		mode.CryptBlocks(plaintext, cipher_text)

		// Remove padding
		plaintext = plaintext[:pre_encoded_data_length]

		// Decompress the data
		b_source := bytes.NewReader(plaintext)
		r, err := zlib.NewReader(b_source)
		if err != nil {
			return nil, err
		}
		result, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		r.Close()

		return result, nil
	} else if method == AES256_FLAT {
		if len(data) < 23 {
			return nil, errors.New("Invalid data provided")
		}

		// Read params
		pre_encoded_data_length := int(binary.BigEndian.Uint32(data[2:6]))
		iv := data[6:22]
		cipher_text := data[22:]

		if pre_encoded_data_length < 0 || pre_encoded_data_length > len(cipher_text) {
			return nil, errors.New("Invalid method")
		}

		// Decrypt
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		mode := cipher.NewCBCDecrypter(block, iv)
		plaintext := make([]byte, len(cipher_text))
		mode.CryptBlocks(plaintext, cipher_text)

		// Remove padding
		plaintext = plaintext[:pre_encoded_data_length]

		return plaintext, nil
	} else {
		return nil, errors.New("Invalid method")
	}
}

// Add padding to the data, so a block cipher can encrypt it
// cipher_text - data to pad
// blockSize - Size of the blocks to encrypt
// Returns the padded data
func add_padding(cipher_text []byte, blockSize int) []byte {
	padding := (blockSize - len(cipher_text)%blockSize)
	pad_text := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(cipher_text, pad_text...)
}

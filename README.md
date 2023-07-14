# Encrypted storage tools

This library implement a collection of tools to create an encrypted storage:

 - Functions to encrypt and decrypt, using `AES-256`, with the option to compress the data using `ZLIB`.
 - Read and write streams to create anb read encrypted files in chunks.
 - Read and write streams to pack multiple small encrypted files into a single container file.

[Documentation](https://pkg.go.dev/github.com/AgustinSRG/encrypted-storage)

## Installation

In order to add it to your project, use 

```sh
go get github.com/AgustinSRG/encrypted-storage
```

## File encryption

You can encrypt a buffer of data using `EncryptFileContents`, with a key of 64 bytes.

You can then decrypt it using `DecryptFileContents` and the same key.

[Example](./file_encrypt_test.go)

### Details

The encrypted data returned by `EncryptFileContents` and accepted by `DecryptFileContents` is binary-encoded, with the following structure:

| Starting byte | Size (bytes) | Value name   | Description                                                                                                    |
| ------------- | ------------ | ------------ | -------------------------------------------------------------------------------------------------------------- |
| `0`           | `2`          | Algorithm ID | Identifier of the algorithm, stored as a **Big Endian unsigned integer**                                       |
| `2`           | `H`          | Header       | Header containing any parameters required by the encryption algorithm. The size depends on the algorithm used. |
| `2 + H`       | `N`          | Body         | Body containing the raw encrypted data.  The size depends on the initial unencrypted data and algorithm used.  |

The system is flexible enough to allow multiple encryption algorithms. Currently, there are 2 supported ones:

 - `AES256_FLAT`: ID = `1`, Uses ZLIB ([RFC 1950](https://datatracker.ietf.org/doc/html/rfc1950)) to compress the data, and then uses AES with a key of 256 bits to encrypt the data, CBC as the mode of operation and an IV of 128 bits. This algorithm uses a header containing the following fields:

| Starting byte | Size (bytes) | Value name                | Description                                                        |
| ------------- | ------------ | ------------------------- | ------------------------------------------------------------------ |
| `2 + H`       | `4`          | Compressed plaintext size | Size of the compressed plaintext, in bytes, used to remove padding |
| `2 + H + 4`   | `16`         | IV                        | Initialization vector for AES_256_CBC algorithm                    |

 - `AES256_FLAT`: ID = `2`, Uses AES with a key of 256 bits to encrypt the data, CBC as the mode of operation and an IV of 128 bits. This algorithm uses a header containing the following fields:

| Starting byte | Size (bytes) | Value name     | Description                                             |
| ------------- | ------------ | -------------- | ------------------------------------------------------- |
| `2 + H`       | `4`          | Plaintext size | Size of the plaintext, in bytes, used to remove padding |
| `2 + H + 4`   | `16`         | IV             | Initialization vector for AES_256_CBC algorithm         |


## Block-Encrypted Files

Block encrypted files are used to encrypt an arbitrarily large file, splitting it's contents in blocks (or chunks) with a set max size. Each block is then encrypted using the file encryption method detailed above.

For creating / writing files:

- You can create a file using `CreateFileBlockEncryptWriteStream`, a function that returns a new instance of `FileBlockEncryptWriteStream`. 
- After it's creation, you must call `FileBlockEncryptWriteStream.Initialize` to set the file size, the block size and the encryption key. 
- Once it is initialized, you may call `FileBlockEncryptWriteStream.Write` to write data into the file. When the data reached a block limit, that block is encrypted and stored into the file.
- After you wrote all the data, you must call `FileBlockEncryptWriteStream.Close` to close the file.

For reading files:

 - You can open a file calling `CreateFileBlockEncryptReadStream`, a function that returns an instance of `FileBlockEncryptReadStream`
 - After it's opened, you may call `FileBlockEncryptReadStream.FileSize`, `FileBlockEncryptReadStream.BlockSize` or `FileBlockEncryptReadStream.BlockCount` to retrieve the parameters of the file.
 - You may call `FileBlockEncryptReadStream.Read` to decrypt and read the data.
 - You can call `FileBlockEncryptReadStream.Seek` to change the cursor position. You may also call `FileBlockEncryptReadStream.Cursor` to retrieve the cursor position if needed.
 - After you are done, you must call `FileBlockEncryptReadStream.Close` to close the file.

[Example](./file_block_encrypt_test.go)

### Details

They are binary files consisting of 3 contiguous sections: The header, the chunk index and the encrypted chunks.

The header contains the following fields:

| Starting byte | Size (bytes) | Value name       | Description                                                                      |
| ------------- | ------------ | ---------------- | -------------------------------------------------------------------------------- |
| `0`           | `8`          | File size        | Size of the original file, in bytes, stored as a **Big Endian unsigned integer** |
| `8`           | `8`          | Chunk size limit | Max size of a chunk, in bytes, stored as a **Big Endian unsigned integer**       |

After the header, the chunk index is stored. **For each chunk** the file was split into, the chunk index will store a metadata entry, withe the following fields:

| Starting byte | Size (bytes) | Value name    | Description                                                              |
| ------------- | ------------ | ------------- | ------------------------------------------------------------------------ |
| `0`           | `8`          | Chunk pointer | Starting byte of the chunk, stored as a  **Big Endian unsigned integer** |
| `8`           | `8`          | Chunk size    | Size of the chunk, in bytes, stored as a **Big Endian unsigned integer** |

After the chunk index, the encrypted chunks are stored following the same structure described above.

This chunked structure allows to randomly access any point in the file as a low cost, since you don't need to decrypt the entire file, only the corresponding chunks.

## Multi-File Pack

Multi-file pack container files are used to store multiple small files inside a single container.

For creating / writing files:

 - You can create a file by calling `CreateMultiFilePackWriteStream`, a function that returns an instance of `MultiFilePackWriteStream`
 - You must call `MultiFilePackWriteStream.Initialize`, setting the number of files you want to store
 - You may call `MultiFilePackWriteStream.PutFile` for each file you want to store, in order.
 - After all files are written, you must call `MultiFilePackWriteStream.Close` to close the file.

For reading files:

 - You can open a file by calling `CreateMultiFilePackReadStream`, a function that returns an instance of `MultiFilePackReadStream`
 - You may call `MultiFilePackReadStream.FileCount` to retrieve the number of stored files.
 - You may call `MultiFilePackReadStream.GetFile` to read a file, by its index.
 - After you are done, you must call `MultiFilePackReadStream.Close` to close the file.

### Details

They are binary files consisting of 3 contiguous sections: The header, the file table and the encrypted files.

The header contains the following fields:

| Starting byte | Size (bytes) | Value name | Description                                                                      |
| ------------- | ------------ | ---------- | -------------------------------------------------------------------------------- |
| `0`           | `8`          | File count | Number of files stored by the asset, stored as a **Big Endian unsigned integer** |

After the header, a file table is stored. **For each file** stored by the asset, a metadata entry is stored, with the following fields:

| Starting byte | Size (bytes) | Value name        | Description                                                                            |
| ------------- | ------------ | ----------------- | -------------------------------------------------------------------------------------- |
| `0`           | `8`          | File data pointer | Starting byte of the file encrypted data, stored as a  **Big Endian unsigned integer** |
| `8`           | `8`          | File size         | Size of the encrypted file, in bytes, stored as a **Big Endian unsigned integer**      |

After the file table, each file is stored following the same structure described above.

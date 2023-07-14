// File encryption and decryption (Test)

package encrypted_storage

import (
	"crypto/rand"
	"crypto/subtle"
	"testing"
)

func TestFileEncryption(t *testing.T) {
	var original []byte
	var encrypted []byte
	var decrypted []byte
	var err error

	// Generate random key
	key := make([]byte, 32)
	_, err = rand.Read(key)

	if err != nil {
		panic(err)
	}

	// Test with string
	original = []byte("Test string")
	encrypted, err = EncryptFileContents(original, AES256_ZIP, key)
	if err != nil {
		t.Error(err)
	}
	decrypted, err = DecryptFileContents(encrypted, key)
	if err != nil {
		t.Error(err)
	}

	if subtle.ConstantTimeCompare(decrypted, original) != 1 {
		t.Errorf("Test failed for size = %d bytes | Original: %s | Final: %s", len(original), string(original), string(decrypted))
	}

	// Test with medium size data slice
	original = make([]byte, 1024)
	_, err = rand.Read(original)
	if err != nil {
		panic(err)
	}
	encrypted, err = EncryptFileContents(original, AES256_ZIP, key)
	if err != nil {
		t.Error(err)
	}
	decrypted, err = DecryptFileContents(encrypted, key)
	if err != nil {
		t.Error(err)
	}

	if subtle.ConstantTimeCompare(decrypted, original) != 1 {
		t.Errorf("Test failed for size = %d bytes", len(original))
	}

	// Test with big size data slice
	original = make([]byte, 1024*1024)
	_, err = rand.Read(original)
	if err != nil {
		panic(err)
	}
	encrypted, err = EncryptFileContents(original, AES256_ZIP, key)
	if err != nil {
		t.Error(err)
	}
	decrypted, err = DecryptFileContents(encrypted, key)
	if err != nil {
		t.Error(err)
	}

	if subtle.ConstantTimeCompare(decrypted, original) != 1 {
		t.Errorf("Test failed for size = %d bytes", len(original))
	}
}

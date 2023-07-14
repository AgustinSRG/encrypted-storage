// Tests for block-encrypted files

package encrypted_storage

import (
	"crypto/rand"
	"os"
	"path"
	"testing"
)

const TEST_BLOCK_SIZE = 5 * 1024 * 1024

func TestFileBlockEncrypt(t *testing.T) {
	test_path_base := "./temp"

	err := os.MkdirAll(test_path_base, 0700)

	if err != nil {
		t.Error(err)
		return
	}

	test_file := path.Join(test_path_base, "test_block_file_enc")
	size := int64(48 * 1024 * 1024)
	key := make([]byte, 32)
	_, err = rand.Read(key)

	if err != nil {
		panic(err)
	}

	// Write file
	ws, err := CreateFileBlockEncryptWriteStream(test_file, 0600)

	if err != nil {
		t.Error(err)
		return
	}

	err = ws.Initialize(size, TEST_BLOCK_SIZE, key)

	if err != nil {
		t.Error(err)
		return
	}

	buf := make([]byte, 269*1024)

	for j := 0; j < len(buf); j++ {
		buf[j] = 'A'
	}

	for i := 0; i < 48; i++ {
		err = ws.Write(buf)
		if err != nil {
			t.Error(err)
			return
		}
	}

	err = ws.Close()
	if err != nil {
		t.Error(err)
		return
	}

	// Read file

	rs, err := CreateFileBlockEncryptReadStream(test_file, key, 0600)

	if err != nil {
		t.Error(err)
		return
	}

	if rs.file_size != size {
		t.Errorf("Expected file_size = (%d), but got (%d)", size, rs.file_size)
	}

	if rs.block_size != TEST_BLOCK_SIZE {
		t.Errorf("Expected block_size = (%d), but got (%d)", TEST_BLOCK_SIZE, rs.block_size)
	}

	if rs.block_count != ws.block_count {
		t.Errorf("Expected block_count = (%d), but got (%d)", ws.block_count, rs.block_count)
	}

	buf2 := make([]byte, 269*1024)

	for i := 0; i < 48; i++ {
		n, err := rs.Read(buf2)

		if err != nil {
			t.Error(err)
			return
		}

		if n != len(buf) {
			t.Errorf("Expected n = (%d), but got (%d)", len(buf), n)
		}

		for j := 0; j < len(buf2); j++ {
			if buf2[j] != 'A' {
				t.Errorf("Expected buf[%d] = (A), but got (%c)", j, buf2[j])
			}
		}
	}

	rs.Close()

	// Remove temp file

	os.Remove(test_file)
}

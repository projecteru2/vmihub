package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetImageDigest(t *testing.T) {
	// 创建一个测试文件
	fileContent := []byte("test file content")
	tmpfile, err := ioutil.TempFile("", "test*.txt")
	assert.Nil(t, err)

	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write(fileContent)
	assert.Nil(t, err)

	err = tmpfile.Close()
	assert.Nil(t, err)

	// 计算文件的SHA-256哈希值
	expectedDigest := sha256.Sum256(fileContent)
	file, err := os.Open(tmpfile.Name())
	assert.Nil(t, err)
	defer file.Close()

	digest, err := GetImageDigest(file)
	assert.Nil(t, err)

	// Compare the calculated hash value with the expected hash value
	if digest != fmt.Sprintf("%x", expectedDigest) {
		t.Errorf("Expected digest %x, but got %s", expectedDigest, digest)
	}
}

func TestDigest(t *testing.T) {
	ss := "hello world"
	h := sha256.New()
	_, err := h.Write([]byte(ss))
	assert.Nil(t, err)
	res1 := hex.EncodeToString(h.Sum(nil))

	res2, err := CalcDigestOfStr(ss)
	assert.Nil(t, err)
	assert.Equal(t, res1, res2)
}

package utils

import (
	"crypto/sha256"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"strings"

	"github.com/projecteru2/vmihub/client/terrors"
)

func Cached(digest, filePath string) (ans bool, err error) {
	parts := strings.Split(digest, ":")
	if len(parts) == 2 {
		digest = parts[1]
	}
	localHash, err := CalcDigestOfFile(filePath)
	if err != nil {
		return
	}
	return digest == localHash, nil
}

func CalcDigestOfFile(fname string) (string, error) {
	f, err := os.Open(fname)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func CalcDigestOfStr(ss string) (string, error) {
	h := sha256.New()
	_, err := h.Write([]byte(ss))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// GetImageDigest Get file digest
func GetImageDigest(file multipart.File) (string, error) {
	// 创建 SHA-256 哈希对象
	h := sha256.New()

	// 将文件内容写入哈希对象中
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}

	// 计算哈希值
	sum := h.Sum(nil)

	// 将哈希值转换为十六进制字符串
	digest := fmt.Sprintf("%x", sum)
	return digest, nil
}

// PartRight partitions the str by the sep.
func PartRight(str, sep string) (string, string) {
	switch i := strings.LastIndex(str, sep); {
	case i < 0:
		return "", str
	case i >= len(str)-1:
		return str[:i], ""
	default:
		return str[:i], str[i+1:]
	}
}

func ParseImageName(imgName string) (user, name, tag string, err error) {
	var nameTag string
	user, nameTag = PartRight(imgName, "/")
	idx := strings.Index(nameTag, ":")
	if idx < 0 {
		name = nameTag
	} else {
		name, tag = nameTag[:idx], nameTag[idx+1:]
	}
	if tag == "" {
		tag = "latest"
	}
	if user == "" {
		user = "_"
	}
	if strings.Contains(tag, ":") {
		err = fmt.Errorf("%w invalid tag %s", terrors.ErrInvalidImageName, tag)
	}
	return
}

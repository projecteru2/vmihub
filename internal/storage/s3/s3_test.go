package s3

import (
	"bytes"
	"context"
	"io"
	"testing"

	stotypes "github.com/projecteru2/vmihub/internal/storage/types"
	pkgutils "github.com/projecteru2/vmihub/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestPut(t *testing.T) {
	stor, err := New("", "xxxx", "yyyyyy", "eru", "images", t)
	assert.Nil(t, err)

	name := "test-put1"
	content := []byte("hello world ")
	digest, err := pkgutils.CalcDigestOfStr(string(content))
	assert.Nil(t, err)
	err = stor.Put(context.Background(), name, digest, bytes.NewReader([]byte(content)))
	assert.Nil(t, err)

	reader, err := stor.Get(context.Background(), name)
	assert.Nil(t, err)
	newVal, err := io.ReadAll(reader)
	assert.Nil(t, err)
	assert.Equal(t, string(content), string(newVal))
}

func TestPutWithChunk(t *testing.T) {
	stor, err := New("", "xxxx", "yyyyyy", "eru", "images", t)
	assert.Nil(t, err)

	name := "test-put-with-chunk1"
	content := bytes.Repeat([]byte("hello world "), 2048)
	size := len(content)
	chunkSize := 5 * 1024 * 1024
	digest, err := pkgutils.CalcDigestOfStr(string(content))
	assert.Nil(t, err)
	err = stor.PutWithChunk(context.Background(), name, digest, size, chunkSize, bytes.NewReader([]byte(content)))
	assert.Nil(t, err)

	reader, err := stor.Get(context.Background(), name)
	assert.Nil(t, err)
	newVal, err := io.ReadAll(reader)
	assert.Nil(t, err)
	assert.Equal(t, string(content), string(newVal))
}

func TestChunkUpload(t *testing.T) {
	stor, err := New("", "xxxx", "yyyyyy", "eru", "images", t)
	assert.Nil(t, err)

	chunkSize := 5 * 1024 * 1024
	totalSize := 13 * 1024 * 1024
	nChunks := 3

	ctx := context.Background()
	name := "test-chunk-upload1"
	content := bytes.Repeat([]byte{'c'}, totalSize)
	tID, err := stor.CreateChunkWrite(context.Background(), name)
	assert.Nil(t, err)
	assert.NotEmpty(t, tID)

	ciInfoList := make([]*stotypes.ChunkInfo, 0, nChunks)
	for idx := 0; idx < nChunks; idx++ {
		start := idx * chunkSize
		end := (idx + 1) * chunkSize
		if end > totalSize {
			end = totalSize
		}
		reader := bytes.NewReader(content[start:end])
		ciInfo := &stotypes.ChunkInfo{
			Idx:       int(idx),
			Size:      int64(end - start),
			ChunkSize: int64(chunkSize),
			Digest:    "",
			In:        reader,
		}
		ciInfoList = append(ciInfoList, ciInfo)
		err := stor.ChunkWrite(context.Background(), name, tID, ciInfo)
		assert.Nil(t, err)
	}
	err = stor.CompleteChunkWrite(ctx, name, tID, ciInfoList)
	assert.Nil(t, err)

	reader, err := stor.Get(context.Background(), name)
	assert.Nil(t, err)
	newVal, err := io.ReadAll(reader)
	assert.Nil(t, err)
	assert.Equal(t, string(content), string(newVal))
}

func TestChunkDownload(t *testing.T) {
	stor, err := New("", "xxxx", "yyyyyy", "eru", "images", t)
	assert.Nil(t, err)

	chunkSize := 5 * 1024 * 1024
	totalSize := 13 * 1024 * 1024
	nChunks := 3

	name := "test-chunk-download1"
	content := bytes.Repeat([]byte{'c'}, totalSize)
	digest, err := pkgutils.CalcDigestOfStr(string(content))
	assert.Nil(t, err)
	err = stor.Put(context.Background(), name, digest, bytes.NewReader(content))
	assert.Nil(t, err)

	reader, err := stor.Get(context.Background(), name)
	assert.Nil(t, err)
	newVal, err := io.ReadAll(reader)
	assert.Nil(t, err)
	assert.Equal(t, string(content), string(newVal))

	ckNewVal := make([]byte, 0, totalSize)

	for idx := 0; idx < nChunks; idx++ {
		rc, err := stor.SeekRead(context.Background(), name, int64(idx*chunkSize))
		assert.Nil(t, err)
		reader := io.LimitReader(rc, int64(chunkSize))
		buf, err := io.ReadAll(reader)
		assert.True(t, err == nil || err == io.EOF)
		assert.LessOrEqual(t, len(buf), int(chunkSize))
		assert.Equal(t, content[0], buf[0])
		ckNewVal = append(ckNewVal, buf...)
	}
	assert.Equal(t, string(content), string(ckNewVal))
}

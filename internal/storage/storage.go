package storage

import (
	"context"
	"io"

	stotypes "github.com/projecteru2/vmihub/internal/storage/types"
)

type Storage interface { //nolint:interfacebloat
	Get(ctx context.Context, name string) (io.ReadCloser, error)
	Delete(ctx context.Context, name string, ignoreNotExists bool) error
	Put(ctx context.Context, name string, digest string, in io.ReadSeeker) error
	PutWithChunk(ctx context.Context, name string, digest string, size int, chunkSize int, in io.ReaderAt) error
	SeekRead(ctx context.Context, name string, start int64) (io.ReadCloser, error)
	CreateChunkWrite(ctx context.Context, name string) (string, error)
	ChunkWrite(ctx context.Context, name string, transactionID string, info *stotypes.ChunkInfo) error
	CompleteChunkWrite(ctx context.Context, name string, transactionID string, chunkList []*stotypes.ChunkInfo) error
	Move(ctx context.Context, src, dest string) error
	GetSize(ctx context.Context, name string) (int64, error)
	GetDigest(ctx context.Context, name string) (string, error)
}

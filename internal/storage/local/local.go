package local

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	stotypes "github.com/projecteru2/vmihub/internal/storage/types"
	"github.com/projecteru2/vmihub/internal/utils"
	"github.com/projecteru2/vmihub/pkg/terrors"
	pkgutils "github.com/projecteru2/vmihub/pkg/utils"
)

type Store struct {
	BaseDir string
}

func New(d string) *Store {
	return &Store{
		BaseDir: d,
	}
}

func (s *Store) Get(_ context.Context, name string) (io.ReadCloser, error) {
	filename := filepath.Join(s.BaseDir, name)

	// 如果文件不存在，则返回错误响应
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, errors.New("file not found")
	}

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Store) Delete(_ context.Context, name string, ignoreNotExists bool) error { //nolint:nolintlint    //nolint
	fullName := filepath.Join(s.BaseDir, name)
	err := os.Remove(fullName)
	if err != nil {
		if ignoreNotExists && os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func (s *Store) Put(_ context.Context, name string, digest string, in io.ReadSeeker) error { //nolint:nolintlint    //nolint
	fullName := filepath.Join(s.BaseDir, name)
	if err := utils.EnsureDir(filepath.Dir(fullName)); err != nil {
		return errors.Wrapf(err, "failed to create dir")
	}

	if err := utils.Invoke(func() error {
		f, err := os.OpenFile(fullName, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0766)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err = io.Copy(f, in); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	fileDigest, err := pkgutils.CalcDigestOfFile(fullName)
	if err != nil {
		return err
	}
	if fileDigest != digest {
		return terrors.ErrInvalidDigest
	}
	return nil
}

func (s *Store) PutWithChunk(ctx context.Context, name string, digest string, size int, chunkSize int, in io.ReaderAt) error { //nolint
	return nil
}

func (s *Store) SeekRead(_ context.Context, name string, start int64) (io.ReadCloser, error) {
	filename := filepath.Join(s.BaseDir, name)

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	if _, err = f.Seek(start, 0); err != nil {
		return nil, err
	}

	return f, nil
}

func (s *Store) CreateChunkWrite(_ context.Context, _ string) (string, error) {
	return uuid.New().String(), nil
}

func (s *Store) ChunkWrite(_ context.Context, name string, _ string, info *stotypes.ChunkInfo) error {
	in := info.In
	offset := int64(info.Idx) * info.ChunkSize
	filename := filepath.Join(s.BaseDir, name)
	if err := utils.EnsureDir(filepath.Dir(filename)); err != nil {
		return errors.Wrapf(err, "failed to create dir")
	}

	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0766)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Seek(offset, 0); err != nil {
		return errors.Wrapf(err, "failed to seek file")
	}
	_, err = io.Copy(f, in)
	return err
}

func (s *Store) CompleteChunkWrite(_ context.Context, _ string, _ string, _ []*stotypes.ChunkInfo) error {
	return nil
}

func (s *Store) Copy(_ context.Context, src, dest string) error {
	destName := filepath.Join(s.BaseDir, dest)
	srcName := filepath.Join(s.BaseDir, src)
	srcF, err := os.OpenFile(srcName, os.O_RDONLY, 0766)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", srcName)
	}
	destF, err := os.OpenFile(destName, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0766)
	if err := utils.EnsureDir(filepath.Dir(destName)); err != nil {
		return errors.Wrapf(err, "failed to create dir for %s", destName)
	}
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", destName)
	}
	_, err = io.Copy(destF, srcF)
	return err
}

func (s *Store) Move(ctx context.Context, src, dest string) error {
	if err := s.Copy(ctx, src, dest); err != nil {
		return err
	}
	srcName := filepath.Join(s.BaseDir, src)
	return os.Remove(srcName)
}

func (s *Store) GetSize(_ context.Context, name string) (int64, error) {
	filename := filepath.Join(s.BaseDir, name)
	info, err := os.Stat(filename)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (s *Store) GetDigest(_ context.Context, name string) (string, error) {
	filename := filepath.Join(s.BaseDir, name)
	return pkgutils.CalcDigestOfFile(filename)
}

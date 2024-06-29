package factory

import (
	"github.com/cockroachdb/errors"
	"github.com/projecteru2/vmihub/config"
	"github.com/projecteru2/vmihub/internal/storage"
	"github.com/projecteru2/vmihub/internal/storage/local"
	"github.com/projecteru2/vmihub/internal/storage/mocks"
	"github.com/projecteru2/vmihub/internal/storage/s3"
)

var (
	stor storage.Storage
)

func Init(cfg *config.StorageConfig) (storage.Storage, error) {
	var err error
	if stor == nil {
		switch cfg.Type {
		case "local":
			stor = local.New(cfg.Local.BaseDir)
		case "s3":
			stor, err = s3.New(cfg.S3.Endpoint, cfg.S3.AccessKey, cfg.S3.SecretKey, cfg.S3.Bucket, cfg.S3.BaseDir, nil)
		case "mock":
			stor = &mocks.Storage{}
		default:
			err = errors.Newf("unknown storage type %s", cfg.Type)
		}
	}
	return stor, err
}

func Instance() storage.Storage {
	return stor
}

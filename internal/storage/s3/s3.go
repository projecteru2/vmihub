package s3

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/projecteru2/core/log"
	stotypes "github.com/projecteru2/vmihub/internal/storage/types"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	s3svc "github.com/aws/aws-sdk-go/service/s3"
	"github.com/cockroachdb/errors"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
)

type Store struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	BaseDir   string
	s3Client  *s3svc.S3
}

type s3ReaderCloser struct {
	store    *Store
	name     string
	offset   int64
	tempNext int64
}

func (rc *s3ReaderCloser) Read(p []byte) (n int, err error) {
	expectedLen := len(p)
	body, err := rc.store.readRange(context.Background(), rc.name, rc.offset+rc.tempNext, rc.offset+rc.tempNext+int64(expectedLen)-1)
	if err != nil {
		if strings.HasPrefix(err.Error(), "InvalidRange") {
			return 0, io.EOF
		}
		return 0, err
	}
	temp, err := io.ReadAll(body)
	if err != nil {
		return 0, err
	}
	n = len(temp)
	copy(p, temp)
	rc.tempNext += int64(n)
	return n, nil
}

func (rc *s3ReaderCloser) Close() error {
	return nil
}

func New(endpoint string, accessKey string, secretKey string, bucket string, baseDir string, t *testing.T) (*Store, error) {
	var (
		s3Client *s3svc.S3
		err      error
	)
	if t == nil {
		s3Client, err = newS3Client(endpoint, accessKey, secretKey)
	} else {
		s3Client = newMockS3Client(t, accessKey, secretKey, bucket)
	}
	if err != nil {
		return nil, err
	}
	return &Store{
		Endpoint:  endpoint,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Bucket:    bucket,
		BaseDir:   baseDir,
		s3Client:  s3Client,
	}, nil
}

func (s *Store) Get(_ context.Context, name string) (io.ReadCloser, error) {
	resp, err := s.s3Client.GetObject(&s3svc.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(path.Join(s.BaseDir, name)),
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (s *Store) Delete(_ context.Context, name string, ignoreNotExists bool) error {
	if !ignoreNotExists {
		_, err := s.s3Client.HeadObject(&s3svc.HeadObjectInput{
			Bucket: aws.String(s.Bucket),
			Key:    aws.String(path.Join(s.BaseDir, name))})
		if err != nil {
			return err
		}
	}
	_, err := s.s3Client.DeleteObject(&s3svc.DeleteObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(path.Join(s.BaseDir, name)),
	})
	return err
}

func (s *Store) Put(_ context.Context, name string, digest string, in io.ReadSeeker) error {
	_, err := s.s3Client.PutObject(&s3svc.PutObjectInput{
		Body:     in,
		Bucket:   aws.String(s.Bucket),
		Key:      aws.String(path.Join(s.BaseDir, name)),
		Metadata: map[string]*string{"sha256": aws.String(digest)},
	})
	return err
}

func (s *Store) PutWithChunk(ctx context.Context, name string, digest string, size int, chunkSize int, in io.ReaderAt) error { //nolint
	logger := log.WithFunc("s3.PutWithChunk")
	respInit, err := s.s3Client.CreateMultipartUpload(&s3svc.CreateMultipartUploadInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(path.Join(s.BaseDir, name)),
	})
	if err != nil {
		return err
	}
	uploadID := *respInit.UploadId
	nChunks := (size + chunkSize - 1) / chunkSize
	completes := make([]*s3svc.CompletedPart, 0, nChunks)
	logger.Debugf(ctx, "total size: %d, nChunks: %d, chunkSize: %d", size, nChunks, chunkSize)
	for chunkIdx := 0; chunkIdx < nChunks; chunkIdx++ {
		offset := int64(chunkIdx * chunkSize)
		curSize := int64(chunkSize)
		if chunkIdx == (nChunks - 1) {
			curSize = int64(size) - offset
		}
		sReader := io.NewSectionReader(in, offset, curSize)
		logger.Debugf(ctx, "write chunk %d, size %d", chunkIdx, curSize)
		partNum := chunkIdx + 1
		param := &s3svc.UploadPartInput{
			Bucket:        aws.String(s.Bucket),
			Key:           aws.String(path.Join(s.BaseDir, name)),
			PartNumber:    aws.Int64(int64(partNum)), // Required 每次的序号唯一且递增
			UploadId:      aws.String(uploadID),
			Body:          sReader,
			ContentLength: aws.Int64(curSize),
		}
		respChunk, err := s.s3Client.UploadPart(param)
		if err != nil {
			s.s3Client.AbortMultipartUploadRequest(&s3svc.AbortMultipartUploadInput{
				UploadId: aws.String(uploadID),
			})
			return errors.Wrapf(err, "upload part %d", partNum)
		}
		cp := &s3svc.CompletedPart{
			PartNumber: aws.Int64(int64(partNum)),
			ETag:       respChunk.ETag,
		}
		completes = append(completes, cp)
	}
	_, err = s.s3Client.CompleteMultipartUpload(&s3svc.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.Bucket),
		Key:      aws.String(path.Join(s.BaseDir, name)),
		UploadId: aws.String(uploadID),
		MultipartUpload: &s3svc.CompletedMultipartUpload{
			Parts: completes,
		},
	})
	return err
}

func (s *Store) readRange(_ context.Context, name string, start int64, end int64) (io.ReadCloser, error) {
	resp, err := s.s3Client.GetObject(&s3svc.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(path.Join(s.BaseDir, name)),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", start, end)),
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (s *Store) SeekRead(_ context.Context, name string, start int64) (io.ReadCloser, error) {
	return &s3ReaderCloser{
		store:  s,
		name:   name,
		offset: start,
	}, nil
}

func (s *Store) CreateChunkWrite(_ context.Context, name string) (string, error) {
	respInit, err := s.s3Client.CreateMultipartUpload(&s3svc.CreateMultipartUploadInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(path.Join(s.BaseDir, name)),
	})
	if err != nil {
		return "", err
	}
	transactionID := *respInit.UploadId

	return transactionID, nil
}

func (s *Store) ChunkWrite(_ context.Context, name string, transactionID string, info *stotypes.ChunkInfo) error {
	partNum := info.Idx + 1

	param := &s3svc.UploadPartInput{
		Bucket:        aws.String(s.Bucket),
		Key:           aws.String(path.Join(s.BaseDir, name)),
		PartNumber:    aws.Int64(int64(partNum)), // Required 每次的序号唯一且递增
		UploadId:      aws.String(transactionID),
		Body:          info.In,
		ContentLength: aws.Int64(info.Size),
	}
	respChunk, err := s.s3Client.UploadPart(param)
	if err != nil {
		s.s3Client.AbortMultipartUploadRequest(&s3svc.AbortMultipartUploadInput{
			UploadId: aws.String(transactionID),
		})
		return err
	}
	var c s3svc.CompletedPart
	c.PartNumber = aws.Int64(int64(partNum))
	c.ETag = respChunk.ETag
	info.Raw = c
	return nil
}

func (s *Store) CompleteChunkWrite(
	_ context.Context,
	name string,
	transactionID string,
	chunkList []*stotypes.ChunkInfo,
) error {
	completes := make([]*s3svc.CompletedPart, 0, len(chunkList))
	for _, chunk := range chunkList {
		v := s3svc.CompletedPart{}
		err := mapstructure.Decode(chunk.Raw, &v)
		if err != nil {
			return err
		}
		completes = append(completes, &v)
	}
	_, err := s.s3Client.CompleteMultipartUpload(&s3svc.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.Bucket),
		Key:      aws.String(path.Join(s.BaseDir, name)),
		UploadId: aws.String(transactionID),
		MultipartUpload: &s3svc.CompletedMultipartUpload{
			Parts: completes,
		},
	})
	return err
}

func (s *Store) Move(ctx context.Context, src, dest string) error {
	head, err := s.s3Client.HeadObject(&s3svc.HeadObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(path.Join(s.BaseDir, src))})
	if err != nil {
		return err
	}

	objectSize := aws.Int64Value(head.ContentLength)
	copyLimit := int64(5*1024*1024*1024 - 1)

	if objectSize < copyLimit { //nolint
		_, err = s.s3Client.CopyObject(&s3svc.CopyObjectInput{
			Bucket:     aws.String(s.Bucket),
			CopySource: aws.String(path.Join(s.Bucket, s.BaseDir, src)),
			Key:        aws.String(path.Join(s.BaseDir, dest))})
		if err != nil {
			return err
		}
	} else {
		respInit, err := s.s3Client.CreateMultipartUpload(&s3svc.CreateMultipartUploadInput{
			Bucket: aws.String(s.Bucket),
			Key:    aws.String(path.Join(s.BaseDir, dest)),
		})
		if err != nil {
			return err
		}
		partLimit := int64(100*1024*1024 - 1)
		partCount := objectSize / partLimit
		if objectSize > partCount*partLimit {
			partCount++
		}

		completes := make([]*s3svc.CompletedPart, 0)
		for i := int64(0); i < partCount; i++ {
			partNumber := aws.Int64(i + 1)
			startRange := i * partLimit
			stopRange := (i+1)*partLimit - 1
			if i == partCount-1 {
				stopRange = objectSize - 1
			}
			respChunk, err := s.s3Client.UploadPartCopy(&s3svc.UploadPartCopyInput{
				Bucket:          aws.String(s.Bucket),
				CopySource:      aws.String(path.Join(s.Bucket, s.BaseDir, src)),
				CopySourceRange: aws.String(fmt.Sprintf("bytes=%d-%d", startRange, stopRange)),
				Key:             aws.String(path.Join(s.BaseDir, dest)),
				PartNumber:      partNumber,
				UploadId:        respInit.UploadId,
			})
			if err != nil {
				s.s3Client.AbortMultipartUploadRequest(&s3svc.AbortMultipartUploadInput{
					UploadId: respInit.UploadId,
				})
				return err
			}

			cPart := s3svc.CompletedPart{
				ETag:       aws.String(strings.Trim(*respChunk.CopyPartResult.ETag, "\"")),
				PartNumber: partNumber,
			}
			completes = append(completes, &cPart)
		}
		_, err = s.s3Client.CompleteMultipartUpload(&s3svc.CompleteMultipartUploadInput{
			Bucket:   aws.String(s.Bucket),
			Key:      aws.String(path.Join(s.BaseDir, dest)),
			UploadId: respInit.UploadId,
			MultipartUpload: &s3svc.CompletedMultipartUpload{
				Parts: completes,
			},
		})
		if err != nil {
			return err
		}
	}

	err = s.Delete(ctx, src, true)
	return err
}

func (s *Store) GetSize(_ context.Context, name string) (int64, error) {
	head, err := s.s3Client.HeadObject(&s3svc.HeadObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(path.Join(s.BaseDir, name))})
	if err != nil {
		return 0, err
	}
	return aws.Int64Value(head.ContentLength), nil
}

func (s *Store) GetDigest(ctx context.Context, name string) (string, error) {
	head, err := s.s3Client.HeadObject(&s3svc.HeadObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(path.Join(s.BaseDir, name))})
	if err != nil {
		return "", err
	}
	hashSha256, ok := head.Metadata["Sha256"]
	if !ok {
		hasher := sha256.New()
		body, err := s.Get(ctx, name)
		if err != nil {
			return "", err
		}
		_, err = io.Copy(hasher, body)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%x", hasher.Sum(nil)), nil
	}
	return *hashSha256, nil
}

func newS3Client(endpoint string, accessKey string, secretKey string) (*s3svc.S3, error) {
	s3ForcePathStyle := true
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region:           aws.String("default"),
			Endpoint:         &endpoint,
			S3ForcePathStyle: &s3ForcePathStyle,
			Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		},
	})

	return s3svc.New(sess), err
}

func newMockS3Client(t *testing.T, accessKey, secretKey, bucket string) *s3svc.S3 {
	backend := s3mem.New()
	faker := gofakes3.New(backend)
	ts := httptest.NewServer(faker.Server())
	// defer ts.Close()

	// configure S3 client
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Endpoint:         aws.String(ts.URL),
		Region:           aws.String("default"),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
	}
	newSession, err := session.NewSession(s3Config)
	assert.Nil(t, err)

	s3Client := s3svc.New(newSession)
	cparams := &s3svc.CreateBucketInput{
		Bucket: aws.String(bucket),
	}
	// Create a new bucket using the CreateBucket call.
	_, err = s3Client.CreateBucket(cparams)
	assert.Nil(t, err)
	return s3Client
}

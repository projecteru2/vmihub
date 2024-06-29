package types

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/projecteru2/vmihub/client/util"
	svctypes "github.com/projecteru2/vmihub/pkg/types"
	svcutils "github.com/projecteru2/vmihub/pkg/utils"
	bolt "go.etcd.io/bbolt"
)

type Metadata struct {
	Digest      string `mapstructure:"digest" json:"digest"`
	Size        int64  `mapstructure:"size" json:"size"`
	ActualSize  int64  `mapstructure:"actual_size" json:"actualSize"`
	VirtualSize int64  `mapstructure:"virtual_size" json:"virtualSize"`
}

type MetadataDB struct {
	baseDir string
	bucket  string
	db      *bolt.DB
}

func NewMetadataDB(baseDir, bucket string) (*MetadataDB, error) {
	db, err := bolt.Open(filepath.Join(baseDir, "metadata.db"), 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open db")
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucket))
		return err
	}); err != nil {
		return nil, errors.Wrapf(err, "failed to create bucket for image")
	}
	return &MetadataDB{
		baseDir: baseDir,
		bucket:  bucket,
		db:      db,
	}, nil
}

func (mdb *MetadataDB) Remove(img *Image) error {
	return mdb.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(mdb.bucket))
		return b.Delete([]byte(img.Fullname()))
	})
}

func (mdb *MetadataDB) NewImage(imgName string) (img *Image, err error) {
	user, name, tag, err := svcutils.ParseImageName(imgName)
	if err != nil {
		return nil, err
	}
	img = &Image{
		ImageInfoResp: svctypes.ImageInfoResp{
			Username: user,
			Name:     name,
			Tag:      tag,
		},
		BaseDir: mdb.baseDir,
		MDB:     mdb,
	}
	md, err := mdb.Load(img)
	if err != nil {
		return
	}
	if md == nil {
		return img, nil
	}
	img.ActualSize, img.VirtualSize = md.ActualSize, md.VirtualSize
	img.Size = md.Size
	img.Digest = md.Digest
	return img, nil
}

func (mdb *MetadataDB) RemoveImage(img *Image) error {
	if err := mdb.Remove(img); err != nil {
		return err
	}
	err := os.RemoveAll(img.Filepath())
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func (mdb *MetadataDB) CopyFile(img *Image, src io.Reader) (err error) {
	if err := util.EnsureDir(filepath.Dir(img.Filepath())); err != nil {
		return err
	}
	destF, err := os.OpenFile(img.Filepath(), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0766)
	if err != nil {
		return err
	}
	if _, err = io.Copy(destF, src); err != nil {
		return err
	}
	destF.Close()

	md, err := mdb.update(img, false)
	if err != nil {
		return err
	}
	img.ActualSize, img.VirtualSize = md.ActualSize, md.VirtualSize
	img.Digest = md.Digest
	return err
}

// before calling this method,you should ensure the local image file exists.
func (mdb *MetadataDB) Load(img *Image) (meta *Metadata, err error) {
	fullname := img.Fullname()
	localfile := img.Filepath()
	fi, err := os.Stat(localfile)
	if err != nil {
		return nil, nil //nolint
	}
	meta = &Metadata{
		Size: fi.Size(),
	}
	var exists bool
	err = mdb.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(mdb.bucket))
		v := b.Get([]byte(fullname))
		if v != nil {
			exists = true
			return json.Unmarshal(v, meta)
		}
		return nil
	})
	if exists {
		return meta, err
	}
	return mdb.update(img, true)
}

func (mdb *MetadataDB) update(img *Image, oldEmpty bool) (meta *Metadata, err error) {
	fullname := img.Fullname()
	localfile := img.Filepath()
	fi, err := os.Stat(localfile)
	if err != nil {
		return nil, nil //nolint
	}
	meta = &Metadata{
		Size: fi.Size(),
	}
	if meta.Digest, err = svcutils.CalcDigestOfFile(localfile); err != nil {
		return nil, err
	}
	if meta.ActualSize, meta.VirtualSize, err = util.ImageSize(localfile); err != nil {
		return nil, err
	}

	bs, err := json.Marshal(*meta)
	if err != nil {
		return nil, err
	}
	err = mdb.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(mdb.bucket))
		if oldEmpty && b.Get([]byte(fullname)) != nil {
			return errors.Newf("canflict when load metadata of image %s", fullname)
		}
		return b.Put([]byte(fullname), bs)
	})
	return
}

type OSInfo = svctypes.OSInfo

type Image struct {
	svctypes.ImageInfoResp

	BaseDir     string      `mapstructure:"-" json:"-"`
	URL         string      `mapstructure:"-" json:"-"`
	VirtualSize int64       `mapstructure:"-" json:"-"`
	ActualSize  int64       `mapstructure:"-" json:"-"`
	MDB         *MetadataDB `mapstructure:"-" json:"-"`
}

func (img *Image) CopyFrom(fname string) error {
	if fname == img.Filepath() {
		return nil
	}
	srcF, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer srcF.Close()

	return img.MDB.CopyFile(img, srcF)
}

// before calling this method,you should ensure the local image file exists.
func (img *Image) LoadLocalMetadata() (meta *Metadata, err error) {
	return img.MDB.Load(img)
}

func (img *Image) Filepath() string {
	user := img.Username
	if user == "" {
		user = "_"
	}
	return filepath.Join(img.BaseDir, "image", fmt.Sprintf("%s/%s:%s.img", user, img.Name, img.Tag))
}

func (img *Image) Cached() (ans bool, err error) {
	meta, err := img.MDB.Load(img)
	if err != nil || meta == nil {
		return false, err
	}
	return img.Digest == meta.Digest, nil
}

type ChunkSlice struct {
	Image

	UploadID  string `mapstructure:"uploadId" json:"uploadId"`
	ChunkSize int64  `mapstructure:"chunkSize" json:"chunkSize"`
}

func (chunk *ChunkSlice) SliceFilePath() string {
	user := chunk.Username
	if user == "" {
		user = "_"
	}
	return filepath.Join(chunk.BaseDir, "image", fmt.Sprintf("%s/__slice_%s:%s.img", user, chunk.Name, chunk.Tag))
}

func (chunk *ChunkSlice) SliceFileIndexPath(idx int) string {
	user := chunk.Username
	if user == "" {
		user = "_"
	}
	return filepath.Join(chunk.BaseDir, "image", fmt.Sprintf("%s/__slice_%s:%s-%d.img", user, chunk.Name, chunk.Tag, idx))
}

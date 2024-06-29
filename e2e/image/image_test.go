package image

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	libimage "github.com/projecteru2/vmihub/client/image"
	libtypes "github.com/projecteru2/vmihub/client/types"
	"github.com/projecteru2/vmihub/client/util"
	e2etypes "github.com/projecteru2/vmihub/e2e/types"
	"github.com/projecteru2/vmihub/pkg/types"
)

var (
	configFile string
)

func init() {
	flag.StringVar(&configFile, "config", "./config.toml", "config file")
}

var _ = BeforeSuite(func() {
})

var _ = AfterSuite(func() {
})

func TestGuest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Image Suite")
}

func newImageAPI(chunkSize int64, threshold int64) (libimage.API, string) {
	cfg, err := e2etypes.LoadConfig(configFile)
	Expect(err).To(BeNil())
	cred := &libtypes.Credential{
		Username: cfg.Username,
		Password: cfg.Password,
	}
	err = util.EnsureDir(cfg.BaseDir)
	Expect(err).To(BeNil())
	baseDir, err := os.MkdirTemp(cfg.BaseDir, "e2e-test")
	Expect(err).To(BeNil())

	var opts []libimage.Option
	if chunkSize > 0 {
		opts = append(opts, libimage.WithChunSize(strconv.FormatInt(chunkSize, 10)))
	}
	if threshold > 0 {
		opts = append(opts, libimage.WithChunkThreshold(strconv.FormatInt(threshold, 10)))
	}
	imageAPI, err := libimage.NewAPI(cfg.URL, baseDir, cred, opts...)
	Expect(err).To(BeNil())
	return imageAPI, baseDir
}

func createImage(ctx context.Context, imageAPI libimage.API, user, name, tag string, contentByte byte, sz int64) *libtypes.Image {
	fullname := fmt.Sprintf("%s/%s:%s", user, name, tag)
	testImg, err := imageAPI.NewImage(fullname)
	Expect(err).To(BeNil())

	testImg.Format = "qcow2"
	testImg.OS = types.OSInfo{
		Type:    "linux",
		Distrib: "ubuntu",
		Version: "20.04",
		Arch:    "amd64",
	}
	fname, err := prepareFile(sz, contentByte)
	Expect(err).To(BeNil())
	defer os.Remove(fname)

	err = testImg.CopyFrom(fname)
	Expect(err).To(BeNil())

	err = imageAPI.Push(ctx, testImg, false)
	Expect(err).To(BeNil())
	return testImg
}

package local

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	pkgutils "github.com/projecteru2/vmihub/pkg/utils"
	"github.com/stretchr/testify/suite"
)

var (
	name = "hahaha"
	val  = "kakakaka"
)

type testSuite struct {
	suite.Suite
	sto Store
}

func TestLocalTestSuite(t *testing.T) {
	suite.Run(t, new(testSuite))
}

func (s *testSuite) SetupSuite() {
	basename := uuid.NewString()[:5]
	baseDir := filepath.Join("/tmp", basename)
	err := os.MkdirAll(baseDir, 0755)
	s.Nil(err)

	s.sto = Store{
		BaseDir: baseDir,
	}
}

func (s *testSuite) TearDownSuite() {
	err := os.RemoveAll(s.sto.BaseDir)
	s.Nil(err)
}

func (s *testSuite) SetupTest() {
	buf := bytes.NewReader([]byte(val))
	digest, err := pkgutils.CalcDigestOfStr(val)
	s.Nil(err)
	err = s.sto.Put(context.Background(), name, digest, buf)
	s.Nil(err)
}

func (s *testSuite) TearDownTest() {
	err := s.sto.Delete(context.Background(), name, true)
	s.Nil(err)
}

func (s *testSuite) TestGet() {
	out, err := s.sto.Get(context.Background(), name)
	s.Nil(err)
	res, err := io.ReadAll(out)
	s.Nil(err)
	s.Equal(val, string(res))
}

func (s *testSuite) TestDelete() {
	err := s.sto.Delete(context.Background(), name, false)
	s.Nil(err)
	err = s.sto.Delete(context.Background(), name, false)
	s.NotNil(err)
	err = s.sto.Delete(context.Background(), name, true)
	s.Nil(err)
}

func (s *testSuite) TestPut() {
	v1 := "gagagaga"

	buf := bytes.NewReader([]byte(v1))
	digest, err := pkgutils.CalcDigestOfStr(v1)
	s.Nil(err)
	err = s.sto.Put(context.Background(), name, digest, buf)
	s.Nil(err)
	out, err := s.sto.Get(context.Background(), name)
	s.Nil(err)
	res, err := io.ReadAll(out)
	s.Nil(err)
	s.Equal(v1, string(res))
}

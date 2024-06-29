package image

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/projecteru2/vmihub/internal/middlewares"
	"github.com/projecteru2/vmihub/internal/models"
	"github.com/projecteru2/vmihub/internal/testutils"
	"github.com/projecteru2/vmihub/internal/utils"
	"github.com/projecteru2/vmihub/pkg/types"
	pkgutils "github.com/projecteru2/vmihub/pkg/utils"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	testContent = "test content"
)

var (
	repoTableName = ((*models.Repository)(nil)).TableName()
	repoColumns   = ((*models.Repository)(nil)).ColumnNames()
	imgTableName  = ((*models.Image)(nil)).TableName()
	imgColumns    = ((*models.Image)(nil)).ColumnNames()
)

type imageTestSuite struct {
	suite.Suite
	r *gin.Engine
}

// func (suite *imageTestSuite) SetupSuite() {
// 	gomonkey.ApplyFuncReturn(task.SendImageTask, nil)
// }

func (suite *imageTestSuite) SetupTest() {
	t := suite.T()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err := testutils.Prepare(ctx, t)
	require.NoError(t, err)

	r, err := testutils.PrepareGinEngine()
	require.NoError(t, err)
	apiGroup := r.Group("/api/v1", middlewares.Authenticate())

	SetupRouter(apiGroup)
	suite.r = r
}

func (suite *imageTestSuite) TestGetRepoList() {
	{
		utils.MockRedis.FlushAll()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/repositories", nil)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusBadRequest, w.Code)
	}
	{
		// anonymous user
		// prepare db data
		utils.MockRedis.FlushAll()
		wantRows := sqlmock.NewRows([]string{"id", "username", "name"}).
			AddRow(1, "user1", "name1").
			AddRow(2, "user1", "name2")
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND private = ? ORDER BY updated_at DESC LIMIT ?, ?", repoColumns, repoTableName)).
			WithArgs("user1", false, 0, 10).
			WillReturnRows(wantRows)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/repositories?username=user1", nil)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusOK, w.Code)
		raw := map[string]any{}
		err := json.Unmarshal(w.Body.Bytes(), &raw)
		suite.Nil(err)
		bs, err := json.Marshal(raw["data"])
		suite.Nil(err)
		repos := []models.Repository{}
		err = json.Unmarshal(bs, &repos)
		suite.Nil(err)
		suite.Len(repos, 2)
		suite.Equal("user1", repos[0].Username)
		suite.Equal("name1", repos[0].Name)
		suite.Equal("user1", repos[1].Username)
		suite.Equal("name2", repos[1].Name)
	}
	{
		// logined user
		utils.MockRedis.FlushAll()
		user, pass := "user1", "pass1"
		err := testutils.PrepareUserData(user, pass)
		suite.Nil(err)
		// prepare db data
		wantRows := sqlmock.NewRows([]string{"id", "username", "name"}).
			AddRow(1, "user1", "name1").
			AddRow(2, "user1", "name2")
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? ORDER BY updated_at DESC LIMIT ?, ?", repoColumns, repoTableName)).
			WithArgs("user1", 0, 10).
			WillReturnRows(wantRows)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/repositories", nil)
		testutils.AddAuth(req, user, pass)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusOK, w.Code)
	}
}

func (suite *imageTestSuite) TestGetImageInfo() {
	{
		//repository doesn't exist
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", repoColumns, repoTableName)).
			WithArgs("user1", "name1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "username", "name", "private"}))
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/image/user1/name1/info", nil)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusNotFound, w.Code)
	}
	{
		// anonymous user can't read private image
		utils.MockRedis.FlushAll()
		wantRows := sqlmock.NewRows([]string{"id", "username", "name", "private"}).
			AddRow(1, "user1", "name1", true)
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", repoColumns, repoTableName)).
			WithArgs("user1", "name1").
			WillReturnRows(wantRows)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/image/user1/name1/info", nil)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusForbidden, w.Code)
	}
	{
		utils.MockRedis.FlushAll()
		user, pass := "user1", "pass1"
		err := testutils.PrepareUserData(user, pass)
		suite.Nil(err)
		wantRows := sqlmock.NewRows([]string{"id", "username", "name", "private"}).
			AddRow(1, "user1", "name1", true)

		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", repoColumns, repoTableName)).
			WithArgs("user1", "name1").
			WillReturnRows(wantRows)

		// image doesn't exist
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE repo_id = ? ORDER BY created_at DESC LIMIT 1", imgColumns, imgTableName)).
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "repo_id", "tag"}))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/image/user1/name1/info", nil)
		testutils.AddAuth(req, user, pass)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusNotFound, w.Code)
	}
	{
		utils.MockRedis.FlushAll()
		user, pass := "user1", "pass1"
		err := testutils.PrepareUserData(user, pass)
		suite.Nil(err)
		wantRows := sqlmock.NewRows([]string{"id", "username", "name", "private"}).
			AddRow(1, "user1", "name1", true)
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", repoColumns, repoTableName)).
			WithArgs("user1", "name1").
			WillReturnRows(wantRows)

		wantRows = sqlmock.NewRows([]string{"id", "repo_id", "tag", "os", "created_at"}).
			AddRow(2, 1, "tag1", []byte("{}"), time.Now())
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE repo_id = ? AND tag = ?", imgColumns, imgTableName)).
			WithArgs(1, "tag1").
			WillReturnRows(wantRows)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/image/user1/name1/info?tag=tag1", nil)
		testutils.AddAuth(req, user, pass)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusOK, w.Code)
		raw := map[string]any{}
		err = json.Unmarshal(w.Body.Bytes(), &raw)
		suite.Nil(err)
		var resp types.ImageInfoResp
		bs, _ := json.Marshal(raw["data"])
		err = json.Unmarshal(bs, &resp)
		suite.Nil(err)
		suite.Equal(resp.Username, "user1")
		suite.Equal(resp.Name, "name1")
	}
}

func (suite *imageTestSuite) TestCache() {
	utils.MockRedis.FlushAll()
	{
		wantRows := sqlmock.NewRows([]string{"id", "username", "name", "private"}).
			AddRow(1, "user1", "name1", false)
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", repoColumns, repoTableName)).
			WithArgs("user1", "name1").
			WillReturnRows(wantRows)
		wantRows = sqlmock.NewRows([]string{"id", "repo_id", "tag", "os"}).
			AddRow(2, 1, "tag1", []byte("{}"))
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE repo_id = ? ORDER BY created_at DESC LIMIT 1", imgColumns, imgTableName)).
			WithArgs(1).
			WillReturnRows(wantRows)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/image/user1/name1/info", nil)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusOK, w.Code)
		raw := map[string]any{}
		err := json.Unmarshal(w.Body.Bytes(), &raw)
		suite.Nil(err)
		var resp types.ImageInfoResp
		bs, _ := json.Marshal(raw["data"])
		err = json.Unmarshal(bs, &resp)
		suite.Nil(err)
		suite.Equal(resp.Username, "user1")
		suite.Equal(resp.Name, "name1")
	}
	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/image/user1/name1/info?tag=tag1", nil)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusOK, w.Code)
		raw := map[string]any{}
		err := json.Unmarshal(w.Body.Bytes(), &raw)
		suite.Nil(err)
		var resp types.ImageInfoResp
		bs, _ := json.Marshal(raw["data"])
		err = json.Unmarshal(bs, &resp)
		suite.Nil(err)
		suite.Equal(resp.Username, "user1")
		suite.Equal(resp.Name, "name1")
		suite.Equal(resp.Tag, "tag1")
	}
}

func (suite *imageTestSuite) TestDownloadImage() {
	{
		utils.MockRedis.FlushAll()
		// anonymous user can't download private image
		wantRows := sqlmock.NewRows([]string{"id", "username", "name", "private"}).
			AddRow(1, "user1", "name1", true)
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", repoColumns, repoTableName)).
			WithArgs("user1", "name1").
			WillReturnRows(wantRows)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/image/user1/name1/download", nil)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusForbidden, w.Code)
		// suite.Equal("ok", w.Body.String())
	}
	{
		utils.MockRedis.FlushAll()
		// a user can't read a private image which belongs other user
		user, pass := "user2", "pass2"
		err := testutils.PrepareUserData(user, pass)
		suite.Nil(err)
		wantRows := sqlmock.NewRows([]string{"id", "username", "name", "private"}).
			AddRow(1, "user1", "name1", true)
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", repoColumns, repoTableName)).
			WithArgs("user1", "name1").
			WillReturnRows(wantRows)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/image/user1/name1/download", nil)
		testutils.AddAuth(req, user, pass)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusForbidden, w.Code)
	}
	{
		utils.MockRedis.FlushAll()
		// normal case
		user, pass := "user1", "pass1"
		err := testutils.PrepareUserData(user, pass)
		suite.Nil(err)
		wantRows := sqlmock.NewRows([]string{"id", "username", "name", "private"}).
			AddRow(1, "user1", "name1", true)

		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", repoColumns, repoTableName)).
			WithArgs("user1", "name1").
			WillReturnRows(wantRows)

		wantRows = sqlmock.NewRows([]string{"id", "repo_id", "tag"}).
			AddRow(2, 1, "tag1")
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE repo_id = ? AND tag = ?", imgColumns, imgTableName)).
			WithArgs(1, "tag1").
			WillReturnRows(wantRows)
		sto := testutils.GetMockStorage()
		defer sto.AssertExpectations(suite.T())

		sto.On("Get", mock.Anything, mock.Anything).Return(io.NopCloser(bytes.NewBufferString(testContent)), nil).Once()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/image/user1/name1/download?tag=tag1", nil)
		testutils.AddAuth(req, user, pass)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusOK, w.Code)
		suite.Equal(testContent, w.Body.String())
	}
}

func (suite *imageTestSuite) TestUploadImage() {
	digest, err := pkgutils.CalcDigestOfStr(testContent)
	suite.Nil(err)

	// gomonkey.ApplyFuncReturn(task.SendImageTask, nil)

	body := types.ImageCreateRequest{
		Username: "user1",
		Name:     "name1",
		Tag:      "tag1",
		Size:     int64(len(testContent)),
		Digest:   digest,
		Format:   "qcow2",
		OS: types.OSInfo{
			Arch:    "arm64",
			Type:    "linux",
			Distrib: "ubuntu",
			Version: "22.04",
		},
	}
	bs, _ := json.Marshal(body)

	{
		utils.MockRedis.FlushAll()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/image/user1/name1/startUpload", nil)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusBadRequest, w.Code)
	}
	{
		utils.MockRedis.FlushAll()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/image/user1/name1/startUpload", bytes.NewReader(bs))
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusUnauthorized, w.Code)
	}
	{
		utils.MockRedis.FlushAll()
		// a user can't write a image which belongs other user
		user, pass := "user2", "pass2"
		err := testutils.PrepareUserData(user, pass)
		suite.Nil(err)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/image/user1/name1/startUpload", bytes.NewReader(bs))
		testutils.AddAuth(req, user, pass)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusForbidden, w.Code)
	}
	{
		utils.MockRedis.FlushAll()
		// normal case
		user, pass := "user1", "pass1"
		err := testutils.PrepareUserData(user, pass)
		suite.Nil(err)
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", repoColumns, repoTableName)).
			WithArgs("user1", "name1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "username", "name", "private"}))

		// models.Mock.ExpectQuery("SELECT * FROM image WHERE repo_id = ? AND tag = ?").
		// 	WithArgs(1, "latest").
		// 	WillReturnRows(sqlmock.NewRows([]string{"id", "repo_id", "tag"}))

		models.Mock.ExpectBegin()
		models.Mock.ExpectExec("INSERT INTO repository(username, name, private) VALUES(?, ?, ?)").
			WithArgs("user1", "name1", false).
			WillReturnResult(sqlmock.NewResult(1234, 1))

		osBytes, _ := json.Marshal(body.OS)
		models.Mock.ExpectExec("INSERT INTO image(repo_id, tag, labels, size, format, os, digest, snapshot, description) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)").
			WithArgs(1234, "tag1", sqlmock.AnyArg(), len(testContent), "qcow2", osBytes, digest, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1234, 1))
		models.Mock.ExpectCommit()

		stor := testutils.GetMockStorage()
		defer stor.AssertExpectations(suite.T())

		var once sync.Once
		stor.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(reader io.ReadSeeker) bool {
			// AssertExpectations will call this function second time
			once.Do(func() {
				bs, err := io.ReadAll(reader)
				suite.Nil(err)
				suite.Equal(testContent, string(bs))
			})
			return true
		})).Return(nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/image/user1/name1/startUpload", bytes.NewReader(bs))
		testutils.AddAuth(req, user, pass)
		// req.Header.Set("Content-Type", writer.FormDataContentType())
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusOK, w.Code)
		raw := map[string]any{}
		err = json.Unmarshal(w.Body.Bytes(), &raw)
		suite.Nil(err)
		var resp map[string]string
		bs, _ := json.Marshal(raw["data"])
		err = json.Unmarshal(bs, &resp)
		suite.Nil(err)
		uploadID := resp["uploadID"]
		suite.True(len(uploadID) > 0)

		w = httptest.NewRecorder()
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "/tmp/haha")
		suite.Nil(err)
		_, err = part.Write([]byte(testContent))
		suite.Nil(err)
		writer.Close()

		req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/image/user1/name1/upload?uploadID=%s", uploadID), body)
		testutils.AddAuth(req, user, pass)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		suite.r.ServeHTTP(w, req)

		suite.Equalf(http.StatusOK, w.Code, "error: %s", w.Body.String())
	}
}

func (suite *imageTestSuite) TestDeleteImage() {
}

func TestImageTestSuite(t *testing.T) {
	suite.Run(t, new(imageTestSuite))
}

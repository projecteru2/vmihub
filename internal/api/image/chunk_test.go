package image

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/projecteru2/vmihub/internal/models"
	"github.com/projecteru2/vmihub/internal/testutils"
	"github.com/projecteru2/vmihub/internal/utils"
	"github.com/projecteru2/vmihub/pkg/types"
	pkgutils "github.com/projecteru2/vmihub/pkg/utils"
	"github.com/stretchr/testify/mock"
)

func (suite *imageTestSuite) TestDownloadImageChunk() {
	{
		utils.MockRedis.FlushAll()
		// anonymous user can't download private image
		wantRows := sqlmock.NewRows([]string{"id", "username", "name", "private"}).
			AddRow(1, "user1", "name1", true)
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", repoColumns, repoTableName)).
			WithArgs("user1", "name1").
			WillReturnRows(wantRows)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/image/user1/name1/chunk/0/download?chunkSize=2", nil)
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
		req, _ := http.NewRequest("GET", "/api/v1/image/user1/name1/chunk/0/download?chunkSize=2", nil)
		testutils.AddAuth(req, user, pass)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusForbidden, w.Code)
	}
	{
		utils.MockRedis.FlushAll()
		chunkIdx := 1
		chunkSize := 2
		// normal case
		user, pass := "user1", "pass1"
		err := testutils.PrepareUserData(user, pass)
		suite.Nil(err)
		wantRows := sqlmock.NewRows([]string{"id", "username", "name", "private"}).
			AddRow(1, "user1", "name1", true)
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", repoColumns, repoTableName)).
			WithArgs("user1", "name1").
			WillReturnRows(wantRows)

		wantRows = sqlmock.NewRows([]string{"id", "repo_id", "tag", "size", "os"}).
			AddRow(2, 1, "tag1", len(testContent), []byte("{}"))
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE repo_id = ? AND tag = ?", imgColumns, imgTableName)).
			WithArgs(1, "tag1").
			WillReturnRows(wantRows)
		sto := testutils.GetMockStorage()
		defer sto.AssertExpectations(suite.T())

		offset := chunkIdx * chunkSize
		sto.On("SeekRead", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(io.NopCloser(bytes.NewBufferString(testContent[offset:])), nil).Once()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/image/user1/name1/chunk/%d/download?chunkSize=%d&tag=tag1", chunkIdx, chunkSize), nil)
		testutils.AddAuth(req, user, pass)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusOK, w.Code)
		suite.Equal(testContent[offset:offset+chunkSize], w.Body.String())
	}
}

func (suite *imageTestSuite) TestStartChunkUpload() {
	digest, err := pkgutils.CalcDigestOfStr(testContent)
	suite.Nil(err)
	url := "/api/v1/image/user1/name1/startChunkUpload?chunkSize=2&nChunks=2"

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
		// invalid arguments
		utils.MockRedis.FlushAll()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", url, nil)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusBadRequest, w.Code)
		// suite.Equal("ok", w.Body.String())
	}
	{
		utils.MockRedis.FlushAll()
		// anonymous user can't upload image
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", url, bytes.NewReader(bs))
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusUnauthorized, w.Code)
		// suite.Equal("ok", w.Body.String())
	}
	{
		utils.MockRedis.FlushAll()
		// a user can't write a image which belongs other user
		user, pass := "user2", "pass2"
		err := testutils.PrepareUserData(user, pass)
		suite.Nil(err)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", url, bytes.NewReader(bs))
		testutils.AddAuth(req, user, pass)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusForbidden, w.Code)
	}
	{
		utils.MockRedis.FlushAll()
		// conflict cases
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

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", url, bytes.NewReader(bs))
		testutils.AddAuth(req, user, pass)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusConflict, w.Code)
	}
	{
		utils.MockRedis.FlushAll()
		// normal cases
		user, pass := "user1", "pass1"
		err := testutils.PrepareUserData(user, pass)
		suite.Nil(err)
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", repoColumns, repoTableName)).
			WithArgs("user1", "name1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "username", "name", "private"}))

		sto := testutils.GetMockStorage()
		sto.On("CreateChunkWrite", mock.Anything, mock.Anything).Return(mock.Anything, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", url, bytes.NewReader(bs))
		testutils.AddAuth(req, user, pass)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusOK, w.Code)
		raw := map[string]any{}
		bs, err := io.ReadAll(w.Body)
		err = json.Unmarshal(bs, &raw)
		suite.Nil(err)
		suite.NotNilf(raw["data"], "++++++ %v", raw)
		data := raw["data"].(map[string]any)
		uploadID := data["uploadID"].(string)
		suite.NotEmpty(uploadID)
	}
}

func (suite *imageTestSuite) TestUploadChunk() {
	// gomonkey.ApplyFuncReturn(task.SendImageTask, nil)

	digest, err := pkgutils.CalcDigestOfStr(testContent)
	suite.Nil(err)

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

	chunkSize := 2
	nChunks := int64(math.Ceil(float64(len(testContent)) / float64(chunkSize)))
	{
		utils.MockRedis.FlushAll()
		// normal cases
		user, pass := "user1", "pass1"
		err := testutils.PrepareUserData(user, pass)
		suite.Nil(err)
		models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", repoColumns, repoTableName)).
			WithArgs("user1", "name1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "username", "name", "private"}))

		sto := testutils.GetMockStorage()
		defer sto.AssertExpectations(suite.T())
		sto.On("CreateChunkWrite", mock.Anything, mock.Anything).Return(mock.Anything, nil)

		w := httptest.NewRecorder()
		url := fmt.Sprintf("/api/v1/image/user1/name1/startChunkUpload?chunkSize=%d&nChunks=%d", chunkSize, nChunks)
		req, _ := http.NewRequest("POST", url, bytes.NewReader(bs))
		testutils.AddAuth(req, user, pass)
		suite.r.ServeHTTP(w, req)

		suite.Equal(http.StatusOK, w.Code)
		raw := map[string]any{}
		bs, err := io.ReadAll(w.Body)
		err = json.Unmarshal(bs, &raw)
		suite.Nil(err)
		data := raw["data"].(map[string]any)
		uploadID := data["uploadID"].(string)

		// upload chunks
		nChunks := math.Ceil(float64(len(testContent)) / float64(chunkSize))
		for cIdx := int(0); cIdx < int(nChunks); cIdx++ {
			start, end := cIdx*chunkSize, (cIdx+1)*chunkSize
			if end > len(testContent) {
				end = len(testContent)
			}

			w = httptest.NewRecorder()
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, err := writer.CreateFormFile("file", "/tmp/haha")
			suite.Nil(err)
			_, err = part.Write([]byte(testContent[start:end]))
			suite.Nil(err)
			writer.Close()

			// copyCIdx := cIdx
			// sto.On("ChunkWrite", mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(chunk *stotypes.ChunkInfo) bool {
			// 	// AssertExpectations will call this function second time
			// 	fmt.Printf("++++++%d => %v\n", copyCIdx, chunk)
			// 	if chunk.Idx == 0 {
			// 		return true
			// 	}
			// 	suite.Equal(chunk.Idx, copyCIdx)
			// 	bs, err := io.ReadAll(chunk.In)
			// 	suite.Nil(err)
			// 	suite.Equal(testContent[start:end], string(bs))
			// 	chunk.In = bytes.NewReader(bs)
			// 	return true
			// })).Return(nil).Once()

			sto.On("ChunkWrite", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			url := fmt.Sprintf("/api/v1/image/chunk/%d/upload?uploadID=%s", cIdx, uploadID)
			req, _ = http.NewRequest("POST", url, body)
			testutils.AddAuth(req, user, pass)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			suite.r.ServeHTTP(w, req)

			suite.Equalf(http.StatusOK, w.Code, "error: %v", w.Body.String())
		}

		fmt.Printf("++++++ all chunk upload done\n")
		suite.testMergeChunk(uploadID, digest)
	}
}

func (suite *imageTestSuite) testMergeChunk(uploadID, digest string) {
	user, pass := "user1", "pass1"
	models.Mock.ExpectBegin()
	models.Mock.ExpectExec("INSERT INTO repository(username, name, private) VALUES(?, ?, ?)").
		WithArgs("user1", "name1", false).
		WillReturnResult(sqlmock.NewResult(1234, 1))

	models.Mock.ExpectExec("INSERT INTO image(repo_id, tag, labels, size, format, os, digest, snapshot, description) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)").
		WithArgs(1234, "tag1", sqlmock.AnyArg(), len(testContent), "qcow2", sqlmock.AnyArg(), digest, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1234, 1))
	models.Mock.ExpectCommit()

	sto := testutils.GetMockStorage()
	// defer sto.AssertExpectations(suite.T())

	sto.On("Move", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	sto.On("GetSize", mock.Anything, mock.Anything).Return(int64(len(testContent)), nil)
	sto.On("GetDigest", mock.Anything, mock.Anything).Return(digest, nil)

	sto.On("CompleteChunkWrite", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	w := httptest.NewRecorder()
	url := fmt.Sprintf("/api/v1/image/chunk/merge?uploadID=%s", uploadID)
	req, _ := http.NewRequest("POST", url, nil)
	testutils.AddAuth(req, user, pass)
	suite.r.ServeHTTP(w, req)

	fmt.Printf("+++++++++ merge: %s\n", w.Body.String())
	suite.Equal(http.StatusOK, w.Code)
}

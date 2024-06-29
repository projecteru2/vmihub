package image

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/mcuadros/go-defaults"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/vmihub/internal/common"
	"github.com/projecteru2/vmihub/internal/models"
	storFact "github.com/projecteru2/vmihub/internal/storage/factory"
	stotypes "github.com/projecteru2/vmihub/internal/storage/types"
	"github.com/projecteru2/vmihub/internal/utils"
	"github.com/projecteru2/vmihub/pkg/terrors"
	"github.com/projecteru2/vmihub/pkg/types"
	"github.com/redis/go-redis/v9"
)

const (
	redisInfoKey  = "/vmihub/chunk/info/%s"
	redisSliceKey = "/vmihub/chunk/slice/%s"

	redisImageHKey    = "image"
	redisForceHKey    = "force"
	redisSizeHKey     = "chunkSize"
	redisDigestHkey   = "digest"
	redisChunkNumHkey = "nChunks"

	chunkRedisExpire = 60 * 60 * time.Second
	defaultChunkSize = "50M" // 1024 * 1024 * 50
)

// DownloadImageChunk  download image chunk
//
// @Summary download image chunk
// @Description DownloadImageChunk download image chunk
// @Tags 镜像管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param username path string true "仓库用户名"
// @Param name path string true "仓库名"
// @Param chunkIdx path int true "分片序号"
// @Param tag query string false "标签"  default("latest")
// @Param chunkSize query string false "分片大小"  default("50M")
// @Success  200
// @Router  /image/{username}/{name}/chunk/{chunkIdx}/download [get]
func DownloadImageChunk(c *gin.Context) {
	username := c.Param("username")
	name := c.Param("name")
	chunkIdx := c.Param("chunkIdx")
	tag := validateParamTag(c.Query("tag"))
	chunkSizeStr := c.DefaultQuery("chunkSize", defaultChunkSize)
	cIdx, _ := strconv.Atoi(chunkIdx) //nolint:nolintlint,errcheck

	if err := validateRepoName(username, name); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
		return
	}

	chunkSize, err := humanize.ParseBytes(chunkSizeStr)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid chunk size"})
		return
	}

	// check upload image is exist in db
	repo, err := getRepo(c, username, name, "read")
	if err != nil {
		return
	}
	img, err := getRepoImage(c, repo, tag)
	if err != nil {
		return
	}

	if img.Format == models.ImageFormatRBD {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "image created from system disk doesn't support download"})
		return
	}
	sliceNum := uint64(math.Ceil(float64(img.Size) / float64(chunkSize)))
	if uint64(cIdx) > sliceNum {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("sliceIndex: %d, out of range\n", cIdx),
		})
		return
	}
	sto := storFact.Instance()
	offset := int64(uint64(cIdx) * chunkSize)
	rc, err := sto.SeekRead(c, img.Fullname(), offset)
	if err != nil {
		log.WithFunc("DownloadImageChunk").Error(c, err, "failed to get seek reader")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error, please try again.",
		})
		return
	}
	defer rc.Close()

	contentSize := chunkSize
	if offset+int64(contentSize) > img.Size {
		contentSize = uint64(img.Size) - uint64(offset)
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename="+img.SliceName())
	c.Header("Content-Length", fmt.Sprintf("%d", contentSize))

	reader := io.LimitReader(rc, int64(contentSize))
	// write content to response
	_, err = io.Copy(c.Writer, reader)
	if err != nil {
		log.WithFunc("DownloadImageChunk").Error(c, err, "Failed to download file")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to download file"})
		return
	}
}

// StartImageChunkUpload  start chunk upload session
//
// @Summary upload image chunk
// @Description UploadImageChunk upload image chunk
// @Tags 镜像管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param username path string true "用户名"
// @Param name path string true "镜像名"
// @Param force query bool false "强制上传（覆盖）" default("false")
// @Param chunkSize query int true "chunk大小"
// @Param nChunks query int true "chunk数量"
// @Success  200
// @Router  /image/:username/:name/startChunkUpload [post]
func StartImageChunkUpload(c *gin.Context) {
	logger := log.WithFunc("StartImageChunkUpload")
	username := c.Param("username")
	name := c.Param("name")
	force := utils.GetBooleanQuery(c, "force", false)
	chunkSize := c.Query("chunkSize")
	nChunks := c.Query("nChunks")
	if err := validateChunkSize(c, chunkSize); err != nil {
		return
	}
	if err := validateNChunks(c, nChunks); err != nil {
		return
	}
	var req types.ImageCreateRequest
	defaults.SetDefaults(&req)
	if err := c.ShouldBindWith(&req, binding.JSON); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := req.Check(); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tag := utils.NormalizeTag(req.Tag, req.Digest)
	if err := validateRepoName(username, name); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
		return
	}
	repo, img, err := common.GetRepoImageForUpload(c, username, name, tag)
	if err != nil {
		return
	}

	// if img exists and not force update, upload failed!
	if img != nil && !force {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"error": "Upload failed, image already exists. You can use force upload to overwrite.",
		})
		return
	}

	if repo == nil {
		repo = &models.Repository{
			Username: username,
			Name:     name,
			Private:  req.Private,
		}
	}
	if img == nil {
		reqLabels := models.Labels{}
		if req.Labels != nil {
			reqLabels = models.Labels(req.Labels)
		}
		img = &models.Image{
			RepoID:      repo.ID,
			Tag:         tag,
			Labels:      models.NewJSONColumn(&reqLabels),
			Size:        req.Size,
			Digest:      req.Digest,
			Format:      req.Format,
			OS:          models.NewJSONColumn(&req.OS),
			Description: req.Description,
			Repo:        repo,
		}
	}

	rdb := utils.GetRedisConn()

	sto := storFact.Instance()
	uploadID, err := sto.CreateChunkWrite(c, img.SliceName())
	if err != nil {
		logger.Error(c, err, "Failed to save file to storage [CreateChunkWrite]")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error",
		})
		return
	}

	// set redis expiration time
	for _, fStr := range []string{redisInfoKey, redisSliceKey} {
		rKey := fmt.Sprintf(fStr, uploadID)
		err = rdb.Expire(c, rKey, chunkRedisExpire).Err()
		if err != nil {
			logger.Errorf(c, err, "Failed to set expiration for %s", rKey)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
	}
	bs, err := json.Marshal(img)
	if err != nil {
		logger.Error(c, err, "failed marshal image tag")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error, please try again",
		})
		return
	}
	err = rdb.HSet(c, fmt.Sprintf(redisInfoKey, uploadID),
		redisImageHKey, string(bs),
		redisForceHKey, strconv.FormatBool(force),
		redisSizeHKey, chunkSize,
		redisDigestHkey, req.Digest,
		redisChunkNumHkey, nChunks,
	).Err()
	if err != nil {
		logger.Error(c, err, "Failed to set information and slices")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to set set expiration",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": map[string]any{
			"uploadID": uploadID,
		},
	})
}

// UploadImageChunk  upload image chunk
//
// @Summary upload image chunk
// @Description UploadImageChunk upload image chunk
// @Tags 镜像管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param chunkIdx path string true "分片序列"
// @Param uploadID query string true "上传uploadID"
// @Param file formData file true "文件"
// @Success  200
// @Router  /image/chunk/{chunkIdx}/upload [post]
func UploadImageChunk(c *gin.Context) {
	logger := log.WithFunc("UploadImageChunk")
	chunkIdxStr := c.Param("chunkIdx")
	uploadID := c.Query("uploadID")
	digest := c.Query("digest")

	if uploadID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "empty uploadID"})
		return
	}
	chunkIdx, err := strconv.ParseInt(chunkIdxStr, 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "invalid chunk index",
		})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("get upload file failed: %s", err),
		})
		return
	}

	rdb := utils.GetRedisConn()
	rAns := rdb.HGetAll(c, fmt.Sprintf(redisInfoKey, uploadID))
	if rAns.Err() != nil {
		logger.Error(c, rAns.Err(), "Failed to set information and slices")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to set set expiration",
		})
		return
	}
	var (
		img       models.Image
		chunkSize uint64
		nChunks   int
	)
	for k, v := range rAns.Val() {
		switch k {
		case redisImageHKey:
			err = json.Unmarshal([]byte(v), &img)
		case redisSizeHKey:
			chunkSize, err = humanize.ParseBytes(v)
		case redisChunkNumHkey:
			nChunks, err = strconv.Atoi(v)
		}
		if err != nil {
			logger.Errorf(c, err, "incorrect redis value: %s %s", k, v)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "internal error, please try again",
			})
			return
		}
	}
	if chunkIdx >= int64(nChunks) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Only need %d chunks, but got chunk index %d", nChunks, chunkIdx),
		})
		return
	}
	sto := storFact.Instance()

	fileOpen, err := file.Open()
	if err != nil {
		logger.Error(c, err, "Failed to open FileHeader")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "bad request",
		})
		return
	}
	defer fileOpen.Close()

	fp, err := os.CreateTemp("/tmp", "image-chunk-upload-")
	if err != nil {
		logger.Error(c, err, "failed to create temp file")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	defer os.Remove(fp.Name())
	defer fp.Close()

	h := sha256.New()
	reader := io.TeeReader(fileOpen, h)

	nwritten, err := io.Copy(fp, reader)
	if err != nil {
		logger.Errorf(c, err, "failed to save upload chunk to local temporary file")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	if _, err := fp.Seek(0, 0); err != nil {
		logger.Error(c, err, "failed to seek file")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	cInfo := &stotypes.ChunkInfo{
		Idx:       int(chunkIdx),
		Size:      nwritten,
		ChunkSize: int64(chunkSize),
		Digest:    "",
		In:        fp,
	}
	err = sto.ChunkWrite(c, img.SliceName(), uploadID, cInfo)
	if err != nil {
		logger.Error(c, err, "Failed to save file to storage [ChunkWrite]")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error",
		})
		return
	}

	// 计算哈希值
	sum := h.Sum(nil)
	contentDigest := fmt.Sprintf("%x", sum)

	// check if the digest equals to user-passed digest
	if digest != "" && contentDigest != digest {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid digest: got: %s, user passed: %s", img.Digest, digest),
		})
		return
	}
	cInfo.Digest = contentDigest

	if err := rdb.HSet(c, fmt.Sprintf(redisSliceKey, uploadID), chunkIdx, cInfo).Err(); err != nil {
		logger.Errorf(c, err, "failed to save chunk info %d to redis", chunkIdx)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error, please try again"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"msg": "upload chunk successfully",
	})
}

// MergeChunk merge chunk slice file
//
// @Summary merge chunk slice file
// @Description MergeChunk merge chunk slice file
// @Tags 镜像管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param uploadID query string true "上传uploadID"
// @Success  200
// @Router  /image/chunk/merge [post]
func MergeChunk(c *gin.Context) {
	logger := log.WithFunc("MergeChunk")
	uploadID := c.Query("uploadID")

	if uploadID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "you must specify upload id",
		})
		return
	}

	rdb := utils.GetRedisConn()
	kv, err := rdb.HGetAll(c, fmt.Sprintf(redisInfoKey, uploadID)).Result()
	if err == redis.Nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "you should start chunk upload first",
		})
		return
	}
	if err != nil {
		logger.Error(c, err, "Failed to get information and slices")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	img := &models.Image{}
	var (
		// force   bool
		digest  string
		nChunks int
	)
	for k, v := range kv {
		switch k {
		case redisImageHKey:
			err = json.Unmarshal([]byte(v), img)
		case redisForceHKey:
			// force, err = strconv.ParseBool(v)
			_, err = strconv.ParseBool(v)
		case redisDigestHkey:
			digest = v
		case redisChunkNumHkey:
			nChunks, err = strconv.Atoi(v)
		}
		if err != nil {
			logger.Errorf(c, err, "incorrect redis value: %s %s", k, v)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "internal error, please try again",
			})
			return
		}
	}
	repo := img.Repo

	chunkList, err := checkChunkSlices(c, uploadID, nChunks)
	if err != nil {
		return
	}
	sto := storFact.Instance()

	err = sto.CompleteChunkWrite(c, img.SliceName(), uploadID, chunkList)
	if err != nil {
		logger.Error(c, err, "Failed to save file to storage [CompleteChunkWrite]")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error",
		})
		return
	}

	img.Size, err = sto.GetSize(c, img.SliceName())
	if err != nil {
		logger.Error(c, err, "failed get size of %s", img.SliceName())
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error, please try again",
		})
		return
	}
	img.Digest, err = sto.GetDigest(c, img.SliceName())
	if err != nil {
		logger.Error(c, err, "failed get digest of %s", img.SliceName())
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error, please try again",
		})
		return
	}
	if digest != "" && digest != img.Digest {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid digest: got: %s, user passed: %s", img.Digest, digest),
		})
		return
	}

	if err = sto.Move(c, img.SliceName(), img.Fullname()); err != nil {
		logger.Error(c, err, "failed move %s to %s", img.SliceName(), img.Fullname())
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error, please try again",
		})
		return
	}

	tx, err := models.Instance().Beginx()
	if err != nil {
		logger.Error(c, err, "failed get transaction")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error, please try again",
		})
		return
	}
	if repo.ID == 0 {
		if err = repo.Save(tx); err != nil {
			logger.Error(c, err, "failed vsave image to db")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "internal error, please try again",
			})
			return
		}
	}
	if err = repo.SaveImage(tx, img); err != nil {
		logger.Error(c, err, "failed save image tag to db")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error, please try again",
		})
		return
	}
	_ = tx.Commit()
	err = rdb.Del(c, fmt.Sprintf(redisInfoKey, uploadID), fmt.Sprintf(redisSliceKey, uploadID)).Err()
	if err != nil {
		// just log error
		logger.Error(c, err, "Failed to delete chunk keys in redis for %d", uploadID)
	}
	// if err := task.SendImageTask(img.ID, force); err != nil {
	// 	logger.Warnf(c, "failed to sned image preparation task")
	// }
	c.JSON(http.StatusOK, gin.H{
		"msg":  "merge success",
		"data": "",
	})

}

func checkChunkSlices(c *gin.Context, uploadID string, nChunks int) (ans []*stotypes.ChunkInfo, err error) {
	logger := log.WithFunc("checkChunkSlice")
	rdb := utils.GetRedisConn()
	kv, err := rdb.HGetAll(c, fmt.Sprintf(redisSliceKey, uploadID)).Result()
	if err == redis.Nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "you should start chunk upload first",
		})
		return nil, terrors.ErrPlaceholder
	}
	if err != nil {
		logger.Error(c, err, "Failed to get slices from redis")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get chunk info from redis",
		})
		return nil, terrors.ErrPlaceholder
	}

	if nChunks != len(kv) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("need %d chunks, but only got %d chunks", nChunks, len(kv)),
		})
		return nil, terrors.ErrPlaceholder
	}
	intSet := map[int]struct{}{}
	for idx := 0; idx < len(kv); idx++ {
		intSet[idx] = struct{}{}
	}
	for k, v := range kv {
		cIdx, err := strconv.Atoi(k)
		if err != nil {
			logger.Errorf(c, err, "invalid slice key %s", k)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error, please try again"})
			return nil, terrors.ErrPlaceholder
		}
		cInfo := &stotypes.ChunkInfo{}
		if err = json.Unmarshal([]byte(v), cInfo); err != nil {
			logger.Errorf(c, err, "invalid slice value %s %s", k, v)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error, please try again"})
			return nil, terrors.ErrPlaceholder
		}
		ans = append(ans, cInfo)

		if _, ok := intSet[cIdx]; !ok {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("miss chunks, current chunks %v", kv),
			})
			return nil, terrors.ErrPlaceholder
		}
		delete(intSet, cIdx)
	}
	if len(intSet) != 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("miss chunks, current chunks %v", kv),
		})
		return nil, terrors.ErrPlaceholder
	}
	return ans, nil
}

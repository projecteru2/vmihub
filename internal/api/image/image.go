package image

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/vmihub/internal/common"
	storFact "github.com/projecteru2/vmihub/internal/storage/factory"
	storTypes "github.com/projecteru2/vmihub/internal/storage/types"

	"github.com/projecteru2/vmihub/internal/utils"
	"github.com/projecteru2/vmihub/pkg/terrors"
	"github.com/projecteru2/vmihub/pkg/types"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/mcuadros/go-defaults"
	"github.com/panjf2000/ants/v2"
	_ "github.com/projecteru2/vmihub/cmd/vmihub/docs" // for doc
	"github.com/projecteru2/vmihub/internal/models"
	"github.com/redis/go-redis/v9"
)

const (
	defaultTag     = "latest"
	chunkThreshold = 4 * utils.GB
)

func SetupRouter(r *gin.RouterGroup) {
	imageGroup := r.Group("/image")

	repoGroup := r.Group("/repository")

	imageGroup.POST("/:username/:name/startChunkUpload", StartImageChunkUpload)
	imageGroup.POST("/chunk/:chunkIdx/upload", UploadImageChunk)
	imageGroup.POST("/chunk/merge", MergeChunk)
	imageGroup.GET("/:username/:name/chunk/:chunkIdx/download", DownloadImageChunk)

	// Get image information
	imageGroup.GET("/:username/:name/info", GetImageInfo)
	// download image file
	imageGroup.GET("/:username/:name/download", DownloadImage)

	// upload image file
	imageGroup.POST("/:username/:name/startUpload", StartImageUpload)
	imageGroup.POST("/:username/:name/upload", UploadImage)

	// delete image info form db and file from store
	imageGroup.DELETE("/:username/:name", DeleteImage)

	// Return image Info list of current user
	r.GET("/repositories", ListRepositories)
	// List image
	r.GET("/images", ListImages)
	// Return image list of specified repository.
	repoGroup.GET("/:username/:name/images", ListRepoImages)
	repoGroup.DELETE("/:username/:name", DeleteRepository)
}

// ListRepositories get repository list of specified user or current user
//
// @Summary get repository list
// @Description ListRepositories get repository list
// @Tags 镜像管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param username query string false "用户名"
// @Param page query int false "页码"  default(1)
// @Param pageSize query int false "每一页数量"  default(10)
// @Success  200
// @Router  /repositories [get]
func ListRepositories(c *gin.Context) {
	username := c.Query("username")
	pNum := 1
	page := c.DefaultQuery("page", "1")
	pSize := 10
	pageSize := c.DefaultQuery("pageSize", "10")
	if page != "" {
		pNum, _ = strconv.Atoi(page)
	}
	if pageSize != "" {
		pSize, _ = strconv.Atoi(pageSize)
	}

	curUser, ok := common.LoginUser(c)
	if !ok && username == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "you need login or provide a username as query parameter.",
		})
		return
	}
	var repos []models.Repository
	var err error
	if username == "" {
		repos, err = models.QueryRepoList(curUser.Username, pNum, pSize)
	} else {
		if curUser != nil && (curUser.Admin || curUser.Username == username) {
			repos, err = models.QueryRepoList(username, pNum, pSize)
		} else {
			repos, err = models.QueryPublicRepoList(username, pNum, pSize)
		}
	}
	if err != nil {
		log.WithFunc("ListRepositories").Error(c, err, "Can't query repositories from database")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"msg": "internal error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": repos,
	})
}

// GetImageInfo get image meta info
//
// @Summary get image meta info
// @Description GetImageInfo get image meta info
// @Tags 镜像管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param username path string true "仓库用户名"
// @Param name path string true "仓库名"
// @Param tag query string false "镜像标签"  default("latest")
// @Success  200
// @Router  /image/{username}/{name}/info [get]
func GetImageInfo(c *gin.Context) {
	username := c.Param("username")
	name := c.Param("name")
	tag := c.DefaultQuery("tag", defaultTag)
	err := validateRepoName(username, name)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}
	repo, err := getRepo(c, username, name, "read")
	if err != nil {
		return
	}
	img, err := getRepoImage(c, repo, tag)
	if err != nil {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"msg":  "success",
		"data": convImageInfoResp(img),
	})
}

// ListRepoImages get image list of repository
//
// @Summary get image list of specified repository
// @Description ListRepoImages get image list of specified repo
// @Tags 镜像管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param username path string true "用户名"
// @Param name path string true "仓库名"
// @Success  200
// @Router  /repository/{username}/{name} [get]
func ListRepoImages(c *gin.Context) {
	username := c.Param("username")
	name := c.Param("name")
	err := validateRepoName(username, name)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"msg": "invaid name",
		})
		return
	}
	repo, err := getRepo(c, username, name, "read")
	if err != nil {
		return
	}

	images, err := repo.GetImages()
	if err != nil {
		log.WithFunc("ListImageTags").Error(c, err, "Can't get image tags from database")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"msg": "query image error",
		})
		return
	}

	resp := make([]*types.ImageInfoResp, len(images))
	for idx := 0; idx < len(images); idx++ {
		resp[idx] = convImageInfoResp(&images[idx])
	}
	c.JSON(http.StatusOK, gin.H{
		"data": resp,
	})
}

// DeleteImage delete repository
//
// @Summary delete specified repository
// @Description DeleteRepository
// @Tags 镜像管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param username path string true "用户名"
// @Param name path string true "仓库名"
// @Success  200
// @Router  /repository/{username}/{name} [delete]
func DeleteRepository(c *gin.Context) {
	name := c.Param("name")
	username := c.Param("username")

	err := validateRepoName(username, name)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
		return
	}
	repo, err := getRepo(c, username, name, "write")
	if err != nil {
		return
	}

	images, err := repo.GetImages()
	if err != nil {
		log.WithFunc("DeleteImage").Error(c, err, "internal error")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"msg": "internal error",
		})
	}
	stor := storFact.Instance()
	for _, img := range images {
		err = stor.Delete(context.Background(), img.Fullname(), true)
		if err != nil {
			// Try best bahavior, so just log error
			log.WithFunc("DeleteImage").Errorf(c, err, "failed to remove image %s from storage", img.Fullname())
		}
	}

	tx, err := models.Instance().Beginx()
	if err != nil {
		log.WithFunc("DeleteImage").Error(c, err, "failed to get transaction")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"msg": "internal error",
		})
	}
	if err = repo.Delete(tx); err != nil {
		log.WithFunc("DeleteImage").Error(c, err, "internal error")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"msg": "internal error",
		})
	}
	_ = tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"msg":  "delete success",
		"data": "",
	})
}

// StartUpload  start single file upload session
//
// @Summary upload image file
// @Description StartUpload upload image file
// @Tags 镜像管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param username path string true "用户名"
// @Param name path string true "镜像名"
// @Param force query bool false "强制上传（覆盖）" default("false")
// @Param body body types.ImageCreateRequest true "镜像配置"
// @Success  200
// @Router  /image/:username/:name/startUpload [post]
func StartImageUpload(c *gin.Context) {
	logger := log.WithFunc("StartImageUpload")
	username := c.Param("username")
	name := c.Param("name")
	force := utils.GetBooleanQuery(c, "force", false)
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

	if req.URL != "" {
		if err := processRemoteImageFile(c, img, req.URL); err != nil {
			return
		}
		// logger.Debugf(c, "send image task")
		// if err := task.SendImageTask(img.ID, force); err != nil {
		// 	logger.Warnf(c, "failed to send image preparation task")
		// }
		c.JSON(http.StatusOK, gin.H{
			"msg": "upload remote file successfully",
			"data": map[string]any{
				"uploadID": "",
			},
		})
		return
	}
	uploadID, err := newUploadID()
	if err != nil {
		logger.Error(c, err, "failed to generate upload id")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}
	rdb := utils.GetRedisConn()
	bs, _ := json.Marshal(img)
	logger.Debugf(c, "uploadID : %s", uploadID)
	if err := rdb.HSet(
		c, fmt.Sprintf(redisInfoKey, uploadID),
		redisImageHKey, string(bs),
		redisForceHKey, strconv.FormatBool(force),
	).Err(); err != nil {
		logger.Error(c, err, "Failed to set image information to redis")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to set image information to redis",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": map[string]any{
			"uploadID": uploadID,
		},
	})
}

// UploadImage upload image
//
// @Summary upload image
// @Description UploadImage upload image
// @Tags 镜像管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param username path string true "仓库用户名"
// @Param name path string true "仓库名"
// @Param force query bool false "强制上传（覆盖）" default("false")
// @Param file formData file true "文件"
// @Success  200
// @Router  /image/{username}/{name}/upload [post]
func UploadImage(c *gin.Context) {
	logger := log.WithFunc("UploadImage")
	username := c.Param("username")
	name := c.Param("name")

	uploadID := c.Query("uploadID")
	if uploadID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "empty uploadID"})
		return
	}
	err := validateRepoName(username, name)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invaid name(%s)", err),
		})
		return
	}

	// upload single file
	file, err := c.FormFile("file")
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request: bad file(%s)", err),
		})
		return
	}

	fileOpen, err := file.Open()
	if err != nil {
		logger.Error(c, err, "Failed to open FileHeader")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "bad request",
		})
		return
	}
	defer fileOpen.Close()

	rdb := utils.GetRedisConn()
	kv, err := rdb.HGetAll(c, fmt.Sprintf(redisInfoKey, uploadID)).Result()
	if err == redis.Nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "you should start image upload first",
		})
		return
	}
	if err != nil {
		logger.Error(c, err, "Failed to get upload information from redis")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	logger.Debugf(c, "uploadID : %s", uploadID)
	img := &models.Image{}
	// var force bool
	for k, v := range kv {
		switch k {
		case redisImageHKey:
			err = json.Unmarshal([]byte(v), &img)
		case redisForceHKey:
			// force, err = strconv.ParseBool(v)
			_, err = strconv.ParseBool(v)
		default:
			err = errors.Newf("unknown redis hash key %s", k)
		}
		if err != nil {
			logger.Errorf(c, err, "incorrect redis value: %s %s", k, v)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "internal error",
			})
			return
		}
	}
	if img.Repo == nil {
		logger.Errorf(c, err, "there is no image info in redis for uploadID: %s", uploadID)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error",
		})
		return
	}

	// 这里之所以要写入一个临时文件是因为文件很大的时候需要分片上传
	// 分片上传为了加快进度是做了并发处理的，这时候需要并发的open， seek，用一个本地文件更方便
	fp, err := os.CreateTemp("/tmp", "image-upload-")
	if err != nil {
		logger.Error(c, err, "failed to create temp file")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	defer os.Remove(fp.Name())
	defer fp.Close()

	nwritten, err := io.Copy(fp, fileOpen)
	if err != nil {
		logger.Errorf(c, err, "failed to save upload file to local")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	if err = writeDataToStorage(c, img, fp.Name(), nwritten); err != nil {
		return
	}

	// logger.Debugf(c, "send image task")
	// if err := task.SendImageTask(img.ID, force); err != nil {
	// 	logger.Warnf(c, "failed to send image preparation task")
	// }

	c.JSON(http.StatusOK, gin.H{
		"msg": "upload image successfully",
	})
}

// DownloadImage download image
//
// @Summary download image
// @Description DownloadImage download image
// @Tags 镜像管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param username path string true "仓库用户名"
// @Param name path string true "仓库名"
// @Param tag query string false "镜像标签" default("latest")
// @Success 200
// @Router /image/{username}/{name}/download [get]
func DownloadImage(c *gin.Context) {
	username := c.Param("username")
	name := c.Param("name")

	tag := c.DefaultQuery("tag", "latest")
	err := validateRepoName(username, name)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
		return
	}
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
	sto := storFact.Instance()

	file, err := sto.Get(context.Background(), img.Fullname())
	if err != nil {
		log.WithFunc("DownloadImage").Error(c, err, "Failed to get image file")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"msg": "Failed to get image file",
		})
		return
	}
	defer file.Close()

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename="+img.Fullname())
	c.Header("Content-Length", fmt.Sprintf("%d", img.Size))

	// write content to response
	_, err = io.Copy(c.Writer, file)
	if err != nil {
		log.WithFunc("DownloadImage").Error(c, err, "Failed to get copy file form storage")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to download file"})
		return
	}
}

// DeleteImage delete image
//
// @Summary delete image
// @Description DeleteImage delete image
// @Tags 镜像管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param username path string true "仓库用户名"
// @Param name path string true "仓库名"
// @Param tag query string false "镜像标签" default("latest")
// @Success  200
// @Router  /image/{username}/{name} [delete]
func DeleteImage(c *gin.Context) {
	logger := log.WithFunc("DeleteImage")
	name := c.Param("name")
	username := c.Param("username")
	tag := c.DefaultQuery("tag", defaultTag)

	err := validateRepoName(username, name)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
		return
	}
	repo, err := getRepo(c, username, name, "write")
	if err != nil {
		return
	}
	img, err := getRepoImage(c, repo, tag)
	if err != nil {
		return
	}

	sto := storFact.Instance()
	if err = sto.Delete(c, img.Fullname(), true); err != nil {
		// Try best bahavior, so just log error
		logger.Errorf(c, err, "failed to remove image %s from storage", img.Fullname())
	}
	if err = repo.DeleteImage(nil, img.Tag); err != nil {
		logger.Error(c, err, "failed to delete image")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}

	c.JSON(http.StatusOK, gin.H{
		"msg": "delete image successfully",
	})
}

// ListImages get image list of current user or specified user
//
// @Summary get image list
// @Description ListImages get images list
// @Tags 镜像管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param keyword query string false "搜索关键字"  default()
// @Param username query string false "用户名"
// @Param page query int false "页码"  default(1)
// @Param pageSize query int false "每一页数量"  default(10)
// @success 200 {object} types.JSONResult{data=[]types.ImageInfoResp} "desc"
// @Router  /images [get]
func ListImages(c *gin.Context) {
	logger := log.WithFunc("ListImages")
	username := c.Query("username")
	pNum := 1
	page := c.DefaultQuery("page", "1")
	pSize := 10
	pageSize := c.DefaultQuery("pageSize", "10")
	keyword := c.DefaultQuery("keyword", "")
	if page != "" {
		pNum, _ = strconv.Atoi(page)
	}
	if pageSize != "" {
		pSize, _ = strconv.Atoi(pageSize)
	}
	if pNum <= 0 || pSize <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid page or page size"})
		return
	}

	curUser, ok := common.LoginUser(c)
	if !ok && username == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "you need login or provide a username as query parameter.",
		})
		return
	}
	var (
		imgs  []models.Image
		total int
		err   error
	)
	if username == "" {
		username = curUser.Username
	}
	regionCode := c.DefaultQuery("regionCode", "ap-yichang-1")
	req := types.ImagesByUsernameRequest{Username: username, Keyword: keyword, PageNum: pNum, PageSize: pSize, RegionCode: regionCode}
	if curUser != nil && (curUser.Admin || curUser.Username == username) {
		imgs, total, err = models.QueryImagesByUsername(req)
	} else {
		imgs, total, err = models.QueryPublicImagesByUsername(req)
	}
	if err != nil {
		logger.Error(c, err, "Can't query images from database")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error",
		})
		return
	}
	resps, err := convImageListResp(nil, imgs)
	if err != nil {
		logger.Error(c, err, "failed to conv images repsonses")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":  resps,
		"total": total,
	})
}

func writeDataToStorage(c *gin.Context, img *models.Image, fname string, size int64) (err error) {
	logger := log.WithFunc("writeDataToStorage")
	logger.Debugf(c, "starting to write file to storage, size %d", size)
	defer logger.Debugf(c, "exit writing file to storage, err: %s", err)

	if size < chunkThreshold {
		if err := writeSingleFile(c, img, fname); err != nil {
			return err
		}
	} else {
		if err := writeSingleFileWithChunk(c, img, fname, size); err != nil {
			return err
		}
	}

	repo := img.Repo
	tx, err := models.Instance().Beginx()
	if err != nil {
		logger.Error(c, err, "failed to get transaction")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error, please try again",
		})
		return err
	}
	// 将镜像信息写入数据库或者覆盖
	if err = repo.Save(tx); err != nil {
		logger.Error(c, err, "failed to save repository to db")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error",
		})
		return err
	}
	if err = repo.SaveImage(tx, img); err != nil {
		logger.Error(c, err, "failed to save image to db")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error",
		})
		return err
	}
	if err := tx.Commit(); err != nil {
		logger.Error(c, err, "failed to commit transaction")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return err
	}

	return nil
}

func processRemoteImageFile(c *gin.Context, img *models.Image, url string) error {
	logger := log.WithFunc("processRremoteImageFile")
	resp, err := http.Get(url) //nolint
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("failed to download remote file %s", url)})
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "failed to download url"})
		return errors.Newf("failed to get remote file, http code: %s", resp.StatusCode)
	}
	fp, err := os.CreateTemp("/tmp", "download-remote-")
	if err != nil {
		logger.Error(c, err, "failed to create temp file")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return err
	}
	defer os.Remove(fp.Name())
	defer fp.Close()

	h := sha256.New()
	reader := io.TeeReader(resp.Body, h)
	nwritten, err := io.Copy(fp, reader)
	if err != nil {
		logger.Error(c, err, "failed to download file")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "failed to download url"})
		return err
	}
	if err := fp.Sync(); err != nil {
		logger.Error(c, err, "failed to sync file")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return err
	}
	// check size
	if img.Size == 0 {
		img.Size = nwritten
	}
	if img.Size != nwritten {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "size mismatch"})
		return terrors.ErrPlaceholder
	}
	// check digest
	digest := fmt.Sprintf("%x", h.Sum(nil))
	if img.Digest == "" {
		img.Digest = digest
	}
	if digest != img.Digest {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "digest mismatch"})
		return terrors.ErrPlaceholder
	}
	if img.Tag == utils.FakeTag {
		img.Tag = digest[:10]
	}
	// set file pointer to start
	if _, err := fp.Seek(0, 0); err != nil {
		logger.Error(c, err, "failed to seek file")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return err
	}
	return writeDataToStorage(c, img, fp.Name(), nwritten)
}

func writeSingleFileWithChunk(c *gin.Context, img *models.Image, fname string, size int64) error {
	logger := log.WithFunc("writeSingleFileWithChunk")
	sto := storFact.Instance()
	uploadID, err := sto.CreateChunkWrite(c, img.Fullname())
	if err != nil {
		logger.Error(c, err, "Failed to save file to storage [CreateChunkWrite]")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error",
		})
		return err
	}
	chunkSize := int64(300 * utils.MB)
	nChunks := (size + chunkSize - 1) / chunkSize
	chunkList := make([]*storTypes.ChunkInfo, nChunks)
	errList := make([]error, nChunks)
	var (
		hasErr atomic.Bool
		wg     sync.WaitGroup
	)
	logger.Debug(c, "start uploading chunks, hasErr: %v", hasErr.Load())
	p, _ := ants.NewPoolWithFunc(30, func(i any) {
		defer wg.Done()
		chunkIdx := i.(int64) //nolint
		fp, err := os.Open(fname)
		if err != nil {
			errList[chunkIdx] = errors.Wrapf(err, "failed to open file %s", fname)
			hasErr.Store(true)
			return
		}
		defer fp.Close()

		logger.Debugf(c, "write chunk %d", chunkIdx)
		curSize := chunkSize
		if chunkIdx == nChunks-1 {
			curSize = size - chunkIdx*chunkSize
		}
		sReader := io.NewSectionReader(fp, chunkIdx*chunkSize, curSize)
		cInfo := &storTypes.ChunkInfo{
			Idx:       int(chunkIdx),
			Size:      curSize,
			ChunkSize: chunkSize,
			Digest:    "",
			In:        sReader,
		}
		chunkList[chunkIdx] = cInfo
		if err = sto.ChunkWrite(c, img.Fullname(), uploadID, cInfo); err != nil {
			errList[chunkIdx] = errors.Wrapf(err, "failed to write chunk %d", chunkIdx)
			logger.Errorf(c, errList[chunkIdx], "failed to write chunk %d", chunkIdx)
			hasErr.Store(true)
			return
		}
	})
	defer p.Release()
	for chunkIdx := int64(0); chunkIdx < nChunks; chunkIdx++ {
		wg.Add(1)
		if err = p.Invoke(chunkIdx); err != nil {
			logger.Errorf(c, err, "failed to submit pool task %d", chunkIdx)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return err
		}
	}
	wg.Wait()

	logger.Debug(c, "finished uploading chunks, hasErr: %v", hasErr.Load())
	if hasErr.Load() {
		var outErr error
		for idx := range errList {
			outErr = errors.CombineErrors(outErr, errList[idx])
		}
		logger.Error(c, outErr, "Failed to save file to storage [ChunkWrite]")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error",
		})
		return outErr
	}
	logger.Debug(c, "completing chunk write")
	err = sto.CompleteChunkWrite(c, img.Fullname(), uploadID, chunkList)
	if err != nil {
		logger.Error(c, err, "Failed to save file to storage [CompleteChunkWrite]")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error",
		})
		return err
	}

	return nil
}

func writeSingleFile(c *gin.Context, img *models.Image, fname string) error {
	logger := log.WithFunc("writeSingleFile")
	digest := img.Digest
	sto := storFact.Instance()
	fp, err := os.Open(fname)
	if err != nil {
		return err
	}
	if err := sto.Put(c, img.Fullname(), digest, fp); err != nil {
		logger.Error(c, err, "Failed to save file to storage")
		if errors.Is(err, terrors.ErrInvalidDigest) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("invalid digest: got: %s, user passed: %s", img.Digest, digest),
			})
		} else {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "internal error",
			})
		}
		return terrors.ErrPlaceholder
	}
	return nil
}

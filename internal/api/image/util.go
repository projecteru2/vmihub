package image

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/vmihub/internal/common"
	"github.com/projecteru2/vmihub/internal/models"
	"github.com/projecteru2/vmihub/internal/utils"
	"github.com/projecteru2/vmihub/pkg/terrors"
	"github.com/projecteru2/vmihub/pkg/types"
)

func newUploadID() (string, error) {
	raw, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func checkRepoReadPerm(c *gin.Context, repo *models.Repository) bool {
	if !repo.Private {
		return true
	}
	curUser, exists := common.LoginUser(c)
	if !exists {
		return false
	}
	if curUser.Admin {
		return true
	}
	return strings.EqualFold(curUser.Username, repo.Username)
}

func checkRepoWritePerm(c *gin.Context, repo *models.Repository) bool {
	curUser, exists := common.LoginUser(c)
	if !exists {
		return false
	}
	if curUser.Admin {
		return true
	}
	return strings.EqualFold(curUser.Username, repo.Username)
}

func getRepo(c *gin.Context, username, name string, perm string) (repo *models.Repository, err error) {
	repo, err = models.QueryRepo(c, username, name)
	if err != nil {
		log.WithFunc("getRepo").Error(c, err, "failed to get repo from db")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error, please try again",
		})
		return
	}
	if repo == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "image doesn't exist",
		})
		err = errors.New("placeholder")
		return
	}

	switch perm {
	case "read":
		if !checkRepoReadPerm(c, repo) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "you don't have perssion",
			})
			err = errors.New("placeholder")
			return
		}
	case "write":
		if !checkRepoWritePerm(c, repo) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "you don't have perssion",
			})
			err = errors.New("placeholder")
			return
		}
	}
	return
}

func getRepoImage(c *gin.Context, repo *models.Repository, tag string) (img *models.Image, err error) {
	img, err = repo.GetImage(c, tag)
	if err != nil {
		log.WithFunc("getRepoImage").Errorf(c, err, "failed to get image  %s:%s from db", repo.Fullname(), tag)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"msg": "internal error",
		})
		return nil, err
	}
	if img == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"msg": "image doesn't exist",
		})
		return nil, errors.New("placeholder")
	}
	return img, nil
}

func validateRepoName(username, name string) (err error) {
	if username == "" {
		return fmt.Errorf("empty username")
	}
	if username == "_" {
		return checkNames(name)
	} else { //nolint:revive
		return checkNames(username, name)
	}
}

func checkNames(names ...string) (err error) {
	for _, p := range names {
		var matched bool
		matched, err = regexp.MatchString(utils.NameRegex, p) //nolint
		if err != nil {
			return
		}
		if !matched {
			err = fmt.Errorf("invalid name %s", p)
			return
		}
	}
	return
}

func validateParamTag(tag string) string {
	if tag == "" {
		tag = defaultTag
	}
	return tag
}

func validateChunkSize(c *gin.Context, chunkSize string) error {
	if chunkSize == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "empty chunkSize",
		})
		return terrors.ErrPlaceholder
	}
	if _, err := humanize.ParseBytes(chunkSize); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid chunk size"})
		return terrors.ErrPlaceholder
	}
	return nil
}

func validateNChunks(c *gin.Context, nChunks string) error {
	if nChunks == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "empty nChunks",
		})
		return terrors.ErrPlaceholder
	}
	n, err := strconv.Atoi(nChunks)
	if err != nil || n <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid nChunks %s", nChunks),
		})
		return terrors.ErrPlaceholder
	}
	return nil
}

func convImageInfoResp(img *models.Image) *types.ImageInfoResp {
	resp := &types.ImageInfoResp{
		ID:          img.ID,
		RepoID:      img.RepoID,
		Username:    img.Repo.Username,
		Name:        img.Repo.Name,
		Tag:         img.Tag,
		Private:     img.Repo.Private,
		Format:      img.Format,
		OS:          *img.OS.Get(),
		Size:        img.Size,
		Digest:      img.Digest,
		Snapshot:    img.Snapshot,
		Description: img.Description,
		CreatedAt:   img.CreatedAt,
		UpdatedAt:   img.UpdatedAt,
	}
	return resp
}

func convImageListResp(repos []models.Repository, imgs []models.Image) ([]*types.ImageInfoResp, error) {
	repoMap := map[int64]*models.Repository{}
	for idx := range repos {
		repoMap[repos[idx].ID] = &repos[idx]
	}
	resps := make([]*types.ImageInfoResp, 0, len(imgs))
	for idx := range imgs {
		img := &imgs[idx]
		repo := repoMap[img.RepoID]
		if repo == nil {
			repo = img.Repo
		}
		if repo == nil {
			return nil, fmt.Errorf("not repo found for image %v", img)
		}
		img.Repo = repo
		resp := convImageInfoResp(img)
		resps = append(resps, resp)
	}
	return resps, nil
}

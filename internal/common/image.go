package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/vmihub/internal/models"
	"github.com/projecteru2/vmihub/pkg/terrors"
)

func GetRepoImageForUpload(c *gin.Context, imgUser, name, tag string) (repo *models.Repository, img *models.Image, err error) {
	logger := log.WithFunc("GetRepoImageForUpload")
	curUser, exists := LoginUser(c)
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "please login",
		})
		err = terrors.ErrPlaceholder
		return
	}
	if !curUser.Admin && curUser.Username != imgUser {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "you don't have permission to upload to this image",
		})
		err = terrors.ErrPlaceholder
		return
	}
	repo, err = models.QueryRepo(c, imgUser, name)
	if err != nil {
		logger.Errorf(c, err, "can't query image: %s/%s", imgUser, name)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error",
		})
		return
	}
	if repo == nil {
		return
	}
	img, err = repo.GetImage(c, tag)
	if err != nil {
		logger.Errorf(c, err, "can't query image tag: %s/%s:%s", imgUser, name, tag)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error",
		})
		return
	}
	return
}

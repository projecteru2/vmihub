package common

import (
	"github.com/gin-gonic/gin"
	"github.com/projecteru2/vmihub/internal/models"
)

func LoginUser(c *gin.Context) (user *models.User, exists bool) {
	value, exists := c.Get("user")
	if exists {
		user = value.(*models.User) //nolint
	}
	return
}

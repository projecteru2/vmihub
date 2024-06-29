package middlewares

import (
	"net/http"

	//nolint:nolintlint,goimports
	"github.com/gin-gonic/gin"
)

// LoginRequired middleware
func LoginRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "not logged in",
			})
			return
		}
		c.Next()
	}
}

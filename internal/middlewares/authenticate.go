package middlewares

import (
	"fmt"
	"net/http" //nolint:nolintlint,goimports
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/projecteru2/vmihub/internal/common"
)

func getPrivateToken(c *gin.Context) string {
	privateToken := c.Request.Header.Get("PRIVATE-TOKEN")
	if privateToken != "" {
		return privateToken
	}
	return c.Query("private_token")
}

// Authenticate jwt middleware
func Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// first check session
		if err := common.AuthWithSession(c); err == nil {
			c.Next()
			return
		}
		// check private token
		privateToken := getPrivateToken(c)
		if privateToken != "" {
			err := common.AuthWithPrivateToken(c, privateToken)
			if err != nil {
				return
			}
			c.Next()
			return
		}
		// check jwt and basic
		token := c.Request.Header.Get("Authorization")
		if token == "" {
			c.Next()
			return
		}

		var err error
		parts := strings.Split(token, " ")
		if len(parts) != 2 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": fmt.Sprintf("invalid token %s", token),
			})
			return
		}
		switch parts[0] {
		case "Bearer":
			err = common.AuthWithJWT(c, parts[1])
		case "Basic":
			err = common.AuthWithBasic(c, parts[1])
		default:
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": fmt.Sprintf("invalid token %s", token),
			})
			return
		}
		if err != nil {
			return
		}
		c.Next()
	}
}

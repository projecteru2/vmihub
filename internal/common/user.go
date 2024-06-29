package common

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/vmihub/config"
	"github.com/projecteru2/vmihub/internal/models"
	"github.com/projecteru2/vmihub/pkg/terrors"
)

const (
	userIDSessionKey = "userID"
)

func SaveUserSession(c *gin.Context, u *models.User) error {
	// set session
	session := sessions.Default(c)
	session.Set(userIDSessionKey, u.ID)
	return session.Save()
}

func DeleteUserSession(c *gin.Context) error {
	// set session
	session := sessions.Default(c)
	session.Delete(userIDSessionKey)
	return session.Save()
}

func AuthWithSession(c *gin.Context) (err error) {
	sess := sessions.Default(c)
	userID := sess.Get(userIDSessionKey)
	var user *models.User
	if userID != nil {
		if user, err = models.GetUserByID(c, userID.(int64)); err != nil {
			return err
		}
	}
	if user == nil {
		return errors.Newf("can't find user %d", userID)
	}
	log.WithFunc("authWithSession").Debugf(c, "authenticate with session successfully %s", user.Username)

	attachUserToCtx(c, user)
	return nil
}

func AuthWithBasic(c *gin.Context, bs64Token string) error {
	token, err := base64.StdEncoding.DecodeString(bs64Token)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": fmt.Sprintf("invalid basic token %s", err),
		})
		return terrors.ErrPlaceholder
	}
	parts := strings.Split(string(token), ":")
	if len(parts) != 2 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "invalid username or password",
		})
		return terrors.ErrPlaceholder
	}
	username, password := parts[0], parts[1]
	user, err := models.CheckAndGetUser(c, username, password)
	if err != nil {
		if errors.Is(err, terrors.ErrInvalidUserPass) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
		} else {
			log.WithFunc("authWithUserPass").Error(c, err, "failed query user from db")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "internal error, please try again",
			})
		}
		return terrors.ErrPlaceholder
	}
	attachUserToCtx(c, user)
	return nil
}

func AuthWithPrivateToken(c *gin.Context, token string) error {
	t, err := models.GetPrivateToken(token)
	if err != nil {
		log.WithFunc("authWithPrivateToken").Error(c, err, "failed query access token from db")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error, please try again",
		})
		return err
	}
	if t == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "invalid private token",
		})
		return terrors.ErrPlaceholder
	}
	if t.ExpiredAt.Before(time.Now()) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "private token expired",
		})
		return terrors.ErrPlaceholder
	}
	_ = t.UpdateLastUsed()

	// set user login info
	user, err := t.GetUser()
	if err != nil {
		log.WithFunc("authWithPrivateToken").Error(c, err, "failed query user from db")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error, please try again",
		})
		return terrors.ErrPlaceholder
	}
	if user == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "invalid private token",
		})
		return terrors.ErrPlaceholder
	}
	attachUserToCtx(c, user)
	return nil
}

func AuthWithJWT(c *gin.Context, token string) error {
	j := NewJWT(config.GetCfg().JWT.SigningKey)
	// parseToken parse token contain info
	claims, err := j.ParseToken(token)
	if err != nil {
		if err == terrors.ErrTokenExpired {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authorization has expired",
			})
			c.Abort()
			return terrors.ErrPlaceholder
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "please login",
		})
		return terrors.ErrPlaceholder
	}

	// set user login info
	user, err := models.GetUser(c, claims.UserName)
	if err != nil {
		log.WithFunc("authWithUserPass").Error(c, err, "failed query user from db")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal error, please try again",
		})
		return terrors.ErrPlaceholder
	}
	if user == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "invalid token",
		})
		return terrors.ErrPlaceholder
	}
	c.Set("claims", claims)
	attachUserToCtx(c, user)
	return nil
}

func attachUserToCtx(c *gin.Context, u *models.User) {
	c.Set("userid", u.ID)
	c.Set("username", u.Username)
	c.Set("user", u)
}

package user

import (
	"errors"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/vmihub/config"
	"github.com/projecteru2/vmihub/internal/common"
	"github.com/projecteru2/vmihub/internal/middlewares"
	"github.com/projecteru2/vmihub/internal/models"
	"github.com/projecteru2/vmihub/internal/utils"
	"github.com/projecteru2/vmihub/pkg/terrors"
	"github.com/projecteru2/vmihub/pkg/types"
)

const (
	refreshPrefix = "refresh__"
)

func SetupRouter(basePath string, r *gin.Engine) {
	userGroup := r.Group(path.Join(basePath, "/user"))

	// userGroup.Use(AuthRequired)

	// Login user
	userGroup.POST("/login", LoginUser)
	// Logout user
	userGroup.POST("/logout", LogoutUser)
	// Get token
	userGroup.POST("/token", GetUserToken)
	// Refresh token
	userGroup.POST("/refreshToken", RefreshToken)
	// Get user information
	userGroup.GET("/info", middlewares.Authenticate(), GetUserInfo)
	// Update user
	userGroup.POST("/info", middlewares.Authenticate(), UpdateUser)

	// change password
	userGroup.POST("/changePwd", middlewares.Authenticate(), changePwd)
	// reset password
	userGroup.POST("/resetPwd", resetPwd)
	// Create private token
	userGroup.POST("/privateToken", middlewares.Authenticate(), CreatePrivateToken)
	// List private tokens
	userGroup.GET("/privateTokens", middlewares.Authenticate(), ListPrivateToken)
	// Delete private token
	userGroup.DELETE("/privateToken", middlewares.Authenticate(), DeletePrivateToken)
}

// LoginUser login the user
//
// LoginUser @Summary login user
// @Description LoginUser login user
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param body body types.LoginRequest true  "用户结构体"
// @success 200 {object} types.JSONResult{} "desc"
// @Router  /user/login [post]
func LoginUser(c *gin.Context) {
	logger := log.WithFunc("LoginUser")
	// check request params
	var req types.LoginRequest
	if err := c.ShouldBindWith(&req, binding.JSON); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := req.Check(); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := models.CheckAndGetUser(c, req.Username, req.Password)
	// query user
	if err != nil {
		if errors.Is(err, terrors.ErrInvalidUserPass) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		} else {
			logger.Error(c, err, "failed query user from db")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	if user == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if err := common.SaveUserSession(c, user); err != nil {
		logger.Errorf(c, err, "failed to save user session")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}

	c.JSON(http.StatusOK, gin.H{
		"msg": "login successfully",
	})
}

// LogoutUser logout the user
//
// LogoutUser @Summary logout user
// @Description LogoutUser logout user
// @Tags 用户管理
// @Accept json
// @Produce json
// @success 200 {object} types.JSONResult{} "desc"
// @Router  /user/logout [post]
func LogoutUser(c *gin.Context) {
	_, exists := common.LoginUser(c)
	if !exists {
		c.JSON(http.StatusOK, gin.H{
			"msg": "logout successfully",
		})
		return
	}
	_ = common.DeleteUserSession(c)

	c.JSON(http.StatusOK, gin.H{
		"msg": "logout successfully",
	})
}

// change user password
//
// @Summary change user password
// @Description ChangePwd register user
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param body body types.ChangeUserPwdRequest true "修改密码"
// @success 200 {object} types.JSONResult{data=types.UserInfoResp} "desc"
// @Router /user/changePwd [post]
func changePwd(c *gin.Context) {
	value, exists := c.Get("user")
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "please login or authenticate first"})
		return
	}
	user := value.(*models.User) //nolint
	// 解析请求参数
	var req types.ChangeUserPwdRequest
	if err := c.ShouldBindWith(&req, binding.JSON); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// 修改密码
	err := user.UpdatePwd(req.NewPassword)
	if err != nil {
		log.WithFunc("changePwd").Error(c, err, "Failed to change user password")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change user password"})
		return
	}

	_ = common.DeleteUserSession(c)

	resp := types.UserInfoResp{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Nickname: user.Nickname,
	}
	c.JSON(http.StatusCreated, gin.H{
		"msg":  "changed successfully",
		"data": resp,
	})
}

// reset user password
//
// @Summary reset user password
// @Description ResetPwd resrt user password
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param body body types.ResetUserPwdRequest true "重置密码"
// @success 200 {object} types.JSONResult{data=types.UserInfoResp} "desc"
// @Router /user/resetPwd [post]
func resetPwd(c *gin.Context) {
	logger := log.WithFunc("resetPwd")
	// 解析请求参数
	var req types.ResetUserPwdRequest
	if err := c.ShouldBindWith(&req, binding.JSON); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := req.Check(); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := models.GetUser(c, req.Phone)
	if err != nil {
		logger.Errorf(c, err, "failed to query user")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}
	// 修改密码
	if err := user.UpdatePwd(req.Password); err != nil {
		logger.Error(c, err, "Failed to change user password")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change user password"})
		return
	}

	resp := convUserResp(user)
	c.JSON(http.StatusCreated, gin.H{
		"msg":  "reset password successfully",
		"data": resp,
	})
}

// GetUserToken get user token
//
// GetUserToken @Summary get token
// @Description GetUserToken get user token
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param body body types.LoginRequest true  "用户结构体"
// @success 200 {object} types.JSONResult{data=types.TokenResponse} "desc"
// @Router  /user/token [post]
func GetUserToken(c *gin.Context) {
	logger := log.WithFunc("GetUserToken")
	// check request params
	var req types.LoginRequest
	if err := c.ShouldBindWith(&req, binding.JSON); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := req.Check(); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := models.CheckAndGetUser(c, req.Username, req.Password)
	// query user
	if err != nil {
		if errors.Is(err, terrors.ErrInvalidUserPass) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		} else {
			logger.Error(c, err, "failed query user from db")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	if user == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	j := common.NewJWT(config.GetCfg().JWT.SigningKey)

	// generate access token
	accessClaims := models.CustomClaims{
		ID:       user.ID,
		UserName: user.Username,
		StandardClaims: jwt.StandardClaims{
			NotBefore: time.Now().Unix(),           // signature takes effect time
			ExpiresAt: time.Now().Unix() + 60*60*2, // 2 hours later expires
			Issuer:    "eru",
			Subject:   "access",
		},
	}
	accessTokenString, err := j.CreateToken(accessClaims)

	if err != nil {
		logger.Error(c, err, "failed to sign token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sign token"})
		return
	}

	// generate refresh token
	refreshClaims := models.CustomClaims{
		ID:       user.ID,
		UserName: user.Username,
		StandardClaims: jwt.StandardClaims{
			NotBefore: time.Now().Unix(),            // signature takes effect time
			ExpiresAt: time.Now().Unix() + 60*60*24, // 24 hours later expires
			Issuer:    "eru",
			Subject:   refreshPrefix + accessTokenString,
		},
	}
	refreshTokenString, err := j.CreateToken(refreshClaims)
	if err != nil {
		logger.Error(c, err, "failed to sign token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sign token"})
		return
	}
	if err := common.SaveUserSession(c, user); err != nil {
		logger.Warnf(c, "failed to save user session: %s", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"data": types.TokenResponse{AccessToken: accessTokenString, RefreshToken: refreshTokenString},
		"msg":  "Success",
	})
}

// RefreshToken refresh token
//
// @Summary refresh token
// @Description RefreshToken refresh token
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param body body types.RefreshRequest true  "刷新Token结构体"
// @success 200 {object} types.JSONResult{data=types.TokenResponse} "desc"
// @Router  /user/refreshToken [post]
func RefreshToken(c *gin.Context) {
	// check request params
	var req types.RefreshRequest
	if err := c.ShouldBindWith(&req, binding.JSON); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	j := common.NewJWT(config.GetCfg().JWT.SigningKey)
	token, err := j.ParseToken(req.RefreshToken)
	if err != nil {
		if err == terrors.ErrTokenExpired {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "refresh token has expired",
			})
			c.Abort()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "please login",
		})
		return
	}
	subject := token.Subject
	if !strings.HasPrefix(subject, refreshPrefix) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "invalid refresh token",
		})
		return
	}
	if subject != refreshPrefix+req.AccessToken {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "invalid refresh token or access token",
		})
		return
	}

	// generate access token
	accessClaims := models.CustomClaims{
		ID:       token.ID,
		UserName: token.UserName,
		StandardClaims: jwt.StandardClaims{
			NotBefore: time.Now().Unix(),           // signature takes effect time
			ExpiresAt: time.Now().Unix() + 60*60*2, // 2 hours later expires
			Issuer:    "eru",
			Subject:   "access",
		},
	}
	accessTokenString, err := j.CreateToken(accessClaims)

	if err != nil {
		log.WithFunc("RefreshToken").Error(c, err, "failed to sign token")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to sign token"})
		return
	}

	// generate refresh token
	refreshClaims := models.CustomClaims{
		ID:       token.ID,
		UserName: token.UserName,
		StandardClaims: jwt.StandardClaims{
			NotBefore: time.Now().Unix(),            // signature takes effect time
			ExpiresAt: time.Now().Unix() + 60*60*24, // 24 hours later expires
			Issuer:    "eru",
			Subject:   refreshPrefix + accessTokenString,
		},
	}
	refreshTokenString, err := j.CreateToken(refreshClaims)
	if err != nil {
		log.WithFunc("RefreshToken").Error(c, err, "failed to sign token")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to sign token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": types.TokenResponse{AccessToken: accessTokenString, RefreshToken: refreshTokenString},
	})
}

// GetUserInfo get user info
//
// @Summary get user info
// @Description GetUserInfo get user info
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @success 200 {object} types.JSONResult{data=types.UserInfoResp} "desc"
// @Router /user/info [get]
func GetUserInfo(c *gin.Context) {
	user, exists := common.LoginUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "please login",
		})
		return
	}
	resp := convUserResp(user)
	c.JSON(http.StatusOK, gin.H{
		"data": resp,
		"msg":  "Success",
	})
}

// update user information
//
// @Summary update user information
// @Description UpdateUser updatrs user information
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param body body types.UpdateUserRequest true "重置密码"
// @success 200 {object} types.JSONResult{data=types.UserInfoResp} "desc"
// @Router /user/info [post]
func UpdateUser(c *gin.Context) {
	logger := log.WithFunc("UpdateUser")
	// 解析请求参数
	var req types.UpdateUserRequest
	if err := c.ShouldBindWith(&req, binding.JSON); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, exists := common.LoginUser(c)
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "please login"})
		return
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if err := user.Update(nil); err != nil {
		logger.Errorf(c, err, "failed to update user")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}
	resp := convUserResp(user)
	if err := common.SaveUserSession(c, user); err != nil {
		logger.Warnf(c, "failed to save user session: %s", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"msg":  "update user successfully",
		"data": resp,
	})
}

// createPrivateToken create a private token for currrent user

// @Summary create private token
// @Description CreatePrivateToken create a private token for currrent user
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param body body types.PrivateTokenRequest true  "用户结构体"
// @success 200 {object} types.JSONResult{data=models.PrivateToken} "desc"
// @Router  /user/privateToken [post]
func CreatePrivateToken(c *gin.Context) {
	var req types.PrivateTokenRequest
	if err := c.ShouldBindWith(&req, binding.JSON); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.ExpiredAt.Before(time.Now()) {
		req.ExpiredAt = time.Now().AddDate(1, 0, 0)
	}
	value, exists := c.Get("user")
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "please login"})
		return
	}
	user := value.(*models.User) //nolint

	token, err := utils.GetUniqueStr()
	if err != nil {
		log.WithFunc("CreatePrivateToken").Error(c, err, "failed to get token")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to get token"})
		return
	}
	tokenObj := models.PrivateToken{
		Name:      req.Name,
		UserID:    user.ID,
		Token:     token,
		ExpiredAt: req.ExpiredAt,
	}
	if err := tokenObj.Save(nil); err != nil {
		log.WithFunc("CreatePrivateToken").Error(c, err, "failed to save token")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to save token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": tokenObj,
	})
}

// ListPrivateToken list all private tokens of current user

// @Summary list private token
// @Description ListPrivateToken list all private tokens of current user
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @success 200 {object} types.JSONResult{data=[]models.PrivateToken} "desc"
// @Router  /user/privateTokens [GET]
func ListPrivateToken(c *gin.Context) {
	value, exists := c.Get("user")
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "please login"})
		return
	}
	user := value.(*models.User) //nolint

	tokens, err := models.QueryPrivateTokensByUser(c, user.ID)
	if err != nil {
		log.WithFunc("CreatePrivateToken").Error(c, err, "failed to save token")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to save token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": tokens,
	})
}

// DeletePrivateToken delete a private token for currrent user

// @Summary delete private token
// @Description DeletePrivateToken delete a private token for currrent user
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param Authorization header string true "token"
// @Param body body types.PrivateTokenDeleteRequest true  "用户结构体"
// @success 200 {object} types.JSONResult{msg=string} "desc"
// @Router  /user/privateToken [delete]
func DeletePrivateToken(c *gin.Context) {
	var req types.PrivateTokenDeleteRequest
	if err := c.ShouldBindWith(&req, binding.JSON); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	value, exists := c.Get("user")
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "please login"})
		return
	}
	user := value.(*models.User) //nolint
	t, err := models.GetPrivateTokenByUserAndName(user.ID, req.Name)
	if err != nil {
		log.WithFunc("DeletePrivateToken").Error(c, err, "failed to get token")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if t == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}
	if err := t.Delete(nil); err != nil {
		log.WithFunc("DeletePrivateToken").Error(c, err, "failed to delete token")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"msg": "success",
	})
}

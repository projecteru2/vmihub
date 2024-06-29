package types

import (
	"time"

	"github.com/cockroachdb/errors"
)

// RegisterRequest 定义用户登录结构体
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password" binding:"required,min=3,max=20"`
	Email    string `json:"email"`
	SMSID    string `json:"smsId" binding:"required"`
	Code     string `json:"code" binding:"required,len=6"`
	Nickname string `json:"nickname"`
}

type UpdateUserRequest struct {
	Email    string `json:"email"`
	Nickname string `json:"nickname"`
}

// ChangeUserPwdRequest 定义用户修改密码请求结构体
type ChangeUserPwdRequest struct {
	NewPassword string `json:"newPassword" binding:"required,min=3,max=20"`
}

type ResetUserPwdRequest struct {
	Phone     string `json:"phone" binding:"required"`
	Password  string `json:"password" binding:"required,min=3,max=20"`
	Password1 string `json:"password1" binding:"required,min=3,max=20"`
	SMSID     string `json:"smsId" binding:"required"`
	Code      string `json:"code" binding:"required"`
}

func (req *ResetUserPwdRequest) Check() error {
	if req.Password != req.Password1 {
		return errors.New("password not match")
	}
	return nil
}

// LoginRequest 定义用户登录结构体
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (req *LoginRequest) Check() error {
	if req.Password == "" {
		return errors.New("password should not both be empty")
	}
	if req.Username == "" {
		return errors.New("username should not be empty")
	}
	return nil
}

// RefreshRequest 定义刷新Token结构体
type RefreshRequest struct {
	AccessToken  string `json:"accessToken" binding:"required"`
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type UserInfoResp struct {
	ID       int64  `json:"id"`
	Username string `json:"username" binding:"required,min=1,max=20"`
	Nickname string `json:"nickname" binding:"required,min=1,max=20"`
	Email    string `json:"email" binding:"required,email"`
	IsAdmin  bool   `json:"isAdmin"`
	Type     string `json:"type"`
}

type TokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type PrivateTokenRequest struct {
	Name      string    `json:"name" binding:"required,min=1,max=20" example:"my-token"`
	ExpiredAt time.Time `json:"expiredAt" example:"RFC3339: 2023-11-30T14:30:00.123+08:00"`
}

type PrivateTokenDeleteRequest struct {
	Name string `json:"name" binding:"required,min=1,max=20"`
}

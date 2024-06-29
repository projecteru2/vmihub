package models

import "github.com/dgrijalva/jwt-go"

// CustomClaims 加解密需要生成的结构
type CustomClaims struct {
	ID       int64
	UserName string
	// AuthorityId uint // 角色认证ID
	jwt.StandardClaims
}

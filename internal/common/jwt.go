package common

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/projecteru2/vmihub/internal/models"
	"github.com/projecteru2/vmihub/pkg/terrors"
)

type JWT struct {
	SigningKey []byte
}

func NewJWT(key string) *JWT {
	return &JWT{
		[]byte(key),
	}
}

// CreateToken  create token
func (j *JWT) CreateToken(claims models.CustomClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.SigningKey)
}

// ParseToken parse token
func (j *JWT) ParseToken(tokenString string) (*models.CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &models.CustomClaims{}, func(_ *jwt.Token) (any, error) {
		return j.SigningKey, nil
	})
	if err != nil { //nolint:nolintlint,nestif
		if ve, ok := err.(*jwt.ValidationError); ok {
			switch {
			case ve.Errors&jwt.ValidationErrorMalformed != 0:
				return nil, terrors.ErrTokenMalformed
			case ve.Errors&jwt.ValidationErrorExpired != 0:
				return nil, terrors.ErrTokenExpired
			case ve.Errors&jwt.ValidationErrorNotValidYet != 0:
				return nil, terrors.ErrTokenNotValidYet
			default:
				return nil, terrors.ErrTokenInvalid
			}
		}
	}
	if token != nil {
		if claims, ok := token.Claims.(*models.CustomClaims); ok && token.Valid {
			return claims, nil
		}
		return nil, terrors.ErrTokenInvalid
	}
	return nil, terrors.ErrTokenInvalid
}

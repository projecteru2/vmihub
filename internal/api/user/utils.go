package user

import (
	"github.com/projecteru2/vmihub/internal/models"
	"github.com/projecteru2/vmihub/pkg/types"
)

func convUserResp(u *models.User) *types.UserInfoResp {
	return &types.UserInfoResp{
		ID:       u.ID,
		Username: u.Username,
		IsAdmin:  u.Admin,
		Email:    u.Email,
		Nickname: u.Nickname,
	}
}

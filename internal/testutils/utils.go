package testutils

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/vmihub/config"
	"github.com/projecteru2/vmihub/internal/middlewares"
	"github.com/projecteru2/vmihub/internal/models"
	storFact "github.com/projecteru2/vmihub/internal/storage/factory"
	storageMocks "github.com/projecteru2/vmihub/internal/storage/mocks"
	"github.com/projecteru2/vmihub/internal/utils"
	"github.com/projecteru2/vmihub/internal/utils/redissession"
)

func Prepare(ctx context.Context, t *testing.T) error {
	cfg, err := config.LoadTestConfig()
	if err != nil {
		return err
	}
	logCfg := &types.ServerLogConfig{
		Level:      cfg.Log.Level,
		UseJSON:    cfg.Log.UseJSON,
		Filename:   cfg.Log.Filename,
		MaxSize:    cfg.Log.MaxSize,
		MaxAge:     cfg.Log.MaxAge,
		MaxBackups: cfg.Log.MaxBackups,
	}
	if err := log.SetupLog(ctx, logCfg, cfg.Log.SentryDSN); err != nil {
		return fmt.Errorf("Can't initialize log: %w", err)
	}
	if err := models.Init(&cfg.Mysql, t); err != nil {
		return err
	}
	cfg.Storage.Type = "mock"
	if _, err := storFact.Init(&cfg.Storage); err != nil {
		return err
	}

	utils.SetupRedis(&cfg.Redis, t)
	return nil
}

func PrepareGinEngine() (*gin.Engine, error) {
	r := gin.New()
	redisCli := utils.GetRedisConn()
	sessStor, err := redissession.NewStore(context.TODO(), redisCli)
	if err != nil {
		return nil, err
	}

	r.Use(sessions.Sessions("mysession", sessStor))
	r.Use(middlewares.Cors())
	r.Use(middlewares.Logger("vmihub"))
	return r, nil
}

func PrepareUserData(username, passwd string) error {
	ePasswd, err := utils.EncryptPassword(passwd)
	if err != nil {
		return err
	}

	tblName := ((*models.User)(nil)).TableName()
	columes := ((*models.User)(nil)).ColumnNames()

	wantRows := sqlmock.NewRows([]string{"id", "username", "password"}).
		AddRow(1, username, ePasswd)
	models.Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ?", columes, tblName)).
		WithArgs(username).
		WillReturnRows(wantRows)
	return nil
}

func GetMockStorage() *storageMocks.Storage {
	sto := storFact.Instance()
	return sto.(*storageMocks.Storage)
}

func AddAuth(req *http.Request, username, password string) {
	val := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", val))
}

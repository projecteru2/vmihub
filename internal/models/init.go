package models

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/go-sql-driver/mysql" // ignore lint error
	"github.com/jmoiron/sqlx"
	"github.com/projecteru2/vmihub/config"
)

var (
	db   *sqlx.DB
	Mock sqlmock.Sqlmock
)

type Labels map[string]string

func Init(cfg *config.MysqlConfig, t *testing.T) (err error) {
	if t != nil {
		sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			return err
		}
		db = sqlx.NewDb(sqlDB, "sqlmock")
		Mock = mock
		return nil
	}
	db, err = sqlx.Open("mysql", cfg.DSN)
	if err != nil {
		return
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	return
}

func Instance() *sqlx.DB {
	return db
}

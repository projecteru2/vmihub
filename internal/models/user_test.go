package models

import (
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/projecteru2/vmihub/internal/utils"
	"github.com/stretchr/testify/assert"
)

var (
	tableName = ((*User)(nil)).TableName()
	columns   = ((*User)(nil)).ColumnNames()
)

func TestGetUser(t *testing.T) {
	utils.SetupRedis(nil, t)
	err := Init(nil, t)
	assert.Nil(t, err)

	defer func() {
		err = Mock.ExpectationsWereMet()
		assert.Nil(t, err)
	}()

	username := "user1"
	passwd := "passwd1"
	ePasswd, err := utils.EncryptPassword(passwd)
	assert.Nil(t, err)

	wantRows := sqlmock.NewRows([]string{"id", "username", "password"}).
		AddRow(1, username, ePasswd)

	Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ?", columns, tableName)).
		WithArgs(username).
		WillReturnRows(wantRows)
	user, err := GetUser(context.Background(), username)
	assert.Nil(t, err)
	assert.Equal(t, username, user.Username)

}

func TestCheckAndGetUser(t *testing.T) {
	utils.SetupRedis(nil, t)
	err := Init(nil, t)
	assert.Nil(t, err)

	defer func() {
		err = Mock.ExpectationsWereMet()
		assert.Nil(t, err)
	}()

	username := "user1"
	passwd := "passwd1"
	ePasswd, err := utils.EncryptPassword(passwd)
	assert.Nil(t, err)

	wantRows := sqlmock.NewRows([]string{"id", "username", "password"}).
		AddRow(1, username, ePasswd)
	Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ?", columns, tableName)).
		WithArgs(username).
		WillReturnRows(wantRows)
	user2, err := CheckAndGetUser(context.Background(), username, passwd)
	assert.Nil(t, err)
	assert.Equal(t, user2.Username, username)
}

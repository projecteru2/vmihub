package models

import (
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/projecteru2/vmihub/internal/utils"
	"github.com/projecteru2/vmihub/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestQueryRepo(t *testing.T) {
	utils.SetupRedis(nil, t)
	err := Init(nil, t)
	assert.Nil(t, err)
	defer func() {
		err = Mock.ExpectationsWereMet()
		assert.Nil(t, err)
	}()

	tableName := ((*Repository)(nil)).TableName()
	columns := ((*Repository)(nil)).ColumnNames()
	{
		// empty result
		Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? ORDER BY updated_at DESC LIMIT ?, ?", columns, tableName)).
			WithArgs("user2", 0, 10).
			WillReturnRows(sqlmock.NewRows([]string{"id", "username", "name"}))

		repos, err := QueryRepoList("user2", 1, 10)
		assert.Nil(t, err)
		assert.Len(t, repos, 0)
	}
	{
		wantRows := sqlmock.NewRows([]string{"id", "username", "name"}).
			AddRow(1, "user1", "name1").
			AddRow(1, "user1", "name2")
		Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? ORDER BY updated_at DESC LIMIT ?, ?", columns, tableName)).
			WithArgs("user1", 0, 10).
			WillReturnRows(wantRows)
		repos, err := QueryRepoList("user1", 1, 10)
		assert.Nil(t, err)
		assert.Len(t, repos, 2)
	}
	{
		wantRows := sqlmock.NewRows([]string{"id", "username", "name"}).
			AddRow(1, "user1", "name2")
		Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", columns, tableName)).
			WithArgs("user1", "name2").
			WillReturnRows(wantRows)
		repo, err := QueryRepo(context.Background(), "user1", "name2")
		assert.Nil(t, err)
		assert.Equal(t, "name2", repo.Name)
	}
}

func TestQueryRepoCache(t *testing.T) {
	utils.SetupRedis(nil, t)
	err := Init(nil, t)
	assert.Nil(t, err)
	defer func() {
		err = Mock.ExpectationsWereMet()
		assert.Nil(t, err)
	}()

	tableName := ((*Repository)(nil)).TableName()
	columns := ((*Repository)(nil)).ColumnNames()

	{
		wantRows := sqlmock.NewRows([]string{"id", "username", "name"}).
			AddRow(1, "user1", "name2")
		Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", columns, tableName)).
			WithArgs("user1", "name2").
			WillReturnRows(wantRows)
		repo, err := QueryRepo(context.Background(), "user1", "name2")
		assert.Nil(t, err)
		assert.Equal(t, "name2", repo.Name)
	}
	{
		repo, err := QueryRepo(context.Background(), "user1", "name2")
		assert.Nil(t, err)
		assert.Equal(t, "name2", repo.Name)
	}
}
func TestGetImages(t *testing.T) {
	utils.SetupRedis(nil, t)
	err := Init(nil, t)
	assert.Nil(t, err)
	defer func() {
		err = Mock.ExpectationsWereMet()
		assert.Nil(t, err)
	}()

	tableName := ((*Image)(nil)).TableName()
	columns := ((*Image)(nil)).ColumnNames()

	repo := *&Repository{
		ID:       1,
		Username: "user1",
		Name:     "name1",
	}

	{
		utils.MockRedis.FlushAll()
		wantRows := sqlmock.NewRows([]string{"id", "repo_id", "tag"}).
			AddRow(1, 1, "tag1").
			AddRow(2, 1, "tag2")
		Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE repo_id = ? ORDER BY updated_at DESC", columns, tableName)).
			WithArgs(1).
			WillReturnRows(wantRows)
		images, err := repo.GetImages()
		assert.Nil(t, err)
		assert.Len(t, images, 2)
	}

	{
		utils.MockRedis.FlushAll()
		wantRows := sqlmock.NewRows([]string{"id", "repo_id", "tag"}).
			AddRow(2, 1, "tag2")
		Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE repo_id = ? AND tag = ?", columns, tableName)).
			WithArgs(1, "tag2").
			WillReturnRows(wantRows)
		img, err := repo.GetImage(context.Background(), "tag2")
		assert.Nil(t, err)
		assert.NotNil(t, img)
		assert.Equal(t, "tag2", img.Tag)
	}
}

func TestGetImageCache(t *testing.T) {
	utils.SetupRedis(nil, t)
	err := Init(nil, t)
	assert.Nil(t, err)
	defer func() {
		err = Mock.ExpectationsWereMet()
		assert.Nil(t, err)
	}()

	tableName := ((*Image)(nil)).TableName()
	columns := ((*Image)(nil)).ColumnNames()

	repo := *&Repository{
		ID:       1,
		Username: "user1",
		Name:     "name1",
	}
	{
		wantRows := sqlmock.NewRows([]string{"id", "repo_id", "tag"}).
			AddRow(2, 1, "tag2")
		Mock.ExpectQuery(fmt.Sprintf("SELECT %s FROM %s WHERE repo_id = ? AND tag = ?", columns, tableName)).
			WithArgs(1, "tag2").
			WillReturnRows(wantRows)
		img, err := repo.GetImage(context.Background(), "tag2")
		assert.Nil(t, err)
		assert.NotNil(t, img)
		assert.Equal(t, "tag2", img.Tag)
	}
	{
		img, err := repo.GetImage(context.Background(), "tag2")
		assert.Nil(t, err)
		assert.NotNil(t, img)
		assert.Equal(t, "tag2", img.Tag)
	}
}

func TestSaveRepo(t *testing.T) {
	utils.SetupRedis(nil, t)
	err := Init(nil, t)
	assert.Nil(t, err)
	defer func() {
		err = Mock.ExpectationsWereMet()
		assert.Nil(t, err)
	}()
	tableName := "repository"

	repo := &Repository{
		Username: "user1",
		Name:     "name1",
	}
	Mock.ExpectBegin()
	Mock.ExpectExec(fmt.Sprintf("INSERT INTO %s(username, name, private) VALUES(?, ?, ?)", tableName)).
		WithArgs(repo.Username, repo.Name, repo.Private).
		WillReturnResult(sqlmock.NewResult(1234, 1))
	tx, err := db.Beginx()
	assert.Nil(t, err)

	err = repo.Save(tx)
	assert.Nil(t, err)
	assert.Equal(t, int64(1234), repo.ID)

}

func TestSaveImage(t *testing.T) {
	utils.SetupRedis(nil, t)
	err := Init(nil, t)
	assert.Nil(t, err)
	defer func() {
		err = Mock.ExpectationsWereMet()
		assert.Nil(t, err)
	}()
	tableName := ((*Image)(nil)).TableName()

	repo := &Repository{
		ID:       2323,
		Username: "user1",
		Name:     "name1",
	}
	img := &Image{
		Tag:    "latest",
		Labels: NewJSONColumn(&Labels{}),
		Size:   12345,
		Digest: "12345",
		Format: "qcow2",
		OS: NewJSONColumn(&types.OSInfo{
			Type:    "linux",
			Distrib: "ubuntu",
		}),
	}
	osVal, err := img.OS.Value()
	assert.Nil(t, err)
	Mock.ExpectBegin()
	Mock.ExpectExec(fmt.Sprintf("INSERT INTO %s(repo_id, tag, labels, size, format, os, digest, snapshot, description) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)", tableName)).
		WithArgs(repo.ID, img.Tag, sqlmock.AnyArg(), img.Size, img.Format, osVal, img.Digest, img.Snapshot, img.Description).
		WillReturnResult(sqlmock.NewResult(1234, 1))
	tx, err := db.Beginx()
	assert.Nil(t, err)

	err = repo.SaveImage(tx, img)
	assert.Nil(t, err)
	assert.Equal(t, int64(1234), img.ID)

}

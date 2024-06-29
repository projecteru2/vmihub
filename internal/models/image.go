package models

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/duke-git/lancet/strutil"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/projecteru2/vmihub/internal/utils"
	"github.com/projecteru2/vmihub/pkg/types"
	pkgutils "github.com/projecteru2/vmihub/pkg/utils"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
)

const (
	ImageFormatQcow2 = "qcow2"
	ImageFormatRaw   = "raw"
	ImageFormatRBD   = "rbd"
)

type Repository struct {
	ID        int64     `db:"id" json:"id"`
	Username  string    `db:"username" json:"username" description:"image's username"`
	Name      string    `db:"name" json:"name" description:"image name"`
	Private   bool      `db:"private" json:"private" description:"image is private"`
	CreatedAt time.Time `db:"created_at" json:"createdAt" description:"image create time"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt" description:"image update time"`
	Images    []Image   `db:"-" json:"-"`
}

func (*Repository) TableName() string {
	return "repository"
}

func (repo *Repository) ColumnNames() string {
	names := GetColumnNames(repo)
	return strings.Join(names, ", ")
}

type Image struct {
	ID          int64                    `db:"id" json:"id"`
	RepoID      int64                    `db:"repo_id" json:"repoId"`
	Tag         string                   `db:"tag" json:"tag" description:"image tag, default:latest"`
	Labels      JSONColumn[Labels]       `db:"labels" json:"labels"`
	Size        int64                    `db:"size" json:"size" description:"actual file size(in bytes)"`
	VirtualSize int64                    `db:"virtual_size" json:"virtualSize" description:"virtual size of image file"`
	Digest      string                   `db:"digest" json:"digest" description:"image digest"`
	Format      string                   `db:"format" json:"format" description:"image format"`
	OS          JSONColumn[types.OSInfo] `db:"os" json:"os"`
	Snapshot    string                   `db:"snapshot" json:"snapshot" description:"RBD Snapshot for this image, eg: eru/ubuntu-18.04@v1"`
	Description string                   `db:"description" json:"description" description:"image description"`
	CreatedAt   time.Time                `db:"created_at" json:"createdAt" description:"image create time"`
	UpdatedAt   time.Time                `db:"updated_at" json:"updatedAt" description:"image update time"`
	Repo        *Repository              `db:"-" json:"repo"`
}

func (*Image) TableName() string {
	return "image"
}

func (img *Image) ColumnNames() string {
	names := GetColumnNames(img)
	return strings.Join(names, ", ")
}

func (repo *Repository) Fullname() string {
	return fmt.Sprintf("%s/%s", repo.Username, repo.Name)
}

func (img *Image) Fullname() string {
	return fmt.Sprintf("%s/%s:%s", img.Repo.Username, img.Repo.Name, img.Tag)
}

func (img *Image) NormalizeName() string {
	if img.Repo.Username == "_" {
		return fmt.Sprintf("%s:%s", img.Repo.Name, img.Tag)
	}
	return fmt.Sprintf("%s/%s:%s", img.Repo.Username, img.Repo.Name, img.Tag)
}

func (img *Image) SliceName() string {
	return fmt.Sprintf("%s/_slice_%s:%s", img.Repo.Username, img.Repo.Name, img.Tag)
}

func (img *Image) GetRepo() (*Repository, error) {
	var err error
	if img.Repo == nil {
		img.Repo, err = QueryRepoByID(context.TODO(), img.RepoID)
	}
	return img.Repo, err
}

func (repo *Repository) GetImages() ([]Image, error) {
	var err error

	tblName := ((*Image)(nil)).TableName()
	columns := ((*Image)(nil)).ColumnNames()
	sqlStr := fmt.Sprintf("SELECT %s FROM %s WHERE repo_id = ? ORDER BY updated_at DESC", columns, tblName)
	err = db.Select(&repo.Images, sqlStr, repo.ID)
	if err != nil {
		return nil, err
	}
	for idx := 0; idx < len(repo.Images); idx++ {
		repo.Images[idx].Repo = repo
	}
	return repo.Images, nil
}

func (repo *Repository) GetImage(ctx context.Context, tag string) (*Image, error) {
	var (
		img *Image
		err error
	)
	if !utils.IsDefaultTag(tag) {
		if img, err = getImageFromRedis(ctx, repo, tag); err != nil {
			return nil, err
		}
	}
	if img != nil {
		return img, nil
	}
	img = &Image{}
	tblName := ((*Image)(nil)).TableName()
	columns := ((*Image)(nil)).ColumnNames()
	if utils.IsDefaultTag(tag) {
		sqlStr := fmt.Sprintf("SELECT %s FROM %s WHERE repo_id = ? ORDER BY created_at DESC LIMIT 1", columns, tblName)
		err = db.Get(img, sqlStr, repo.ID)
	} else {
		sqlStr := fmt.Sprintf("SELECT %s FROM %s WHERE repo_id = ? AND tag = ?", columns, tblName)
		err = db.Get(img, sqlStr, repo.ID, tag)
	}

	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, err
	}
	img.Repo = repo
	if err = setImageToRedis(ctx, img); err != nil {
		return nil, err
	}
	return img, nil
}

func (repo *Repository) Delete(tx *sqlx.Tx) (err error) {
	if tx == nil {
		tx, _ = db.Beginx()
		defer func() {
			if err == nil {
				_ = tx.Commit()
			}
		}()
	}
	defer func() {
		if err == nil {
			_ = deleteRepoInRedis(context.TODO(), repo)
		}
	}()
	// delete images belongs to this repository
	sqlStr := "DELETE FROM image WHERE repo_id = ?"

	if _, err := tx.Exec(sqlStr, repo.ID); err != nil {
		_ = tx.Rollback()
		return errors.Wrapf(err, "falid to delete images")
	}
	sqlStr = "DELETE FROM repository WHERE id = ?"
	_, err = tx.Exec(sqlStr, repo.ID)
	if err != nil {
		tx.Rollback() //nolint:errcheck
		return errors.Wrapf(err, "falid to delete image")
	}
	return nil
}

func (repo *Repository) DeleteImage(tx *sqlx.Tx, tag string) (err error) {
	if tx == nil {
		tx, _ = db.Beginx()
		defer func() {
			if err == nil {
				_ = tx.Commit()
			}
		}()
	}
	defer func() {
		if err == nil {
			_ = deleteImageInRedis(context.TODO(), repo, tag)
		}
	}()
	// delete tags
	sqlStr := "DELETE FROM image WHERE repo_id = ? AND tag = ?"

	if _, err := tx.Exec(sqlStr, repo.ID, tag); err != nil {
		_ = tx.Rollback()
		return errors.Wrapf(err, "falid to delete image")
	}
	return nil
}

// only save image itself, don't save tags
func (repo *Repository) Save(tx *sqlx.Tx) (err error) {
	if tx == nil {
		tx, _ = db.Beginx()
		defer func() {
			if err == nil {
				_ = tx.Commit()
			}
		}()
	}
	defer func() {
		if err == nil {
			_ = deleteRepoInRedis(context.TODO(), repo)
		}
	}()
	var sqlRes sql.Result
	if repo.ID > 0 {
		sqlStr := "UPDATE repository SET private = ? WHERE username = ? and name = ?"
		_, err = tx.Exec(sqlStr, repo.Private, repo.Username, repo.Name)
		if err != nil {
			_ = tx.Rollback()
			return errors.Wrapf(err, "failed to update repository: %v", repo)
		}
	} else {
		sqlStr := "INSERT INTO repository(username, name, private) VALUES(?, ?, ?)"
		sqlRes, err = tx.Exec(sqlStr, repo.Username, repo.Name, repo.Private)
		if err != nil {
			_ = tx.Rollback()
			return errors.Wrapf(err, "failed to insert repository: %v", repo)
		}
		// fetch image id
		repo.ID, err = sqlRes.LastInsertId()
		if err != nil { //nolint
			// TODO query the new record
		}
	}
	return
}

func (repo *Repository) SaveImage(tx *sqlx.Tx, img *Image) (err error) {
	if tx == nil {
		tx, _ = db.Beginx()
		defer func() {
			if err == nil {
				_ = tx.Commit()
			}
		}()
	}
	defer func() {
		if err == nil {
			_ = deleteImageInRedis(context.TODO(), repo, img.Tag)
		}
	}()
	labels, err := img.Labels.Value()
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	var sqlRes sql.Result
	if img.ID > 0 { //nolint
		sqlStr := "UPDATE image SET digest = ?, size=?, snapshot=? WHERE id = ?"
		_, err = tx.Exec(sqlStr, img.Digest, img.Size, img.Snapshot, img.ID)
		if err != nil {
			_ = tx.Rollback()
			return errors.Wrapf(err, "failed to update image: %v", img)
		}
	} else {
		osVal, err := img.OS.Value()
		if err != nil {
			_ = tx.Rollback()
			return errors.Wrapf(err, "failed to insert image: %v", img)
		}
		sqlStr := "INSERT INTO image(repo_id, tag, labels, size, format, os, digest, snapshot, description) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)"
		img.RepoID = repo.ID
		sqlRes, err = tx.Exec(sqlStr, img.RepoID, img.Tag, labels, img.Size, img.Format, osVal, img.Digest, img.Snapshot, img.Description)
		if err != nil {
			_ = tx.Rollback()
			return errors.Wrapf(err, "failed to insert image: %v", img)
		}
		img.ID, err = sqlRes.LastInsertId()
		if err != nil { //nolint
			// TODO query the new record
		}
	}
	return nil
}

func QueryRepoList(user string, pNum, pSize int) (ans []Repository, err error) {
	tblName := ((*Repository)(nil)).TableName()
	columns := ((*Repository)(nil)).ColumnNames()
	offset := (pNum - 1) * pSize
	sqlStr := fmt.Sprintf("SELECT %s FROM %s WHERE username = ? ORDER BY updated_at DESC LIMIT ?, ?", columns, tblName)
	err = db.Select(&ans, sqlStr, user, offset, pSize)
	if err != nil {
		return
	}
	return
}

func QueryPublicRepoList(user string, pNum, pSize int) (ans []Repository, err error) {
	tblName := ((*Repository)(nil)).TableName()
	columns := ((*Repository)(nil)).ColumnNames()
	offset := (pNum - 1) * pSize
	sqlStr := fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND private = ? ORDER BY updated_at DESC LIMIT ?, ?", columns, tblName)
	err = db.Select(&ans, sqlStr, user, false, offset, pSize)
	if err != nil {
		return
	}
	return
}

/*
QueryRepo get image info
@Param username string false "用户名"
@Param name string true "镜像名"
@Param tag string true "镜像标签"
*/
func QueryRepo(ctx context.Context, username, name string) (*Repository, error) {
	tblName := ((*Repository)(nil)).TableName()
	columns := ((*Repository)(nil)).ColumnNames()
	repo, err := getRepoFromRedis(ctx, username, name)
	if err != nil {
		return nil, err
	}
	if repo != nil {
		return repo, nil
	}
	repo = &Repository{}
	sqlStr := fmt.Sprintf("SELECT %s FROM %s WHERE username = ? AND name = ?", columns, tblName)
	err = db.Get(repo, sqlStr, username, name)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, err
	}
	if err = setRepoToRedis(ctx, repo); err != nil {
		return nil, err
	}
	return repo, nil
}

func QueryRepoByID(ctx context.Context, id int64) (*Repository, error) {
	tblName := ((*Repository)(nil)).TableName()
	columns := ((*Repository)(nil)).ColumnNames()
	repo := &Repository{}
	sqlStr := fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", columns, tblName)

	err := db.Get(repo, sqlStr, id)

	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, err
	}
	if err = setRepoToRedis(ctx, repo); err != nil {
		return nil, err
	}
	return repo, nil
}

func QueryImagesByRepoIDs(repoIDs []int64, keyword string, pNum, pSize int) (ans []Image, count int, err error) {
	if len(repoIDs) == 0 {
		return nil, 0, nil
	}
	offset := (pNum - 1) * pSize
	repoSIDs := lo.Map(repoIDs, func(id int64, _ int) string {
		return strconv.FormatInt(id, 10)
	})
	sqlStr := "SELECT * FROM image WHERE repo_id IN (?) AND name like CONCAT('%', CONCAT(?, '%')) ORDER BY updated_at DESC LIMIT ?, ?"
	idsStr := strings.Join(repoSIDs, ",")
	err = db.Select(&ans, sqlStr, idsStr, keyword, offset, pSize)
	if err != nil {
		return
	}
	sqlStr = "SELECT count(*) FROM image WHERE repo_id IN (?) AND name like CONCAT('%', CONCAT(?, '%'))"
	row := db.QueryRow(sqlStr, idsStr, keyword)
	if err = row.Scan(&count); err != nil {
		return
	}
	return
}

type combainResult struct {
	Image
	Username string `db:"username" json:"username"`
	Name     string `db:"name" json:"name"`
	Private  bool   `db:"private" json:"private"`
}

func QueryImagesByUsername(req types.ImagesByUsernameRequest) (ans []Image, count int, err error) {
	var rows *sqlx.Rows
	var sRow *sql.Row
	offset := (req.PageNum - 1) * req.PageSize
	sqlStr := `SELECT i.id, i.repo_id, r.username, r.name, r.private, i.tag, i.size, i.digest, i.format, i.os, 
	                  i.snapshot, i.description, i.created_at, i.updated_at, i.labels, i.region_code
	           FROM image i, repository r 
			   WHERE r.id=i.repo_id AND 
			         r.username=? AND
			         r.name like CONCAT('%', CONCAT(?, '%')) 
			   ORDER BY i.updated_at DESC LIMIT ?, ?`
	if strutil.IsBlank(req.RegionCode) {
		rows, err = db.Queryx(sqlStr, req.Username, req.Keyword, offset, req.PageSize)
	} else {
		sqlStr = `SELECT i.id, i.repo_id, r.username, r.name, r.private, i.tag, i.size, i.digest, i.format, i.os, 
		i.snapshot, i.description, i.created_at, i.updated_at, i.labels, i.region_code 
 FROM image i, repository r 
 WHERE r.id=i.repo_id AND 
	   r.username=? AND
	   r.name like CONCAT('%', CONCAT(?, '%')) AND
	   r.region_code=? AND
	   i.region_code=?
 ORDER BY i.updated_at DESC LIMIT ?, ?`
		rows, err = db.Queryx(sqlStr, req.Username, req.Keyword, req.RegionCode, req.RegionCode, offset, req.PageSize)
	}
	if err != nil {
		return nil, 0, err
	}
	for rows.Next() {
		var res combainResult
		if err = rows.StructScan(&res); err != nil {
			return nil, 0, err
		}
		res.Image.Repo = &Repository{
			Username: res.Username,
			Name:     res.Name,
			Private:  res.Private,
		}
		ans = append(ans, res.Image)
	}
	sqlStr = `SELECT count(*) 
	           FROM image i, repository r 
			   WHERE r.id=i.repo_id AND 
			         r.username=? AND
			         r.name like CONCAT('%', CONCAT(?, '%')) 
			   `
	if strutil.IsBlank(req.RegionCode) {
		sRow = db.QueryRow(sqlStr, req.Username, req.Keyword)
	} else {
		sqlStr = `SELECT count(*) 
		FROM image i, repository r 
		WHERE r.id=i.repo_id AND 
			  r.username=? AND
			  r.name like CONCAT('%', CONCAT(?, '%')) AND
			  r.region_code=? AND
	   		  i.region_code=?
		`
		sRow = db.QueryRow(sqlStr, req.Username, req.Keyword, req.RegionCode, req.RegionCode)
	}
	if err = sRow.Scan(&count); err != nil {
		return nil, 0, err
	}
	return ans, count, nil

}

func QueryPublicImagesByUsername(req types.ImagesByUsernameRequest) (ans []Image, count int, err error) {
	var rows *sqlx.Rows
	var sRow *sql.Row
	offset := (req.PageNum - 1) * req.PageSize
	sqlStr := `SELECT i.id, i.repo_id, r.username, r.name, i.tag, i.size, i.digest, i.format, i.os, 
	                  i.snapshot, i.description, i.created_at, i.updated_at, i.labels, i.region_code
	           FROM image i, repository r 
			   WHERE r.id=i.repo_id AND 
			         r.username=? AND
					 r.private=0 AND
			         r.name like CONCAT('%', CONCAT(?, '%')) 
			   ORDER BY i.updated_at DESC LIMIT ?, ?`
	if strutil.IsBlank(req.RegionCode) {
		rows, err = db.Queryx(sqlStr, req.Username, req.Keyword, offset, req.PageSize)
	} else {
		sqlStr = `SELECT i.id, i.repo_id, r.username, r.name, i.tag, i.size, i.digest, i.format, i.os, 
				i.snapshot, i.description, i.created_at, i.updated_at, i.labels,i.region_code
		FROM image i, repository r 
		WHERE r.id=i.repo_id AND 
			r.username=? AND
			r.private=0 AND
			r.name like CONCAT('%', CONCAT(?, '%')) AND
			r.region_code=? AND
			i.region_code=?
		ORDER BY i.updated_at DESC LIMIT ?, ?`
		rows, err = db.Queryx(sqlStr, req.Username, req.Keyword, req.RegionCode, req.RegionCode, offset, req.PageSize)
	}
	if err != nil {
		return nil, 0, err
	}
	for rows.Next() {
		var res combainResult
		if err = rows.StructScan(&res); err != nil {
			return nil, 0, err
		}
		res.Image.Repo = &Repository{
			Username: res.Username,
			Name:     res.Name,
			Private:  res.Private,
		}
		ans = append(ans, res.Image)
	}
	sqlStr = `SELECT count(*) 
	           FROM image i, repository r 
			   WHERE r.id=i.repo_id AND 
			         r.username=? AND
					 r.private=0 AND
			         r.name like CONCAT('%', CONCAT(?, '%')) 
			   `
	if strutil.IsBlank(req.RegionCode) {
		sRow = db.QueryRow(sqlStr, req.Username, req.Keyword)
	} else {
		sqlStr = `SELECT count(*) 
		FROM image i, repository r 
		WHERE r.id=i.repo_id AND 
			  r.username=? AND
			  r.private=0 AND
			  r.name like CONCAT('%', CONCAT(?, '%')) AND
			  r.region_code=? AND
	   		  i.region_code=?
		`
		sRow = db.QueryRow(sqlStr, req.Username, req.Keyword, req.RegionCode, req.RegionCode)
	}
	if err = sRow.Scan(&count); err != nil {
		return nil, 0, err
	}
	return ans, count, nil

}

func getRepoFromRedis(ctx context.Context, username, name string) (repo *Repository, err error) {
	rKey := fmt.Sprintf(redistRepoKey, username, name)
	repo = &Repository{}
	err = utils.GetObjFromRedis(ctx, rKey, repo)
	if err == redis.Nil {
		return nil, nil //nolint
	}
	return
}

func setRepoToRedis(ctx context.Context, repo *Repository) error {
	rKey := fmt.Sprintf(redistRepoKey, repo.Username, repo.Name)
	return utils.SetObjToRedis(ctx, rKey, repo, 10*time.Minute)
}

func deleteRepoInRedis(ctx context.Context, repo *Repository) error {
	rKey := fmt.Sprintf(redistRepoKey, repo.Username, repo.Name)
	return utils.DeleteObjectsInRedis(ctx, rKey)
}

func getImageFromRedis(ctx context.Context, repo *Repository, tag string) (img *Image, err error) {
	rKey := fmt.Sprintf(redisImageKey, repo.Username, repo.Name, tag)
	img = &Image{}
	err = utils.GetObjFromRedis(ctx, rKey, img)
	if err == redis.Nil {
		return nil, nil //nolint
	}
	return
}

func setImageToRedis(ctx context.Context, img *Image) error {
	rKey := fmt.Sprintf(redisImageKey, img.Repo.Username, img.Repo.Name, img.Tag)
	return utils.SetObjToRedis(ctx, rKey, img, 10*time.Minute)
}

func deleteImageInRedis(ctx context.Context, repo *Repository, tag string) error {
	rKey := fmt.Sprintf(redisImageKey, repo.Username, repo.Name, tag)
	return utils.DeleteObjectsInRedis(ctx, rKey)
}

func GetPublicImages(_ context.Context) ([]Image, error) {
	images := make([]Image, 0)
	repos := make([]*Repository, 0)
	tblName := ((*Repository)(nil)).TableName()
	columns := ((*Repository)(nil)).ColumnNames()
	sqlStr := fmt.Sprintf("SELECT %s FROM %s WHERE private = ? ORDER BY updated_at DESC", columns, tblName)
	err := db.Select(&repos, sqlStr, false)
	if err != nil {
		return nil, err
	}
	for _, repo := range repos {
		repoImages, err := repo.GetImages()
		if err != nil {
			return nil, err
		}
		for _, image := range repoImages { //nolint
			images = append(images, image)
		}
	}
	return images, nil
}

func GetImageByFullname(ctx context.Context, fullname string) (*Image, error) {
	username, name, tag, err := pkgutils.ParseImageName(fullname)
	if err != nil {
		return nil, err
	}
	repo, err := QueryRepo(ctx, username, name)
	if err != nil || repo == nil {
		return nil, err
	}
	image, err := repo.GetImage(ctx, tag)
	if err != nil {
		return nil, err
	}

	return image, nil
}

func GetImageByID(_ context.Context, id int64) (*Image, error) {
	imgTblName := ((*Image)(nil)).TableName()
	imgColumnsName := ((*Image)(nil)).ColumnNames()
	sqlStr := fmt.Sprintf("SELECT %s FROM %s where id = ?", imgColumnsName, imgTblName)
	image := Image{}
	err := db.Get(&image, sqlStr, id)
	if err != nil {
		return nil, err
	}
	repoTblName := ((*Repository)(nil)).TableName()
	repoColumnsName := ((*Repository)(nil)).ColumnNames()
	sqlStr = fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", repoColumnsName, repoTblName)
	repo := Repository{}
	err = db.Get(&repo, sqlStr, image.RepoID)
	if err != nil {
		return nil, err
	}
	image.Repo = &repo
	return &image, nil
}

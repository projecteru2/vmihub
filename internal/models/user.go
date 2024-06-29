package models

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/jmoiron/sqlx"
	"github.com/projecteru2/vmihub/internal/utils"
	"github.com/projecteru2/vmihub/pkg/terrors"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        int64     `db:"id" json:"id"`
	Username  string    `db:"username" json:"username" description:"Login user name"`
	Password  string    `db:"password" json:"password" description:"user login password"`
	Nickname  string    `db:"nickname" json:"nickname" description:"user's nickname"`
	Email     string    `db:"email" json:"email" description:"user's email"`
	Admin     bool      `db:"admin" json:"admin" description:"is a admin"`
	CreatedAt time.Time `db:"created_at" json:"createdAt" description:"user create time"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt" description:"user update time"`
}

func (*User) TableName() string {
	return "user"
}

func (user *User) ColumnNames() string {
	names := GetColumnNames(user)
	return strings.Join(names, ", ")
}

type PrivateToken struct {
	ID        int64     `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	UserID    int64     `db:"user_id" json:"userId"`
	Token     string    `db:"token" json:"token"`
	ExpiredAt time.Time `db:"expired_at" json:"expiredAt"`
	CreatedAt time.Time `db:"created_at" json:"createdAt" description:"user create time"`
	LastUsed  time.Time `db:"last_used" json:"lastUsed"`
}

func (*PrivateToken) tableName() string {
	return "private_token"
}

func (t *PrivateToken) columnNames() string {
	names := GetColumnNames(t)
	return strings.Join(names, ", ")
}

func (t *PrivateToken) GetUser() (*User, error) {
	return GetUserByID(context.TODO(), t.UserID)
}

func (t *PrivateToken) Save(tx *sqlx.Tx) (err error) {
	if tx == nil {
		tx, _ = db.Beginx()
		defer func() {
			if err == nil {
				_ = tx.Commit()
			}
		}()
	}
	sqlStr := "INSERT INTO private_token(user_id, name, token, expired_at) VALUES(?, ?, ?, ?)"
	sqlRes, err := tx.Exec(sqlStr, t.UserID, t.Name, t.Token, t.ExpiredAt)
	if err != nil {
		_ = tx.Rollback()
		return errors.Wrapf(err, "failed to insert private token: %v", t)
	}
	// fetch guest id
	t.ID, _ = sqlRes.LastInsertId()
	return nil
}

func (t *PrivateToken) Delete(tx *sqlx.Tx) (err error) {
	if tx == nil {
		tx, _ = db.Beginx()
		defer func() {
			if err == nil {
				_ = tx.Commit()
			}
		}()
	}
	sqlStr := "DELETE FROM private_token WHERE id = ?"
	if _, err = tx.Exec(sqlStr, t.ID); err != nil {
		_ = tx.Rollback()
		return errors.Wrapf(err, "failed to delete private token: %v", t)
	}
	return
}

func (t *PrivateToken) UpdateLastUsed() error {
	sqlStr := "UPDATE private_token SET last_used = ? WHERE id = ?"
	_, err := db.Exec(sqlStr, time.Now(), t.ID)
	return err
}

func QueryPrivateTokensByUser(ctx context.Context, userID int64) (tokens []*PrivateToken, err error) {
	tblName := ((*PrivateToken)(nil)).tableName()
	columns := ((*PrivateToken)(nil)).columnNames()
	sqlStr := fmt.Sprintf("SELECT %s FROM %s WHERE user_id = ?", columns, tblName)
	err = db.SelectContext(ctx, &tokens, sqlStr, userID)
	return
}

func (user *User) Update(tx *sqlx.Tx) (err error) {
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
			deleteUserInRedis(context.TODO(), user)
		}
	}()
	sqlStr := "UPDATE user SET nickname = ?, email = ? WHERE id = ?"
	if _, err = tx.Exec(sqlStr, user.Nickname, user.Email, user.ID); err != nil {
		_ = tx.Rollback()
		return errors.Wrapf(err, "failed to update user: %v", user)
	}
	return
}

func (user *User) UpdatePwd(password string) (err error) {
	defer func() {
		if err == nil {
			deleteUserInRedis(context.TODO(), user)
		}
	}()
	ePasswd, err := utils.EncryptPassword(password)
	if err != nil {
		return err
	}
	sqlStr := "UPDATE user set password = ? where id = ?"
	if _, err = db.Exec(sqlStr, ePasswd, user.ID); err != nil {
		return err
	}
	return nil
}

func CreateUser(tx *sqlx.Tx, user *User, password string) (err error) {
	if tx == nil {
		tx, _ = db.Beginx()
		defer func() {
			if err == nil {
				_ = tx.Commit()
			}
		}()
	}
	if user.Password, err = utils.EncryptPassword(password); err != nil {
		return err
	}
	sqlStr := "INSERT INTO user (username, password, email, nickname) VALUES (?, ?, ?, ?)"
	sqlRes, err := tx.Exec(sqlStr, user.Username, user.Password, user.Email, user.Nickname)
	if err != nil {
		return err
	}
	user.ID, _ = sqlRes.LastInsertId()
	return nil
}

func GetPrivateToken(token string) (*PrivateToken, error) {
	privToken := &PrivateToken{}
	tblName := ((*PrivateToken)(nil)).tableName()
	columns := ((*PrivateToken)(nil)).columnNames()
	sqlStr := fmt.Sprintf("SELECT %s FROM %s WHERE token = ?", columns, tblName)
	err := db.Get(privToken, sqlStr, token)
	if err == sql.ErrNoRows {
		return nil, nil //nolint
	}
	if err != nil {
		return nil, err
	}
	return privToken, nil
}

func GetPrivateTokenByUserAndName(userID int64, name string) (*PrivateToken, error) {
	privToken := &PrivateToken{}
	tblName := ((*PrivateToken)(nil)).tableName()
	columns := ((*PrivateToken)(nil)).columnNames()
	sqlStr := fmt.Sprintf("SELECT %s FROM %s WHERE user_id = ? AND name = ?", columns, tblName)
	err := db.Get(privToken, sqlStr, userID, name)
	if err == sql.ErrNoRows {
		return nil, nil //nolint
	}
	if err != nil {
		return nil, err
	}
	return privToken, nil
}

func GetUser(ctx context.Context, idStr string) (*User, error) {
	tblName := ((*User)(nil)).TableName()
	columes := ((*User)(nil)).ColumnNames()
	user, err := getUserFromRedis(ctx, idStr)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}
	user = &User{}
	sqlStr := fmt.Sprintf("SELECT %s FROM %s WHERE username = ?", columes, tblName)
	err = db.Get(user, sqlStr, idStr)
	if err == sql.ErrNoRows {
		return nil, nil //nolint
	}
	if err != nil {
		return nil, err
	}
	if err = setUserToRedis(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func GetUserByID(ctx context.Context, id int64) (*User, error) {
	tblName := ((*User)(nil)).TableName()
	user, err := getUserFromRedisByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}

	sqlStr := fmt.Sprintf("SELECT * FROM %s WHERE id = ?", tblName)
	user = &User{}
	err = db.Get(user, sqlStr, id)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return user, err
	}
	if err = setUserToRedisByID(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func CheckAndGetUser(ctx context.Context, idStr, password string) (*User, error) {
	user, err := GetUser(ctx, idStr)
	if err != nil {
		return user, err
	}
	if user == nil {
		return nil, terrors.ErrInvalidUserPass
	}
	// compare password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, terrors.ErrInvalidUserPass
	}
	return user, nil
}

func getUserFromRedis(ctx context.Context, username string) (u *User, err error) {
	rKey := fmt.Sprintf(redisUserKey, username)
	u = &User{}
	err = utils.GetObjFromRedis(ctx, rKey, u)
	if err == redis.Nil {
		return nil, nil //nolint
	}
	return
}

func setUserToRedis(ctx context.Context, u *User) (err error) {
	rKey := fmt.Sprintf(redisUserKey, u.Username)
	return utils.SetObjToRedis(ctx, rKey, u, 10*time.Minute)
}

func getUserFromRedisByID(ctx context.Context, id int64) (u *User, err error) {
	rKey := fmt.Sprintf(redisUserIDKey, id)
	u = &User{}
	err = utils.GetObjFromRedis(ctx, rKey, u)
	if err == redis.Nil {
		return nil, nil //nolint
	}
	return
}

func setUserToRedisByID(ctx context.Context, u *User) (err error) {
	rKey := fmt.Sprintf(redisUserIDKey, u.ID)
	return utils.SetObjToRedis(ctx, rKey, u, 10*time.Minute)
}

func deleteUserInRedis(ctx context.Context, u *User) {
	rKey1 := fmt.Sprintf(redisUserKey, u.Username)
	rKey2 := fmt.Sprintf(redisUserIDKey, u.ID)
	_ = utils.DeleteObjectsInRedis(ctx, rKey1, rKey2)
}

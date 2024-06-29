package user

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/projecteru2/vmihub/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/bcrypt"
)

type userTestSuite struct {
	suite.Suite
	r *gin.Engine
}

func (suite *userTestSuite) SetupTest() {
	t := suite.T()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err := testutils.Prepare(ctx, t)
	assert.Nil(t, err)
	r, err := testutils.PrepareGinEngine()
	assert.Nil(t, err)

	SetupRouter("/api/v1", r)
	suite.r = r
}

type passwdMatcher struct {
	passwd string
}

// Match satisfies sqlmock.Argument interface
func (a passwdMatcher) Match(v driver.Value) bool {
	ss, ok := v.(string)
	if !ok {
		return false
	}
	if err := bcrypt.CompareHashAndPassword([]byte(ss), []byte(a.passwd)); err != nil {
		return false
	}
	return true
}

// func (suite *userTestSuite) TestRegistr() {
// 	obj := types.RegisterRequest{
// 		Username: "user11",
// 		Password: "pass11",
// 		Email:    "haha@qq.com",
// 		Phone:    "12345678901",
// 	}
// 	sqlStr := "SELECT * FROM user WHERE phone = ?"
// 	models.Mock.ExpectQuery(sqlStr).
// 		WithArgs(obj.Phone).
// 		WillReturnError(sql.ErrNoRows)
// 	sqlStr = "INSERT INTO user (username, phone, password, email, namespace, nickname) VALUES (?, ?, ?, ?, ?, ?)"

// 	models.Mock.ExpectExec(sqlStr).
// 		WithArgs(obj.Username, obj.Phone, passwdMatcher{obj.Password}, obj.Email, obj.Username, obj.Username).
// 		WillReturnResult(sqlmock.NewResult(1234, 1))
// 	models.Mock.ExpectCommit()
// 	bs, err := json.Marshal(obj)
// 	suite.Nil(err)
// 	w := httptest.NewRecorder()
// 	req, _ := http.NewRequest("POST", "/api/v1/user/register", bytes.NewBuffer(bs))
// 	req.Header.Set("Content-Type", "application/json")
// 	suite.r.ServeHTTP(w, req)

// 	// fmt.Printf("+++++++++ %s\n", w.Body.String())
// 	suite.Equal(http.StatusCreated, w.Code)
// }

func TestUserTestSuite(t *testing.T) {
	suite.Run(t, new(userTestSuite))
}

package image

import (
	"github.com/projecteru2/vmihub/client/types"
	svctypes "github.com/projecteru2/vmihub/pkg/types"
)

const (
	baseDir     = "/tmp/libvmihub-test"
	testContent = "Test file contents"
)

var testImg = &types.Image{
	ImageInfoResp: svctypes.ImageInfoResp{
		Username: "test-user",
		Name:     "test-image",
		Tag:      "test-tag",
	},

	BaseDir: baseDir,
}

// func TestPullImage(t *testing.T) {
// 	defer os.RemoveAll(baseDir)

// 	// 创建一个测试服务器
// 	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// 模拟响应头中的Content-Disposition
// 		w.Header().Set("Content-Disposition", "attachment; filename=test_image.tar")
// 		// 模拟文件内容
// 		w.Write([]byte(testContent))
// 	}))
// 	defer server.Close()

// 	// 调用PullImage函数进行测试
// 	cred := &types.Credential{
// 		Token: "testtoken",
// 	}
// 	// 设置测试参数
// 	api, err := New(server.URL, baseDir, cred)
// 	assert.Nil(t, err)

// 	err = api.Pull(context.Background(), testImg)
// 	assert.Nil(t, err)

// 	// 验证生成的本地文件是否存在
// 	filename := testImg.Filepath()
// 	assert.True(t, util.FileExists(filename))

// 	// 验证生成的本地文件内容是否正确
// 	content, err := os.ReadFile(filename)
// 	assert.Nil(t, err)

// 	assert.Equal(t, testContent, string(content))

// }

//func TestListLocalImages(t *testing.T) {
//	defer os.RemoveAll(baseDir)
//
//	cred := &types.Credential{
//		Token: "Bearer test-token",
//	}
//	api, err := NewAPI("", baseDir, cred)
//	assert.Nil(t, err)
//	imgNames := map[string]string{
//		"user1/img1:tag1": "user1/img1:tag1",
//		"user1/img2:tag1": "user1/img2:tag1",
//		"user2/img1:tag1": "user2/img1:tag1",
//		"img3:tag1":       "img3:tag1",
//		"img4":            "img4:latest",
//	}
//	expectVals := map[string]bool{}
//	for imgName, eVal := range imgNames {
//		img1, err := api.NewImage(imgName)
//		assert.Nil(t, err)
//		err = util.CreateQcow2File(img1.Filepath(), "qcow2", 1024*1024)
//		assert.Nil(t, err)
//		expectVals[eVal] = true
//	}
//
//	imgs, err := api.ListLocalImages()
//	assert.Nil(t, err)
//	assert.Len(t, imgs, 5)
//	for _, img := range imgs {
//		_, ok := expectVals[img.Fullname()]
//		assert.Equal(t, int64(1024*1024), img.VirtualSize)
//		assert.Truef(t, ok, "%s", img.Fullname())
//	}
//}

//func TestGetToken(t *testing.T) {
//	// 创建一个测试服务器
//	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		// 模拟登录请求的处理
//		var loginReq svctypes.LoginRequest
//		err := json.NewDecoder(r.Body).Decode(&loginReq)
//		assert.Nil(t, err)
//
//		// 模拟根据用户名和密码返回token
//		if loginReq.Username == "testuser" && loginReq.Password == "testpassword" {
//			resp := svctypes.TokenResponse{
//				AccessToken: "testtoken",
//			}
//			jsonResp, err := json.Marshal(resp)
//			assert.Nil(t, err)
//
//			w.Header().Set("Content-Type", "application/json")
//			w.WriteHeader(http.StatusOK)
//			w.Write(jsonResp)
//		} else {
//			w.WriteHeader(http.StatusUnauthorized)
//		}
//	}))
//	defer server.Close()
//
//	// 设置测试参数
//	serverURL := server.URL
//	username := "testuser"
//	password := "testpassword"
//
//	// 调用GetToken函数进行测试
//	token, _, err := auth.GetToken(context.Background(), serverURL, username, password)
//	assert.Nil(t, err)
//
//	expectedToken := "testtoken"
//	assert.Equal(t, expectedToken, token)
//}

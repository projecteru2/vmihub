package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	svctypes "github.com/projecteru2/vmihub/pkg/types"
)

func TestGetToken(t *testing.T) {
	// create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟登录请求的处理
		var loginReq svctypes.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
			t.Errorf("Failed to decode login request: %v", err)
		}

		// 模拟根据用户名和密码返回token
		if loginReq.Username == "testuser" && loginReq.Password == "testpassword" {
			resp := svctypes.TokenResponse{
				AccessToken: "testtoken",
			}
			jsonResp, err := json.Marshal(resp)
			if err != nil {
				t.Errorf("Failed to marshal login response: %v", err)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(jsonResp)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer server.Close()

	// set test params
	serverURL := server.URL
	username := "testuser"
	password := "testpassword"

	// test GetToken
	token, _, err := GetToken(context.Background(), serverURL, username, password)
	if err != nil {
		t.Errorf("GetToken returned an error: %v", err)
	}

	expectedToken := "testtoken"
	if token != expectedToken {
		t.Errorf("Token is incorrect. Expected: %s, Got: %s", expectedToken, token)
	}
}

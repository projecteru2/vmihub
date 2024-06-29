package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	svctypes "github.com/projecteru2/vmihub/pkg/types"
)

func Register(ctx context.Context, serverURL string, body *svctypes.RegisterRequest) error {
	reqURL := fmt.Sprintf("%s/api/v1/user/register", serverURL)
	u, err := url.Parse(reqURL)
	if err != nil {
		return err
	}
	query := u.Query()
	u.RawQuery = query.Encode()

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to login, status code %d, body: %s", resp.StatusCode, string(bs))
	}

	return nil
}

func GetToken(ctx context.Context, serverURL, username, password string) (string, string, error) {
	reqURL := fmt.Sprintf("%s/api/v1/user/token", serverURL)
	u, err := url.Parse(reqURL)
	if err != nil {
		return "", "", err
	}
	query := u.Query()
	u.RawQuery = query.Encode()

	body := svctypes.LoginRequest{
		Username: username,
		Password: password,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose
	if err != nil {
		return "", "", err
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("登录失败，HTTP状态码续：为 %d", resp.StatusCode)
	}

	var tokenResp svctypes.TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", "", err
	}

	return tokenResp.AccessToken, tokenResp.RefreshToken, nil
}

func RefreshToken(ctx context.Context, serverURL, accessToken, refreshToken string) (string, string, error) {
	reqURL := fmt.Sprintf("%s/api/v1/user/refreshToken", serverURL)
	u, err := url.Parse(reqURL)
	if err != nil {
		return "", "", err
	}
	query := u.Query()
	u.RawQuery = query.Encode()

	body := svctypes.RefreshRequest{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose
	if err != nil {
		return "", "", err
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("登录失败，HTTP状态码续：为 %d", resp.StatusCode)
	}

	var tokenResp svctypes.TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", "", err
	}

	return tokenResp.AccessToken, tokenResp.RefreshToken, nil
}

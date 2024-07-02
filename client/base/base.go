package base

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"

	"github.com/projecteru2/vmihub/client/terrors"
	"github.com/projecteru2/vmihub/client/types"
)

type APIImpl struct {
	ServerURL string
	Cred      *types.Credential
}

func NewAPI(addr string, cred *types.Credential) *APIImpl {
	impl := &APIImpl{
		ServerURL: addr,
		Cred:      cred,
	}
	return impl
}

func (i *APIImpl) AddAuth(req *http.Request) error {
	var val string
	if i.Cred.Username != "" && i.Cred.Password != "" {
		val = fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", i.Cred.Username, i.Cred.Password))))
	} else if i.Cred.Token != "" {
		val = fmt.Sprintf("Bearer %s", i.Cred.Token)
	}
	req.Header.Set("Authorization", val)
	return nil
}

func (i *APIImpl) AddAuthToHeader(req *http.Header) error {
	var val string
	if i.Cred.Username != "" && i.Cred.Password != "" {
		val = fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", i.Cred.Username, i.Cred.Password))))
	} else if i.Cred.Token != "" {
		val = fmt.Sprintf("Bearer %s", i.Cred.Token)
	}
	req.Set("Authorization", val)
	return nil
}

func (i *APIImpl) HTTPRequest(ctx context.Context, reqURL, method string, urlQueryValues map[string]string, bodyData any) (resRaw map[string]any, err error) {
	u, err := url.Parse(reqURL)
	if err != nil {
		return
	}

	query := u.Query()
	if len(urlQueryValues) > 0 {
		for key, value := range urlQueryValues {
			query.Add(key, value)
		}
	}
	u.RawQuery = query.Encode()

	var bodyBytes []byte
	if bodyData != nil {
		bodyBytes, err = json.Marshal(bodyData)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	_ = i.AddAuth(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := GetCommonRawResponse(resp)
	return data, err
}

func (i *APIImpl) HTTPPut(ctx context.Context, reqURL string, urlQueryValues map[string]string, bodyData any) (resRaw map[string]any, err error) {
	return i.HTTPRequest(ctx, reqURL, http.MethodPut, urlQueryValues, bodyData)
}

func (i *APIImpl) HTTPPost(ctx context.Context, reqURL string, urlQueryValues map[string]string, bodyData any) (resRaw map[string]any, err error) {
	return i.HTTPRequest(ctx, reqURL, http.MethodPut, urlQueryValues, bodyData)
}

func (i *APIImpl) HTTPGet(ctx context.Context, reqURL string, urlQueryValues map[string]string) (resRaw map[string]any, err error) {
	return i.HTTPRequest(ctx, reqURL, http.MethodGet, urlQueryValues, nil)
}

func (i *APIImpl) HTTPDelete(ctx context.Context, reqURL string, urlQueryValues map[string]string) (resRaw map[string]any, err error) {
	return i.HTTPRequest(ctx, reqURL, http.MethodDelete, urlQueryValues, nil)
}

func GetCommonRawResponse(resp *http.Response) (resRaw map[string]any, err error) {
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("status: %d, error: %v, %w", resp.StatusCode, resRaw["error"], terrors.ErrHTTPError)
		return
	}
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resRaw = map[string]any{}
	err = json.Unmarshal(bs, &resRaw)
	if err != nil {
		err = fmt.Errorf("failed to decode response: %s, %w", string(bs), err)
		return
	}
	return
}

func GetCommonResponseData(resRaw map[string]any) (data []byte, err error) {
	val, ok := resRaw["data"]
	if !ok {
		return nil, nil
	}
	data, err = json.Marshal(val)
	return
}

func GetCommonPageListResponse(resRaw map[string]any) (data []byte, total int64, err error) {
	val, ok := resRaw["data"]
	if !ok {
		return nil, 0, nil
	}
	total = int64(math.Round(resRaw["total"].(float64)))
	data, err = json.Marshal(val)
	return
}

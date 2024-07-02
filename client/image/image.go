package image

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cenkalti/backoff/v4"
	"github.com/dustin/go-humanize"
	"github.com/panjf2000/ants/v2"

	"github.com/projecteru2/vmihub/client/base"
	"github.com/projecteru2/vmihub/client/terrors"
	"github.com/projecteru2/vmihub/client/types"
	"github.com/projecteru2/vmihub/client/util"
	svctypes "github.com/projecteru2/vmihub/pkg/types"
	svcutils "github.com/projecteru2/vmihub/pkg/utils"
)

type API interface {
	NewImage(imgName string) (img *types.Image, err error)
	ListImages(ctx context.Context, user string, pageN, pageSize int) ([]*types.Image, int, error)
	ListLocalImages() ([]*types.Image, error)
	Push(ctx context.Context, img *types.Image, force bool) error
	Pull(ctx context.Context, imgName string, policy PullPolicy) (img *types.Image, err error)
	GetInfo(ctx context.Context, imgFullname string) (info *types.Image, err error)
	RemoveLocalImage(ctx context.Context, img *types.Image) (err error)
	RemoveImage(ctx context.Context, img *types.Image) (err error)
}

type APIImpl struct {
	base.APIImpl
	opts      *Options
	baseDir   string
	chunkSize int64
	threshold int64
	mdb       *types.MetadataDB
}

func NewAPI(addr string, baseDir string, cred *types.Credential, options ...Option) (*APIImpl, error) {
	opts := &Options{"100M", "1G"}
	for _, option := range options {
		option(opts)
	}
	chunkSize, err := humanize.ParseBytes(opts.chunkSize)
	if err != nil {
		return nil, err
	}
	threshold, err := humanize.ParseBytes(opts.threshold)
	if err != nil {
		return nil, err
	}
	if err := util.EnsureDir(filepath.Join(baseDir, "image")); err != nil {
		return nil, fmt.Errorf("failed to create dir %w: %w", err, terrors.ErrFSError)
	}
	mdb, err := types.NewMetadataDB(baseDir, "images")
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}
	img := &APIImpl{
		APIImpl:   *base.NewAPI(addr, cred),
		opts:      opts,
		baseDir:   baseDir,
		chunkSize: int64(chunkSize),
		threshold: int64(threshold),
		mdb:       mdb,
	}
	return img, nil
}

func (i *APIImpl) NewImage(imgName string) (img *types.Image, err error) {
	return i.mdb.NewImage(imgName)
}

// ListImages get all images list
func (i *APIImpl) ListImages(ctx context.Context, username string, pageN, pageSize int) (images []*types.Image, total int, err error) {
	reqURL := fmt.Sprintf("%s/api/v1/images", i.ServerURL)

	u, err := url.Parse(reqURL)
	if err != nil {
		return
	}
	query := u.Query()
	query.Add("username", username)
	query.Add("page", strconv.FormatInt(int64(pageN), 10))
	query.Add("pageSize", strconv.FormatInt(int64(pageSize), 10))
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return
	}
	_ = i.AddAuth(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	resRaw := map[string]any{}
	err = json.Unmarshal(bs, &resRaw)
	if err != nil {
		err = fmt.Errorf("failed to decode response: %w %s", err, string(bs))
		return
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("status: %d, error: %v, %w", resp.StatusCode, resRaw["error"], terrors.ErrHTTPError)
		return
	}

	if obj, ok := resRaw["data"]; ok {
		bs, _ = json.Marshal(obj)
		images = []*types.Image{}
		err = json.Unmarshal(bs, &images)
	}
	if obj, ok := resRaw["total"]; ok {
		total = int(obj.(float64))
	}

	return images, total, err
}

func (i *APIImpl) ListLocalImages() ([]*types.Image, error) {
	var ans []*types.Image
	baseDir := filepath.Join(i.baseDir, "image/")
	err := filepath.WalkDir(baseDir, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !strings.HasSuffix(path, ".img") {
			return nil
		}
		imgName := strings.TrimSuffix(path, ".img")
		imgName = strings.TrimPrefix(imgName, baseDir)
		imgName = strings.TrimPrefix(imgName, "/")
		img, err := i.NewImage(imgName)
		if err != nil {
			return err
		}
		ans = append(ans, img)
		return nil
	})
	return ans, err
}

func (i *APIImpl) uploadWithChunk(ctx context.Context, img *types.Image, force bool) error {
	ck := &types.ChunkSlice{
		Image:     *img,
		ChunkSize: i.chunkSize,
	}
	if err := i.StartUploadImageChunk(ctx, ck, force); err != nil {
		return err
	}

	nChunks := int64(math.Ceil(float64(ck.Size) / float64(ck.ChunkSize)))
	retries := nChunks
	success := 0
	resCh := make(chan *execResult, nChunks)
	defer close(resCh)

	// set 10 to the capacity of goroutine pool and 1 second for expired duration.
	p, _ := ants.NewPoolWithFunc(10, func(idx any) {
		cIdx, _ := idx.(int64)
		err := i.UploadImageChunk(ctx, ck, cIdx)
		resCh <- &execResult{cIdx, err}
	})
	defer p.Release()

	// Submit tasks one by one.
	for idx := int64(0); idx < nChunks; idx++ {
		_ = p.Invoke(idx)
	}
	for res := range resCh {
		if res.err == nil {
			success++
			if success == int(nChunks) {
				break
			}
			continue
		}
		if retries > 0 {
			_ = p.Invoke(res.chunkIdx)
			retries--
		} else {
			return res.err
		}
	}
	return i.MergeChunk(ctx, ck.UploadID)
}

func (i *APIImpl) startUpload(ctx context.Context, img *types.Image, force bool) (uploadID string, err error) {
	reqURL := fmt.Sprintf("%s/api/v1/image/%s/%s/startUpload", i.ServerURL, img.Username, img.Name)

	metadata, err := img.LoadLocalMetadata()
	if err != nil {
		return "", err
	}
	var digest string
	if metadata != nil {
		digest = metadata.Digest
	}

	u, _ := url.Parse(reqURL)
	body := &svctypes.ImageCreateRequest{
		Username:    img.Username,
		Name:        img.Name,
		Tag:         img.Tag,
		Size:        img.Size,
		Private:     img.Private,
		Digest:      digest,
		Format:      img.Format,
		OS:          img.OS,
		Description: img.Description,
		URL:         img.URL, // just used for passing remote file when pushing
	}
	query := u.Query()
	query.Add("force", strconv.FormatBool(force))
	u.RawQuery = query.Encode()

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	_ = i.AddAuth(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute http request: %w", err)
	}
	defer resp.Body.Close()

	data, err := util.GetRespData(resp)
	if err != nil {
		return "", err
	}
	obj := map[string]string{}
	err = json.Unmarshal(data, &obj)
	if err != nil {
		return "", err
	}
	return obj["uploadID"], nil
}

func (i *APIImpl) upload(ctx context.Context, img *types.Image, uploadID string) (err error) {
	reqURL := fmt.Sprintf("%s/api/v1/image/%s/%s/upload", i.ServerURL, img.Username, img.Name)

	filePath := img.Filepath()
	fp, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", filePath, err)
	}
	defer fp.Close()

	r, w := io.Pipe()
	m := multipart.NewWriter(w)
	errCh := make(chan error, 1)
	go func() {
		defer w.Close()
		defer m.Close()

		part, err := m.CreateFormFile("file", filepath.Base(filePath))
		if err != nil {
			errCh <- fmt.Errorf("failed to create form file: %w", err)
			return
		}
		if _, err = io.Copy(part, fp); err != nil {
			errCh <- fmt.Errorf("failed to copy file: %w", err)
			return
		}
	}()

	u, _ := url.Parse(reqURL)
	query := u.Query()
	query.Add("uploadID", uploadID)
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), r)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Content-Type", m.FormDataContentType())
	_ = i.AddAuth(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := io.ReadAll(resp.Body)
		select {
		case err = <-errCh:
		default:
		}
		if err != nil {
			return fmt.Errorf("failed to push: %s, %s: %w", resp.Status, string(bs), err)
		} else { //nolint:revive
			return fmt.Errorf("failed to push: %s, %s", resp.Status, string(bs))
		}
	}
	return nil
}

func (i *APIImpl) uploadSingle(ctx context.Context, img *types.Image, force bool) (err error) {
	remoteUpload := img.URL != ""
	uploadID, err := i.startUpload(ctx, img, force)
	if err != nil {
		return err
	}
	if !remoteUpload {
		err = i.upload(ctx, img, uploadID)
	}
	return
}

func (i *APIImpl) Push(ctx context.Context, img *types.Image, force bool) error {
	var (
		size int64
		err  error
	)
	if img.URL == "" {
		size, err = util.GetFileSize(img.Filepath())
		if err != nil {
			return err
		}
	}
	img.Size = size
	if size > i.threshold {
		err = i.uploadWithChunk(ctx, img, force)
	} else {
		err = i.uploadSingle(ctx, img, force)
	}
	return err
}

func (i *APIImpl) GetInfo(ctx context.Context, imgFullname string) (info *types.Image, err error) {
	username, name, tag, err := svcutils.ParseImageName(imgFullname)
	if err != nil {
		return nil, err
	}
	reqURL := fmt.Sprintf(`%s/api/v1/image/%s/%s/info`, i.ServerURL, username, name)

	u, err := url.Parse(reqURL)
	if err != nil {
		return nil, err
	}
	query := u.Query()
	query.Add("tag", tag)
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	_ = i.AddAuth(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, terrors.ErrImageNotFound
	default:
		return nil, fmt.Errorf("status: %d: %w", resp.StatusCode, terrors.ErrHTTPError)
	}
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resRaw := map[string]any{}
	err = json.Unmarshal(bs, &resRaw)
	if err != nil {
		return nil, err
	}
	dataObj, ok := resRaw["data"]
	if !ok {
		return nil, fmt.Errorf("response json object needs contain data field: %s: %w", string(bs), terrors.ErrHTTPError)
	}
	info = &types.Image{
		BaseDir: i.baseDir,
		MDB:     i.mdb,
	}
	dataBs, _ := json.Marshal(dataObj)
	if err = json.Unmarshal(dataBs, &info.ImageInfoResp); err != nil {
		return nil, err
	}
	info.BaseDir = i.baseDir
	if util.FileExists(info.Filepath()) {
		info.ActualSize, info.VirtualSize, err = util.ImageSize(info.Filepath()) //nolint
	}
	return info, nil
}

func (i *APIImpl) Pull(ctx context.Context, imgName string, policy PullPolicy) (*types.Image, error) {
	img, err := i.mdb.NewImage(imgName)
	if err != nil {
		return nil, err
	}
	switch policy {
	case PullPolicyNever:
		return nil, nil //nolint
	case "":
		if img.Tag == "latest" {
			policy = PullPolicyAlways
		} else {
			policy = PullPolicyIfNotPresent
		}
	}
	filePath := img.Filepath()
	if (policy == PullPolicyIfNotPresent) && util.FileExists(filePath) {
		meta, err := img.LoadLocalMetadata()
		if err != nil {
			return nil, err
		}
		img.Digest = meta.Digest
		img.Size = meta.Size
		return img, nil
	}
	// GetInfo can return different image object when the tag is empty or latest.
	// so just pass the value of GetInfo to img here
	if img, err = i.GetInfo(ctx, img.Fullname()); err != nil {
		return nil, err
	}

	if img.Format == "rbd" {
		return nil, fmt.Errorf("image in rbd format is not alllowed to download")
	}
	if cached, _ := img.Cached(); cached {
		return img, nil
	}

	// download image from server
	if img.Size > i.threshold {
		err = i.downloadWithChunk(ctx, img)
	} else {
		err = i.download(ctx, img)
	}
	if err != nil {
		return nil, err
	}

	// check digest again
	if cached, err := img.Cached(); err != nil || (!cached) {
		if err == nil {
			err = terrors.ErrInvalidDigest
		}
		return nil, err
	}

	return img, nil
}

func (i *APIImpl) RemoveLocalImage(_ context.Context, img *types.Image) (err error) {
	return i.mdb.RemoveImage(img)
}

func (i *APIImpl) RemoveImage(ctx context.Context, img *types.Image) (err error) {
	if err := i.RemoveLocalImage(ctx, img); err != nil {
		return err
	}
	reqURL := fmt.Sprintf("%s/api/v1/image/%s/%s", i.ServerURL, img.Username, img.Name)
	u, err := url.Parse(reqURL)
	if err != nil {
		return
	}
	query := u.Query()
	query.Add("tag", img.Tag)
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		return
	}
	_ = i.AddAuth(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	_, err = util.GetRespData(resp)
	return err
}

type execResult struct {
	chunkIdx int64
	err      error
}

func (i *APIImpl) downloadWithChunk(ctx context.Context, img *types.Image) (err error) {
	ck := &types.ChunkSlice{
		Image:     *img,
		ChunkSize: i.chunkSize,
	}

	nChunks := int64(math.Ceil(float64(ck.Size) / float64(ck.ChunkSize)))
	resCh := make(chan *execResult, nChunks)
	defer close(resCh)

	// set 10 to the capacity of goroutine pool and 1 second for expired duration.
	p, _ := ants.NewPoolWithFunc(10, func(idx any) {
		cIdx, _ := idx.(int64)

		backoffStrategy := backoff.NewExponentialBackOff()
		// Use the Retry operation to perform the operation with exponential backoff
		err := backoff.Retry(func() error {
			return i.DownloadImageChunk(ctx, ck, cIdx)
		}, backoff.WithContext(backoffStrategy, ctx))
		resCh <- &execResult{cIdx, err}
	})
	defer p.Release()

	// Submit tasks one by one.
	for idx := int64(0); idx < nChunks; idx++ {
		_ = p.Invoke(idx)
	}
	var (
		downloadErr error
		finished    int
	)
	for res := range resCh {
		finished++
		if res.err != nil {
			downloadErr = errors.Join(downloadErr, res.err)
		}
		if finished >= int(nChunks) {
			break
		}
	}
	if downloadErr != nil {
		return downloadErr
	}
	if err := mergeSliceFile(ck, int(nChunks)); err != nil {
		return err
	}
	if err := img.CopyFrom(ck.SliceFilePath()); err != nil {
		return err
	}
	_ = os.Remove(ck.SliceFilePath())
	return nil
}

func (i *APIImpl) download(ctx context.Context, img *types.Image) (err error) {
	reqURL := fmt.Sprintf("%s/api/v1/image/%s/%s/download", i.ServerURL, img.Username, img.Name)

	u, err := url.Parse(reqURL)
	if err != nil {
		return
	}
	query := u.Query()
	query.Add("tag", img.Tag)

	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	_ = i.AddAuth(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to pull image, status code: %d, body: %s", resp.StatusCode, string(bs))
	}
	return i.mdb.CopyFile(img, resp.Body)
}

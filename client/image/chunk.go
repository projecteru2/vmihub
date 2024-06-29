package image

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/dustin/go-humanize"
	"github.com/projecteru2/vmihub/client/terrors"
	"github.com/projecteru2/vmihub/client/types"
	"github.com/projecteru2/vmihub/client/util"
	svctypes "github.com/projecteru2/vmihub/pkg/types"
)

func (i *APIImpl) StartUploadImageChunk(ctx context.Context, chunk *types.ChunkSlice, force bool) (err error) {
	metadata, err := chunk.LoadLocalMetadata()
	if err != nil {
		return err
	}
	reqURL := fmt.Sprintf("%s/api/v1/image/%s/%s/startChunkUpload",
		i.ServerURL, chunk.Username, chunk.Name)

	u, err := url.Parse(reqURL)
	if err != nil {
		return err
	}
	body := &svctypes.ImageCreateRequest{
		Username:    chunk.Username,
		Name:        chunk.Name,
		Tag:         chunk.Tag,
		Size:        chunk.Size,
		Private:     chunk.Private,
		Digest:      metadata.Digest,
		Format:      chunk.Format,
		OS:          chunk.OS,
		Description: chunk.Description,
	}
	bodyBytes, _ := json.Marshal(body)
	nChunks := math.Ceil(float64(chunk.Size) / float64(chunk.ChunkSize))
	query := u.Query()
	query.Add("force", strconv.FormatBool(force))
	query.Add("chunkSize", strconv.FormatInt(chunk.ChunkSize, 10))
	query.Add("nChunks", strconv.FormatInt(int64(nChunks), 10))

	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	err = i.AddAuth(req)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := util.GetRespData(resp)
	if err != nil {
		return err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	uploadIDRaw := obj["uploadID"]
	chunk.UploadID, _ = uploadIDRaw.(string)
	return err
}

// MergeChunk after uploaded big size file slice, need merge slice
func (i *APIImpl) MergeChunk(ctx context.Context, uploadID string) error {
	reqURL := fmt.Sprintf("%s/api/v1/image/chunk/merge", i.ServerURL)

	u, err := url.Parse(reqURL)
	if err != nil {
		return err
	}
	query := u.Query()
	query.Add("uploadID", uploadID)

	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return err
	}
	err = i.AddAuth(req)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = util.GetRespData(resp)
	return err
}

func (i *APIImpl) DownloadImageChunk(ctx context.Context, chunk *types.ChunkSlice, cIdx int64) error {
	reqURL := fmt.Sprintf("%s/api/v1/image/%s/%s/chunk/%d/download",
		i.ServerURL, chunk.Username, chunk.Name, cIdx)

	u, err := url.Parse(reqURL)
	if err != nil {
		return err
	}
	query := u.Query()
	query.Add("tag", chunk.Tag)
	query.Add("chunkSize", humanize.Bytes(uint64(chunk.ChunkSize)))

	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	err = i.AddAuth(req)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.Wrapf(terrors.ErrNetworkError, "status: %s", resp.StatusCode)
	}

	chunkSliceFile := chunk.SliceFileIndexPath(int(cIdx))
	if err := util.EnsureDir(filepath.Dir(chunkSliceFile)); err != nil {
		return err
	}

	out, err := os.OpenFile(chunkSliceFile, os.O_WRONLY|os.O_CREATE, 0766)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", chunkSliceFile)
	}
	defer out.Close()

	// 将下载的文件内容写入本地文件
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func (i *APIImpl) UploadImageChunk(ctx context.Context, chunk *types.ChunkSlice, cIdx int64) error {
	reqURL := fmt.Sprintf("%s/api/v1/image/chunk/%d/upload",
		i.ServerURL, cIdx)

	filePath := chunk.Filepath()
	fp, err := os.Open(filePath)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", filePath)
	}
	defer fp.Close()
	// get slice part
	offset := cIdx * chunk.ChunkSize
	_, err = fp.Seek(offset, 0)
	if err != nil {
		return errors.Wrapf(err, "failed to seek to %d", offset)
	}
	reader := io.LimitReader(fp, chunk.ChunkSize)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return errors.Wrapf(err, "failed to create form field")
	}

	if _, err := io.Copy(part, reader); err != nil {
		return errors.Wrapf(err, "failed to copy")
	}
	_ = writer.Close()

	u, err := url.Parse(reqURL)
	if err != nil {
		return err
	}
	query := u.Query()
	query.Add("uploadID", chunk.UploadID)

	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	err = i.AddAuth(req)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = util.GetRespData(resp)
	return err
}

func mergeSliceFile(chunk *types.ChunkSlice, nChunks int) error {
	chunkSliceFile := chunk.SliceFilePath()
	dest, err := os.OpenFile(chunk.SliceFilePath(), os.O_WRONLY|os.O_CREATE, 0766)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", chunkSliceFile)
	}
	defer dest.Close()

	for cIdx := 0; cIdx < nChunks; cIdx++ {
		src, err := os.OpenFile(chunk.SliceFileIndexPath(cIdx), os.O_RDONLY, 0766)
		if err != nil {
			return err
		}
		defer src.Close()
		if _, err = io.Copy(dest, src); err != nil {
			return err
		}
	}
	return nil
}

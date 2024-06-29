package util

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/projecteru2/vmihub/client/terrors"
)

func EnsureDir(dirPath string) error {
	// 检查文件夹是否存在
	_, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		// 文件夹不存在，创建文件夹
		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			return err
		}
	} else if err != nil {
		// 其他错误
		return err
	}

	return nil
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func ImageSize(fname string) (int64, int64, error) {
	cmdArgs := []string{"qemu-img", "info", "--output=json", fname}
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...) //nolint:gosec
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return 0, 0, errors.Wrap(err, stderr.String())
	}
	res := map[string]any{}
	err = json.Unmarshal(stdout.Bytes(), &res)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "failed to unmarshal json: %s", stdout.String())
	}
	virtualSize := res["virtual-size"]
	actualSize := res["actual-size"]
	return int64(actualSize.(float64)), int64(virtualSize.(float64)), nil
}

func GetFileSize(filepath string) (int64, error) {
	fi, err := os.Stat(filepath)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func CreateQcow2File(fname string, fmt string, cap int64) error {
	if err := EnsureDir(filepath.Dir(fname)); err != nil {
		return err
	}

	cmd := exec.Command("qemu-img", "create", "-q", "-f", fmt, fname, strconv.FormatInt(cap, 10)) //nolint:gosec
	bs, err := cmd.CombinedOutput()
	return errors.Wrapf(err, "failed to create qemu image: %s", string(bs))
}

func Copy(src, dest string) error {
	srcF, err := os.OpenFile(src, os.O_RDONLY, 0766)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", src)
	}
	defer srcF.Close()

	if err := EnsureDir(filepath.Dir(dest)); err != nil {
		return errors.Wrapf(err, "failed to create dir for %s", dest)
	}
	destF, err := os.OpenFile(dest, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0766)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", dest)
	}
	defer destF.Close()

	if _, err = io.Copy(destF, srcF); err != nil {
		return err
	}
	return nil
}

func Move(src, dest string) error {
	if err := Copy(src, dest); err != nil {
		return err
	}
	// Move need to remove source file
	return os.Remove(src)
}

func GetRespData(resp *http.Response) (data []byte, err error) {
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resRaw := map[string]any{}
	err = json.Unmarshal(bs, &resRaw)
	if err != nil {
		err = errors.Wrapf(err, "failed to decode response: %s", string(bs))
		return
	}
	if resp.StatusCode != http.StatusOK {
		err = errors.Wrapf(terrors.ErrHTTPError, "status: %d, error: %v", resp.StatusCode, resRaw["error"])
		return
	}
	val, ok := resRaw["data"]
	if !ok {
		return nil, nil
	}
	data, err = json.Marshal(val)
	return
}

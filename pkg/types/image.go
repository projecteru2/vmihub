package types

import (
	"fmt"
	"strings"
	"time"

	"github.com/projecteru2/vmihub/pkg/terrors"
)

type JSONResult struct {
	Code    int    `json:"code" `
	Message string `json:"msg"`
	Data    any    `json:"data"`
}

type OSInfo struct {
	Type    string `json:"type" default:"linux"`
	Distrib string `json:"distrib" default:"ubuntu"`
	Version string `json:"version"`
	Arch    string `json:"arch" default:"amd64"`
}

func (info *OSInfo) String() string {
	ty := strings.ToLower(info.Type)
	switch ty {
	case "linux":
		return fmt.Sprintf("%s:%s [%s]", info.Distrib, info.Version, info.Arch)
	default:
		return fmt.Sprintf("%s:%s [%s]", info.Type, info.Version, info.Arch)
	}
}

type ImageCreateRequest struct {
	Username    string            `json:"username"`
	Name        string            `json:"name"`
	Tag         string            `json:"tag" default:"latest"`
	Labels      map[string]string `json:"labels"`
	Size        int64             `json:"size"`
	Private     bool              `json:"private" default:"false"`
	Digest      string            `json:"digest"`
	Format      string            `json:"format"`
	OS          OSInfo            `json:"os"`
	Description string            `json:"description"`
	URL         string            `json:"url"`
	RegionCode  string            `json:"region_code" default:"ap-yichang-1"`
}

func (req *ImageCreateRequest) Check() error {
	if req.URL == "" && len(req.Digest) != 64 {
		return terrors.ErrInvalidSha1
	}
	if req.OS.Type == "" {
		return terrors.ErrInvalidOS
	}
	req.OS.Type = strings.ToLower(req.OS.Type)
	if req.OS.Type == "linux" && req.OS.Distrib == "" {
		return terrors.ErrInvalidOS
	}
	if req.OS.Arch == "" {
		return terrors.ErrInvalidArch
	}
	if req.Format == "" {
		return terrors.ErrInvalidFormat
	}
	return nil
}

type ImageInfoRequest struct {
	Username   string
	ImgName    string
	Perm       string
	RegionCode string
}

type ImagesByUsernameRequest struct {
	Username   string
	Keyword    string
	PageNum    int
	PageSize   int
	RegionCode string
}

type ImageInfoResp struct {
	ID          int64     `json:"id"`
	RepoID      int64     `json:"repo_id"`
	Username    string    `json:"username"`
	Name        string    `json:"name"`
	Tag         string    `json:"tag" description:"image tag, default:latest"`
	Format      string    `json:"format"`
	OS          OSInfo    `json:"os"`
	Private     bool      `json:"private"`
	Size        int64     `json:"size"`
	Digest      string    `json:"digest" description:"image digest"`
	Snapshot    string    `json:"snapshot"`
	Description string    `json:"description" description:"image description"`
	CreatedAt   time.Time `json:"createdAt,omitempty" description:"image create time" example:"format: RFC3339"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty" description:"image update time" example:"format: RFC3339"`
}

func (img *ImageInfoResp) Fullname() string {
	if img.Username == "" || img.Username == "_" {
		return fmt.Sprintf("%s:%s", img.Name, img.Tag)
	}
	return fmt.Sprintf("%s/%s:%s", img.Username, img.Name, img.Tag)
}

func (img *ImageInfoResp) RBDName() string {
	name := strings.ReplaceAll(img.Fullname(), "/", ".")
	return strings.ReplaceAll(name, ":", "-")
}

type ImageListResp struct {
}

type ContextKey string

type ImageTreeNode struct {
	Name     string                    `json:"name"`
	Image    string                    `json:"image,omitempty"`
	Children []*ImageTreeNode          `json:"children"`
	ChildMap map[string]*ImageTreeNode `json:"-"`
}
type ImageListTreeResp struct {
	Images []*ImageTreeNode `json:"images"`
}

func (resp *ImageListTreeResp) Refine() {
	var handleTreeNode func(*ImageTreeNode)
	handleTreeNode = func(node *ImageTreeNode) {
		if len(node.ChildMap) == 0 {
			return
		}
		for _, child := range node.ChildMap {
			handleTreeNode(child)
			node.Children = append(node.Children, child)
		}
	}
	for _, node := range resp.Images {
		handleTreeNode(node)
	}
}

type ImageLabel struct {
	GPUTplList []string `json:"gputpl_list,omitempty"`
}

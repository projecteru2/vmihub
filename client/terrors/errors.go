package terrors

import "github.com/cockroachdb/errors"

var (
	ErrInvalidImageName = errors.New("invalid image name")
	ErrFSError          = errors.New("filesystem error")
	ErrNetworkError     = errors.New("network error")
	ErrHTTPError        = errors.New("http error")
	ErrInvalidHash      = errors.New("invalid hash type")
	ErrInvalidDigest    = errors.New("invalid digest")
	ErrImageNotFound    = errors.New("image not found")

	ErrPlaceholder = errors.New("placeholder error")
)

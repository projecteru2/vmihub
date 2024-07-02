package terrors

import "errors"

var (
	ErrPlaceholder      = errors.New("placeholder")
	ErrUnauthenticated  = errors.New("please login")
	ErrInvalidStore     = errors.New("invalid storage")
	ErrInvalidUserPass  = errors.New("invalid username or password")
	ErrInvalidDigest    = errors.New("invalid digest")
	ErrCreateWorkload   = errors.New("failed to create workload")
	ErrAllocateDataDisk = errors.New("failed to allocate data disk")
	ErrConfict          = errors.New("conflict")
	ErrInvalidUserKey   = errors.New("invalid username or key")
	ErrInvalidState     = errors.New("guest state is invalid")
	ErrInvalidUserName  = errors.New("invalid username")
	ErrInvalidPassword  = errors.New("invalid password")

	ErrIPAMNoAvailableIP    = errors.New("no available IP")
	ErrIPAMNotReserved      = errors.New("IP is not reserved")
	ErrIPAMAlreadyAllocated = errors.New("IP is already allocated")
	ErrIPAMInvalidIP        = errors.New("invalid IP")
	ErrIPAMInvalidIndex     = errors.New("invalid index")

	ErrRBDBusy       = errors.New("rbd or snapshot is busy")
	ErrRBDDependency = errors.New("rbd or snapshot is dependency")

	ErrTokenExpired     = errors.New("token is expired")
	ErrTokenNotValidYet = errors.New("token not active yet")
	ErrTokenMalformed   = errors.New("that's not even a token")
	ErrTokenInvalid     = errors.New("couldn't handle this token")

	ErrNotUploadYet = errors.New("file not upload yet")

	ErrPublicPortNotReserved      = errors.New("Public port is not reserved")
	ErrPublicPortAlreadyAllocated = errors.New("Public port is already allocated")

	ErrInvalidSha1    = errors.New("invalid digest, only accept sha256")
	ErrInvalidOS      = errors.New("os type is empty")
	ErrInvalidDistrib = errors.New("os distrib is empty")
	ErrInvalidArch    = errors.New("os arch is empty")
	ErrInvalidFormat  = errors.New("format is empty")
)

type ErrHTTPResp struct { //nolint
	Code int
	Msg  string
	Err  error
}

func (e *ErrHTTPResp) Error() string {
	return e.Msg
}

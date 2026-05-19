package space

import "errors"

var (
	ErrNoSpacesRegistered = errors.New("no spaces registered for this user")
	ErrNoDefaultSpace     = errors.New("no default space configured; run 'lv spaces use <alias>'")
	ErrSpaceNotFound      = errors.New("space not found")
	ErrInvalidSpaceScope  = errors.New("--spaces and --all-spaces are mutually exclusive")
	ErrNonceAlreadyUsed   = errors.New("nonce already consumed (replay rejected)")
)

// ExitCodePartialFailure は複数スペース実行で一部が失敗した場合の CLI exit code。
// 既存の exit code 定義（internal/app/exitcode.go）:
//
//	0=ExitSuccess, 1=ExitGenericError, 2=ExitArgumentError, 3=ExitAuthenticationError,
//	4=ExitPermissionError, 5=ExitNotFoundError, 6=ExitAPIError, 7=ExitDigestError, 10=ExitConfigError
//
// 8 は未使用のため割り当てる（H6/RH2 対応: partial_failure を argument error と分離）。
const ExitCodePartialFailure = 8

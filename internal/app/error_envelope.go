package app

import (
	"encoding/json"
	"io"

	"github.com/youyo/logvalet/internal/domain"
)

// ExitCoder は exit code を持つエラー用インターフェース。
// backlog.BacklogError 等がこれを実装する。
type ExitCoder interface {
	error
	ExitCode() int
}

// ErrorCoder はエラーコード文字列を持つエラー用インターフェース。
// spec §9 の error.code に対応する値を返す。
type ErrorCoder interface {
	ErrorCode() string
}

// Retryabler は retryable フラグを持つエラー用インターフェース。
// spec §9 の error.retryable に対応する値を返す。
type Retryabler interface {
	Retryable() bool
}

// エラーコード文字列の定数。
const (
	ErrorCodeGeneric        = "generic_error"
	ErrorCodeArgument       = "argument_error"
	ErrorCodeAuthentication = "authentication_error"
	ErrorCodePermission     = "permission_error"
	ErrorCodeNotFound       = "not_found"
	ErrorCodeAPI            = "api_error"
	ErrorCodeDigest         = "digest_error"
	ErrorCodeConfig         = "config_error"
)

// ExitCodeToErrorCode は exit code からエラーコード文字列を返す。
// ErrorCoder を実装しないエラーのフォールバック用。
func ExitCodeToErrorCode(exitCode int) string {
	switch exitCode {
	case ExitArgumentError:
		return ErrorCodeArgument
	case ExitAuthenticationError:
		return ErrorCodeAuthentication
	case ExitPermissionError:
		return ErrorCodePermission
	case ExitNotFoundError:
		return ErrorCodeNotFound
	case ExitAPIError:
		return ErrorCodeAPI
	case ExitDigestError:
		return ErrorCodeDigest
	case ExitConfigError:
		return ErrorCodeConfig
	default:
		return ErrorCodeGeneric
	}
}

// ExitCodeRetryable は exit code に基づいて retryable かどうかを返す。
// Retryabler を実装しないエラーのフォールバック用。
func ExitCodeRetryable(exitCode int) bool {
	return exitCode == ExitAPIError
}

// NewErrorEnvelope は spec §9 のエラーエンベロープを構築する。
func NewErrorEnvelope(code, message string, retryable bool) *domain.ErrorEnvelope {
	return &domain.ErrorEnvelope{
		SchemaVersion: "1",
		Error: domain.ErrorDetail{
			Code:      code,
			Message:   message,
			Retryable: retryable,
		},
	}
}

// HandleError は err を JSON エラーエンベロープとして w に出力し、exit code を返す。
// defaultExitCode は ExitCoder を実装しないエラーに使うデフォルトの exit code。
//
// 判定順序:
// 1. ExitCoder インターフェースで exit code を取得（なければ defaultExitCode）
// 2. ErrorCoder インターフェースで error code 文字列を取得（なければ exit code からフォールバック）
// 3. Retryabler インターフェースで retryable を取得（なければ exit code からフォールバック）
func HandleError(w io.Writer, err error, defaultExitCode int) int {
	// exit code の決定
	exitCode := defaultExitCode
	if ec, ok := err.(ExitCoder); ok {
		exitCode = ec.ExitCode()
	}

	// error code 文字列の決定
	code := ExitCodeToErrorCode(exitCode)
	if ecr, ok := err.(ErrorCoder); ok {
		if c := ecr.ErrorCode(); c != "" {
			code = c
		}
	}

	// retryable の決定
	retryable := ExitCodeRetryable(exitCode)
	if r, ok := err.(Retryabler); ok {
		retryable = r.Retryable()
	}

	// エンベロープの構築と出力
	envelope := NewErrorEnvelope(code, err.Error(), retryable)
	enc := json.NewEncoder(w)
	_ = enc.Encode(envelope) // JSON エンコードエラーは無視（致命的な状況のため）

	return exitCode
}

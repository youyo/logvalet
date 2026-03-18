// Package backlog は Backlog API クライアントを提供する。
//
// Client interface、HTTPClient 実装、MockClient（テスト用）、
// typed errors、request options、write request types を含む。
// spec §18 準拠。
package backlog

import (
	"errors"
	"fmt"

	"github.com/youyo/logvalet/internal/app"
)

// Sentinel errors — Backlog API のエラー種別を表す。
// errors.Is() で判定できる。
var (
	// ErrNotFound は HTTP 404 に対応する。
	ErrNotFound = errors.New("backlog: not found")

	// ErrUnauthorized は HTTP 401 に対応する。
	ErrUnauthorized = errors.New("backlog: unauthorized")

	// ErrForbidden は HTTP 403 に対応する。
	ErrForbidden = errors.New("backlog: forbidden")

	// ErrRateLimited は HTTP 429 に対応する。
	ErrRateLimited = errors.New("backlog: rate limited")

	// ErrValidation は HTTP 422 等のバリデーションエラーに対応する。
	ErrValidation = errors.New("backlog: validation error")

	// ErrAPI は HTTP 5xx 等の API エラーに対応する。
	ErrAPI = errors.New("backlog: api error")
)

// BacklogError は Backlog API から返されるエラー情報を保持する。
// Unwrap() で sentinel error (ErrNotFound 等) に辿れる。
type BacklogError struct {
	// Err は sentinel error (ErrNotFound, ErrUnauthorized, etc.)
	Err error
	// Code は Backlog API のエラーコード (例: "issue_not_found")
	Code string
	// Message はエラーの人間が読めるメッセージ
	Message string
	// StatusCode は HTTP ステータスコード
	StatusCode int
}

// Error は BacklogError の文字列表現を返す。
func (e *BacklogError) Error() string {
	if e.Code != "" && e.Message != "" {
		return fmt.Sprintf("backlog API error (HTTP %d, code=%s): %s", e.StatusCode, e.Code, e.Message)
	}
	if e.Message != "" {
		return fmt.Sprintf("backlog API error (HTTP %d): %s", e.StatusCode, e.Message)
	}
	if e.Err != nil {
		return fmt.Sprintf("backlog API error (HTTP %d): %s", e.StatusCode, e.Err.Error())
	}
	return fmt.Sprintf("backlog API error (HTTP %d)", e.StatusCode)
}

// Unwrap は sentinel error を返す。errors.Is() でのチェックに使用される。
func (e *BacklogError) Unwrap() error {
	return e.Err
}

// ExitCode は BacklogError に対応する exit code を返す。
// app.ExitCoder インターフェースの実装。
func (e *BacklogError) ExitCode() int {
	return ExitCodeFor(e)
}

// ErrorCode はエラーコード文字列を返す。
// app.ErrorCoder インターフェースの実装。
// BacklogError.Code フィールドの値をそのまま返す。
func (e *BacklogError) ErrorCode() string {
	return e.Code
}

// Retryable はリトライ可能かを返す。
// app.Retryabler インターフェースの実装。
// ErrRateLimited または ErrAPI の場合に true を返す。
func (e *BacklogError) Retryable() bool {
	return errors.Is(e.Err, ErrRateLimited) || errors.Is(e.Err, ErrAPI)
}

// ExitCodeFor は error から app.Exit* 定数へのマッピングを返す。
// typed errors (ErrNotFound 等) および BacklogError のラップにも対応する。
func ExitCodeFor(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return app.ExitNotFoundError
	case errors.Is(err, ErrUnauthorized):
		return app.ExitAuthenticationError
	case errors.Is(err, ErrForbidden):
		return app.ExitPermissionError
	case errors.Is(err, ErrValidation):
		return app.ExitArgumentError
	case errors.Is(err, ErrRateLimited):
		return app.ExitAPIError
	case errors.Is(err, ErrAPI):
		return app.ExitAPIError
	default:
		return app.ExitGenericError
	}
}

package cli

import (
	"fmt"

	"github.com/youyo/logvalet/internal/app"
)

// ErrNotImplemented は未実装コマンドのプレースホルダーエラーを返す。
func ErrNotImplemented(command string) error {
	return fmt.Errorf("%s: not implemented", command)
}

// partialFailureError は fan-out 実行で一部スペースが失敗した場合のエラー。
// app.ExitCoder を実装し、exit code 8 を返す。
type partialFailureError struct {
	msg string
}

func (e *partialFailureError) Error() string { return e.msg }
func (e *partialFailureError) ExitCode() int { return app.ExitPartialFailure }

// allFailureError は fan-out 実行で全スペースが失敗した場合のエラー。
// app.ExitCoder を実装し、exit code 1 を返す。
type allFailureError struct {
	msg string
}

func (e *allFailureError) Error() string { return e.msg }
func (e *allFailureError) ExitCode() int { return app.ExitGenericError }

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

// argumentError は引数や入力ファイルの内容が不正な場合のエラー。
type argumentError struct {
	msg string
}

func (e *argumentError) Error() string { return e.msg }
func (e *argumentError) ExitCode() int { return app.ExitArgumentError }

// quietExitError は結果を自前で stdout に出力済みのコマンドが、
// エラーエンベロープを重複出力せずに exit code だけを返すためのエラー。
type quietExitError struct {
	code int
	msg  string
}

func (e *quietExitError) Error() string   { return e.msg }
func (e *quietExitError) ExitCode() int   { return e.code }
func (e *quietExitError) QuietExit() bool { return true }

// allFailureError は fan-out 実行で全スペースが失敗した場合のエラー。
// app.ExitCoder を実装し、exit code 1 を返す。
type allFailureError struct {
	msg string
}

func (e *allFailureError) Error() string { return e.msg }
func (e *allFailureError) ExitCode() int { return app.ExitGenericError }

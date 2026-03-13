package cli

import "fmt"

// ErrNotImplemented は未実装コマンドのプレースホルダーエラーを返す。
func ErrNotImplemented(command string) error {
	return fmt.Errorf("%s: not implemented", command)
}

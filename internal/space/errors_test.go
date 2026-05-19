package space_test

import (
	"errors"
	"testing"

	"github.com/youyo/logvalet/internal/space"
)

// T5: ExitCodePartialFailure の値検証
// exitcode.go の既存定数:
//
//	0=ExitSuccess, 1=ExitGenericError, 2=ExitArgumentError, 3=ExitAuthenticationError,
//	4=ExitPermissionError, 5=ExitNotFoundError, 6=ExitAPIError, 7=ExitDigestError, 10=ExitConfigError
//
// 8 は未使用のため ExitCodePartialFailure に割り当てる。
func TestExitCodePartialFailure_Value(t *testing.T) {
	if space.ExitCodePartialFailure != 8 {
		t.Errorf("ExitCodePartialFailure = %d, want 8", space.ExitCodePartialFailure)
	}
}

// T6: エラーセンチネル値の検証
func TestErrors_Sentinel(t *testing.T) {
	if !errors.Is(space.ErrNoSpacesRegistered, space.ErrNoSpacesRegistered) {
		t.Error("ErrNoSpacesRegistered should satisfy errors.Is with itself")
	}
	if !errors.Is(space.ErrNoDefaultSpace, space.ErrNoDefaultSpace) {
		t.Error("ErrNoDefaultSpace should satisfy errors.Is with itself")
	}
	if !errors.Is(space.ErrSpaceNotFound, space.ErrSpaceNotFound) {
		t.Error("ErrSpaceNotFound should satisfy errors.Is with itself")
	}
	if !errors.Is(space.ErrInvalidSpaceScope, space.ErrInvalidSpaceScope) {
		t.Error("ErrInvalidSpaceScope should satisfy errors.Is with itself")
	}
	if space.ErrNonceAlreadyUsed == nil {
		t.Error("ErrNonceAlreadyUsed should not be nil")
	}
}

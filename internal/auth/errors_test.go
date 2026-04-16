package auth_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/youyo/logvalet/internal/auth"
)

func TestSentinelErrors_ErrorsIs(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrUnauthenticated", auth.ErrUnauthenticated},
		{"ErrProviderNotConnected", auth.ErrProviderNotConnected},
		{"ErrTokenExpired", auth.ErrTokenExpired},
		{"ErrTokenRefreshFailed", auth.ErrTokenRefreshFailed},
		{"ErrProviderUserMismatch", auth.ErrProviderUserMismatch},
		{"ErrInvalidTenant", auth.ErrInvalidTenant},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/直接判定", func(t *testing.T) {
			if !errors.Is(tt.err, tt.err) {
				t.Errorf("errors.Is(%s, %s) = false, want true", tt.name, tt.name)
			}
		})
	}
}

func TestSentinelErrors_ErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrUnauthenticated", auth.ErrUnauthenticated},
		{"ErrProviderNotConnected", auth.ErrProviderNotConnected},
		{"ErrTokenExpired", auth.ErrTokenExpired},
		{"ErrTokenRefreshFailed", auth.ErrTokenRefreshFailed},
		{"ErrProviderUserMismatch", auth.ErrProviderUserMismatch},
		{"ErrInvalidTenant", auth.ErrInvalidTenant},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/Error()メッセージ非空", func(t *testing.T) {
			msg := tt.err.Error()
			if msg == "" {
				t.Errorf("%s.Error() = empty string, want non-empty actionable message", tt.name)
			}
		})
	}
}

func TestSentinelErrors_WrappedErrorsIs(t *testing.T) {
	wrapped := fmt.Errorf("wrap: %w", auth.ErrTokenExpired)
	if !errors.Is(wrapped, auth.ErrTokenExpired) {
		t.Errorf("errors.Is(fmt.Errorf(\"wrap: %%w\", ErrTokenExpired), ErrTokenExpired) = false, want true")
	}
}

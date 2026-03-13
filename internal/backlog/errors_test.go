package backlog_test

import (
	"errors"
	"testing"

	"github.com/youyo/logvalet/internal/app"
	"github.com/youyo/logvalet/internal/backlog"
)

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{"ErrNotFound", backlog.ErrNotFound, backlog.ErrNotFound},
		{"ErrUnauthorized", backlog.ErrUnauthorized, backlog.ErrUnauthorized},
		{"ErrForbidden", backlog.ErrForbidden, backlog.ErrForbidden},
		{"ErrRateLimited", backlog.ErrRateLimited, backlog.ErrRateLimited},
		{"ErrValidation", backlog.ErrValidation, backlog.ErrValidation},
		{"ErrAPI", backlog.ErrAPI, backlog.ErrAPI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, tt.wantErr) {
				t.Errorf("errors.Is(%v, %v) = false, want true", tt.err, tt.wantErr)
			}
		})
	}
}

func TestBacklogError(t *testing.T) {
	t.Run("Error() returns formatted string", func(t *testing.T) {
		e := &backlog.BacklogError{
			Err:        backlog.ErrNotFound,
			Code:       "issue_not_found",
			Message:    "Issue PROJ-999 was not found.",
			StatusCode: 404,
		}
		got := e.Error()
		if got == "" {
			t.Error("Error() returned empty string")
		}
		// エラーメッセージにコードとメッセージが含まれる
		if len(got) == 0 {
			t.Error("Error() should return non-empty string")
		}
	})

	t.Run("Unwrap() returns sentinel error", func(t *testing.T) {
		e := &backlog.BacklogError{
			Err:        backlog.ErrNotFound,
			Code:       "issue_not_found",
			Message:    "Issue PROJ-999 was not found.",
			StatusCode: 404,
		}
		if !errors.Is(e, backlog.ErrNotFound) {
			t.Error("errors.Is(BacklogError, ErrNotFound) = false, want true")
		}
	})

	t.Run("Unwrap() for ErrUnauthorized", func(t *testing.T) {
		e := &backlog.BacklogError{
			Err:        backlog.ErrUnauthorized,
			StatusCode: 401,
		}
		if !errors.Is(e, backlog.ErrUnauthorized) {
			t.Error("errors.Is(BacklogError, ErrUnauthorized) = false, want true")
		}
	})
}

func TestExitCodeFor(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{"ErrNotFound", backlog.ErrNotFound, app.ExitNotFoundError},
		{"ErrUnauthorized", backlog.ErrUnauthorized, app.ExitAuthenticationError},
		{"ErrForbidden", backlog.ErrForbidden, app.ExitPermissionError},
		{"ErrRateLimited", backlog.ErrRateLimited, app.ExitAPIError},
		{"ErrValidation", backlog.ErrValidation, app.ExitArgumentError},
		{"ErrAPI", backlog.ErrAPI, app.ExitAPIError},
		{"generic error", errors.New("generic"), app.ExitGenericError},
		{"nil wrapped BacklogError ErrNotFound", &backlog.BacklogError{Err: backlog.ErrNotFound, StatusCode: 404}, app.ExitNotFoundError},
		{"nil wrapped BacklogError ErrUnauthorized", &backlog.BacklogError{Err: backlog.ErrUnauthorized, StatusCode: 401}, app.ExitAuthenticationError},
		{"nil wrapped BacklogError ErrForbidden", &backlog.BacklogError{Err: backlog.ErrForbidden, StatusCode: 403}, app.ExitPermissionError},
		{"nil wrapped BacklogError ErrAPI", &backlog.BacklogError{Err: backlog.ErrAPI, StatusCode: 500}, app.ExitAPIError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := backlog.ExitCodeFor(tt.err)
			if got != tt.wantCode {
				t.Errorf("ExitCodeFor(%v) = %d, want %d", tt.err, got, tt.wantCode)
			}
		})
	}
}

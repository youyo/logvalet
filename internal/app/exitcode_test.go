package app_test

import (
	"testing"

	"github.com/youyo/logvalet/internal/app"
)

func TestExitCodes(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"Success", app.ExitSuccess, 0},
		{"GenericError", app.ExitGenericError, 1},
		{"ArgumentError", app.ExitArgumentError, 2},
		{"AuthenticationError", app.ExitAuthenticationError, 3},
		{"PermissionError", app.ExitPermissionError, 4},
		{"NotFoundError", app.ExitNotFoundError, 5},
		{"APIError", app.ExitAPIError, 6},
		{"DigestError", app.ExitDigestError, 7},
		{"ConfigError", app.ExitConfigError, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.code, tt.want)
			}
		})
	}
}

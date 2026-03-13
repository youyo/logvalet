package version_test

import (
	"testing"

	"github.com/youyo/logvalet/internal/version"
)

func TestDefaultValues(t *testing.T) {
	t.Run("Version デフォルト値は dev", func(t *testing.T) {
		if version.Version != "dev" {
			t.Errorf("Version = %q, want %q", version.Version, "dev")
		}
	})

	t.Run("Commit デフォルト値は none", func(t *testing.T) {
		if version.Commit != "none" {
			t.Errorf("Commit = %q, want %q", version.Commit, "none")
		}
	})

	t.Run("Date デフォルト値は unknown", func(t *testing.T) {
		if version.Date != "unknown" {
			t.Errorf("Date = %q, want %q", version.Date, "unknown")
		}
	})
}

func TestString(t *testing.T) {
	got := version.String()
	if got == "" {
		t.Error("String() は空であってはならない")
	}
}

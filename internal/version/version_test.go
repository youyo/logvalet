package version_test

import (
	"encoding/json"
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

func TestNewInfo(t *testing.T) {
	info := version.NewInfo()

	t.Run("Version フィールドが反映される", func(t *testing.T) {
		if info.Version != version.Version {
			t.Errorf("Info.Version = %q, want %q", info.Version, version.Version)
		}
	})

	t.Run("Commit フィールドが反映される", func(t *testing.T) {
		if info.Commit != version.Commit {
			t.Errorf("Info.Commit = %q, want %q", info.Commit, version.Commit)
		}
	})

	t.Run("Date フィールドが反映される", func(t *testing.T) {
		if info.Date != version.Date {
			t.Errorf("Info.Date = %q, want %q", info.Date, version.Date)
		}
	})
}

func TestInfo_JSON(t *testing.T) {
	info := version.NewInfo()
	b, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal(Info) エラー: %v", err)
	}

	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal エラー: %v", err)
	}

	t.Run("version キーが存在する", func(t *testing.T) {
		if _, ok := m["version"]; !ok {
			t.Error("JSON に version キーが存在しない")
		}
	})

	t.Run("commit キーが存在する", func(t *testing.T) {
		if _, ok := m["commit"]; !ok {
			t.Error("JSON に commit キーが存在しない")
		}
	})

	t.Run("date キーが存在する", func(t *testing.T) {
		if _, ok := m["date"]; !ok {
			t.Error("JSON に date キーが存在しない")
		}
	})
}

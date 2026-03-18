package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/youyo/logvalet/internal/config"
)

func TestWriter_WriteNewFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := &config.Config{
		Version:        1,
		DefaultProfile: "work",
		DefaultFormat:  "json",
		Profiles: map[string]config.ProfileConfig{
			"work": {
				Space:   "example-space",
				BaseURL: "https://example-space.backlog.com",
				AuthRef: "work",
			},
		},
	}

	w := config.NewWriter()
	if err := w.Write(path, cfg); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// ファイルが作成されたことを確認
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}

	// 読み戻してパースできることを確認
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.Version != 1 {
		t.Errorf("loaded.Version = %d, want 1", loaded.Version)
	}
	if loaded.DefaultProfile != "work" {
		t.Errorf("loaded.DefaultProfile = %q, want work", loaded.DefaultProfile)
	}
	p, ok := loaded.Profiles["work"]
	if !ok {
		t.Fatal("loaded.Profiles[work] not found")
	}
	if p.Space != "example-space" {
		t.Errorf("p.Space = %q, want example-space", p.Space)
	}
	if p.BaseURL != "https://example-space.backlog.com" {
		t.Errorf("p.BaseURL = %q, want https://example-space.backlog.com", p.BaseURL)
	}
	if p.AuthRef != "work" {
		t.Errorf("p.AuthRef = %q, want work", p.AuthRef)
	}
}

func TestWriter_WriteExistingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// 既存ファイルを書き出し
	initial := &config.Config{
		Version:        1,
		DefaultProfile: "work",
		DefaultFormat:  "json",
		Profiles: map[string]config.ProfileConfig{
			"work": {
				Space:   "example-space",
				BaseURL: "https://example-space.backlog.com",
				AuthRef: "work",
			},
		},
	}
	w := config.NewWriter()
	if err := w.Write(path, initial); err != nil {
		t.Fatalf("initial Write() error: %v", err)
	}

	// 新しいプロファイルを追加した Config で上書き
	updated := &config.Config{
		Version:        1,
		DefaultProfile: "work",
		DefaultFormat:  "json",
		Profiles: map[string]config.ProfileConfig{
			"work": {
				Space:   "example-space",
				BaseURL: "https://example-space.backlog.com",
				AuthRef: "work",
			},
			"dev": {
				Space:   "dev-space",
				BaseURL: "https://dev-space.backlog.com",
				AuthRef: "dev",
			},
		},
	}
	if err := w.Write(path, updated); err != nil {
		t.Fatalf("updated Write() error: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(loaded.Profiles) != 2 {
		t.Errorf("len(loaded.Profiles) = %d, want 2", len(loaded.Profiles))
	}
	if _, ok := loaded.Profiles["dev"]; !ok {
		t.Error("loaded.Profiles[dev] not found")
	}
}

func TestWriter_CreateDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// ネストしたディレクトリ内のパスを指定
	path := filepath.Join(dir, "nested", "dir", "config.toml")

	cfg := &config.Config{
		Version: 1,
		Profiles: map[string]config.ProfileConfig{
			"test": {Space: "s", BaseURL: "https://s.backlog.com", AuthRef: "test"},
		},
	}

	w := config.NewWriter()
	if err := w.Write(path, cfg); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created in nested dir: %v", err)
	}
}

func TestWriter_FilePermissions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := &config.Config{Version: 1}

	w := config.NewWriter()
	if err := w.Write(path, cfg); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permission = %04o, want 0600", perm)
	}
}

package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Writer は config.toml の書き出しを担当する。
type Writer interface {
	// Write は cfg を TOML 形式で path に書き出す。
	// ディレクトリが存在しない場合は自動作成する（0700）。
	// ファイルパーミッションは 0600。
	Write(path string, cfg *Config) error
}

// defaultWriter は Writer の標準実装。
type defaultWriter struct{}

// NewWriter は標準的な Writer を返す。
func NewWriter() Writer {
	return &defaultWriter{}
}

// Write は Config を TOML 形式でファイルに書き出す。
func (w *defaultWriter) Write(path string, cfg *Config) error {
	// ディレクトリを作成（存在する場合は何もしない）
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("config: failed to create directory %s: %w", dir, err)
	}

	// TOML エンコード
	var buf bytes.Buffer
	encoder := toml.NewEncoder(&buf)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("config: failed to encode config: %w", err)
	}

	// ファイル書き出し（0600）
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("config: failed to write %s: %w", path, err)
	}

	return nil
}

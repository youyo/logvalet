// Package version は goreleaser ldflags で埋め込まれるバージョン情報を提供する。
package version

import "fmt"

// goreleaser ldflags で main.go から注入される。
// デフォルト値はローカルビルド用。
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Info はバージョン情報を構造化して保持する。
// JSON/YAML シリアライズに対応。
type Info struct {
	Version string `json:"version" yaml:"version"`
	Commit  string `json:"commit"  yaml:"commit"`
	Date    string `json:"date"    yaml:"date"`
}

// NewInfo は現在のバージョン情報を Info として返す。
func NewInfo() Info {
	return Info{Version: Version, Commit: Commit, Date: Date}
}

// String はバージョン情報を人間が読みやすい形式で返す。
func String() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, Date)
}

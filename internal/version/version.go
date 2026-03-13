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

// String はバージョン情報を人間が読みやすい形式で返す。
func String() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, Date)
}

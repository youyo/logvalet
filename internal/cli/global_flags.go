// Package cli は logvalet CLI のコマンド定義を提供する。
package cli

import (
	"fmt"

	"github.com/alecthomas/kong"
)

// GlobalFlags は全サブコマンドで共有するグローバルフラグ。
// spec §4, §17.1 準拠。
type GlobalFlags struct {
	// Profile は使用する設定プロファイル名。
	Profile string `short:"p" help:"profile name to use" env:"LOGVALET_PROFILE"`

	// Format は出力フォーマット (json|yaml|md|gantt)。
	Format string `short:"f" help:"output format (json|yaml|md|gantt)" default:"json" env:"LOGVALET_FORMAT"`

	// Pretty はインデント付き JSON/YAML 出力を有効にする。
	Pretty bool `help:"output with indentation" env:"LOGVALET_PRETTY"`

	// Config は設定ファイルパスの直接指定。
	Config string `short:"c" help:"specify config file path" type:"path" env:"LOGVALET_CONFIG"`

	// APIKey は Backlog API キーの直接指定。
	APIKey string `help:"specify API key directly" env:"LOGVALET_API_KEY"`

	// AccessToken は Backlog アクセストークンの直接指定。
	AccessToken string `help:"specify access token directly" env:"LOGVALET_ACCESS_TOKEN"`

	// BaseURL は Backlog ベース URL の直接指定。
	BaseURL string `help:"specify Backlog base URL directly" env:"LOGVALET_BASE_URL"`

	// Space は Backlog スペース名の直接指定。
	Space string `short:"s" help:"specify Backlog space name directly" env:"LOGVALET_SPACE"`

	// Verbose は詳細なデバッグ出力を有効にする (stderr)。
	Verbose bool `short:"v" help:"enable verbose debug output" env:"LOGVALET_VERBOSE"`

	// NoColor はカラー出力を無効にする。
	NoColor bool `help:"disable color output" env:"LOGVALET_NO_COLOR"`

	// Version は --version フラグ。Kong が自動的にバージョン文字列を出力して exit する。
	// kong.Vars{"version": "..."} で設定されたバージョン文字列を表示する。
	Version kong.VersionFlag `help:"display version information and exit"`
}

// Validate は GlobalFlags のバリデーションを行う。
// Kong が Parse 後に自動で呼び出す。
// spec §17 Validation rules: --api-key と --access-token は同時に指定できない。
func (g *GlobalFlags) Validate() error {
	if g.APIKey != "" && g.AccessToken != "" {
		return fmt.Errorf("--api-key and --access-token are mutually exclusive")
	}
	return nil
}

// DigestFlags は digest コマンドで共通するフラグ。
// spec §17.2 準拠。
type DigestFlags struct {
	// Since はアクティビティ等の取得開始日時 (ISO 8601)。
	Since string `help:"start date/time (ISO 8601)" env:"LOGVALET_SINCE"`
	// Until はアクティビティ等の取得終了日時 (ISO 8601)。
	Until string `help:"end date/time (ISO 8601)" env:"LOGVALET_UNTIL"`
	// Limit は最大取得件数。
	Limit int `help:"max number of items" default:"100" env:"LOGVALET_LIMIT"`
}

// ListFlags は list コマンドで共通するフラグ。
type ListFlags struct {
	// Offset は取得オフセット。
	Offset int `help:"offset" default:"0"`
	// Count は1ページの件数。
	Count int `help:"count per page" default:"100"`
}

// WriteFlags は write (create/update) コマンドで共通するフラグ。
// spec §17.2 準拠。
type WriteFlags struct {
	// DryRun は実際の書き込みを行わず、実行内容をプレビューする。
	DryRun bool `help:"preview without actually writing" env:"LOGVALET_DRY_RUN"`
}

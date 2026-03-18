// Package cli は logvalet CLI のコマンド定義を提供する。
package cli

import "fmt"

// GlobalFlags は全サブコマンドで共有するグローバルフラグ。
// spec §4, §17.1 準拠。
type GlobalFlags struct {
	// Profile は使用する設定プロファイル名。
	Profile string `short:"p" help:"使用するプロファイル名" env:"LOGVALET_PROFILE"`

	// Format は出力フォーマット (json|yaml|markdown|text)。
	Format string `short:"f" help:"出力フォーマット (json|yaml|markdown|text)" default:"json" env:"LOGVALET_FORMAT"`

	// Pretty はインデント付き JSON/YAML 出力を有効にする。
	Pretty bool `help:"インデント付きで出力する" env:"LOGVALET_PRETTY"`

	// Config は設定ファイルパスの直接指定。
	Config string `short:"c" help:"設定ファイルパスを指定する" type:"path" env:"LOGVALET_CONFIG"`

	// APIKey は Backlog API キーの直接指定。
	APIKey string `help:"API キーを直接指定する" env:"LOGVALET_API_KEY"`

	// AccessToken は Backlog アクセストークンの直接指定。
	AccessToken string `help:"アクセストークンを直接指定する" env:"LOGVALET_ACCESS_TOKEN"`

	// BaseURL は Backlog ベース URL の直接指定。
	BaseURL string `help:"Backlog ベース URL を直接指定する" env:"LOGVALET_BASE_URL"`

	// Space は Backlog スペース名の直接指定。
	Space string `short:"s" help:"Backlog スペース名を直接指定する" env:"LOGVALET_SPACE"`

	// Verbose は詳細なデバッグ出力を有効にする (stderr)。
	Verbose bool `short:"v" help:"詳細なデバッグ出力を有効にする" env:"LOGVALET_VERBOSE"`

	// NoColor はカラー出力を無効にする。
	NoColor bool `help:"カラー出力を無効にする" env:"LOGVALET_NO_COLOR"`

	// Version はバージョン情報。goreleaser ldflags で注入される。
	Version string `kong:"-"`
	// Commit は git commit hash。goreleaser ldflags で注入される。
	Commit string `kong:"-"`
	// Date はビルド日時。goreleaser ldflags で注入される。
	Date string `kong:"-"`
}

// Validate は GlobalFlags のバリデーションを行う。
// Kong が Parse 後に自動で呼び出す。
// spec §17 Validation rules: --api-key と --access-token は同時に指定できない。
func (g *GlobalFlags) Validate() error {
	if g.APIKey != "" && g.AccessToken != "" {
		return fmt.Errorf("--api-key と --access-token は同時に指定できません")
	}
	return nil
}

// DigestFlags は digest コマンドで共通するフラグ。
// spec §17.2 準拠。
type DigestFlags struct {
	// Since はアクティビティ等の取得開始日時 (ISO 8601)。
	Since string `help:"取得開始日時 (ISO 8601)" env:"LOGVALET_SINCE"`
	// Until はアクティビティ等の取得終了日時 (ISO 8601)。
	Until string `help:"取得終了日時 (ISO 8601)" env:"LOGVALET_UNTIL"`
	// Limit は最大取得件数。
	Limit int `help:"最大取得件数" default:"100" env:"LOGVALET_LIMIT"`
}

// ListFlags は list コマンドで共通するフラグ。
type ListFlags struct {
	// Offset は取得オフセット。
	Offset int `help:"取得オフセット" default:"0"`
	// Count は1ページの件数。
	Count int `help:"1ページの件数" default:"20"`
}

// WriteFlags は write (create/update) コマンドで共通するフラグ。
// spec §17.2 準拠。
type WriteFlags struct {
	// DryRun は実際の書き込みを行わず、実行内容をプレビューする。
	DryRun bool `help:"実際の書き込みを行わずプレビュー" env:"LOGVALET_DRY_RUN"`
}

package cli

// CLI は logvalet のルート Kong struct。
// 全サブコマンドはここに登録される。
type CLI struct {
	GlobalFlags

	Auth       AuthCmd       `cmd:"" help:"Backlog 認証の管理"`
	Completion CompletionCmd `cmd:"" help:"シェル補完スクリプトを出力する"`
	Issue      IssueCmd      `cmd:"" help:"課題の操作"`
	Project    ProjectCmd    `cmd:"" help:"プロジェクトの操作"`
	Activity   ActivityCmd   `cmd:"" help:"アクティビティの操作"`
	User       UserCmd       `cmd:"" help:"ユーザーの操作"`
	Document   DocumentCmd   `cmd:"" help:"ドキュメントの操作"`
	Meta       MetaCmd       `cmd:"" help:"メタデータ（ステータス・カテゴリー等）の取得"`
	Team       TeamCmd       `cmd:"" help:"チームの操作"`
	Space      SpaceCmd      `cmd:"" help:"スペースの操作"`
}

package cli

// ActivityCmd は activity コマンド群のルート。
type ActivityCmd struct {
	List   ActivityListCmd   `cmd:"" help:"アクティビティ一覧を取得する"`
	Digest ActivityDigestCmd `cmd:"" help:"アクティビティのダイジェストを生成する"`
}

// ActivityListCmd は activity list コマンド（spec §14.12）。
type ActivityListCmd struct {
	ListFlags
	// Project はプロジェクトキーでフィルタ（指定時はプロジェクトのアクティビティのみ取得）。
	Project string `help:"プロジェクトキーでフィルタ" env:"LOGVALET_PROJECT"`
	// Since は取得開始日時（ISO 8601 または duration: 30d, 1w 等）。
	Since string `help:"取得開始日時（ISO 8601 または duration）" env:"LOGVALET_SINCE"`
}

func (c *ActivityListCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("activity list")
}

// ActivityDigestCmd は activity digest コマンド（spec §14.13）。
// Since/Until/Limit は DigestFlags から継承する。
type ActivityDigestCmd struct {
	DigestFlags
	// Project はプロジェクトキーでフィルタ（指定時はプロジェクトのアクティビティのみ取得）。
	Project string `help:"プロジェクトキーでフィルタ" env:"LOGVALET_PROJECT"`
}

func (c *ActivityDigestCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("activity digest")
}

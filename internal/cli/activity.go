package cli

// ActivityCmd は activity コマンド群のルート。
type ActivityCmd struct {
	List   ActivityListCmd   `cmd:"" help:"アクティビティ一覧を取得する"`
	Digest ActivityDigestCmd `cmd:"" help:"アクティビティのダイジェストを生成する"`
}

// ActivityListCmd は activity list コマンド。
type ActivityListCmd struct {
	DigestFlags
}

func (c *ActivityListCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("activity list")
}

// ActivityDigestCmd は activity digest コマンド。
type ActivityDigestCmd struct {
	DigestFlags
}

func (c *ActivityDigestCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("activity digest")
}

package cli

// TeamCmd は team コマンド群のルート。
type TeamCmd struct {
	List    TeamListCmd    `cmd:"" help:"チーム一覧を取得する"`
	Project TeamProjectCmd `cmd:"" help:"プロジェクトのチーム一覧を取得する"`
	Digest  TeamDigestCmd  `cmd:"" help:"チームのダイジェストを生成する"`
}

// TeamListCmd は team list コマンド。
type TeamListCmd struct {
	ListFlags
}

func (c *TeamListCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("team list")
}

// TeamProjectCmd は team project コマンド。
type TeamProjectCmd struct {
	ProjectKey string `arg:"" required:"" help:"プロジェクトキー"`
}

func (c *TeamProjectCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("team project")
}

// TeamDigestCmd は team digest コマンド。
type TeamDigestCmd struct {
	DigestFlags
	TeamID int `arg:"" required:"" help:"チームID"`
}

func (c *TeamDigestCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("team digest")
}

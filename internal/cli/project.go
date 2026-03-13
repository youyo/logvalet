package cli

// ProjectCmd は project コマンド群のルート。
type ProjectCmd struct {
	Get    ProjectGetCmd    `cmd:"" help:"プロジェクトを取得する"`
	List   ProjectListCmd   `cmd:"" help:"プロジェクト一覧を取得する"`
	Digest ProjectDigestCmd `cmd:"" help:"プロジェクトのダイジェストを生成する"`
}

// ProjectGetCmd は project get コマンド。
type ProjectGetCmd struct {
	ProjectKeyOrID string `arg:"" required:"" help:"プロジェクトキー または ID"`
}

func (c *ProjectGetCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("project get")
}

// ProjectListCmd は project list コマンド。
type ProjectListCmd struct {
	ListFlags
}

func (c *ProjectListCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("project list")
}

// ProjectDigestCmd は project digest コマンド。
type ProjectDigestCmd struct {
	DigestFlags
	ProjectKeyOrID string `arg:"" required:"" help:"プロジェクトキー または ID"`
}

func (c *ProjectDigestCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("project digest")
}

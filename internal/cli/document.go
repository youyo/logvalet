package cli

// DocumentCmd は document コマンド群のルート。
type DocumentCmd struct {
	Get    DocumentGetCmd    `cmd:"" help:"ドキュメントを取得する"`
	List   DocumentListCmd   `cmd:"" help:"ドキュメント一覧を取得する"`
	Tree   DocumentTreeCmd   `cmd:"" help:"ドキュメントツリーを取得する"`
	Digest DocumentDigestCmd `cmd:"" help:"ドキュメントのダイジェストを生成する"`
	Create DocumentCreateCmd `cmd:"" help:"ドキュメントを作成する"`
}

// DocumentGetCmd は document get コマンド。
type DocumentGetCmd struct {
	NodeID string `arg:"" required:"" help:"ドキュメントノードID"`
}

func (c *DocumentGetCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("document get")
}

// DocumentListCmd は document list コマンド。
type DocumentListCmd struct {
	ListFlags
	ProjectKey string `arg:"" required:"" help:"プロジェクトキー"`
}

func (c *DocumentListCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("document list")
}

// DocumentTreeCmd は document tree コマンド。
type DocumentTreeCmd struct {
	ProjectKey string `arg:"" required:"" help:"プロジェクトキー"`
}

func (c *DocumentTreeCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("document tree")
}

// DocumentDigestCmd は document digest コマンド。
type DocumentDigestCmd struct {
	DigestFlags
	NodeID string `arg:"" required:"" help:"ドキュメントノードID"`
}

func (c *DocumentDigestCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("document digest")
}

// DocumentCreateCmd は document create コマンド。
type DocumentCreateCmd struct {
	WriteFlags
	ProjectKey string `required:"" help:"プロジェクトキー"`
	Title      string `required:"" help:"ドキュメントのタイトル"`
}

func (c *DocumentCreateCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("document create")
}

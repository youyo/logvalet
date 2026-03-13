package cli

// IssueCmd は issue コマンド群のルート。
type IssueCmd struct {
	Get     IssueGetCmd     `cmd:"" help:"課題を取得する"`
	List    IssueListCmd    `cmd:"" help:"課題一覧を取得する"`
	Digest  IssueDigestCmd  `cmd:"" help:"課題のダイジェストを生成する"`
	Create  IssueCreateCmd  `cmd:"" help:"課題を作成する"`
	Update  IssueUpdateCmd  `cmd:"" help:"課題を更新する"`
	Comment IssueCommentCmd `cmd:"" help:"コメントを操作する"`
}

// IssueGetCmd は issue get コマンド。
type IssueGetCmd struct {
	IssueIDOrKey string `arg:"" required:"" help:"課題ID または 課題キー (例: PROJ-123)"`
}

func (c *IssueGetCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("issue get")
}

// IssueListCmd は issue list コマンド。
type IssueListCmd struct {
	ListFlags
	ProjectKey []string `short:"k" help:"プロジェクトキー"`
}

func (c *IssueListCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("issue list")
}

// IssueDigestCmd は issue digest コマンド。
type IssueDigestCmd struct {
	DigestFlags
	ProjectKey string `arg:"" required:"" help:"プロジェクトキー"`
}

func (c *IssueDigestCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("issue digest")
}

// IssueCreateCmd は issue create コマンド。
type IssueCreateCmd struct {
	WriteFlags
	ProjectKey string `required:"" help:"プロジェクトキー"`
	Summary    string `required:"" help:"課題のサマリー"`
}

func (c *IssueCreateCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("issue create")
}

// IssueUpdateCmd は issue update コマンド。
type IssueUpdateCmd struct {
	WriteFlags
	IssueIDOrKey string `arg:"" required:"" help:"課題ID または 課題キー"`
}

func (c *IssueUpdateCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("issue update")
}

// IssueCommentCmd は issue comment コマンド群。
type IssueCommentCmd struct {
	List   IssueCommentListCmd   `cmd:"" help:"コメント一覧を取得する"`
	Add    IssueCommentAddCmd    `cmd:"" help:"コメントを追加する"`
	Update IssueCommentUpdateCmd `cmd:"" help:"コメントを更新する"`
}

// IssueCommentListCmd は issue comment list コマンド。
type IssueCommentListCmd struct {
	IssueIDOrKey string `arg:"" required:"" help:"課題ID または 課題キー"`
}

func (c *IssueCommentListCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("issue comment list")
}

// IssueCommentAddCmd は issue comment add コマンド。
type IssueCommentAddCmd struct {
	WriteFlags
	IssueIDOrKey string `arg:"" required:"" help:"課題ID または 課題キー"`
	Content      string `help:"コメント本文"`
}

func (c *IssueCommentAddCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("issue comment add")
}

// IssueCommentUpdateCmd は issue comment update コマンド。
type IssueCommentUpdateCmd struct {
	WriteFlags
	IssueIDOrKey string `arg:"" required:"" help:"課題ID または 課題キー"`
	CommentID    int    `arg:"" required:"" help:"コメントID"`
}

func (c *IssueCommentUpdateCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("issue comment update")
}

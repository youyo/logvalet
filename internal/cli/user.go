package cli

// UserCmd は user コマンド群のルート。
type UserCmd struct {
	List     UserListCmd     `cmd:"" help:"ユーザー一覧を取得する"`
	Get      UserGetCmd      `cmd:"" help:"ユーザーを取得する"`
	Activity UserActivityCmd `cmd:"" help:"ユーザーのアクティビティを取得する"`
	Digest   UserDigestCmd   `cmd:"" help:"ユーザーのダイジェストを生成する"`
}

// UserListCmd は user list コマンド。
type UserListCmd struct {
	ListFlags
	ProjectKey string `help:"プロジェクトキーでフィルタ"`
}

func (c *UserListCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("user list")
}

// UserGetCmd は user get コマンド。
type UserGetCmd struct {
	UserID int `arg:"" required:"" help:"ユーザーID"`
}

func (c *UserGetCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("user get")
}

// UserActivityCmd は user activity コマンド。
type UserActivityCmd struct {
	DigestFlags
	UserID int `arg:"" required:"" help:"ユーザーID"`
}

func (c *UserActivityCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("user activity")
}

// UserDigestCmd は user digest コマンド。
type UserDigestCmd struct {
	DigestFlags
	UserID int `arg:"" required:"" help:"ユーザーID"`
}

func (c *UserDigestCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("user digest")
}

package cli

// UserCmd は user コマンド群のルート。
type UserCmd struct {
	List     UserListCmd     `cmd:"" help:"ユーザー一覧を取得する"`
	Get      UserGetCmd      `cmd:"" help:"ユーザーを取得する"`
	Activity UserActivityCmd `cmd:"" help:"ユーザーのアクティビティを取得する"`
	Digest   UserDigestCmd   `cmd:"" help:"ユーザーのダイジェストを生成する"`
}

// UserListCmd は user list コマンド（spec §14.14）。
type UserListCmd struct {
	ListFlags
}

func (c *UserListCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("user list")
}

// UserGetCmd は user get コマンド（spec §14.15）。
// UserID は数値 ID または userKey（文字列）を受け付ける。
type UserGetCmd struct {
	// UserID はユーザーID（数値）またはユーザーキー（文字列）。
	UserID string `arg:"" required:"" help:"ユーザーID（数値）またはユーザーキー（文字列）"`
}

func (c *UserGetCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("user get")
}

// UserActivityCmd は user activity コマンド（spec §14.16）。
// Since/Until/Limit は DigestFlags から継承する。
type UserActivityCmd struct {
	DigestFlags
	// UserID はユーザーID（数値）またはユーザーキー（文字列）。
	UserID string `arg:"" required:"" help:"ユーザーID（数値）またはユーザーキー（文字列）"`
	// Project はプロジェクトキーでフィルタ。
	Project string `help:"プロジェクトキーでフィルタ" env:"LOGVALET_PROJECT"`
	// ActivityType はアクティビティタイプでフィルタ（オプション拡張）。
	ActivityType string `name:"type" help:"アクティビティタイプでフィルタ（例: issue_created, issue_commented）"`
}

func (c *UserActivityCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("user activity")
}

// UserDigestCmd は user digest コマンド（spec §14.17）。
// Since/Until/Limit は DigestFlags から継承する。
type UserDigestCmd struct {
	DigestFlags
	// UserID はユーザーID（数値）またはユーザーキー（文字列）。
	UserID string `arg:"" required:"" help:"ユーザーID（数値）またはユーザーキー（文字列）"`
	// Project はプロジェクトキーでフィルタ。
	Project string `help:"プロジェクトキーでフィルタ" env:"LOGVALET_PROJECT"`
}

func (c *UserDigestCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("user digest")
}

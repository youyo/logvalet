package cli

// AuthCmd は auth コマンド群のルート。
type AuthCmd struct {
	Login  AuthLoginCmd  `cmd:"" help:"Backlog にログインする"`
	Logout AuthLogoutCmd `cmd:"" help:"Backlog からログアウトする"`
	Whoami AuthWhoamiCmd `cmd:"" help:"現在のログインユーザーを表示する"`
	List   AuthListCmd   `cmd:"" help:"保存されている認証情報を一覧表示する"`
}

// AuthLoginCmd は auth login コマンド。
type AuthLoginCmd struct{}

func (c *AuthLoginCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("auth login")
}

// AuthLogoutCmd は auth logout コマンド。
type AuthLogoutCmd struct{}

func (c *AuthLogoutCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("auth logout")
}

// AuthWhoamiCmd は auth whoami コマンド。
type AuthWhoamiCmd struct{}

func (c *AuthWhoamiCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("auth whoami")
}

// AuthListCmd は auth list コマンド。
type AuthListCmd struct{}

func (c *AuthListCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("auth list")
}

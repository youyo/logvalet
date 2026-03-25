package cli

// CLI は logvalet のルート Kong struct。
// 全サブコマンドはここに登録される。
type CLI struct {
	GlobalFlags

	Auth        AuthCmd       `cmd:"" help:"manage Backlog authentication"`
	Config      ConfigCmd     `cmd:"" help:"manage configuration"`
	Configure   ConfigureCmd  `cmd:"" help:"interactively initialize configuration (alias for config init)"`
	Completion  CompletionCmd `cmd:"" help:"output shell completion scripts"`
	Digest      DigestCmd     `cmd:"" help:"generate integrated digest"`
	Issue       IssueCmd      `cmd:"" help:"manage issues"`
	Project     ProjectCmd    `cmd:"" help:"manage projects"`
	Activity    ActivityCmd   `cmd:"" help:"manage activities"`
	User        UserCmd       `cmd:"" help:"manage users"`
	Document    DocumentCmd   `cmd:"" help:"manage documents"`
	Meta        MetaCmd       `cmd:"" help:"get metadata (statuses, categories, etc.)"`
	Team        TeamCmd       `cmd:"" help:"manage teams"`
	Space       SpaceCmd      `cmd:"" help:"manage spaces"`
	VersionInfo VersionCmd    `cmd:"" name:"version" help:"display version information"`
}

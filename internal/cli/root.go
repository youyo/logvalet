package cli

// CLI は logvalet のルート Kong struct。
// 全サブコマンドはここに登録される。
type CLI struct {
	GlobalFlags

	Configure   ConfigureCmd  `cmd:"" help:"interactively initialize configuration"`
	Completion  CompletionCmd `cmd:"" help:"output shell completion scripts"`
	Digest      DigestCmd     `cmd:"" help:"generate integrated digest"`
	Issue       IssueCmd      `cmd:"" help:"manage issues"`
	Project     ProjectCmd    `cmd:"" help:"manage projects"`
	Activity    ActivityCmd   `cmd:"" help:"manage activities"`
	User        UserCmd       `cmd:"" help:"manage users"`
	Document    DocumentCmd    `cmd:"" help:"manage documents"`
	Wiki        WikiCmd        `cmd:"" help:"manage Backlog wiki pages"`
	SharedFile  SharedFileCmd  `cmd:"" help:"manage shared files"`
	Star        StarCmd        `cmd:"" help:"manage stars"`
	Watching    WatchingCmd    `cmd:"" help:"manage watchings"`
	Meta        MetaCmd        `cmd:"" help:"get metadata (statuses, categories, etc.)"`
	Team        TeamCmd        `cmd:"" help:"manage teams"`
	Space       SpaceCmd       `cmd:"" help:"manage spaces"`
	Mcp         McpCmd         `cmd:"" help:"start MCP server (Streamable HTTP)"`
	VersionInfo VersionCmd     `cmd:"" name:"version" help:"display version information"`
}

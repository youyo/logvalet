package cli

// SpaceCmd は space コマンド群のルート。
type SpaceCmd struct {
	Info       SpaceInfoCmd       `cmd:"" help:"スペース情報を取得する"`
	DiskUsage  SpaceDiskUsageCmd  `cmd:"" help:"スペースのディスク使用量を取得する"`
	Digest     SpaceDigestCmd     `cmd:"" help:"スペースのダイジェストを生成する"`
}

// SpaceInfoCmd は space info コマンド。
type SpaceInfoCmd struct{}

func (c *SpaceInfoCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("space info")
}

// SpaceDiskUsageCmd は space disk-usage コマンド。
type SpaceDiskUsageCmd struct{}

func (c *SpaceDiskUsageCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("space disk-usage")
}

// SpaceDigestCmd は space digest コマンド。
type SpaceDigestCmd struct {
	DigestFlags
}

func (c *SpaceDigestCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("space digest")
}

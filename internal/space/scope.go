package space

// Scope は --spaces / --all-spaces フラグから解決された操作対象スペースの指定。
// バリデーション（AllSpaces=true かつ len(Aliases)>0 はエラー）は Resolver で行う。
type Scope struct {
	Aliases   []string
	AllSpaces bool
}

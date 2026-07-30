package mcp

// ServerInfo は tool 呼び出し結果の _meta.serverInfo に相当する logvalet 独自型
// (gomcp.Implementation 相当)。
type ServerInfo struct {
	Name    string
	Version string
	Title   string
}

// ResultMeta は tool 呼び出し結果の _meta (serverInfo・authorization_required・
// authorization_url 等) を表す logvalet 独自型。
// toolResultAuthRequired (tools.go) が組み立てている gomcp.Meta.AdditionalFields の
// 内容を構造化したもの。
type ResultMeta struct {
	ServerInfo            *ServerInfo
	AuthorizationRequired bool
	AuthorizationURL      string
	// Extra はプロトコルで明示的に定義されていない追加フィールドを保持する。
	Extra map[string]any
}

// ToMap は ResultMeta を gomcp.Meta.AdditionalFields 互換の map[string]any に変換する。
func (m ResultMeta) ToMap() map[string]any {
	out := make(map[string]any, len(m.Extra)+2)
	for k, v := range m.Extra {
		out[k] = v
	}
	if m.AuthorizationRequired {
		out["authorization_required"] = true
	}
	if m.AuthorizationURL != "" {
		out["authorization_url"] = m.AuthorizationURL
	}
	if m.ServerInfo != nil {
		out["serverInfo"] = map[string]any{
			"name":    m.ServerInfo.Name,
			"version": m.ServerInfo.Version,
			"title":   m.ServerInfo.Title,
		}
	}
	return out
}

// ToolContentType は ToolContent.Type の許容値。
type ToolContentType string

// ToolContentTypeText はテキストコンテンツを表す。
// logvalet の全ツールは JSON をテキスト化した text content のみを返すため、
// 現時点では text 以外の content type (image/audio/embedded resource) は扱わない。
const ToolContentTypeText ToolContentType = "text"

// ToolContent は tool 呼び出し結果の content 配列の1要素を表す logvalet 独自型。
type ToolContent struct {
	Type ToolContentType
	Text string
}

// ToolError は tool 呼び出しがエラーになった場合のエラー表現。
// MCP プロトコル上、tool のエラーは JSON-RPC レベルのエラーではなく
// CallToolResult.IsError=true + content[0].text がエラーメッセージ、という規約で
// 表現される。ToolError はこの規約を型として明示する。
type ToolError struct {
	Message string
}

// ToolResult は tool 呼び出し結果 (content・structuredContent・isError・_meta) の
// logvalet 独自表現。gomcp.CallToolResult と等価な情報を SDK 非依存で保持する。
type ToolResult struct {
	Content           []ToolContent
	StructuredContent any
	IsError           bool
	Meta              *ResultMeta
}

// NewTextToolResult はテキストのみの成功結果を作る。
func NewTextToolResult(text string) ToolResult {
	return ToolResult{Content: []ToolContent{{Type: ToolContentTypeText, Text: text}}}
}

// NewErrorToolResult は ToolError からエラー結果 (IsError=true) を作る。
func NewErrorToolResult(toolErr ToolError) ToolResult {
	return ToolResult{
		Content: []ToolContent{{Type: ToolContentTypeText, Text: toolErr.Message}},
		IsError: true,
	}
}

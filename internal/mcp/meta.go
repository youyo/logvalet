package mcp

import (
	"context"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// meta.go は per-request _meta (params._meta の protocolVersion/clientInfo) を
// backend アダプタ (backend_official.go) から ToolFunc/ToolHandler まで届けるための
// SDK 非依存のヘルパーを提供する。tools.go の spaceRegCtxKey (contextWithSpace /
// space.FromContext) と同じ「ctx 経由で付随情報を伝搬する」パターンに揃えている。
//
// 公式 SDK は SEP-2575 (sessionless/stateless) プロトコルにおいて、
// protocolVersion・clientInfo を params._meta の namespaced keys
// (MetaKeyProtocolVersion/MetaKeyClientInfo) として受け取り、検証・型変換済みの値を
// CallToolParamsRaw.GetMeta() 経由で公開する (shared.go validateRequestMeta /
// decodeMetaValue)。ここでは検証を二重実装せず、SDK が既に検証済みの map を
// RequestMeta (tooldef.go) へ変換するだけに留める。
// 結果側の _meta.serverInfo も同様に SDK が annotateServerInfo (server.go) で
// 自動付与するため、logvalet 側で改めて serverInfo を組み立てる必要はない。

// requestMetaCtxKey は context.Context に RequestMeta を格納するための非公開キー。
type requestMetaCtxKey struct{}

// ContextWithRequestMeta は ctx に RequestMeta を埋め込んだ派生 context を返す。
// backend_official.go の RegisterTool が tool 呼び出しごとに1度だけ呼び出し、
// 以降 ToolFunc/ToolHandler は RequestMetaFromContext で読み出せる。
func ContextWithRequestMeta(ctx context.Context, meta RequestMeta) context.Context {
	return context.WithValue(ctx, requestMetaCtxKey{}, meta)
}

// RequestMetaFromContext は ContextWithRequestMeta で埋め込まれた RequestMeta を取り出す。
// ok=false は、meta が埋め込まれていない経路 (レガシーな initialize ベースのセッションや、
// ctx を直接組み立てるテスト等) であることを示す。
func RequestMetaFromContext(ctx context.Context) (RequestMeta, bool) {
	meta, ok := ctx.Value(requestMetaCtxKey{}).(RequestMeta)
	return meta, ok
}

// RequestMetaFromOfficialSDKMeta は公式 SDK の params._meta
// (officialmcp.CallToolParamsRaw.GetMeta() が返す map[string]any) を
// logvalet 独自の RequestMeta (tooldef.go) に変換する。
// MetaKeyProtocolVersion/MetaKeyClientInfo 以外のキーは Extra にそのまま保持する。
func RequestMetaFromOfficialSDKMeta(meta map[string]any) RequestMeta {
	out := RequestMeta{}
	if len(meta) == 0 {
		return out
	}
	extra := make(map[string]any, len(meta))
	for k, v := range meta {
		switch k {
		case officialmcp.MetaKeyProtocolVersion:
			if s, ok := v.(string); ok {
				out.ProtocolVersion = s
			}
		case officialmcp.MetaKeyClientInfo:
			out.ClientInfo = clientInfoFromOfficialSDKValue(v)
		default:
			extra[k] = v
		}
	}
	if len(extra) > 0 {
		out.Extra = extra
	}
	return out
}

// clientInfoFromOfficialSDKValue は params._meta.clientInfo の値を ClientInfo に変換する。
// 公式 SDK は decodeMetaValue で *officialmcp.Implementation にデコード済みの値を
// GetMeta() 経由で返すが、テストやレガシー経路が生の map[string]any を渡す場合にも
// 対応する。
func clientInfoFromOfficialSDKValue(v any) ClientInfo {
	switch ci := v.(type) {
	case *officialmcp.Implementation:
		if ci == nil {
			return ClientInfo{}
		}
		return ClientInfo{Name: ci.Name, Version: ci.Version, Title: ci.Title}
	case officialmcp.Implementation:
		return ClientInfo{Name: ci.Name, Version: ci.Version, Title: ci.Title}
	case map[string]any:
		name, _ := ci["name"].(string)
		version, _ := ci["version"].(string)
		title, _ := ci["title"].(string)
		return ClientInfo{Name: name, Version: version, Title: title}
	default:
		return ClientInfo{}
	}
}

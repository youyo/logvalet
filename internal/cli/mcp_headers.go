package cli

import (
	"net/http"

	"github.com/youyo/logvalet/internal/backlog"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// mcp_headers.go は S14 (issue #52) の実装。
//
// SEP-2575 の新プロトコル (`Mcp-Protocol-Version` ヘッダが 2026-07-28 以上) で
// 送られたリクエストに対する `Mcp-Method`/`Mcp-Name` ヘッダの検証、および
// それに伴う HeaderMismatch(-32020)・UnsupportedProtocolVersionError(-32022) の
// 応答は、S03 スパイク (docs/specs/spike-go-sdk-2026-07-28.md §(c)) の実測どおり
// 公式 Go SDK (github.com/modelcontextprotocol/go-sdk の
// StreamableHTTPHandler.ServeHTTP → validateMcpHeaders/ServerSession.handle) が
// 完全に行う。判定は「Mcp-Protocol-Version ヘッダが存在し 2026-07-28 以上か」
// だけで決まり、tools/call・resources/read・prompts/get の Mcp-Name 要求も
// method 名を見て機械的に課されるため、logvalet がその method/capability を
// 実際に登録しているかどうかには依存しない(未登録の resources/read・
// prompts/get でも、ヘッダ欠落は method ディスパッチより前に -32020 になる)。
// そのため logvalet 側で二重に実装する必要のあるギャップは無い。本ファイルは
// 検証ロジックそのものは持たず、mcp_headers_test.go の E2E テストが
// `logvalet mcp` (--auth 無効時) の実運用マウントパスと同一のハンドラー
// トポロジーを、idproxy/OAuth 等の認証スタックを構築せずに再現できるようにする
// ヘルパーのみを提供する。

// NewNoAuthMCPMux は mcp.go の Run() が --auth=false (既定 / --auth-mode=none)
// のときに組み立てるのと同一のハンドラートポロジー
// ("/mcp" → 公式 SDK の StreamableHTTPHandler, "/healthz" → healthHandler) を返す。
func NewNoAuthMCPMux(client backlog.Client, ver string, cfg mcpinternal.ServerConfig) http.Handler {
	h := mcpinternal.NewOfficialStreamableHTTPHandler(client, ver, cfg)

	mux := http.NewServeMux()
	mux.Handle("/mcp", h)
	mux.HandleFunc("/healthz", healthHandler)
	return mux
}

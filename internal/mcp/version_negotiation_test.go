package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// version_negotiation_test.go は S15 (issue #52) の実装。
//
// S03 スパイク (docs/specs/spike-go-sdk-2026-07-28.md (c)(d)) および S14
// (mcp_headers_test.go) の実測どおり、旧プロトコル (initialize/session ベース、
// 2025-11-25 以前) と新プロトコル (SEP-2575, 2026-07-28) のバージョン
// ネゴシエーションは公式 Go SDK (github.com/modelcontextprotocol/go-sdk) の
// StreamableHTTPHandler が完全に代行する。supportedProtocolVersions は SDK
// パッケージ内部の非公開変数であり ServerOptions/StreamableHTTPOptions に
// 上書き用のフィールドは存在しない (v1.7.0 時点。詳細は
// docs/specs/legacy-protocol-decision.md 参照)。そのため logvalet 側の追加実装は
// 無く、本ファイルは選択した方式 (A) の done_criteria/tests を回帰テストとして
// 固定するのみ。

// newVersionNegotiationTestServer は logvalet 実運用と同一設定
// (Stateless=true + JSONResponse=true) の StreamableHTTPHandler を httptest で
// 起動する。ListProjectStatusesFunc を設定し、tools/call が実際に成功する
// (ErrNotFound にならない) ようにする。
func newVersionNegotiationTestServer(t *testing.T) string {
	t.Helper()
	mock := backlog.NewMockClient()
	mock.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return []domain.Status{{ID: 1, ProjectID: 100, Name: "Open", DisplayOrder: 1000}}, nil
	}
	srv := newOfficialTestHTTPServer(t, mock)
	return srv.URL
}

type versionNegotiationErrorEnvelope struct {
	Error *struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	} `json:"error"`
}

// parseVersionNegotiationEnvelope はレスポンスボディから jsonrpc エラー部分と
// result.isError (ツールレベルのエラー) を読み取る。プロトコルレベルの成功
// (env.Error == nil) と、ツール呼び出し自体の成功 (isError == false) を区別する。
func parseVersionNegotiationEnvelope(t *testing.T, respBody []byte) (versionNegotiationErrorEnvelope, bool) {
	t.Helper()
	var env versionNegotiationErrorEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, respBody)
	}
	var result struct {
		Result struct {
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		t.Fatalf("unmarshal result: %v; body=%s", err, respBody)
	}
	return env, result.Result.IsError
}

// V01: 未対応のプロトコルバージョンを名乗るリクエストは
// -32022 (UnsupportedProtocolVersionError) と、サーバーが実際にサポートする
// バージョン一覧 (data.supported) を返す。2026-07-28 / 2025-11-25 の双方が
// data.supported に含まれることを確認する (S15 done_criteria: 既定
// 2026-07-28 + 2025-11-25 を含む)。
func TestVersionNegotiation_UnsupportedVersion_Returns32022WithSupportedList(t *testing.T) {
	url := newVersionNegotiationTestServer(t)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{` +
		`"name":"logvalet_meta_statuses","arguments":{"project_key":"TESTPROJ"},"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2099-01-01",` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	status, respBody := rawPostWithHeaders(t, url, body, map[string]string{
		"Mcp-Protocol-Version": "2099-01-01",
		"Mcp-Method":           "tools/call",
		"Mcp-Name":             "logvalet_meta_statuses",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", status, respBody)
	}

	env, _ := parseVersionNegotiationEnvelope(t, respBody)
	if env.Error == nil {
		t.Fatalf("expected JSON-RPC error, got success: %s", respBody)
	}
	if env.Error.Code != -32022 {
		t.Errorf("code = %d, want -32022; message=%q", env.Error.Code, env.Error.Message)
	}

	var data struct {
		Supported []string `json:"supported"`
		Requested string   `json:"requested"`
	}
	if err := json.Unmarshal(env.Error.Data, &data); err != nil {
		t.Fatalf("unmarshal error.data: %v; raw=%s", err, env.Error.Data)
	}
	if data.Requested != "2099-01-01" {
		t.Errorf("data.requested = %q, want 2099-01-01", data.Requested)
	}
	for _, want := range []string{"2026-07-28", "2025-11-25"} {
		found := false
		for _, v := range data.Supported {
			if v == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("data.supported = %v, want it to contain %q", data.Supported, want)
		}
	}
}

// V02: 2026-07-28 (SEP-2575 新プロトコル) クライアントからの tools/call が
// initialize なしで成功する (S15 tests: 新プロトコルクライアントの成功)。
func TestVersionNegotiation_NewProtocol20260728_ToolsCallSucceeds(t *testing.T) {
	url := newVersionNegotiationTestServer(t)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{` +
		`"name":"logvalet_meta_statuses","arguments":{"project_key":"TESTPROJ"},"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	status, respBody := rawPostWithHeaders(t, url, body, map[string]string{
		"Mcp-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "tools/call",
		"Mcp-Name":             "logvalet_meta_statuses",
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, respBody)
	}

	env, isError := parseVersionNegotiationEnvelope(t, respBody)
	if env.Error != nil {
		t.Fatalf("unexpected protocol error: %+v; body=%s", env.Error, respBody)
	}
	if isError {
		t.Fatalf("unexpected tool-level error: body=%s", respBody)
	}
}

// V03: 2025-11-25 (initialize/session ベースの旧プロトコル) クライアントを模した
// tools/call が成功する (S15 tests: 旧プロトコルクライアントの成功)。
//
// logvalet の MCP サーバーは常に StreamableHTTPOptions.Stateless=true で
// 構築される (backend_official.go)。SDK は「body._meta.protocolVersion が
// 何であれ、_meta に protocolVersion キーが存在する時点で Mcp-Protocol-Version
// ヘッダの一致を要求する」実装になっている (streamable.go の
// `protocolVersion >= protocolVersion20260728 || metaVersion != ""` 分岐)。
// そのため「2025-11-25 クライアント」の忠実な再現は _meta 自体を一切送らない
// リクエストであり (2025-11-25 世代のクライアントは元々 _meta/SEP-2575 を
// 知らない)、これは S03 スパイク (a) の
// TestA_StatelessDirectToolCall_LegacyProtocol_RequiresInitialize と同じ形。
// Stateless=true のサーバーは、この種の旧プロトコルのリクエストであっても
// initialize を経ずにリクエストごとの一時セッションで直接処理するため、
// initialize なしで tools/call がそのまま成功することを確認する。
func TestVersionNegotiation_LegacyProtocol20251125_ToolsCallSucceeds(t *testing.T) {
	url := newVersionNegotiationTestServer(t)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{` +
		`"name":"logvalet_meta_statuses","arguments":{"project_key":"TESTPROJ"}}}`
	status, respBody := rawPostWithHeaders(t, url, body, nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", status, respBody)
	}

	env, isError := parseVersionNegotiationEnvelope(t, respBody)
	if env.Error != nil {
		t.Fatalf("unexpected protocol error: %+v; body=%s", env.Error, respBody)
	}
	if isError {
		t.Fatalf("unexpected tool-level error: body=%s", respBody)
	}
}

// rawPostWithHeaders は rawPost (backend_official_test.go) にヘッダー指定を
// 加えたもの。mcp_headers_test.go (internal/cli) の postMCP と同じ手法。
func rawPostWithHeaders(t *testing.T, url, body string, headers map[string]string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return resp.StatusCode, b
}

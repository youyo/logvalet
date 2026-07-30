package mcp_test

import (
	"context"
	"errors"
	"testing"

	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/backlog"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// S16: MRTR (SEP-2322, InputRequiredResult) への Backlog 再認可導線の移行。
//
// 設計判断（docs/specs/gateway-request-contract.md §5.3/§5.4 参照）:
// 公式 SDK は CallToolResult の Content と InputRequests を同時設定できない
// (go-sdk mcp/mrtr.go handleMultiRoundTripResult が "server bug" として拒否する)。
// このため「1レスポンスで新旧を完全併記」はできず、代わりに以下の設計を採る:
//   - _meta.authorization_required / _meta.authorization_url は Content/InputRequests
//     と独立したフィールドのため、MRTR 経路・旧経路のどちらでも常に付与する
//     （done_criteria の「旧形式も併記」を _meta レベルで満たす）。
//   - Content 付きの旧形式か、InputRequests 付きの新形式かは、呼び出し元
//     (params._meta.protocolVersion, S13 で導入済みの RequestMeta) が
//     MRTR 対応バージョン (2026-07-28 以降) を宣言しているかで分岐する
//     (supportsMRTR, tools.go)。protocolVersion 未送信のクライアント
//     （stdio の初期化ベースのセッション等）は必ず旧形式のままになるため、
//     S04 契約 §5.4 の「stdio は現行の _meta.authorization_url フローを
//     変更なく維持する」を自然に満たす。
//   - 公式 SDK の URL 型 elicitation (ElicitParams{Mode:"url"}) は本ユースケース
//     （URL 提示 → ブラウザで認可 → 同一呼び出しをリトライ）に構造的に適合するため、
//     S04 §5.3 が留保していた「fail-fast ツールエラーへの後退」は不要と判断した。

// authRequiredFactory は Backlog 未接続を表す factory を返す。
func authRequiredFactory(err error) func(ctx context.Context) (backlog.Client, error) {
	return func(ctx context.Context) (backlog.Client, error) {
		return nil, err
	}
}

// ctxWithProtocolVersion は S13 の RequestMeta を ctx に埋め込む。
func ctxWithProtocolVersion(version string) context.Context {
	return mcpinternal.ContextWithRequestMeta(context.Background(), mcpinternal.RequestMeta{ProtocolVersion: version})
}

// registerAuthTestTool は認可エラーを検証するための最小限のツールを登録する。
func registerAuthTestTool(s *fakeBackend, factory func(ctx context.Context) (backlog.Client, error), authorizationURL string) {
	reg := mcpinternal.NewToolRegistryWithFactory(s, factory, authorizationURL)
	tool := mcpinternal.NewToolDef("mrtr_auth_test_tool", mcpinternal.WithDesc("test"))
	reg.Register(tool, func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
}

// M16-1: protocolVersion 未送信 (RequestMeta なし) のクライアントには、
// MRTR 未導入時と同一の旧形式 (isError + content にURL文言 + _meta.authorization_url) が
// 返ることを確認する (S04 契約 §5.4: stdio は現行フローを維持)。
func TestAuthRequired_NoRequestMeta_UsesLegacyFormat(t *testing.T) {
	s := newFakeBackend()
	registerAuthTestTool(s, authRequiredFactory(auth.ErrProviderNotConnected), toolAuthTestAuthorizeURL)

	result := callToolWithCtx(t, s, context.Background(), "mrtr_auth_test_tool", map[string]any{})

	if !result.IsError {
		t.Fatal("legacy format: expected IsError=true")
	}
	if len(result.Content) == 0 {
		t.Fatal("legacy format: expected content with authorization URL text")
	}
	if result.Meta == nil || result.Meta.AuthorizationURL != toolAuthTestAuthorizeURL {
		t.Fatalf("legacy format: Meta = %#v, want AuthorizationURL=%s", result.Meta, toolAuthTestAuthorizeURL)
	}
	if result.URLInputRequest != nil {
		t.Error("legacy format: URLInputRequest should not be set")
	}
}

// M16-2: protocolVersion が MRTR 対応バージョン (2026-07-28) を宣言しているクライアントには
// MRTR (InputRequiredResult 相当) が返り、かつ旧形式の _meta.authorization_url も
// 併記されることを確認する。
func TestAuthRequired_MRTRProtocolVersion_UsesMRTRFormat(t *testing.T) {
	s := newFakeBackend()
	registerAuthTestTool(s, authRequiredFactory(auth.ErrProviderNotConnected), toolAuthTestAuthorizeURL)

	result := callToolWithCtx(t, s, ctxWithProtocolVersion("2026-07-28"), "mrtr_auth_test_tool", map[string]any{})

	if result.IsError {
		t.Error("MRTR format: input_required is not a protocol error (IsError should be false)")
	}
	if len(result.Content) != 0 {
		t.Errorf("MRTR format: Content must stay empty (official SDK forbids Content+InputRequests), got %#v", result.Content)
	}
	if result.URLInputRequest == nil {
		t.Fatal("MRTR format: expected URLInputRequest to be set")
	}
	if result.URLInputRequest.URL != toolAuthTestAuthorizeURL {
		t.Errorf("URLInputRequest.URL = %q, want %q", result.URLInputRequest.URL, toolAuthTestAuthorizeURL)
	}
	// 後方互換: 旧形式の _meta も併記される。
	if result.Meta == nil || !result.Meta.AuthorizationRequired || result.Meta.AuthorizationURL != toolAuthTestAuthorizeURL {
		t.Fatalf("MRTR format: Meta = %#v, want authorization_required/url 併記", result.Meta)
	}

	// 公式 SDK 変換で InputRequests (SEP-2322) の URL 型 elicitation として
	// 正しく表現されることを検証する。
	sdk := result.ToOfficialSDKResult()
	if len(sdk.Content) != 0 {
		t.Errorf("sdk.Content should be empty, got %#v", sdk.Content)
	}
	if len(sdk.InputRequests) != 1 {
		t.Fatalf("sdk.InputRequests should have exactly 1 entry, got %d", len(sdk.InputRequests))
	}
	for _, ir := range sdk.InputRequests {
		elicit, ok := ir.(*officialmcp.ElicitParams)
		if !ok {
			t.Fatalf("InputRequest is not *officialmcp.ElicitParams: %T", ir)
		}
		if elicit.Mode != "url" {
			t.Errorf("elicit.Mode = %q, want %q", elicit.Mode, "url")
		}
		if elicit.URL != toolAuthTestAuthorizeURL {
			t.Errorf("elicit.URL = %q, want %q", elicit.URL, toolAuthTestAuthorizeURL)
		}
	}
}

// M16-3: 旧バージョン (MRTR より前の protocolVersion) を宣言するクライアントは、
// 引き続き旧形式を受け取ることを確認する（文字列比較による日付境界の検証）。
func TestAuthRequired_OldProtocolVersion_UsesLegacyFormat(t *testing.T) {
	s := newFakeBackend()
	registerAuthTestTool(s, authRequiredFactory(auth.ErrTokenExpired), toolAuthTestAuthorizeURL)

	result := callToolWithCtx(t, s, ctxWithProtocolVersion("2025-11-25"), "mrtr_auth_test_tool", map[string]any{})

	if !result.IsError || result.URLInputRequest != nil {
		t.Errorf("old protocol version should still use legacy format, got IsError=%v URLInputRequest=%#v", result.IsError, result.URLInputRequest)
	}
}

// M16-4: MRTR で input_required を返したあとに、クライアントが同一呼び出しを
// リトライできる（= factory が認可完了後に成功するようになった状態を模擬したとき、
// 通常の成功結果が返る）ことを確認する。
// logvalet 側は RequestState 等の追加状態を持たず、単に同じ tool を再実行するだけで
// よい設計であることの回帰確認。
func TestAuthRequired_RetryAfterMRTR_Succeeds(t *testing.T) {
	s := newFakeBackend()
	attempt := 0
	factory := func(ctx context.Context) (backlog.Client, error) {
		attempt++
		if attempt == 1 {
			return nil, auth.ErrProviderNotConnected
		}
		return backlog.NewMockClient(), nil
	}
	registerAuthTestTool(s, factory, toolAuthTestAuthorizeURL)

	ctx := ctxWithProtocolVersion("2026-07-28")

	first := callToolWithCtx(t, s, ctx, "mrtr_auth_test_tool", map[string]any{})
	if first.URLInputRequest == nil {
		t.Fatal("first call: expected MRTR input_required result")
	}

	retry := callToolWithCtx(t, s, ctx, "mrtr_auth_test_tool", map[string]any{})
	if retry.IsError {
		t.Fatalf("retry: expected success, got error result: %#v", retry.Content)
	}
	if retry.URLInputRequest != nil {
		t.Error("retry: URLInputRequest should be nil on success")
	}
}

// M16-5: 汎用エラー（needsAuthorization に該当しない）の場合は MRTR 対応クライアントに
// 対しても従来どおり単純なエラー結果を返すことを確認する（回帰）。
func TestAuthRequired_GenericError_NeverUsesMRTR(t *testing.T) {
	s := newFakeBackend()
	registerAuthTestTool(s, authRequiredFactory(errors.New("boom")), toolAuthTestAuthorizeURL)

	result := callToolWithCtx(t, s, ctxWithProtocolVersion("2026-07-28"), "mrtr_auth_test_tool", map[string]any{})

	if !result.IsError || result.URLInputRequest != nil || result.Meta != nil {
		t.Errorf("generic error should bypass both auth-required formats, got %#v", result)
	}
}

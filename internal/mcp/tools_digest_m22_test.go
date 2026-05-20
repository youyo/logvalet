package mcp_test

import (
	"context"
	"testing"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
	"github.com/youyo/logvalet/internal/space"
)

// ============================================================================
// M22: digest builder envelope メタデータ回帰テスト
// SpaceDigestBuilder / ActivityDigestBuilder / DocumentDigestBuilder が
// multi-space context から space / base_url を正しく取得することを検証する。
// T4-1 (tools_analysis_m22_test.go) と同じパターン。
// ============================================================================

// newMultiSpaceServerForDigests は digest 系ツールを登録した MCP サーバーを返す。
// cfg.Space / cfg.BaseURL は「heptagon 固定の起動時値」を模擬する（fallback 確認用）。
func newMultiSpaceServerForDigests(t *testing.T, spaceFactory func(context.Context, space.SpaceRegistration) (backlog.Client, error)) (*mcpserver.MCPServer, *space.MemoryStore) {
	t.Helper()
	store := space.NewMemoryStore()
	ctx := context.Background()
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "heptagon", BaseURL: "https://heptagon.backlog.com",
		Status: space.SpaceStatusOK,
	})
	_ = store.Upsert(ctx, &space.SpaceRegistration{
		UserID: "u1", Alias: "megumilog", BaseURL: "https://megumilog.backlog.jp",
		Status: space.SpaceStatusOK,
	})

	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	resolver := space.NewResolver(store)
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", resolver, spaceFactory)

	cfg := mcpinternal.ServerConfig{
		Profile: "default",
		Space:   "heptagon", // 起動時固定値（fallback）
		BaseURL: "https://heptagon.backlog.com",
	}
	mcpinternal.RegisterSpaceTools(reg, cfg)
	mcpinternal.RegisterActivityTools(reg, cfg)
	mcpinternal.RegisterDocumentTools(reg, cfg)
	return s, store
}

// assertEnvelopeSpaceMetadata は fan-out Result の inner Value（DigestEnvelope）が
// 期待する space / base_url を持つことを検証する。
// T4-1 (AnalysisEnvelope) と同じ構造: outer wrapper は space/base_url、inner は DigestEnvelope の space/base_url。
func assertEnvelopeSpaceMetadata(t *testing.T, r map[string]any, wantAlias, wantBaseURL string) {
	t.Helper()
	inner, ok := r["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result.result to be map, got %T", r["result"])
	}
	if got, _ := inner["space"].(string); got != wantAlias {
		t.Errorf("envelope.space: want %q, got %q", wantAlias, got)
	}
	if got, _ := inner["base_url"].(string); got != wantBaseURL {
		t.Errorf("envelope.base_url: want %q, got %q", wantBaseURL, got)
	}
}

// ----------------------------------------------------------------------------
// TD-1: SpaceDigestBuilder 経由で envelope の space/base_url が context の reg から取得される
// ----------------------------------------------------------------------------

func TestSpaceDigestHandler_FanOut_EnvelopeMetadataReflectsSpace(t *testing.T) {
	spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		mc := backlog.NewMockClient()
		mc.GetSpaceFunc = func(ctx context.Context) (*domain.Space, error) {
			return &domain.Space{SpaceKey: reg.Alias, Name: reg.Alias}, nil
		}
		mc.GetSpaceDiskUsageFunc = func(ctx context.Context) (*domain.DiskUsage, error) {
			return &domain.DiskUsage{}, nil
		}
		return mc, nil
	}

	s, _ := newMultiSpaceServerForDigests(t, spaceFactory)
	userCtx := auth.ContextWithUserID(context.Background(), "u1")
	result := callToolWithCtx(t, s, userCtx, "logvalet_space_digest", map[string]any{
		"spaces": []any{"megumilog"},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	raw := decodeTextJSONArray(t, result)
	if len(raw) != 1 {
		t.Fatalf("expected 1 result, got %d", len(raw))
	}
	r := raw[0]

	// outer wrapper
	if got, _ := r["space"].(string); got != "megumilog" {
		t.Errorf("outer.space: want megumilog, got %q", got)
	}
	if got, _ := r["base_url"].(string); got != "https://megumilog.backlog.jp" {
		t.Errorf("outer.base_url: want megumilog url, got %q", got)
	}

	// inner: DigestEnvelope の space / base_url が context 由来であること
	assertEnvelopeSpaceMetadata(t, r, "megumilog", "https://megumilog.backlog.jp")
}

// ----------------------------------------------------------------------------
// TD-2: ActivityDigestBuilder 経由で envelope の space/base_url が context の reg から取得される
// ----------------------------------------------------------------------------

func TestActivityDigestHandler_FanOut_EnvelopeMetadataReflectsSpace(t *testing.T) {
	spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		mc := backlog.NewMockClient()
		mc.ListSpaceActivitiesFunc = func(ctx context.Context, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
			return []domain.Activity{}, nil
		}
		return mc, nil
	}

	s, _ := newMultiSpaceServerForDigests(t, spaceFactory)
	userCtx := auth.ContextWithUserID(context.Background(), "u1")
	result := callToolWithCtx(t, s, userCtx, "logvalet_activity_digest", map[string]any{
		"spaces": []any{"megumilog"},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	raw := decodeTextJSONArray(t, result)
	if len(raw) != 1 {
		t.Fatalf("expected 1 result, got %d", len(raw))
	}
	r := raw[0]

	// outer wrapper
	if got, _ := r["space"].(string); got != "megumilog" {
		t.Errorf("outer.space: want megumilog, got %q", got)
	}
	if got, _ := r["base_url"].(string); got != "https://megumilog.backlog.jp" {
		t.Errorf("outer.base_url: want megumilog url, got %q", got)
	}

	// inner: DigestEnvelope の space / base_url が context 由来であること
	assertEnvelopeSpaceMetadata(t, r, "megumilog", "https://megumilog.backlog.jp")
}

// ----------------------------------------------------------------------------
// TD-3: DocumentDigestBuilder 経由で envelope の space/base_url が context の reg から取得される
// ----------------------------------------------------------------------------

func TestDocumentDigestHandler_FanOut_EnvelopeMetadataReflectsSpace(t *testing.T) {
	spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		mc := backlog.NewMockClient()
		mc.GetDocumentFunc = func(ctx context.Context, documentID string) (*domain.Document, error) {
			return &domain.Document{ID: documentID, Title: "test doc", ProjectID: 1}, nil
		}
		mc.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
			return []domain.Project{{ID: 1, ProjectKey: "PROJ", Name: "Test Project"}}, nil
		}
		mc.ListDocumentAttachmentsFunc = func(ctx context.Context, documentID string) ([]domain.Attachment, error) {
			return []domain.Attachment{}, nil
		}
		return mc, nil
	}

	s, _ := newMultiSpaceServerForDigests(t, spaceFactory)
	userCtx := auth.ContextWithUserID(context.Background(), "u1")
	result := callToolWithCtx(t, s, userCtx, "logvalet_document_digest", map[string]any{
		"document_id": "doc-001",
		"spaces":      []any{"megumilog"},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	raw := decodeTextJSONArray(t, result)
	if len(raw) != 1 {
		t.Fatalf("expected 1 result, got %d", len(raw))
	}
	r := raw[0]

	// outer wrapper
	if got, _ := r["space"].(string); got != "megumilog" {
		t.Errorf("outer.space: want megumilog, got %q", got)
	}
	if got, _ := r["base_url"].(string); got != "https://megumilog.backlog.jp" {
		t.Errorf("outer.base_url: want megumilog url, got %q", got)
	}

	// inner: DigestEnvelope の space / base_url が context 由来であること
	assertEnvelopeSpaceMetadata(t, r, "megumilog", "https://megumilog.backlog.jp")
}

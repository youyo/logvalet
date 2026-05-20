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
// M22 T4: 実 builder 経由で AnalysisEnvelope のメタデータが
//          multi-space context から取得されることを検証する e2e テスト。
// ============================================================================

// buildMockClientForMyTasks は MyTasks 用に最低限のレスポンスを返すモッククライアント。
func buildMockClientForMyTasks() *backlog.MockClient {
	mc := backlog.NewMockClient()
	mc.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return &domain.User{ID: 1, Name: "tester"}, nil
	}
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{}, nil
	}
	mc.ListWatchingsFunc = func(ctx context.Context, userID int, opt backlog.ListWatchingsOptions) ([]domain.Watching, error) {
		return []domain.Watching{}, nil
	}
	return mc
}

// newMultiSpaceServerForAnalysis は analysis 系ツールを登録した MCP サーバーを返す。
// cfg.Space / cfg.BaseURL は「heptagon 固定の起動時値」を模擬する。
func newMultiSpaceServerForAnalysis(t *testing.T) (*mcpserver.MCPServer, *space.MemoryStore) {
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

	spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		return buildMockClientForMyTasks(), nil
	}

	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	resolver := space.NewResolver(store)
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", resolver, spaceFactory)

	cfg := mcpinternal.ServerConfig{
		Profile: "default",
		Space:   "heptagon", // 起動時の固定値（fallback）
		BaseURL: "https://heptagon.backlog.com",
	}
	mcpinternal.RegisterAnalysisTools(reg, cfg)
	return s, store
}

// ----------------------------------------------------------------------------
// T4-1: 実 MyTasksBuilder 経由で envelope の Space/BaseURL が context の reg から取得される
// ----------------------------------------------------------------------------

func TestMyTasksHandler_FanOut_EnvelopeMetadataReflectsSpace(t *testing.T) {
	s, _ := newMultiSpaceServerForAnalysis(t)

	userCtx := auth.ContextWithUserID(context.Background(), "u1")
	result := callToolWithCtx(t, s, userCtx, "logvalet_my_tasks", map[string]any{
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

	// inner: AnalysisEnvelope の space / base_url が context 由来であること
	inner, ok := r["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result.result to be map, got %T", r["result"])
	}
	if got, _ := inner["space"].(string); got != "megumilog" {
		t.Errorf("envelope.space: want megumilog, got %q", got)
	}
	if got, _ := inner["base_url"].(string); got != "https://megumilog.backlog.jp" {
		t.Errorf("envelope.base_url: want megumilog url, got %q", got)
	}
}

// ----------------------------------------------------------------------------
// T4-2: 実 ProjectHealthBuilder 経由で envelope の Space/BaseURL を pin-down
//        sub-builder 連鎖（stale/blocker/workload）でも親 builder の値が伝搬する。
// ----------------------------------------------------------------------------

func TestProjectHealthHandler_MultiSpace_EnvelopeAndSubBuildersMetadata(t *testing.T) {
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

	// ProjectHealth は GetProject + ListIssues を必要とする
	buildMC := func() *backlog.MockClient {
		mc := backlog.NewMockClient()
		mc.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
			return &domain.Project{ID: 100, ProjectKey: projectKey, Name: projectKey}, nil
		}
		mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
			return []domain.Issue{}, nil
		}
		return mc
	}

	spaceFactory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		return buildMC(), nil
	}

	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	resolver := space.NewResolver(store)
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", resolver, spaceFactory)
	cfg := mcpinternal.ServerConfig{
		Profile: "default",
		Space:   "heptagon",
		BaseURL: "https://heptagon.backlog.com",
	}
	mcpinternal.RegisterAnalysisTools(reg, cfg)

	userCtx := auth.ContextWithUserID(ctx, "u1")
	result := callToolWithCtx(t, s, userCtx, "logvalet_project_health", map[string]any{
		"project_key": "PROJ",
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

	// outer
	if got, _ := r["space"].(string); got != "megumilog" {
		t.Errorf("outer.space: want megumilog, got %q", got)
	}

	// 親 envelope（ProjectHealthBuilder.newEnvelope の出力）
	inner, ok := r["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result.result to be map, got %T", r["result"])
	}
	if got, _ := inner["space"].(string); got != "megumilog" {
		t.Errorf("envelope.space: want megumilog, got %q (sub-builder 連鎖の伝搬を確認)", got)
	}
	if got, _ := inner["base_url"].(string); got != "https://megumilog.backlog.jp" {
		t.Errorf("envelope.base_url: want megumilog url, got %q", got)
	}
}

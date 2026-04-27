package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// TestNewServer_ReturnsServer は NewServer が nil でないサーバーを返すことを確認する。
func TestNewServer_ReturnsServer(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "1.0.0", mcpinternal.ServerConfig{})
	if s == nil {
		t.Fatal("expected non-nil MCPServer")
	}
}

// TestNewServer_VersionPassedThrough はバージョン文字列が正しく渡されることを確認する。
// (ListTools がサーバー名/バージョンを公開しないため、実質的に NewServer が panic しないことを確認)
func TestNewServer_VersionPassedThrough(t *testing.T) {
	mock := backlog.NewMockClient()
	// dev バージョン
	s1 := mcpinternal.NewServer(mock, "dev", mcpinternal.ServerConfig{})
	if s1 == nil {
		t.Fatal("expected non-nil MCPServer for dev version")
	}
	// リリースバージョン
	s2 := mcpinternal.NewServer(mock, "1.2.3", mcpinternal.ServerConfig{})
	if s2 == nil {
		t.Fatal("expected non-nil MCPServer for release version")
	}
}

// M12-1: NewServerWithFactory が nil でないサーバーを返すこと
func TestNewServerWithFactory_ReturnsServer(t *testing.T) {
	factory := func(ctx context.Context) (backlog.Client, error) {
		return backlog.NewMockClient(), nil
	}
	s := mcpinternal.NewServerWithFactory(factory, "1.0.0", mcpinternal.ServerConfig{})
	if s == nil {
		t.Fatal("expected non-nil MCPServer")
	}
}

// M12-2: NewServerWithFactory で全ツールが登録されること（既存 NewServer と同じ 42 ツール）
func TestNewServerWithFactory_RegistersAllTools(t *testing.T) {
	factory := func(ctx context.Context) (backlog.Client, error) {
		return backlog.NewMockClient(), nil
	}
	s := mcpinternal.NewServerWithFactory(factory, "test", mcpinternal.ServerConfig{})

	tools := s.ListTools()
	expectedCount := 64
	if len(tools) != expectedCount {
		t.Errorf("expected %d tools, got %d", expectedCount, len(tools))
		for name := range tools {
			t.Logf("  tool: %s", name)
		}
	}
}

// M12 ctx 伝播検証用の key 型
type m12TestCtxKey struct{}

// M12-3: ツール呼び出し時に factory が呼ばれ、ctx が正しく伝播する
func TestNewServerWithFactory_FactoryCalledOnToolInvocation(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
		return &domain.Issue{
			ID:       1,
			IssueKey: issueKey,
			Summary:  "factory invocation test",
		}, nil
	}

	factoryCalled := false
	var capturedSentinel any
	factory := func(ctx context.Context) (backlog.Client, error) {
		factoryCalled = true
		capturedSentinel = ctx.Value(m12TestCtxKey{})
		return mock, nil
	}

	s := mcpinternal.NewServerWithFactory(factory, "test", mcpinternal.ServerConfig{})

	// sentinel 値付きの ctx をツール呼び出しに渡す
	ctx := context.WithValue(context.Background(), m12TestCtxKey{}, "sentinel-value")
	result := callToolWithCtx(t, s, ctx, "logvalet_issue_get", map[string]any{"issue_key": "TEST-1"})

	if !factoryCalled {
		t.Error("expected factory to be called on tool invocation")
	}
	if capturedSentinel != "sentinel-value" {
		t.Errorf("expected factory to receive ctx with sentinel-value, got %v", capturedSentinel)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	textContent, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	var issue domain.Issue
	if err := json.Unmarshal([]byte(textContent.Text), &issue); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if issue.IssueKey != "TEST-1" {
		t.Errorf("expected issue_key TEST-1, got %s", issue.IssueKey)
	}
}

// M12-4: factory がエラーを返した場合 → IsError: true
func TestNewServerWithFactory_FactoryError(t *testing.T) {
	factoryErr := errors.New("user not authenticated")
	factory := func(ctx context.Context) (backlog.Client, error) {
		return nil, factoryErr
	}

	s := mcpinternal.NewServerWithFactory(factory, "test", mcpinternal.ServerConfig{})
	result := callToolWithCtx(t, s, context.Background(), "logvalet_issue_get", map[string]any{"issue_key": "TEST-1"})

	if !result.IsError {
		t.Error("expected IsError=true when factory returns error")
	}
}

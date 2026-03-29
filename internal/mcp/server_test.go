package mcp_test

import (
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// TestNewServer_ReturnsServer は NewServer が nil でないサーバーを返すことを確認する。
func TestNewServer_ReturnsServer(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "1.0.0")
	if s == nil {
		t.Fatal("expected non-nil MCPServer")
	}
}

// TestNewServer_VersionPassedThrough はバージョン文字列が正しく渡されることを確認する。
// (ListTools がサーバー名/バージョンを公開しないため、実質的に NewServer が panic しないことを確認)
func TestNewServer_VersionPassedThrough(t *testing.T) {
	mock := backlog.NewMockClient()
	// dev バージョン
	s1 := mcpinternal.NewServer(mock, "dev")
	if s1 == nil {
		t.Fatal("expected non-nil MCPServer for dev version")
	}
	// リリースバージョン
	s2 := mcpinternal.NewServer(mock, "1.2.3")
	if s2 == nil {
		t.Fatal("expected non-nil MCPServer for release version")
	}
}

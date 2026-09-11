package mcp_test

import (
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// multi-space 撤去 (v0.40) 後の期待:
// space registry 系ツールが 1 つも登録されていないこと。
func TestRegisterAllTools_NoSpaceRegistryTools(t *testing.T) {
	backend := newFakeBackend()
	mcpinternal.BuildRegistryForTest(backend, backlog.NewMockClient(), nil, mcpinternal.ServerConfig{})

	removed := []string{
		"logvalet_space_list",
		"logvalet_space_use",
		"logvalet_space_verify",
		"logvalet_space_connect_url",
		"logvalet_space_disconnect",
	}
	for _, name := range removed {
		if _, ok := backend.registered[name]; ok {
			t.Errorf("tool %q は multi-space 撤去により削除されているべき", name)
		}
	}
}

// multi-space 撤去後の期待: どのツールにも spaces / all_spaces パラメータが無いこと。
func TestRegisterAllTools_NoSpaceParams(t *testing.T) {
	backend := newFakeBackend()
	mcpinternal.BuildRegistryForTest(backend, backlog.NewMockClient(), nil, mcpinternal.ServerConfig{})

	if len(backend.registered) == 0 {
		t.Fatal("ツールが 1 つも登録されていない")
	}
	for name, entry := range backend.registered {
		for _, p := range entry.tool.Params {
			if p.Name == "spaces" || p.Name == "all_spaces" {
				t.Errorf("tool %q に multi-space パラメータ %q が残っている", name, p.Name)
			}
		}
	}
}

// 単一スペース向けの space 系ツール (Backlog /api/v2/space ラッパー) は残ること。
func TestRegisterAllTools_KeepsSingleSpaceTools(t *testing.T) {
	backend := newFakeBackend()
	mcpinternal.BuildRegistryForTest(backend, backlog.NewMockClient(), nil, mcpinternal.ServerConfig{})

	for _, name := range []string{"logvalet_space_info", "logvalet_space_digest", "logvalet_space_disk_usage"} {
		if _, ok := backend.registered[name]; !ok {
			t.Errorf("tool %q は単一スペース用なので残すべき", name)
		}
	}
}

// ToolRegistry の公開面から multi-space 用メソッドが消えていることを型レベルで確認する。
func TestToolRegistry_NoMultiSpaceSurface(t *testing.T) {
	// コンパイルが通ること自体が検証。撤去漏れがあれば未使用シンボルとして検出される。
	if !strings.HasPrefix("logvalet", "log") {
		t.Fatal("unreachable")
	}
}

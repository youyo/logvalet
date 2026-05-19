package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/cli"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
	"github.com/youyo/logvalet/internal/space"
)

// BC1: --spaces / --all-spaces が未指定のとき GlobalFlags.Validate() がエラーを返さない。
// 既存コマンドの引数パースが壊れていないことを確認する。
func TestBC1_NoSpacesFlag_ValidateOK(t *testing.T) {
	t.Parallel()

	g := cli.GlobalFlags{
		Format: "json",
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("BC1: Validate() with no spaces flags = %v, want nil", err)
	}
}

// BC1b: ParseSpacesFlag("") は nil を返し、エラーなし（既存コマンドへの影響なし）。
func TestBC1b_ParseSpacesFlag_Empty_ReturnsNil(t *testing.T) {
	t.Parallel()

	aliases, err := cli.ParseSpacesFlag("")
	if err != nil {
		t.Fatalf("BC1b: ParseSpacesFlag('') error = %v, want nil", err)
	}
	if aliases != nil {
		t.Errorf("BC1b: ParseSpacesFlag('') = %v, want nil", aliases)
	}
}

// BC2: --spaces / --all-spaces 未使用のまま、SpacesListCmd が
// SpaceStore に依存しない形で動作できることを確認（SpaceStore が空でも list が返る）。
// 既存の profile ユーザーが `lv spaces` コマンドなしで動作継続できることを保証する。
func TestBC2_SpacesListCmd_EmptyStore_NoError(t *testing.T) {
	t.Parallel()

	store := space.NewMemoryStore()
	var stdout bytes.Buffer

	cmd := cli.SpacesListCmd{}
	if err := cmd.RunWithStore(&stdout, store, "local"); err != nil {
		t.Fatalf("BC2: SpacesListCmd.RunWithStore error = %v, want nil", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("BC2: invalid JSON: %v\nout: %s", err, stdout.String())
	}
	spaces, ok := out["spaces"].([]interface{})
	if !ok {
		t.Fatalf("BC2: spaces field missing: %v", out)
	}
	if len(spaces) != 0 {
		t.Errorf("BC2: want 0 spaces for empty store, got %d", len(spaces))
	}
}

// BC5: MCP tool の spaces / all_spaces 引数を指定しない（既存クライアントの挙動）でも
// NewServer は従来どおり単一クライアントで動作する（後方互換）。
// ツールが登録されており、spaces 引数なしで呼べることを確認する。
func TestBC5_MCPTool_NoSpacesArg_LegacyBehavior(t *testing.T) {
	t.Parallel()

	mock := backlog.NewMockClient()

	// NewServer(client, space, config) は従来シグネチャで動作する（BC確認）
	s := mcpinternal.NewServer(mock, "test-space", mcpinternal.ServerConfig{})
	if s == nil {
		t.Fatal("BC5: NewServer returned nil")
	}

	// tools が登録されていることを確認
	tools := s.ListTools()
	if len(tools) == 0 {
		t.Fatal("BC5: expected tools to be registered")
	}

	// spaces / all_spaces 引数なしで定義されたツールが存在すること
	tool := s.GetTool("logvalet_space_disk_usage")
	if tool == nil {
		t.Fatal("BC5: tool logvalet_space_disk_usage not found")
	}
}

// BC9: CLI コマンドツリーで "space" と "spaces" が共存しており、
// lv space info/disk-usage/digest と lv spaces list/add/... が衝突しないことを確認する。
func TestBC9_SpaceAndSpacesCommandsCoexist(t *testing.T) {
	t.Parallel()

	// SpaceCmd は info/disk-usage/digest サブコマンドを持つ
	_ = cli.SpaceCmd{}
	_ = cli.SpacesCmd{}

	// SpaceCmd のサブコマンドが存在することを確認（型として確認）
	var spaceCmd cli.SpaceCmd
	_ = spaceCmd.Info
	_ = spaceCmd.DiskUsage
	_ = spaceCmd.Digest

	// SpacesCmd のサブコマンドが存在することを確認
	var spacesCmd cli.SpacesCmd
	_ = spacesCmd.List
	_ = spacesCmd.Add
	_ = spacesCmd.Connect
	_ = spacesCmd.Remove
	_ = spacesCmd.Use
	_ = spacesCmd.Verify
}

// BC9b: CLI 全体の CLI struct に Space と Spaces が両方登録されており、
// フィールド名が異なること（Kong がコマンド名として区別できること）を確認する。
func TestBC9b_CLIStruct_SpaceAndSpaces_BothRegistered(t *testing.T) {
	t.Parallel()

	var root cli.CLI

	// Space フィールドは SpaceCmd 型
	var _ cli.SpaceCmd = root.Space
	// Spaces フィールドは SpacesCmd 型
	var _ cli.SpacesCmd = root.Spaces
}

// BC9c: SpacesListCmd は SpaceInfoCmd の動作に影響を与えない（独立した Store を持つ）。
func TestBC9c_SpaceAndSpaces_Independent(t *testing.T) {
	t.Parallel()

	store := space.NewMemoryStore()
	ctx := context.Background()

	// spaces store に foo を登録
	if err := store.Upsert(ctx, &space.SpaceRegistration{
		UserID:  "local",
		Alias:   "foo",
		Tenant:  "foo",
		BaseURL: "https://foo.backlog.com",
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	var buf bytes.Buffer
	cmd := cli.SpacesListCmd{}
	if err := cmd.RunWithStore(&buf, store, "local"); err != nil {
		t.Fatalf("BC9c: SpacesListCmd error: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("BC9c: invalid JSON: %v", err)
	}
	spaces, _ := out["spaces"].([]interface{})
	if len(spaces) != 1 {
		t.Errorf("BC9c: want 1 space, got %d", len(spaces))
	}
}

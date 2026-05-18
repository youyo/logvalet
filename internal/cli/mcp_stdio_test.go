package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/cli"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// T1: McpStdioCmd.Validate() は常に nil を返す。
func TestMcpStdioCmd_Validate(t *testing.T) {
	cmd := &cli.McpStdioCmd{}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// T2: mcp-stdio コマンドが Kong CLI 構造体に登録されている。
func TestMcpStdioCmd_RegisteredInCLI(t *testing.T) {
	parser, err := kong.New(&cli.CLI{})
	if err != nil {
		t.Fatalf("failed to create kong parser: %v", err)
	}
	found := false
	for _, node := range parser.Model.Node.Children {
		if node.Name == "mcp-stdio" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected mcp-stdio command to be registered in CLI")
	}
}

// T3: stdio MCP サーバーの stdout が JSON 行のみで構成される（stdout 汚染回帰防止）。
func TestMcpStdioCmd_StdoutContainsOnlyJSON(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})

	stdio := mcpserver.NewStdioServer(s)
	var stdout bytes.Buffer

	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}` + "\n"

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = stdio.Listen(ctx, strings.NewReader(req), &stdout)

	if stdout.Len() == 0 {
		t.Fatal("stdio server produced no output for initialize request")
	}

	for _, line := range bytes.Split(bytes.TrimRight(stdout.Bytes(), "\n"), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		if !json.Valid(line) {
			t.Errorf("non-JSON line in stdout: %q", line)
		}
	}
}

// T4: Listen が通常エラーを返した場合は interpretListenResult がそのエラーを伝播する。
func TestInterpretListenResult_PropagatesError(t *testing.T) {
	forcedErr := errors.New("listen failed")
	result := cli.InterpretListenResult(forcedErr, nil)
	if !errors.Is(result, forcedErr) {
		t.Errorf("expected forcedErr, got %v", result)
	}
}

// T5a: Listen が nil を返した場合は nil を返す。
func TestInterpretListenResult_NilError(t *testing.T) {
	if err := cli.InterpretListenResult(nil, nil); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// T5b: ctx がキャンセルされている場合はエラーがあっても nil を返す。
func TestInterpretListenResult_CtxCanceled(t *testing.T) {
	forcedErr := errors.New("listen failed")
	ctxErr := context.Canceled
	if err := cli.InterpretListenResult(forcedErr, ctxErr); err != nil {
		t.Errorf("expected nil on ctx cancel, got %v", err)
	}
}

// T5c: io.EOF の場合は nil を返す（Claude Desktop が stdin を閉じた場合）。
func TestInterpretListenResult_EOF(t *testing.T) {
	if err := cli.InterpretListenResult(io.EOF, nil); err != nil {
		t.Errorf("expected nil on EOF, got %v", err)
	}
}

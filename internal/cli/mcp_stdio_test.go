package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	officialmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/cli"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// syncBuffer は複数ゴルーチンから書き込まれる bytes.Buffer を安全に扱うための
// io.WriteCloser 実装（Close は no-op）。officialmcp.IOTransport.Writer として使う。
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) Close() error { return nil }

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *syncBuffer) responseCount() int {
	s := b.String()
	n := 0
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		if line != "" {
			n++
		}
	}
	return n
}

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

// jsonrpcLine は stdout の1行を initialize リクエスト用に組み立てる。
func newlineDelimited(lines ...string) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString("\n")
	}
	return b.String()
}

// runOfficialStdio は officialmcp.Server を IOTransport 経由で起動し、
// reqs (改行区切りの JSON-RPC リクエスト列) を書き込んだ後、id 付きリクエスト数
// (wantResponses) 分のレスポンス行が stdout に現れるまで待ってからクライアント側を
// EOF で切断する。stdout に書かれたレスポンス群 (改行区切り JSON) を返す。
//
// リクエスト送信直後に Reader を EOF にすると、ハンドラーの応答書き込みと
// コネクションクローズが競合し "server is closing: EOF" のようなエラーになりうる
// ため、レスポンスが出揃うのを確認してから切断する。
func runOfficialStdio(t *testing.T, s *officialmcp.Server, reqs string, wantResponses int) (string, error) {
	t.Helper()
	pr, pw := io.Pipe()
	stdout := &syncBuffer{}
	transport := &officialmcp.IOTransport{
		Reader: pr,
		Writer: stdout,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- s.Run(ctx, transport)
	}()

	if _, err := io.WriteString(pw, reqs); err != nil {
		t.Fatalf("write requests: %v", err)
	}

	deadline := time.Now().Add(4 * time.Second)
	for stdout.responseCount() < wantResponses {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d response(s); got: %q", wantResponses, stdout.String())
		}
		time.Sleep(5 * time.Millisecond)
	}

	_ = pw.Close() // クライアント切断を模擬 (EOF)

	err := <-runErrCh
	return stdout.String(), cli.InterpretListenResult(err, ctx.Err())
}

// T3: 公式 SDK ベースの stdio サーバーの stdout が JSON 行のみで構成される
// （stdout 汚染回帰防止）。tools/list に応答できることも合わせて確認する。
func TestMcpStdioCmd_StdoutContainsOnlyJSON(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewOfficialServer(mock, "test", mcpinternal.ServerConfig{})

	reqs := newlineDelimited(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	)

	stdout, err := runOfficialStdio(t, s, reqs, 2)
	if err != nil {
		t.Fatalf("unexpected error from Run: %v", err)
	}
	if stdout == "" {
		t.Fatal("stdio server produced no output for initialize/tools-list requests")
	}

	for _, line := range bytes.Split(bytes.TrimRight([]byte(stdout), "\n"), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		if !json.Valid(line) {
			t.Errorf("non-JSON line in stdout: %q", line)
		}
	}
}

// T6: DisableFilePaths=true の stdio サーバーで file_paths を使った添付アップロードを
// 呼び出すと、tools/call がプロトコルエラーではなく IsError=true のツールエラーとして
// 拒否されることを確認する（S10 done_criteria: 公式 SDK 移行後も DisableFilePaths の
// 挙動が維持されること）。
func TestMcpStdioCmd_DisableFilePaths_RejectsFilePaths(t *testing.T) {
	mock := backlog.NewMockClient()
	s := mcpinternal.NewOfficialServer(mock, "test", mcpinternal.ServerConfig{DisableFilePaths: true})

	reqs := newlineDelimited(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"logvalet_issue_attachment_upload","arguments":{"issue_key":"PROJ-1","file_paths":"/etc/passwd"}}}`,
	)

	stdout, err := runOfficialStdio(t, s, reqs, 2)
	if err != nil {
		t.Fatalf("unexpected error from Run: %v", err)
	}

	var toolCallResponse struct {
		ID     int `json:"id"`
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	found := false
	for _, line := range bytes.Split(bytes.TrimRight([]byte(stdout), "\n"), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var probe struct {
			ID int `json:"id"`
		}
		if jsonErr := json.Unmarshal(line, &probe); jsonErr == nil && probe.ID == 2 {
			if jsonErr := json.Unmarshal(line, &toolCallResponse); jsonErr != nil {
				t.Fatalf("unmarshal tools/call response: %v; line=%s", jsonErr, line)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no response with id=2 (tools/call) found in stdout: %q", stdout)
	}
	if toolCallResponse.Error != nil {
		t.Fatalf("unexpected protocol-level error (should be a tool-level error): %+v", toolCallResponse.Error)
	}
	if !toolCallResponse.Result.IsError {
		t.Fatal("expected tool error when file_paths is used with DisableFilePaths=true")
	}
	if mock.GetCallCount("UploadAttachment") != 0 {
		t.Errorf("UploadAttachment must not be called when file_paths is disabled, got %d", mock.GetCallCount("UploadAttachment"))
	}
	msg := ""
	if len(toolCallResponse.Result.Content) > 0 {
		msg = toolCallResponse.Result.Content[0].Text
	}
	if !strings.Contains(msg, "file_content_base64") {
		t.Errorf("error message should mention file_content_base64 as alternative; got: %s", msg)
	}
}

// T4: Run が通常エラーを返した場合は InterpretListenResult がそのエラーを伝播する。
func TestInterpretListenResult_PropagatesError(t *testing.T) {
	forcedErr := errors.New("listen failed")
	result := cli.InterpretListenResult(forcedErr, nil)
	if !errors.Is(result, forcedErr) {
		t.Errorf("expected forcedErr, got %v", result)
	}
}

// T5a: Run が nil を返した場合は nil を返す。
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

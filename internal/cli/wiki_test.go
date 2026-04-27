package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// ---- WikiListCmd ----

func TestWikiListCmd_DryRun(t *testing.T) {
	cmd := &WikiListCmd{
		ProjectKey: "TEST",
	}
	// dry-run は WikiListCmd にはないが、buildRunContext 失敗を利用してエラーを確認する
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Error("Run() should return error when config not available")
	}
}

// ---- WikiCountCmd ----

func TestWikiCountCmd_NotAvailableWithoutConfig(t *testing.T) {
	cmd := &WikiCountCmd{ProjectKey: "TEST"}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Error("Run() should return error when config not available")
	}
}

// ---- WikiTagsCmd ----

func TestWikiTagsCmd_NotAvailableWithoutConfig(t *testing.T) {
	cmd := &WikiTagsCmd{ProjectKey: "TEST"}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Error("Run() should return error when config not available")
	}
}

// ---- WikiGetCmd ----

func TestWikiGetCmd_NotAvailableWithoutConfig(t *testing.T) {
	cmd := &WikiGetCmd{WikiID: 1}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Error("Run() should return error when config not available")
	}
}

// ---- WikiHistoryCmd ----

func TestWikiHistoryCmd_NotAvailableWithoutConfig(t *testing.T) {
	cmd := &WikiHistoryCmd{WikiID: 1}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Error("Run() should return error when config not available")
	}
}

// ---- WikiStarsCmd ----

func TestWikiStarsCmd_NotAvailableWithoutConfig(t *testing.T) {
	cmd := &WikiStarsCmd{WikiID: 1}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Error("Run() should return error when config not available")
	}
}

// ---- WikiAttachmentListCmd ----

func TestWikiAttachmentListCmd_NotAvailableWithoutConfig(t *testing.T) {
	cmd := &WikiAttachmentListCmd{WikiID: 1}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Error("Run() should return error when config not available")
	}
}

// ---- WikiSharedFileListCmd ----

func TestWikiSharedFileListCmd_NotAvailableWithoutConfig(t *testing.T) {
	cmd := &WikiSharedFileListCmd{WikiID: 1}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Error("Run() should return error when config not available")
	}
}

// ---- ロジックテスト: mock 経由 ----

// TestWikiListCmd_MockRender は mock Client で ListWikis が呼ばれ JSON 出力されることを確認する。
func TestWikiListCmd_MockRender(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListWikisFunc = func(ctx context.Context, projectKey string, opt backlog.ListWikisOptions) ([]domain.WikiPage, error) {
		return []domain.WikiPage{
			{ID: 1, Name: "TopPage", Content: "hello"},
		}, nil
	}

	cmd := &WikiListCmd{ProjectKey: "TEST"}

	// stdout をキャプチャ
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	runErr := runWikiListWithClient(mock, cmd)

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck

	if runErr != nil {
		t.Fatalf("runWikiListWithClient() error = %v", runErr)
	}

	var pages []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &pages); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}
	if len(pages) != 1 {
		t.Fatalf("len(pages) = %d, want 1", len(pages))
	}
	if pages[0]["name"] != "TopPage" {
		t.Errorf("name = %v, want TopPage", pages[0]["name"])
	}
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	"github.com/youyo/logvalet/internal/render"
)

// runCapturingStdout は fn 実行中の os.Stdout への書き込みをキャプチャして返す。
func runCapturingStdout(t *testing.T, fn func() error) ([]byte, error) {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	runErr := fn()

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck
	return buf.Bytes(), runErr
}

// CLI-IR-1: issue related list は ListRelatedIssues を1回呼び、JSON配列を出力する。
func TestIssueRelatedListCmd_run(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListRelatedIssuesFunc = func(ctx context.Context, issueKey string) ([]domain.RelatedIssue, error) {
		if issueKey != "PROJ-1" {
			t.Errorf("issueKey = %q, want PROJ-1", issueKey)
		}
		return []domain.RelatedIssue{
			{Issue: domain.Issue{ID: 10, IssueKey: "PROJ-2"}, Type: "RELATES"},
		}, nil
	}

	renderer, err := render.NewRenderer("json", false, "")
	if err != nil {
		t.Fatalf("render.NewRenderer error: %v", err)
	}

	cmd := &IssueRelatedListCmd{IssueIDOrKey: "PROJ-1"}
	var out bytes.Buffer
	if err := cmd.run(context.Background(), mc, renderer, &out); err != nil {
		t.Fatalf("run() error: %v", err)
	}

	if got := mc.GetCallCount("ListRelatedIssues"); got != 1 {
		t.Errorf("ListRelatedIssues call count = %d, want 1", got)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out.String())
	}
	if len(result) != 1 {
		t.Fatalf("result length = %d, want 1", len(result))
	}
}

// CLI-IR-2: issue related add は AddRelatedIssue に target ID を渡す。
func TestIssueRelatedAddCmd_run(t *testing.T) {
	mc := backlog.NewMockClient()
	var capturedReq backlog.AddRelatedIssueRequest
	mc.AddRelatedIssueFunc = func(ctx context.Context, issueKey string, req backlog.AddRelatedIssueRequest) (*domain.RelatedIssue, error) {
		capturedReq = req
		return &domain.RelatedIssue{Issue: domain.Issue{ID: int(req.TargetIssueID)}, Type: "RELATES"}, nil
	}

	renderer, err := render.NewRenderer("json", false, "")
	if err != nil {
		t.Fatalf("render.NewRenderer error: %v", err)
	}

	cmd := &IssueRelatedAddCmd{IssueIDOrKey: "PROJ-1", TargetIssueID: 42}
	var out bytes.Buffer
	if err := cmd.run(context.Background(), mc, renderer, &out); err != nil {
		t.Fatalf("run() error: %v", err)
	}

	if got := mc.GetCallCount("AddRelatedIssue"); got != 1 {
		t.Errorf("AddRelatedIssue call count = %d, want 1", got)
	}
	if capturedReq.TargetIssueID != 42 {
		t.Errorf("TargetIssueID = %d, want 42", capturedReq.TargetIssueID)
	}
}

// CLI-IR-3: issue related remove は DeleteRelatedIssue に related-issue-id を渡す。
func TestIssueRelatedRemoveCmd_run(t *testing.T) {
	mc := backlog.NewMockClient()
	var capturedID int64
	mc.DeleteRelatedIssueFunc = func(ctx context.Context, issueKey string, relatedIssueID int64) (*domain.RelatedIssue, error) {
		capturedID = relatedIssueID
		return &domain.RelatedIssue{Issue: domain.Issue{ID: int(relatedIssueID)}, Type: "RELATES"}, nil
	}

	renderer, err := render.NewRenderer("json", false, "")
	if err != nil {
		t.Fatalf("render.NewRenderer error: %v", err)
	}

	cmd := &IssueRelatedRemoveCmd{IssueIDOrKey: "PROJ-1", RelatedIssueID: 99}
	var out bytes.Buffer
	if err := cmd.run(context.Background(), mc, renderer, &out); err != nil {
		t.Fatalf("run() error: %v", err)
	}

	if got := mc.GetCallCount("DeleteRelatedIssue"); got != 1 {
		t.Errorf("DeleteRelatedIssue call count = %d, want 1", got)
	}
	if capturedID != 99 {
		t.Errorf("relatedIssueID = %d, want 99", capturedID)
	}
}

// CLI-IR-4: add --dry-run は formatDryRun の JSON エコーのみ出力し Client を呼ばない。
// DryRun=true の場合 Run() は buildRunContext / Client 呼び出しに一切到達しないため、
// Run() がエラーなく完了すること自体が Client 呼び出し0回であることの証跡になる
// （buildRunContext は本テスト環境の config なしでは必ずエラーになるため、
// 到達すれば即座に検出できる）。
func TestIssueRelatedAddCmd_dry_run(t *testing.T) {
	cmd := &IssueRelatedAddCmd{
		WriteFlags:    WriteFlags{DryRun: true},
		IssueIDOrKey:  "PROJ-1",
		TargetIssueID: 42,
	}
	g := &GlobalFlags{}

	out, runErr := runCapturingStdout(t, func() error { return cmd.Run(g) })
	if runErr != nil {
		t.Fatalf("Run() error: %v", runErr)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if parsed["dry_run"] != true {
		t.Errorf("dry_run = %v, want true", parsed["dry_run"])
	}
	if parsed["operation"] != "add_related_issue" {
		t.Errorf("operation = %v, want add_related_issue", parsed["operation"])
	}
}

// CLI-IR-5: remove --dry-run は formatDryRun の JSON エコーのみ出力し Client を呼ばない
// （理由は CLI-IR-4 のコメントを参照）。
func TestIssueRelatedRemoveCmd_dry_run(t *testing.T) {
	cmd := &IssueRelatedRemoveCmd{
		WriteFlags:     WriteFlags{DryRun: true},
		IssueIDOrKey:   "PROJ-1",
		RelatedIssueID: 99,
	}
	g := &GlobalFlags{}

	out, runErr := runCapturingStdout(t, func() error { return cmd.Run(g) })
	if runErr != nil {
		t.Fatalf("Run() error: %v", runErr)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, out)
	}
	if parsed["dry_run"] != true {
		t.Errorf("dry_run = %v, want true", parsed["dry_run"])
	}
	if parsed["operation"] != "remove_related_issue" {
		t.Errorf("operation = %v, want remove_related_issue", parsed["operation"])
	}
}

// CLI-IR-6: Client エラーがそのまま伝播する。
func TestIssueRelatedListCmd_run_error(t *testing.T) {
	mc := backlog.NewMockClient()
	wantErr := errors.New("boom")
	mc.ListRelatedIssuesFunc = func(ctx context.Context, issueKey string) ([]domain.RelatedIssue, error) {
		return nil, wantErr
	}

	renderer, err := render.NewRenderer("json", false, "")
	if err != nil {
		t.Fatalf("render.NewRenderer error: %v", err)
	}

	cmd := &IssueRelatedListCmd{IssueIDOrKey: "PROJ-1"}
	var out bytes.Buffer
	err = cmd.run(context.Background(), mc, renderer, &out)
	if !errors.Is(err, wantErr) {
		t.Errorf("run() error = %v, want %v", err, wantErr)
	}
}

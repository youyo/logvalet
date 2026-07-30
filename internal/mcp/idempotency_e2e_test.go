package mcp_test

import (
	"context"
	"io"
	"sync/atomic"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// idempotency_e2e_test.go は S17 の done_criteria「CategoryWriteNonIdempotent の
// create 系ツールが同一 idempotency key で二重実行されないこと」を、実際の
// ToolRegistry 登録経路(RegisterWithSpacesWrite → callWithDefaultClient)を通した
// ハンドラー呼び出しレベルで検証する。ユニットレベルの IdempotencyCache 単体の
// 挙動は idempotency_test.go (package mcp) で担保済み。

// TestE2E_IssueCreate_DuplicateIdempotencyKey_ExecutesOnce は logvalet_issue_create を
// 同一 idempotency_key で2回呼び出しても、Backlog API (CreateIssueFunc) は1回しか
// 呼ばれず、2回目は初回のレスポンスがそのまま返ることを検証する。
func TestE2E_IssueCreate_DuplicateIdempotencyKey_ExecutesOnce(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "TEST"}, nil
	}
	var calls int32
	mock.CreateIssueFunc = func(ctx context.Context, req backlog.CreateIssueRequest) (*domain.Issue, error) {
		n := atomic.AddInt32(&calls, 1)
		return &domain.Issue{ID: int(n), IssueKey: "TEST-1"}, nil
	}

	s := newTestServer(t, mock, mcpinternal.ServerConfig{})
	args := map[string]any{
		"project_key":     "TEST",
		"summary":         "Test issue",
		"issue_type_id":   1,
		"idempotency_key": "dup-key-1",
	}

	r1 := callTool(t, s, "logvalet_issue_create", args)
	r2 := callTool(t, s, "logvalet_issue_create", args)

	if r1.IsError || r2.IsError {
		t.Fatalf("unexpected tool error: r1=%v r2=%v", r1.Content, r2.Content)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("CreateIssueFunc は同一 idempotency_key で1回だけ呼ばれるべき, got %d", got)
	}
	if len(r1.Content) == 0 || len(r2.Content) == 0 || r1.Content[0].Text != r2.Content[0].Text {
		t.Fatalf("2回目は初回結果と同一のレスポンスを返すべき: r1=%q r2=%q", r1.Content[0].Text, r2.Content[0].Text)
	}
}

// TestE2E_IssueCreate_DifferentIdempotencyKey_ExecutesEachTime は idempotency_key が
// 異なれば通常どおり毎回実行されることを検証する（対照実験）。
func TestE2E_IssueCreate_DifferentIdempotencyKey_ExecutesEachTime(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "TEST"}, nil
	}
	var calls int32
	mock.CreateIssueFunc = func(ctx context.Context, req backlog.CreateIssueRequest) (*domain.Issue, error) {
		atomic.AddInt32(&calls, 1)
		return &domain.Issue{ID: 1, IssueKey: "TEST-1"}, nil
	}

	s := newTestServer(t, mock, mcpinternal.ServerConfig{})
	base := map[string]any{"project_key": "TEST", "summary": "Test issue", "issue_type_id": 1}

	args1 := map[string]any{}
	for k, v := range base {
		args1[k] = v
	}
	args1["idempotency_key"] = "key-a"
	args2 := map[string]any{}
	for k, v := range base {
		args2[k] = v
	}
	args2["idempotency_key"] = "key-b"

	callTool(t, s, "logvalet_issue_create", args1)
	callTool(t, s, "logvalet_issue_create", args2)

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("異なる idempotency_key はそれぞれ実行されるべき, got %d", got)
	}
}

// TestE2E_IssueCreate_NoIdempotencyKey_SameArgsDeduped は idempotency_key 省略時、
// 同一引数であれば引数ハッシュ代替により重複実行されないことを検証する。
func TestE2E_IssueCreate_NoIdempotencyKey_SameArgsDeduped(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetProjectFunc = func(ctx context.Context, projectKey string) (*domain.Project, error) {
		return &domain.Project{ID: 100, ProjectKey: "TEST"}, nil
	}
	var calls int32
	mock.CreateIssueFunc = func(ctx context.Context, req backlog.CreateIssueRequest) (*domain.Issue, error) {
		atomic.AddInt32(&calls, 1)
		return &domain.Issue{ID: 1, IssueKey: "TEST-1"}, nil
	}

	s := newTestServer(t, mock, mcpinternal.ServerConfig{})
	args := map[string]any{"project_key": "TEST", "summary": "Duplicate risk issue", "issue_type_id": 1}

	callTool(t, s, "logvalet_issue_create", args)
	callTool(t, s, "logvalet_issue_create", args)

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("idempotency_key 未指定でも同一引数なら1回だけ実行されるべき, got %d", got)
	}
}

// TestE2E_NonIdempotentTools_DuplicateKeyExecutesOnce は logvalet_issue_create 以外の
// CategoryWriteNonIdempotent 3ツール（issue_comment_add / document_create /
// issue_attachment_upload）についても、同一 idempotency_key の2回呼び出しで
// 実 API 呼び出しが1回に抑えられることを検証する。
func TestE2E_NonIdempotentTools_DuplicateKeyExecutesOnce(t *testing.T) {
	t.Run("issue_comment_add", func(t *testing.T) {
		mock := backlog.NewMockClient()
		var calls int32
		mock.AddIssueCommentFunc = func(ctx context.Context, issueKey string, req backlog.AddCommentRequest) (*domain.Comment, error) {
			atomic.AddInt32(&calls, 1)
			return &domain.Comment{ID: 1, Content: req.Content}, nil
		}
		s := newTestServer(t, mock, mcpinternal.ServerConfig{})
		args := map[string]any{
			"issue_key":       "TEST-1",
			"content":         "hello",
			"idempotency_key": "comment-key-1",
		}
		callTool(t, s, "logvalet_issue_comment_add", args)
		callTool(t, s, "logvalet_issue_comment_add", args)
		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Fatalf("AddIssueCommentFunc は1回だけ呼ばれるべき, got %d", got)
		}
	})

	t.Run("document_create", func(t *testing.T) {
		mock := backlog.NewMockClient()
		var calls int32
		mock.CreateDocumentFunc = func(ctx context.Context, req backlog.CreateDocumentRequest) (*domain.Document, error) {
			atomic.AddInt32(&calls, 1)
			return &domain.Document{ID: "doc-1", Title: req.Title}, nil
		}
		s := newTestServer(t, mock, mcpinternal.ServerConfig{})
		args := map[string]any{
			"project_id":      100,
			"title":           "Doc title",
			"content":         "Doc content",
			"idempotency_key": "doc-key-1",
		}
		callTool(t, s, "logvalet_document_create", args)
		callTool(t, s, "logvalet_document_create", args)
		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Fatalf("CreateDocumentFunc は1回だけ呼ばれるべき, got %d", got)
		}
	})

	t.Run("issue_attachment_upload", func(t *testing.T) {
		mock := backlog.NewMockClient()
		var calls int32
		mock.UploadAttachmentFunc = func(ctx context.Context, filename string, content io.Reader) (*domain.UploadedAttachment, error) {
			atomic.AddInt32(&calls, 1)
			return &domain.UploadedAttachment{ID: 1, Name: filename}, nil
		}
		mock.UpdateIssueFunc = func(ctx context.Context, issueKey string, req backlog.UpdateIssueRequest) (*domain.Issue, error) {
			return &domain.Issue{ID: 1, IssueKey: issueKey}, nil
		}
		s := newTestServer(t, mock, mcpinternal.ServerConfig{})
		args := map[string]any{
			"issue_key":           "TEST-1",
			"file_name":           "a.txt",
			"file_content_base64": "aGVsbG8=", // "hello"
			"idempotency_key":     "upload-key-1",
		}
		callTool(t, s, "logvalet_issue_attachment_upload", args)
		callTool(t, s, "logvalet_issue_attachment_upload", args)
		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Fatalf("UploadAttachmentFunc は1回だけ呼ばれるべき, got %d", got)
		}
	})
}

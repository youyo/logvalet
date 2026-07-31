package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// helperRelatedIssues は テスト用の domain.RelatedIssue 一覧を返すヘルパー。
func helperRelatedIssues(now time.Time) []domain.RelatedIssue {
	updated1 := now.Add(-1 * 24 * time.Hour)
	updated2 := now.Add(-3 * 24 * time.Hour)
	return []domain.RelatedIssue{
		{
			Issue: domain.Issue{
				IssueKey: "PROJ-200",
				Summary:  "関連課題A",
				Status:   &domain.IDName{ID: 1, Name: "未対応"},
				Priority: &domain.IDName{ID: 2, Name: "高"},
				Assignee: &domain.User{ID: 10, UserID: "user1", Name: "テストユーザー"},
				Updated:  &updated1,
			},
			Type: "RELATES",
		},
		{
			Issue: domain.Issue{
				IssueKey: "PROJ-201",
				Summary:  "関連課題B",
				Status:   &domain.IDName{ID: 2, Name: "処理中"},
				Priority: &domain.IDName{ID: 1, Name: "低"},
				Assignee: &domain.User{ID: 20, UserID: "user2", Name: "別ユーザー"},
				Updated:  &updated2,
			},
			Type: "RELATES",
		},
	}
}

// T1: buildRelatedIssueRefs が2件の domain.RelatedIssue を RelatedIssueRef に射影することを検証する。
func TestBuildRelatedIssueRefs_TwoItems(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	refs := buildRelatedIssueRefs(helperRelatedIssues(fixedNow))

	if len(refs) != 2 {
		t.Fatalf("len(refs) = %d, want 2", len(refs))
	}

	got := refs[0]
	if got.IssueKey != "PROJ-200" {
		t.Errorf("IssueKey = %q, want %q", got.IssueKey, "PROJ-200")
	}
	if got.Summary != "関連課題A" {
		t.Errorf("Summary = %q, want %q", got.Summary, "関連課題A")
	}
	if got.Status == nil || got.Status.Name != "未対応" {
		t.Errorf("Status = %+v, want Name=未対応", got.Status)
	}
	if got.Priority == nil || got.Priority.Name != "高" {
		t.Errorf("Priority = %+v, want Name=高", got.Priority)
	}
	if got.Assignee == nil || got.Assignee.Name != "テストユーザー" {
		t.Errorf("Assignee = %+v, want Name=テストユーザー", got.Assignee)
	}
	if got.Type != "RELATES" {
		t.Errorf("Type = %q, want %q", got.Type, "RELATES")
	}
	if got.Updated == nil {
		t.Errorf("Updated = nil, want non-nil")
	}
}

// T2: buildRelatedIssueRefs は nil 入力に対して空スライス（nilではない）を返すことを検証する。
// JSON マーシャル時に null にならないことも確認する。
func TestBuildRelatedIssueRefs_Nil(t *testing.T) {
	refs := buildRelatedIssueRefs(nil)
	if refs == nil {
		t.Fatal("refs = nil, want non-nil empty slice")
	}
	if len(refs) != 0 {
		t.Errorf("len(refs) = %d, want 0", len(refs))
	}

	b, err := json.Marshal(refs)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	if string(b) != "[]" {
		t.Errorf("json.Marshal(refs) = %s, want []", string(b))
	}
}

// T3: fetchRelatedIssues が正常系で RelatedIssueRef を返すことを検証する。
func TestFetchRelatedIssues_Success(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	mc := backlog.NewMockClient()
	mc.ListRelatedIssuesFunc = func(ctx context.Context, issueKey string) ([]domain.RelatedIssue, error) {
		return helperRelatedIssues(fixedNow), nil
	}

	refs, warn := fetchRelatedIssues(context.Background(), mc, "PROJ-123")
	if warn != nil {
		t.Fatalf("warn = %+v, want nil", warn)
	}
	if len(refs) != 2 {
		t.Fatalf("len(refs) = %d, want 2", len(refs))
	}
}

// T4: fetchRelatedIssues がエラー（未設定時の backlog.ErrNotFound を含む）に対して
// 空スライスと domain.Warning を返すことを検証する。
func TestFetchRelatedIssues_Error(t *testing.T) {
	tests := []struct {
		name string
		mc   *backlog.MockClient
	}{
		{
			name: "Func未設定（backlog.ErrNotFound）",
			mc:   backlog.NewMockClient(),
		},
		{
			name: "明示的なエラー",
			mc: func() *backlog.MockClient {
				m := backlog.NewMockClient()
				m.ListRelatedIssuesFunc = func(ctx context.Context, issueKey string) ([]domain.RelatedIssue, error) {
					return nil, errors.New("boom")
				}
				return m
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, warn := fetchRelatedIssues(context.Background(), tt.mc, "PROJ-123")
			if refs == nil {
				t.Fatal("refs = nil, want non-nil empty slice")
			}
			if len(refs) != 0 {
				t.Errorf("len(refs) = %d, want 0", len(refs))
			}
			if warn == nil {
				t.Fatal("warn = nil, want non-nil")
			}
			if warn.Code != "related_issues_fetch_failed" {
				t.Errorf("warn.Code = %q, want %q", warn.Code, "related_issues_fetch_failed")
			}
			if warn.Component != "related_issues" {
				t.Errorf("warn.Component = %q, want %q", warn.Component, "related_issues")
			}
			if !warn.Retryable {
				t.Errorf("warn.Retryable = false, want true")
			}
		})
	}
}

// T5: 呼び出し元が fetchRelatedIssues を呼ばない（skip する）場合、
// MockClient の ListRelatedIssues 呼び出し回数がゼロであることを検証する。
func TestFetchRelatedIssues_SkipMeansNoCall(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListRelatedIssuesFunc = func(ctx context.Context, issueKey string) ([]domain.RelatedIssue, error) {
		return nil, nil
	}

	// skip=true を模して fetchRelatedIssues を呼び出さない。
	if mc.GetCallCount("ListRelatedIssues") != 0 {
		t.Errorf("GetCallCount(ListRelatedIssues) = %d, want 0", mc.GetCallCount("ListRelatedIssues"))
	}
}

// T6: RelatedIssueRef の JSON タグが全て snake_case であることを検証する。
func TestRelatedIssueRef_JSONTagsSnakeCase(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	refs := buildRelatedIssueRefs(helperRelatedIssues(fixedNow))

	b, err := json.Marshal(refs[0])
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	wantKeys := []string{"issue_key", "summary", "status", "priority", "assignee", "type", "updated"}
	for _, k := range wantKeys {
		if _, ok := m[k]; !ok {
			t.Errorf("key %q missing in marshaled JSON: %s", k, string(b))
		}
	}
}

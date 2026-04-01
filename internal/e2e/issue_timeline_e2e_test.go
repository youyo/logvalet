//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/analysis"
)

// TestE2E_IssueTimeline は実 Backlog API を使って issue timeline を取得する E2E テスト。
func TestE2E_IssueTimeline(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	builder := analysis.NewCommentTimelineBuilder(client, "default", env.Space, baseURL)

	ctx := context.Background()
	envelope, err := builder.Build(ctx, env.IssueKey, analysis.CommentTimelineOptions{
		MaxComments: 10,
	})
	if err != nil {
		t.Fatalf("CommentTimelineBuilder.Build() エラー: %v", err)
	}

	// AnalysisEnvelope 基本検証
	if envelope == nil {
		t.Fatal("envelope が nil")
	}
	if envelope.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want %q", envelope.SchemaVersion, "1")
	}
	if envelope.Resource != "issue_timeline" {
		t.Errorf("Resource = %q, want %q", envelope.Resource, "issue_timeline")
	}
	if envelope.Space != env.Space {
		t.Errorf("Space = %q, want %q", envelope.Space, env.Space)
	}
	if envelope.BaseURL != baseURL {
		t.Errorf("BaseURL = %q, want %q", envelope.BaseURL, baseURL)
	}
	if envelope.GeneratedAt.IsZero() {
		t.Error("GeneratedAt がゼロ値")
	}
	if envelope.Analysis == nil {
		t.Fatal("Analysis フィールドが nil")
	}
	if envelope.Warnings == nil {
		t.Error("Warnings が nil（空スライスであるべき）")
	}

	// CommentTimeline 型アサーション
	ct, ok := envelope.Analysis.(*analysis.CommentTimeline)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.CommentTimeline", envelope.Analysis)
	}

	// IssueKey の検証
	if ct.IssueKey == "" {
		t.Error("CommentTimeline.IssueKey が空")
	}

	// Events スライスが nil でないこと
	if ct.Events == nil {
		t.Error("CommentTimeline.Events が nil（空スライスであるべき）")
	}

	// Meta.TotalEvents が len(Events) と一致すること
	if ct.Meta.TotalEvents != len(ct.Events) {
		t.Errorf("Meta.TotalEvents = %d, len(Events) = %d（不一致）",
			ct.Meta.TotalEvents, len(ct.Events))
	}

	// CommentCount + UpdateCount は TotalEvents 以下であること
	if ct.Meta.CommentCount+ct.Meta.UpdateCount > ct.Meta.TotalEvents {
		t.Errorf("CommentCount(%d) + UpdateCount(%d) > TotalEvents(%d)（不正）",
			ct.Meta.CommentCount, ct.Meta.UpdateCount, ct.Meta.TotalEvents)
	}

	t.Logf("取得した課題: %s - %s", ct.IssueKey, ct.IssueSummary)
	t.Logf("total_events=%d, comments=%d, updates=%d, participants=%d",
		ct.Meta.TotalEvents, ct.Meta.CommentCount, ct.Meta.UpdateCount, ct.Meta.ParticipantCount)
}

// TestE2E_IssueTimeline_NoUpdates は --no-include-updates オプションの E2E テスト。
func TestE2E_IssueTimeline_NoUpdates(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	builder := analysis.NewCommentTimelineBuilder(client, "default", env.Space, baseURL)

	falseVal := false
	ctx := context.Background()
	envelope, err := builder.Build(ctx, env.IssueKey, analysis.CommentTimelineOptions{
		IncludeUpdates: &falseVal,
	})
	if err != nil {
		t.Fatalf("CommentTimelineBuilder.Build(IncludeUpdates=false) エラー: %v", err)
	}

	if envelope == nil || envelope.Analysis == nil {
		t.Fatal("envelope または Analysis が nil")
	}

	ct, ok := envelope.Analysis.(*analysis.CommentTimeline)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.CommentTimeline", envelope.Analysis)
	}

	// IncludeUpdates=false の場合、全イベントが kind=comment であること
	for i, e := range ct.Events {
		if e.Kind != analysis.TimelineEventKindComment {
			t.Errorf("Events[%d].Kind = %q, want %q（IncludeUpdates=false なのに update イベントが含まれている）",
				i, e.Kind, analysis.TimelineEventKindComment)
		}
	}

	// UpdateCount は 0 であること
	if ct.Meta.UpdateCount != 0 {
		t.Errorf("Meta.UpdateCount = %d, want 0（IncludeUpdates=false）", ct.Meta.UpdateCount)
	}

	t.Logf("NoUpdates モード: total_events=%d, comments=%d",
		ct.Meta.TotalEvents, ct.Meta.CommentCount)
}

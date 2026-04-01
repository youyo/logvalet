//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/analysis"
)

// TestE2E_IssueContext は実 Backlog API を使って issue context を取得する E2E テスト。
func TestE2E_IssueContext(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	builder := analysis.NewIssueContextBuilder(client, "default", env.Space, baseURL)

	ctx := context.Background()
	envelope, err := builder.Build(ctx, env.IssueKey, analysis.IssueContextOptions{
		MaxComments: 5,
	})
	if err != nil {
		t.Fatalf("IssueContextBuilder.Build() エラー: %v", err)
	}

	// AnalysisEnvelope 基本検証
	if envelope == nil {
		t.Fatal("envelope が nil")
	}
	if envelope.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want %q", envelope.SchemaVersion, "1")
	}
	if envelope.Resource != "issue_context" {
		t.Errorf("Resource = %q, want %q", envelope.Resource, "issue_context")
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

	// IssueContext 型アサーション
	ic, ok := envelope.Analysis.(*analysis.IssueContext)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.IssueContext", envelope.Analysis)
	}

	// issue_key の検証
	if ic.Issue.IssueKey == "" {
		t.Error("IssueContext.Issue.IssueKey が空")
	}
	t.Logf("取得した課題: %s - %s", ic.Issue.IssueKey, ic.Issue.Summary)
	t.Logf("stale=%v, overdue=%v, days_since_update=%d",
		ic.Signals.IsStale, ic.Signals.IsOverdue, ic.Signals.DaysSinceUpdate)
}

// TestE2E_IssueContext_Compact は Compact モードの E2E テスト。
func TestE2E_IssueContext_Compact(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	builder := analysis.NewIssueContextBuilder(client, "default", env.Space, baseURL)

	ctx := context.Background()
	envelope, err := builder.Build(ctx, env.IssueKey, analysis.IssueContextOptions{
		MaxComments: 3,
		Compact:     true,
	})
	if err != nil {
		t.Fatalf("IssueContextBuilder.Build(Compact) エラー: %v", err)
	}

	if envelope == nil || envelope.Analysis == nil {
		t.Fatal("envelope または Analysis が nil")
	}

	ic, ok := envelope.Analysis.(*analysis.IssueContext)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.IssueContext", envelope.Analysis)
	}

	// Compact モード: Description は空
	if ic.Issue.Description != "" {
		t.Errorf("Compact モード: Description = %q, want 空文字", ic.Issue.Description)
	}

	// コメントの Content も空
	for i, c := range ic.RecentComments {
		if c.Content != "" {
			t.Errorf("Compact モード: RecentComments[%d].Content = %q, want 空文字", i, c.Content)
		}
	}
}

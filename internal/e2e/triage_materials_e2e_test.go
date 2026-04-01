//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/analysis"
)

// TestE2E_TriageMaterials は実 Backlog API を使って triage-materials を生成する E2E テスト。
func TestE2E_TriageMaterials(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	builder := analysis.NewTriageMaterialsBuilder(client, "default", env.Space, baseURL)

	ctx := context.Background()
	envelope, err := builder.Build(ctx, env.IssueKey, analysis.TriageMaterialsOptions{})
	if err != nil {
		t.Fatalf("TriageMaterialsBuilder.Build() エラー: %v", err)
	}

	// AnalysisEnvelope 基本検証
	if envelope == nil {
		t.Fatal("envelope が nil")
	}
	if envelope.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want %q", envelope.SchemaVersion, "1")
	}
	if envelope.Resource != "issue_triage_materials" {
		t.Errorf("Resource = %q, want %q", envelope.Resource, "issue_triage_materials")
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

	// TriageMaterials 型アサーション
	tm, ok := envelope.Analysis.(*analysis.TriageMaterials)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.TriageMaterials", envelope.Analysis)
	}

	// Issue フィールド検証
	if tm.Issue.IssueKey == "" {
		t.Error("TriageMaterials.Issue.IssueKey が空")
	}
	if tm.Issue.Summary == "" {
		t.Error("TriageMaterials.Issue.Summary が空")
	}

	// Categories/Milestones は nil でないこと（空スライスであるべき）
	if tm.Issue.Categories == nil {
		t.Error("TriageMaterials.Issue.Categories が nil（空スライスであるべき）")
	}
	if tm.Issue.Milestones == nil {
		t.Error("TriageMaterials.Issue.Milestones が nil（空スライスであるべき）")
	}

	// ProjectStats は non-negative であること
	if tm.ProjectStats.TotalIssues < 0 {
		t.Errorf("ProjectStats.TotalIssues = %d（負の値は不正）", tm.ProjectStats.TotalIssues)
	}

	// SimilarIssues は non-negative であること
	if tm.SimilarIssues.SameCategoryCount < 0 {
		t.Errorf("SimilarIssues.SameCategoryCount = %d（負の値は不正）", tm.SimilarIssues.SameCategoryCount)
	}
	if tm.SimilarIssues.SameMilestoneCount < 0 {
		t.Errorf("SimilarIssues.SameMilestoneCount = %d（負の値は不正）", tm.SimilarIssues.SameMilestoneCount)
	}

	t.Logf("取得した課題: %s - %s", tm.Issue.IssueKey, tm.Issue.Summary)
	t.Logf("overdue=%v, stale=%v, days_since_created=%d, days_since_updated=%d",
		tm.History.IsOverdue, tm.History.IsStale,
		tm.History.DaysSinceCreated, tm.History.DaysSinceUpdated)
	t.Logf("プロジェクト総課題数: %d", tm.ProjectStats.TotalIssues)
	t.Logf("類似課題（同カテゴリ）: %d件, （同マイルストーン）: %d件",
		tm.SimilarIssues.SameCategoryCount, tm.SimilarIssues.SameMilestoneCount)
}

// TestE2E_TriageMaterials_CustomClosedStatus はカスタム完了ステータスを使用した E2E テスト。
func TestE2E_TriageMaterials_CustomClosedStatus(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	builder := analysis.NewTriageMaterialsBuilder(client, "default", env.Space, baseURL)

	ctx := context.Background()
	envelope, err := builder.Build(ctx, env.IssueKey, analysis.TriageMaterialsOptions{
		ClosedStatus: []string{"完了", "Closed"},
	})
	if err != nil {
		t.Fatalf("TriageMaterialsBuilder.Build(custom closed status) エラー: %v", err)
	}

	if envelope == nil || envelope.Analysis == nil {
		t.Fatal("envelope または Analysis が nil")
	}

	tm, ok := envelope.Analysis.(*analysis.TriageMaterials)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.TriageMaterials", envelope.Analysis)
	}

	// カスタム完了ステータス適用後も TotalIssues は取得できること
	if tm.ProjectStats.TotalIssues < 0 {
		t.Errorf("ProjectStats.TotalIssues = %d（負の値は不正）", tm.ProjectStats.TotalIssues)
	}

	t.Logf("カスタム完了ステータス適用後の avg_close_days: %.2f", tm.ProjectStats.AvgCloseDays)
}

//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/analysis"
)

// TestE2E_ProjectBlockers は実 Backlog API を使ってブロッカー課題を検出する E2E テスト。
func TestE2E_ProjectBlockers(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	detector := analysis.NewBlockerDetector(client, "default", env.Space, baseURL)

	cfg := analysis.BlockerConfig{
		InProgressDays: 14,
	}

	ctx := context.Background()
	envelope, err := detector.Detect(ctx, []string{env.ProjectKey}, cfg)
	if err != nil {
		t.Fatalf("BlockerDetector.Detect() エラー: %v", err)
	}

	// AnalysisEnvelope 基本検証
	if envelope == nil {
		t.Fatal("envelope が nil")
	}
	if envelope.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want %q", envelope.SchemaVersion, "1")
	}
	if envelope.Resource != "project_blockers" {
		t.Errorf("Resource = %q, want %q", envelope.Resource, "project_blockers")
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

	// BlockerResult 型アサーション
	result, ok := envelope.Analysis.(*analysis.BlockerResult)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.BlockerResult", envelope.Analysis)
	}

	// 基本フィールド検証
	if result.Issues == nil {
		t.Error("Issues が nil（空スライスであるべき）")
	}
	if result.TotalCount != len(result.Issues) {
		t.Errorf("TotalCount = %d, len(Issues) = %d （不一致）", result.TotalCount, len(result.Issues))
	}
	if result.BySeverity == nil {
		t.Error("BySeverity が nil")
	}

	// BySeverity に HIGH, MEDIUM が含まれることを確認
	if _, ok := result.BySeverity["HIGH"]; !ok {
		t.Error("BySeverity に HIGH キーがない")
	}
	if _, ok := result.BySeverity["MEDIUM"]; !ok {
		t.Error("BySeverity に MEDIUM キーがない")
	}

	t.Logf("プロジェクト %s のブロッカー課題数: %d（HIGH=%d, MEDIUM=%d）",
		env.ProjectKey, result.TotalCount,
		result.BySeverity["HIGH"], result.BySeverity["MEDIUM"])

	// 各ブロッカー課題の signals が空でないことを確認
	for i, issue := range result.Issues {
		if len(issue.Signals) == 0 {
			t.Errorf("BlockerIssues[%d] (%s): Signals が空", i, issue.IssueKey)
		}
		if issue.Severity != "HIGH" && issue.Severity != "MEDIUM" {
			t.Errorf("BlockerIssues[%d] (%s): Severity = %q, want HIGH or MEDIUM",
				i, issue.IssueKey, issue.Severity)
		}
	}
}

// TestE2E_ProjectBlockers_IncludeComments はコメントキーワード検出ありの E2E テスト。
func TestE2E_ProjectBlockers_IncludeComments(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	detector := analysis.NewBlockerDetector(client, "default", env.Space, baseURL)

	cfg := analysis.BlockerConfig{
		InProgressDays:  14,
		IncludeComments: true,
		MaxCommentCount: 3,
	}

	ctx := context.Background()
	envelope, err := detector.Detect(ctx, []string{env.ProjectKey}, cfg)
	if err != nil {
		t.Fatalf("BlockerDetector.Detect(IncludeComments=true) エラー: %v", err)
	}

	if envelope == nil || envelope.Analysis == nil {
		t.Fatal("envelope または Analysis が nil")
	}

	result, ok := envelope.Analysis.(*analysis.BlockerResult)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.BlockerResult", envelope.Analysis)
	}

	t.Logf("コメント検出ありでのブロッカー課題数: %d", result.TotalCount)
}

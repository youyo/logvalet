//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/analysis"
)

// TestE2E_IssueStale は実 Backlog API を使って停滞課題を検出する E2E テスト。
func TestE2E_IssueStale(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	detector := analysis.NewStaleIssueDetector(client, "default", env.Space, baseURL)

	cfg := analysis.StaleConfig{
		DefaultDays: 7,
	}

	ctx := context.Background()
	envelope, err := detector.Detect(ctx, []string{env.ProjectKey}, cfg)
	if err != nil {
		t.Fatalf("StaleIssueDetector.Detect() エラー: %v", err)
	}

	// AnalysisEnvelope 基本検証
	if envelope == nil {
		t.Fatal("envelope が nil")
	}
	if envelope.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want %q", envelope.SchemaVersion, "1")
	}
	if envelope.Resource != "stale_issues" {
		t.Errorf("Resource = %q, want %q", envelope.Resource, "stale_issues")
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

	// StaleIssueResult 型アサーション
	result, ok := envelope.Analysis.(*analysis.StaleIssueResult)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.StaleIssueResult", envelope.Analysis)
	}

	// 基本フィールド検証
	if result.ThresholdDays != 7 {
		t.Errorf("ThresholdDays = %d, want 7", result.ThresholdDays)
	}
	if result.Issues == nil {
		t.Error("Issues が nil（空スライスであるべき）")
	}
	if result.TotalCount != len(result.Issues) {
		t.Errorf("TotalCount = %d, len(Issues) = %d （不一致）", result.TotalCount, len(result.Issues))
	}

	t.Logf("プロジェクト %s の停滞課題数: %d（閾値 %d 日）",
		env.ProjectKey, result.TotalCount, result.ThresholdDays)
}

// TestE2E_IssueStale_CustomDays はカスタム閾値での E2E テスト。
func TestE2E_IssueStale_CustomDays(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	detector := analysis.NewStaleIssueDetector(client, "default", env.Space, baseURL)

	// 1日閾値（ほぼ全課題が stale になる）
	cfg := analysis.StaleConfig{
		DefaultDays: 1,
	}

	ctx := context.Background()
	envelope, err := detector.Detect(ctx, []string{env.ProjectKey}, cfg)
	if err != nil {
		t.Fatalf("StaleIssueDetector.Detect(days=1) エラー: %v", err)
	}

	result, ok := envelope.Analysis.(*analysis.StaleIssueResult)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.StaleIssueResult", envelope.Analysis)
	}

	if result.ThresholdDays != 1 {
		t.Errorf("ThresholdDays = %d, want 1", result.ThresholdDays)
	}

	t.Logf("閾値 1 日での停滞課題数: %d", result.TotalCount)
}

//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/analysis"
)

// TestE2E_ProjectHealth は実 Backlog API を使ってプロジェクト健全性を評価する E2E テスト。
func TestE2E_ProjectHealth(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	builder := analysis.NewProjectHealthBuilder(client, "default", env.Space, baseURL)

	cfg := analysis.ProjectHealthConfig{
		StaleConfig: analysis.StaleConfig{
			DefaultDays: 7,
		},
		BlockerConfig: analysis.BlockerConfig{
			InProgressDays: 14,
		},
		WorkloadConfig: analysis.WorkloadConfig{
			StaleDays: 7,
		},
	}

	ctx := context.Background()
	envelope, err := builder.Build(ctx, env.ProjectKey, cfg)
	if err != nil {
		t.Fatalf("ProjectHealthBuilder.Build() エラー: %v", err)
	}

	// AnalysisEnvelope 基本検証
	if envelope == nil {
		t.Fatal("envelope が nil")
	}
	if envelope.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want %q", envelope.SchemaVersion, "1")
	}
	if envelope.Resource != "project_health" {
		t.Errorf("Resource = %q, want %q", envelope.Resource, "project_health")
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

	// ProjectHealthResult 型アサーション
	result, ok := envelope.Analysis.(*analysis.ProjectHealthResult)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.ProjectHealthResult", envelope.Analysis)
	}

	// 基本フィールド検証
	if result.ProjectKey != env.ProjectKey {
		t.Errorf("ProjectKey = %q, want %q", result.ProjectKey, env.ProjectKey)
	}

	// HealthScore は 0-100 の範囲
	if result.HealthScore < 0 || result.HealthScore > 100 {
		t.Errorf("HealthScore = %d, want 0-100 の範囲", result.HealthScore)
	}

	// HealthLevel は "healthy" | "warning" | "critical" のいずれか
	validLevels := map[string]bool{
		"healthy": true, "warning": true, "critical": true,
	}
	if !validLevels[result.HealthLevel] {
		t.Errorf("HealthLevel = %q, want healthy|warning|critical", result.HealthLevel)
	}

	// score と level の一貫性
	switch {
	case result.HealthScore >= 80 && result.HealthLevel != "healthy":
		t.Errorf("HealthScore=%d >= 80 なのに HealthLevel=%q (want healthy)", result.HealthScore, result.HealthLevel)
	case result.HealthScore >= 60 && result.HealthScore < 80 && result.HealthLevel != "warning":
		t.Errorf("HealthScore=%d (60-79) なのに HealthLevel=%q (want warning)", result.HealthScore, result.HealthLevel)
	case result.HealthScore < 60 && result.HealthLevel != "critical":
		t.Errorf("HealthScore=%d < 60 なのに HealthLevel=%q (want critical)", result.HealthScore, result.HealthLevel)
	}

	// StaleSummary 検証
	if result.StaleSummary.TotalCount < 0 {
		t.Errorf("StaleSummary.TotalCount = %d, 0以上であるべき", result.StaleSummary.TotalCount)
	}

	// BlockerSummary 検証
	if result.BlockerSummary.TotalCount < 0 {
		t.Errorf("BlockerSummary.TotalCount = %d, 0以上であるべき", result.BlockerSummary.TotalCount)
	}

	// WorkloadSummary 検証
	if result.WorkloadSummary.TotalIssues < 0 {
		t.Errorf("WorkloadSummary.TotalIssues = %d, 0以上であるべき", result.WorkloadSummary.TotalIssues)
	}

	t.Logf("プロジェクト %s の健全性: score=%d, level=%s",
		env.ProjectKey, result.HealthScore, result.HealthLevel)
	t.Logf("  停滞課題: %d件, ブロッカー: %d件（HIGH=%d）, 総課題: %d件",
		result.StaleSummary.TotalCount,
		result.BlockerSummary.TotalCount,
		result.BlockerSummary.HighCount,
		result.WorkloadSummary.TotalIssues)
}

// TestE2E_ProjectHealth_LLMHints は LLMHints の存在を確認する E2E テスト。
func TestE2E_ProjectHealth_LLMHints(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	builder := analysis.NewProjectHealthBuilder(client, "default", env.Space, baseURL)

	cfg := analysis.ProjectHealthConfig{}

	ctx := context.Background()
	envelope, err := builder.Build(ctx, env.ProjectKey, cfg)
	if err != nil {
		t.Fatalf("ProjectHealthBuilder.Build() エラー: %v", err)
	}

	result, ok := envelope.Analysis.(*analysis.ProjectHealthResult)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.ProjectHealthResult", envelope.Analysis)
	}

	// LLMHints の検証
	if result.LLMHints.PrimaryEntities == nil {
		t.Error("LLMHints.PrimaryEntities が nil")
	}
	if len(result.LLMHints.PrimaryEntities) == 0 {
		t.Error("LLMHints.PrimaryEntities が空")
	}

	t.Logf("LLMHints.PrimaryEntities: %v", result.LLMHints.PrimaryEntities)
	t.Logf("LLMHints.OpenQuestions: %v", result.LLMHints.OpenQuestions)
}

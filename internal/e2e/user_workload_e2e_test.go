//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/analysis"
)

// TestE2E_UserWorkload は実 Backlog API を使ってユーザーワークロードを計算する E2E テスト。
func TestE2E_UserWorkload(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	calculator := analysis.NewWorkloadCalculator(client, "default", env.Space, baseURL)

	cfg := analysis.WorkloadConfig{
		StaleDays: 7,
	}

	ctx := context.Background()
	envelope, err := calculator.Calculate(ctx, env.ProjectKey, cfg)
	if err != nil {
		t.Fatalf("WorkloadCalculator.Calculate() エラー: %v", err)
	}

	// AnalysisEnvelope 基本検証
	if envelope == nil {
		t.Fatal("envelope が nil")
	}
	if envelope.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want %q", envelope.SchemaVersion, "1")
	}
	if envelope.Resource != "user_workload" {
		t.Errorf("Resource = %q, want %q", envelope.Resource, "user_workload")
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

	// WorkloadResult 型アサーション
	result, ok := envelope.Analysis.(*analysis.WorkloadResult)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.WorkloadResult", envelope.Analysis)
	}

	// 基本フィールド検証
	if result.ProjectKey != env.ProjectKey {
		t.Errorf("ProjectKey = %q, want %q", result.ProjectKey, env.ProjectKey)
	}
	if result.Members == nil {
		t.Error("Members が nil（空スライスであるべき）")
	}
	if result.StaleDays != 7 {
		t.Errorf("StaleDays = %d, want 7", result.StaleDays)
	}
	if result.TotalIssues < 0 {
		t.Errorf("TotalIssues = %d, 0以上であるべき", result.TotalIssues)
	}
	if result.UnassignedCount < 0 {
		t.Errorf("UnassignedCount = %d, 0以上であるべき", result.UnassignedCount)
	}

	// 各メンバーの LoadLevel 検証
	validLoadLevels := map[string]bool{
		"low": true, "medium": true, "high": true, "overloaded": true,
	}
	for i, m := range result.Members {
		if !validLoadLevels[m.LoadLevel] {
			t.Errorf("Members[%d] (%s): LoadLevel = %q, 無効な値", i, m.Name, m.LoadLevel)
		}
		if m.Total < 0 {
			t.Errorf("Members[%d] (%s): Total = %d, 0以上であるべき", i, m.Name, m.Total)
		}
		if m.ByStatus == nil {
			t.Errorf("Members[%d] (%s): ByStatus が nil", i, m.Name)
		}
		if m.ByPriority == nil {
			t.Errorf("Members[%d] (%s): ByPriority が nil", i, m.Name)
		}
	}

	t.Logf("プロジェクト %s のワークロード: 総課題=%d, 担当者なし=%d, メンバー数=%d",
		env.ProjectKey, result.TotalIssues, result.UnassignedCount, len(result.Members))
}

// TestE2E_UserWorkload_ExcludeStatus は除外ステータスありの E2E テスト。
func TestE2E_UserWorkload_ExcludeStatus(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	calculator := analysis.NewWorkloadCalculator(client, "default", env.Space, baseURL)

	cfg := analysis.WorkloadConfig{
		StaleDays:     7,
		ExcludeStatus: []string{"完了", "対応済み"},
	}

	ctx := context.Background()
	envelope, err := calculator.Calculate(ctx, env.ProjectKey, cfg)
	if err != nil {
		t.Fatalf("WorkloadCalculator.Calculate(ExcludeStatus) エラー: %v", err)
	}

	if envelope == nil || envelope.Analysis == nil {
		t.Fatal("envelope または Analysis が nil")
	}

	result, ok := envelope.Analysis.(*analysis.WorkloadResult)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.WorkloadResult", envelope.Analysis)
	}

	t.Logf("完了除外でのワークロード: 総課題=%d", result.TotalIssues)
}

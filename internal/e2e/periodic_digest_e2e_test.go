//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/analysis"
)

// TestE2E_PeriodicDigest_Weekly は実 Backlog API を使って weekly digest を生成する E2E テスト。
func TestE2E_PeriodicDigest_Weekly(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	builder := analysis.NewPeriodicDigestBuilder(client, "default", env.Space, baseURL)

	ctx := context.Background()
	envelope, err := builder.Build(ctx, env.ProjectKey, analysis.PeriodicDigestOptions{
		Period: "weekly",
	})
	if err != nil {
		t.Fatalf("PeriodicDigestBuilder.Build(weekly) エラー: %v", err)
	}

	// AnalysisEnvelope 基本検証
	if envelope == nil {
		t.Fatal("envelope が nil")
	}
	if envelope.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want %q", envelope.SchemaVersion, "1")
	}
	if envelope.Resource != "periodic_digest" {
		t.Errorf("Resource = %q, want %q", envelope.Resource, "periodic_digest")
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

	// PeriodicDigest 型アサーション
	pd, ok := envelope.Analysis.(*analysis.PeriodicDigest)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.PeriodicDigest", envelope.Analysis)
	}

	// 基本フィールド検証
	if pd.ProjectKey != env.ProjectKey {
		t.Errorf("ProjectKey = %q, want %q", pd.ProjectKey, env.ProjectKey)
	}
	if pd.Period != "weekly" {
		t.Errorf("Period = %q, want %q", pd.Period, "weekly")
	}
	if pd.Since.IsZero() {
		t.Error("Since がゼロ値")
	}
	if pd.Until.IsZero() {
		t.Error("Until がゼロ値")
	}
	if pd.Until.Before(pd.Since) {
		t.Errorf("Until (%v) が Since (%v) より前（不正）", pd.Until, pd.Since)
	}

	// スライスは nil でないこと（空スライスであるべき）
	if pd.Completed == nil {
		t.Error("Completed が nil（空スライスであるべき）")
	}
	if pd.Started == nil {
		t.Error("Started が nil（空スライスであるべき）")
	}
	if pd.Blocked == nil {
		t.Error("Blocked が nil（空スライスであるべき）")
	}

	// Summary はカウントと一致すること
	if pd.Summary.CompletedCount != len(pd.Completed) {
		t.Errorf("Summary.CompletedCount = %d, len(Completed) = %d（不一致）",
			pd.Summary.CompletedCount, len(pd.Completed))
	}
	if pd.Summary.StartedCount != len(pd.Started) {
		t.Errorf("Summary.StartedCount = %d, len(Started) = %d（不一致）",
			pd.Summary.StartedCount, len(pd.Started))
	}
	if pd.Summary.BlockedCount != len(pd.Blocked) {
		t.Errorf("Summary.BlockedCount = %d, len(Blocked) = %d（不一致）",
			pd.Summary.BlockedCount, len(pd.Blocked))
	}

	t.Logf("プロジェクト %s の週次digest: since=%v, until=%v",
		env.ProjectKey, pd.Since.Format("2006-01-02"), pd.Until.Format("2006-01-02"))
	t.Logf("completed=%d, started=%d, blocked=%d, total_active=%d",
		pd.Summary.CompletedCount, pd.Summary.StartedCount,
		pd.Summary.BlockedCount, pd.Summary.TotalActiveCount)
}

// TestE2E_PeriodicDigest_Daily は実 Backlog API を使って daily digest を生成する E2E テスト。
func TestE2E_PeriodicDigest_Daily(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	builder := analysis.NewPeriodicDigestBuilder(client, "default", env.Space, baseURL)

	ctx := context.Background()
	envelope, err := builder.Build(ctx, env.ProjectKey, analysis.PeriodicDigestOptions{
		Period: "daily",
	})
	if err != nil {
		t.Fatalf("PeriodicDigestBuilder.Build(daily) エラー: %v", err)
	}

	if envelope == nil || envelope.Analysis == nil {
		t.Fatal("envelope または Analysis が nil")
	}

	pd, ok := envelope.Analysis.(*analysis.PeriodicDigest)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.PeriodicDigest", envelope.Analysis)
	}

	if pd.Period != "daily" {
		t.Errorf("Period = %q, want %q", pd.Period, "daily")
	}

	// daily の場合 Until - Since は約 24h（±1h 許容）
	diff := pd.Until.Sub(pd.Since)
	if diff.Hours() < 23 || diff.Hours() > 25 {
		t.Errorf("daily の期間差 = %.1f 時間（期待値: 24h ±1h）", diff.Hours())
	}

	t.Logf("プロジェクト %s の日次digest: since=%v, until=%v",
		env.ProjectKey, pd.Since.Format("2006-01-02 15:04"), pd.Until.Format("2006-01-02 15:04"))
	t.Logf("completed=%d, started=%d, blocked=%d",
		pd.Summary.CompletedCount, pd.Summary.StartedCount, pd.Summary.BlockedCount)
}

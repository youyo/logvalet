//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/analysis"
)

// TestE2E_ActivityStats_Project は実 Backlog API を使って scope=project の activity stats を取得する E2E テスト。
func TestE2E_ActivityStats_Project(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	builder := analysis.NewActivityStatsBuilder(client, "default", env.Space, baseURL)

	ctx := context.Background()
	envelope, err := builder.Build(ctx, analysis.ActivityStatsOptions{
		Scope:    "project",
		ScopeKey: env.ProjectKey,
	})
	if err != nil {
		t.Fatalf("ActivityStatsBuilder.Build(scope=project) エラー: %v", err)
	}

	// AnalysisEnvelope 基本検証
	if envelope == nil {
		t.Fatal("envelope が nil")
	}
	if envelope.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want %q", envelope.SchemaVersion, "1")
	}
	if envelope.Resource != "activity_stats" {
		t.Errorf("Resource = %q, want %q", envelope.Resource, "activity_stats")
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

	// ActivityStats 型アサーション
	stats, ok := envelope.Analysis.(*analysis.ActivityStats)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.ActivityStats", envelope.Analysis)
	}

	// Scope/ScopeKey 検証
	if stats.Scope != "project" {
		t.Errorf("Scope = %q, want %q", stats.Scope, "project")
	}
	if stats.ScopeKey != env.ProjectKey {
		t.Errorf("ScopeKey = %q, want %q", stats.ScopeKey, env.ProjectKey)
	}

	// TotalCount は非負であること
	if stats.TotalCount < 0 {
		t.Errorf("TotalCount = %d（負の値は不正）", stats.TotalCount)
	}

	// ByType, ByActor が nil でないこと
	if stats.ByType == nil {
		t.Error("ByType が nil（空マップであるべき）")
	}
	if stats.ByActor == nil {
		t.Error("ByActor が nil（空マップであるべき）")
	}

	// TopActiveActors, TopActiveTypes が nil でないこと
	if stats.TopActiveActors == nil {
		t.Error("TopActiveActors が nil（空スライスであるべき）")
	}
	if stats.TopActiveTypes == nil {
		t.Error("TopActiveTypes が nil（空スライスであるべき）")
	}

	// Since/Until が非ゼロであること
	if stats.Since.IsZero() {
		t.Error("Since がゼロ値")
	}
	if stats.Until.IsZero() {
		t.Error("Until がゼロ値")
	}
	if stats.Until.Before(stats.Since) {
		t.Errorf("Until (%v) が Since (%v) より前（不正）", stats.Until, stats.Since)
	}

	t.Logf("プロジェクト %s のアクティビティ統計: total=%d", env.ProjectKey, stats.TotalCount)
	t.Logf("since=%v, until=%v", stats.Since.Format("2006-01-02"), stats.Until.Format("2006-01-02"))
	t.Logf("top_actor=%v, peak_hour=%d, peak_dow=%s",
		stats.TopActiveActors, stats.Patterns.PeakHour, stats.Patterns.PeakDayOfWeek)
}

// TestE2E_ActivityStats_Space は実 Backlog API を使って scope=space の activity stats を取得する E2E テスト。
func TestE2E_ActivityStats_Space(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	baseURL := e2eBaseURL(env)

	builder := analysis.NewActivityStatsBuilder(client, "default", env.Space, baseURL)

	ctx := context.Background()
	envelope, err := builder.Build(ctx, analysis.ActivityStatsOptions{
		Scope: "space",
	})
	if err != nil {
		t.Fatalf("ActivityStatsBuilder.Build(scope=space) エラー: %v", err)
	}

	if envelope == nil || envelope.Analysis == nil {
		t.Fatal("envelope または Analysis が nil")
	}

	stats, ok := envelope.Analysis.(*analysis.ActivityStats)
	if !ok {
		t.Fatalf("Analysis の型 = %T, want *analysis.ActivityStats", envelope.Analysis)
	}

	// Scope 検証
	if stats.Scope != "space" {
		t.Errorf("Scope = %q, want %q", stats.Scope, "space")
	}

	// TotalCount は非負であること
	if stats.TotalCount < 0 {
		t.Errorf("TotalCount = %d（負の値は不正）", stats.TotalCount)
	}

	// ByType, ByActor が nil でないこと
	if stats.ByType == nil {
		t.Error("ByType が nil（空マップであるべき）")
	}
	if stats.ByActor == nil {
		t.Error("ByActor が nil（空マップであるべき）")
	}

	t.Logf("スペース全体のアクティビティ統計: total=%d", stats.TotalCount)
	t.Logf("actor_concentration=%.2f, type_concentration=%.2f",
		stats.Patterns.ActorConcentration, stats.Patterns.TypeConcentration)
}

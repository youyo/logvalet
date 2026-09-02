//go:build e2e

package e2e_test

import (
	"context"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/analysis"
	"github.com/youyo/logvalet/internal/conventions"
)

// TestE2E_ConventionsShow は実 Backlog API から運用規約を読み取る E2E テスト。
func TestE2E_ConventionsShow(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)

	result, err := conventions.Show(context.Background(), client, env.ProjectKey)
	if err != nil {
		t.Fatalf("conventions.Show() エラー: %v", err)
	}
	if result == nil {
		t.Fatal("conventions.Show() の結果が nil")
	}
	if len(result.Glossary) == 0 {
		t.Error("Glossary が空です")
	}
	if result.Adopted && result.Conventions == nil {
		t.Error("Adopted=true なのに Conventions が nil です")
	}
	if !result.Adopted && result.Conventions != nil {
		t.Error("Adopted=false なのに Conventions が nil ではありません")
	}

	t.Logf("プロジェクト %s の conventions: adopted=%v, issue=%s, glossary=%d件",
		result.ProjectKey, result.Adopted, result.IssueKey, len(result.Glossary))
}

// TestE2E_ConventionsSkeletonFromProject は実プロジェクトから規約スケルトンを生成する E2E テスト。
func TestE2E_ConventionsSkeletonFromProject(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)

	data, err := conventions.BuildSkeletonFromProject(context.Background(), client, env.ProjectKey)
	if err != nil {
		t.Fatalf("conventions.BuildSkeletonFromProject() エラー: %v", err)
	}

	conv, err := conventions.Load(data)
	if err != nil {
		t.Fatalf("生成結果を conventions.Load() できません: %v", err)
	}
	violations := conventions.Validate(conv)
	if conventions.HasError(violations) {
		t.Errorf("error severity の violation があります: %v", violations)
	}
	if conv.Project.Key != env.ProjectKey {
		t.Errorf("project.key = %q, want %q", conv.Project.Key, env.ProjectKey)
	}

	t.Logf("プロジェクト %s の conventions スケルトン: %d bytes, violations=%v",
		env.ProjectKey, len(data), violations)
}

// TestE2E_ConventionsBuildPlanDryRun は規約の差分計画を読み取りだけで生成する E2E テスト。
func TestE2E_ConventionsBuildPlanDryRun(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	ctx := context.Background()

	data, err := conventions.BuildSkeletonFromProject(ctx, client, env.ProjectKey)
	if err != nil {
		t.Fatalf("conventions.BuildSkeletonFromProject() エラー: %v", err)
	}
	conv, err := conventions.Load(data)
	if err != nil {
		t.Fatalf("生成結果を conventions.Load() できません: %v", err)
	}

	plan, err := conventions.BuildPlan(ctx, client, conv, conventions.PlanOptions{})
	if err != nil {
		message := err.Error()
		if strings.Contains(message, "複数") || strings.Contains(message, "重複") {
			t.Skipf("実プロジェクトの重複リソースにより BuildPlan をスキップ: %v", err)
		}
		t.Fatalf("conventions.BuildPlan() エラー: %v", err)
	}
	if plan == nil {
		t.Fatal("conventions.BuildPlan() の結果が nil")
	}
	if rendered := conventions.RenderPlan(plan); rendered == "" {
		t.Error("conventions.RenderPlan() の結果が空文字です")
	}

	validActions := map[conventions.Action]struct{}{
		conventions.ActionCreate:    {},
		conventions.ActionUpdate:    {},
		conventions.ActionUnchanged: {},
		conventions.ActionSkip:      {},
	}
	for _, item := range plan.Items {
		if _, ok := validActions[item.Action]; !ok {
			t.Errorf("未対応の action %q: resource=%s, name=%s", item.Action, item.Resource, item.Name)
		}
	}

	t.Logf("プロジェクト %s の dry-run plan: items=%d, create=%d, update=%d, unchanged=%d, skip=%d",
		env.ProjectKey, len(plan.Items), plan.Summary.Create, plan.Summary.Update,
		plan.Summary.Unchanged, plan.Summary.Skip)
}

// TestE2E_AmbiguityDetect は実プロジェクトの曖昧さを読み取る E2E テスト。
func TestE2E_AmbiguityDetect(t *testing.T) {
	env := loadE2EEnv(t)
	client := newE2EClient(env)
	detector := analysis.NewAmbiguityDetector(client, "default", env.Space, e2eBaseURL(env))

	result, err := detector.Detect(context.Background(), env.ProjectKey)
	if err != nil {
		t.Fatalf("AmbiguityDetector.Detect() エラー: %v", err)
	}
	if result == nil {
		t.Fatal("AmbiguityDetector.Detect() の結果が nil")
	}
	if !result.Adopted && len(result.Ambiguities) != 0 {
		t.Errorf("規約未導入なのに Ambiguities が空ではありません: %d件", len(result.Ambiguities))
	}
	if result.TotalCount != len(result.Ambiguities) {
		t.Errorf("TotalCount = %d, len(Ambiguities) = %d", result.TotalCount, len(result.Ambiguities))
	}

	t.Logf("プロジェクト %s の ambiguity: adopted=%v, total=%d",
		env.ProjectKey, result.Adopted, result.TotalCount)
}

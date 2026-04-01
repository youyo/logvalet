package analysis

import (
	"context"
	"fmt"
	"sync"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
	"github.com/youyo/logvalet/internal/domain"
)

// 減点テーブル定数
const (
	PenaltyPerStale          = 5  // stale issue 1件ごと
	PenaltyBlockerHigh       = 10 // blocker HIGH 1件ごと
	PenaltyBlockerMedium     = 5  // blocker MEDIUM 1件ごと
	PenaltyPerOverloaded     = 8  // overloaded メンバー 1人ごと
	PenaltyUnassignedRatio   = 10 // unassigned 課題が total の 20% 超
	UnassignedRatioThreshold = 20 // unassigned ratio のしきい値（%）
)

// ProjectHealthConfig はプロジェクト健全性評価の設定。
type ProjectHealthConfig struct {
	StaleConfig    StaleConfig
	BlockerConfig  BlockerConfig
	WorkloadConfig WorkloadConfig
}

// StaleSummary は stale 課題のサマリー。
type StaleSummary struct {
	TotalCount    int `json:"total_count"`
	ThresholdDays int `json:"threshold_days"`
	OverdueCount  int `json:"overdue_count"`
}

// BlockerSummary はブロッカーのサマリー。
type BlockerSummary struct {
	TotalCount  int `json:"total_count"`
	HighCount   int `json:"high_count"`
	MediumCount int `json:"medium_count"`
}

// WorkloadSummary はワークロードのサマリー。
type WorkloadSummary struct {
	TotalIssues     int `json:"total_issues"`
	UnassignedCount int `json:"unassigned_count"`
	OverloadedCount int `json:"overloaded_count"`
	HighLoadCount   int `json:"high_load_count"`
}

// ProjectHealthResult はプロジェクト健全性評価の結果。
type ProjectHealthResult struct {
	ProjectKey      string                `json:"project_key"`
	StaleSummary    StaleSummary          `json:"stale_summary"`
	BlockerSummary  BlockerSummary        `json:"blocker_summary"`
	WorkloadSummary WorkloadSummary       `json:"workload_summary"`
	HealthScore     int                   `json:"health_score"`
	HealthLevel     string                `json:"health_level"` // "healthy" | "warning" | "critical"
	LLMHints        digest.DigestLLMHints `json:"llm_hints"`
}

// ProjectHealthBuilder はプロジェクト健全性を評価する。
type ProjectHealthBuilder struct {
	BaseAnalysisBuilder
}

// NewProjectHealthBuilder は ProjectHealthBuilder を生成する。
func NewProjectHealthBuilder(client backlog.Client, profile, space, baseURL string, opts ...Option) *ProjectHealthBuilder {
	return &ProjectHealthBuilder{
		BaseAnalysisBuilder: NewBaseAnalysisBuilder(client, profile, space, baseURL, opts...),
	}
}

// Build はプロジェクトの健全性評価を実行する。
// 各分析器（StaleIssueDetector, BlockerDetector, WorkloadCalculator）を goroutine で並列実行し、
// 結果を集約して AnalysisEnvelope を返す。
// 各分析器の失敗は error を返さず warnings に追加する（partial result）。
func (b *ProjectHealthBuilder) Build(ctx context.Context, projectKey string, config ProjectHealthConfig) (*AnalysisEnvelope, error) {
	// clock を各分析器に伝播する
	clockOpt := WithClock(b.now)

	staleDetector := NewStaleIssueDetector(b.client, b.profile, b.space, b.baseURL, clockOpt)
	blockerDetector := NewBlockerDetector(b.client, b.profile, b.space, b.baseURL, clockOpt)
	workloadCalc := NewWorkloadCalculator(b.client, b.profile, b.space, b.baseURL, clockOpt)

	var (
		staleEnv    *AnalysisEnvelope
		blockerEnv  *AnalysisEnvelope
		workloadEnv *AnalysisEnvelope
		allWarnings []domain.Warning
		mu          sync.Mutex
		wg          sync.WaitGroup
	)

	// goroutine 1: StaleIssueDetector
	wg.Add(1)
	go func() {
		defer wg.Done()
		env, err := staleDetector.Detect(ctx, []string{projectKey}, config.StaleConfig)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			allWarnings = append(allWarnings, domain.Warning{
				Code:      "stale_detect_failed",
				Message:   fmt.Sprintf("stale detection failed: %v", err),
				Component: "stale",
				Retryable: true,
			})
			return
		}
		staleEnv = env
		allWarnings = append(allWarnings, env.Warnings...)
	}()

	// goroutine 2: BlockerDetector
	wg.Add(1)
	go func() {
		defer wg.Done()
		env, err := blockerDetector.Detect(ctx, []string{projectKey}, config.BlockerConfig)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			allWarnings = append(allWarnings, domain.Warning{
				Code:      "blocker_detect_failed",
				Message:   fmt.Sprintf("blocker detection failed: %v", err),
				Component: "blocker",
				Retryable: true,
			})
			return
		}
		blockerEnv = env
		allWarnings = append(allWarnings, env.Warnings...)
	}()

	// goroutine 3: WorkloadCalculator
	wg.Add(1)
	go func() {
		defer wg.Done()
		env, err := workloadCalc.Calculate(ctx, projectKey, config.WorkloadConfig)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			allWarnings = append(allWarnings, domain.Warning{
				Code:      "workload_calc_failed",
				Message:   fmt.Sprintf("workload calculation failed: %v", err),
				Component: "workload",
				Retryable: true,
			})
			return
		}
		workloadEnv = env
		allWarnings = append(allWarnings, env.Warnings...)
	}()

	wg.Wait()

	// GetProject/ListIssues 失敗の検出:
	// 全分析器から project_fetch_failed / issues_fetch_failed の warning が返った場合は
	// 情報がないため最悪ケースを仮定して health_score=0, health_level="critical" を返す
	fetchFailureCount := 0
	for _, w := range allWarnings {
		if w.Code == "project_fetch_failed" || w.Code == "issues_fetch_failed" {
			fetchFailureCount++
		}
	}

	// 3分析器全てが同じプロジェクトを参照しているため、
	// project_fetch_failed or issues_fetch_failed が1件でもあれば情報不足とみなす
	if fetchFailureCount > 0 {
		result := &ProjectHealthResult{
			ProjectKey:      projectKey,
			StaleSummary:    StaleSummary{},
			BlockerSummary:  BlockerSummary{},
			WorkloadSummary: WorkloadSummary{},
			HealthScore:     0,
			HealthLevel:     "critical",
			LLMHints:        buildHealthLLMHints(projectKey, StaleSummary{}, BlockerSummary{}, WorkloadSummary{}),
		}
		return b.newEnvelope("project_health", result, deduplicateWarnings(allWarnings)), nil
	}

	// サマリーを集約
	staleSummary := aggregateStaleSummary(staleEnv)
	blockerSummary := aggregateBlockerSummary(blockerEnv)
	workloadSummary := aggregateWorkloadSummary(workloadEnv)

	// health_score 計算
	score := calcHealthScore(
		staleSummary.TotalCount,
		blockerSummary.HighCount,
		blockerSummary.MediumCount,
		workloadSummary.OverloadedCount,
		workloadSummary.UnassignedCount,
		workloadSummary.TotalIssues,
	)
	level := calcHealthLevel(score)

	result := &ProjectHealthResult{
		ProjectKey:      projectKey,
		StaleSummary:    staleSummary,
		BlockerSummary:  blockerSummary,
		WorkloadSummary: workloadSummary,
		HealthScore:     score,
		HealthLevel:     level,
		LLMHints:        buildHealthLLMHints(projectKey, staleSummary, blockerSummary, workloadSummary),
	}

	return b.newEnvelope("project_health", result, deduplicateWarnings(allWarnings)), nil
}

// aggregateStaleSummary は stale 分析結果から StaleSummary を組み立てる。
func aggregateStaleSummary(env *AnalysisEnvelope) StaleSummary {
	if env == nil {
		return StaleSummary{}
	}
	staleResult, ok := env.Analysis.(*StaleIssueResult)
	if !ok {
		return StaleSummary{}
	}

	overdueCount := 0
	for _, si := range staleResult.Issues {
		if si.IsOverdue {
			overdueCount++
		}
	}

	return StaleSummary{
		TotalCount:    staleResult.TotalCount,
		ThresholdDays: staleResult.ThresholdDays,
		OverdueCount:  overdueCount,
	}
}

// aggregateBlockerSummary は blocker 分析結果から BlockerSummary を組み立てる。
func aggregateBlockerSummary(env *AnalysisEnvelope) BlockerSummary {
	if env == nil {
		return BlockerSummary{}
	}
	blockerResult, ok := env.Analysis.(*BlockerResult)
	if !ok {
		return BlockerSummary{}
	}

	highCount := blockerResult.BySeverity["HIGH"]
	mediumCount := blockerResult.BySeverity["MEDIUM"]

	return BlockerSummary{
		TotalCount:  blockerResult.TotalCount,
		HighCount:   highCount,
		MediumCount: mediumCount,
	}
}

// aggregateWorkloadSummary は workload 分析結果から WorkloadSummary を組み立てる。
func aggregateWorkloadSummary(env *AnalysisEnvelope) WorkloadSummary {
	if env == nil {
		return WorkloadSummary{}
	}
	workloadResult, ok := env.Analysis.(*WorkloadResult)
	if !ok {
		return WorkloadSummary{}
	}

	overloadedCount := 0
	highLoadCount := 0
	for _, m := range workloadResult.Members {
		if m.LoadLevel == "overloaded" {
			overloadedCount++
		}
		if m.LoadLevel == "high" {
			highLoadCount++
		}
	}

	return WorkloadSummary{
		TotalIssues:     workloadResult.TotalIssues,
		UnassignedCount: workloadResult.UnassignedCount,
		OverloadedCount: overloadedCount,
		HighLoadCount:   highLoadCount,
	}
}

// calcHealthScore はサマリーから health_score を計算する（減点方式、下限0）。
func calcHealthScore(staleCount, blockerHigh, blockerMedium, overloadedCount, unassignedCount, totalIssues int) int {
	score := 100

	// stale issue 1件ごと -5
	score -= staleCount * PenaltyPerStale

	// blocker HIGH 1件ごと -10
	score -= blockerHigh * PenaltyBlockerHigh

	// blocker MEDIUM 1件ごと -5
	score -= blockerMedium * PenaltyBlockerMedium

	// overloaded メンバー 1人ごと -8
	score -= overloadedCount * PenaltyPerOverloaded

	// unassigned 課題が total の 20% 超 → -10
	if totalIssues > 0 {
		ratio := unassignedCount * 100 / totalIssues
		if ratio > UnassignedRatioThreshold {
			score -= PenaltyUnassignedRatio
		}
	}

	// 下限 0
	if score < 0 {
		score = 0
	}

	return score
}

// calcHealthLevel は score から health_level を決定する。
// 80-100: "healthy", 60-79: "warning", 0-59: "critical"
func calcHealthLevel(score int) string {
	switch {
	case score >= 80:
		return "healthy"
	case score >= 60:
		return "warning"
	default:
		return "critical"
	}
}

// buildHealthLLMHints は project health 結果から LLMHints を生成する。
func buildHealthLLMHints(projectKey string, stale StaleSummary, blocker BlockerSummary, workload WorkloadSummary) digest.DigestLLMHints {
	primaryEntities := []string{fmt.Sprintf("project:%s", projectKey)}

	openQuestions := []string{}
	if stale.TotalCount > 0 {
		openQuestions = append(openQuestions,
			fmt.Sprintf("%d件の停滞課題があります", stale.TotalCount))
	}
	if blocker.HighCount > 0 {
		openQuestions = append(openQuestions,
			fmt.Sprintf("%d件の HIGH ブロッカーがあります", blocker.HighCount))
	}
	if workload.OverloadedCount > 0 {
		openQuestions = append(openQuestions,
			fmt.Sprintf("%d人のメンバーが過負荷状態です", workload.OverloadedCount))
	}

	return digest.DigestLLMHints{
		PrimaryEntities:      primaryEntities,
		OpenQuestions:        openQuestions,
		SuggestedNextActions: []string{},
	}
}

// deduplicateWarnings は warnings の重複を除去せずにそのまま返す（情報欠損を避ける）。
// 複数の分析器が同じ project_fetch_failed を返す場合も全てマージして返す。
func deduplicateWarnings(warnings []domain.Warning) []domain.Warning {
	if warnings == nil {
		return []domain.Warning{}
	}
	return warnings
}

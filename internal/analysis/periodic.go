package analysis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
	"github.com/youyo/logvalet/internal/domain"
)

// defaultBlockedKeywords はブロッカー判定のデフォルトキーワードリスト。
var defaultBlockedKeywords = []string{"対応中断", "保留", "ブロック", "blocked", "on hold"}

// PeriodicDigestOptions は PeriodicDigestBuilder.Build() のオプション。
type PeriodicDigestOptions struct {
	// Period は集計種別。"weekly" | "daily"。
	Period string
	// Since は集計開始日時（UTC）。nil の場合は now - 7days (weekly) / now - 1day (daily)。
	Since *time.Time
	// Until は集計終了日時（UTC）。nil の場合は now。
	Until *time.Time
	// ClosedStatus は完了とみなすステータス名リスト。空の場合は defaultClosedStatus を使用。
	ClosedStatus []string
	// BlockedKeywords はブロッカー判定キーワード（ステータス名または課題タイトル）。
	// 空の場合は defaultBlockedKeywords を使用。
	BlockedKeywords []string
}

// PeriodicDigest は period ベースの課題活動集約（analysis フィールドの型）。
type PeriodicDigest struct {
	ProjectKey string                `json:"project_key"`
	Period     string                `json:"period"`
	Since      time.Time             `json:"since"`
	Until      time.Time             `json:"until"`
	Summary    PeriodicSummary       `json:"summary"`
	Completed  []PeriodicIssueRef    `json:"completed"`
	Started    []PeriodicIssueRef    `json:"started"`
	Blocked    []PeriodicBlockedIssue `json:"blocked"`
	LLMHints   digest.DigestLLMHints `json:"llm_hints"`
}

// PeriodicSummary は期間内の課題活動集計。
type PeriodicSummary struct {
	CompletedCount int `json:"completed_count"`
	StartedCount   int `json:"started_count"`
	BlockedCount   int `json:"blocked_count"`
	// TotalActiveCount は ListIssues で取得した件数（期間フィルタ適用後）。
	TotalActiveCount int `json:"total_active_count"`
}

// PeriodicIssueRef は completed/started の課題参照。
type PeriodicIssueRef struct {
	IssueKey    string          `json:"issue_key"`
	Summary     string          `json:"summary"`
	Assignee    *domain.UserRef `json:"assignee"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
}

// PeriodicBlockedIssue は blocked の課題参照。
type PeriodicBlockedIssue struct {
	IssueKey     string          `json:"issue_key"`
	Summary      string          `json:"summary"`
	Assignee     *domain.UserRef `json:"assignee"`
	BlockSignals []string        `json:"block_signals"`
}

// PeriodicDigestBuilder は期間ベースの課題活動集約を構築する。
type PeriodicDigestBuilder struct {
	BaseAnalysisBuilder
}

// NewPeriodicDigestBuilder は PeriodicDigestBuilder を生成する。
func NewPeriodicDigestBuilder(client backlog.Client, profile, space, baseURL string, opts ...Option) *PeriodicDigestBuilder {
	return &PeriodicDigestBuilder{
		BaseAnalysisBuilder: NewBaseAnalysisBuilder(client, profile, space, baseURL, opts...),
	}
}

// Build は projectKey の期間ベースの課題活動を集約して AnalysisEnvelope を返す。
// GetProject の失敗は error を返す（必須）。
// ListIssues の失敗は warnings に追加して部分結果を返す。
func (b *PeriodicDigestBuilder) Build(
	ctx context.Context,
	projectKey string,
	opt PeriodicDigestOptions,
) (*AnalysisEnvelope, error) {
	// 1. Since/Until 解決
	since, until := resolvePeriodRange(b.now(), opt)

	// 2. ClosedStatus 解決
	closedStatus := opt.ClosedStatus
	if len(closedStatus) == 0 {
		closedStatus = defaultClosedStatus
	}

	// 3. BlockedKeywords 解決
	blockedKeywords := opt.BlockedKeywords
	if len(blockedKeywords) == 0 {
		blockedKeywords = defaultBlockedKeywords
	}

	// 4. GetProject（必須）
	project, err := b.client.GetProject(ctx, projectKey)
	if err != nil {
		return nil, fmt.Errorf("get project %s: %w", projectKey, err)
	}

	// 5. ListIssues（期間フィルタ付き）
	var warnings []domain.Warning
	issues, err := b.client.ListIssues(ctx, backlog.ListIssuesOptions{
		ProjectIDs:   []int{project.ID},
		UpdatedSince: &since,
		UpdatedUntil: &until,
	})
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "issues_fetch_failed",
			Message:   fmt.Sprintf("failed to list issues: %v", err),
			Component: "periodic_digest",
			Retryable: true,
		})
		issues = []domain.Issue{}
	}

	// 6. 課題分類
	completed, started, blocked := classifyPeriodicIssues(issues, since, until, closedStatus, blockedKeywords)

	// 7. 結果組み立て
	result := &PeriodicDigest{
		ProjectKey: projectKey,
		Period:     opt.Period,
		Since:      since,
		Until:      until,
		Summary: PeriodicSummary{
			CompletedCount:   len(completed),
			StartedCount:     len(started),
			BlockedCount:     len(blocked),
			TotalActiveCount: len(issues),
		},
		Completed: completed,
		Started:   started,
		Blocked:   blocked,
		LLMHints:  buildPeriodicLLMHints(projectKey, len(completed), len(started), len(blocked)),
	}

	return b.newEnvelope("periodic_digest", result, warnings), nil
}

// resolvePeriodRange は Since/Until のデフォルト値を解決する。
// Period が "daily" の場合は now - 1日、それ以外（weekly）は now - 7日。
func resolvePeriodRange(now time.Time, opt PeriodicDigestOptions) (since, until time.Time) {
	until = now
	if opt.Until != nil {
		until = *opt.Until
	}

	if opt.Since != nil {
		since = *opt.Since
		return
	}

	switch opt.Period {
	case "daily":
		since = now.Add(-1 * 24 * time.Hour)
	default: // "weekly"
		since = now.Add(-7 * 24 * time.Hour)
	}
	return
}

// classifyPeriodicIssues は issues を completed/started/blocked に分類する。
//
// 優先順位: completed > blocked > started
//   - completed: Updated が [since, until] 内 かつ Status.Name が closedStatus に含まれる
//   - blocked:   Status.Name または Summary が blockedKeywords のいずれかに一致（completed 除外）
//   - started:   Created が [since, until] 内 かつ Status.Name が closedStatus に含まれない（completed/blocked 除外）
func classifyPeriodicIssues(
	issues []domain.Issue,
	since, until time.Time,
	closedStatus []string,
	blockedKeywords []string,
) (completed []PeriodicIssueRef, started []PeriodicIssueRef, blocked []PeriodicBlockedIssue) {
	// 完了ステータスの set を構築
	closedSet := make(map[string]bool, len(closedStatus))
	for _, s := range closedStatus {
		closedSet[s] = true
	}

	// 完了済み課題キーを追跡（blocked/started の重複排除に使用）
	completedKeys := make(map[string]bool)

	// Pass 1: completed を収集
	for i := range issues {
		issue := &issues[i]
		statusName := issueStatusName(issue)

		if !closedSet[statusName] {
			continue
		}
		if issue.Updated == nil {
			continue
		}
		if !inPeriod(*issue.Updated, since, until) {
			continue
		}

		completedKeys[issue.IssueKey] = true
		completed = append(completed, PeriodicIssueRef{
			IssueKey:    issue.IssueKey,
			Summary:     issue.Summary,
			Assignee:    toUserRef(issue.Assignee),
			CompletedAt: issue.Updated,
		})
	}

	// Pass 2: blocked を収集（completed 除外）
	for i := range issues {
		issue := &issues[i]
		if completedKeys[issue.IssueKey] {
			continue // completed 優先
		}

		statusName := issueStatusName(issue)
		signals := blockedSignals(issue, statusName, blockedKeywords)
		if len(signals) == 0 {
			continue
		}

		blocked = append(blocked, PeriodicBlockedIssue{
			IssueKey:     issue.IssueKey,
			Summary:      issue.Summary,
			Assignee:     toUserRef(issue.Assignee),
			BlockSignals: signals,
		})
	}

	// blocked キーを追跡（started 除外に使用）
	blockedKeys := make(map[string]bool, len(blocked))
	for _, b := range blocked {
		blockedKeys[b.IssueKey] = true
	}

	// Pass 3: started を収集（completed/blocked 除外）
	for i := range issues {
		issue := &issues[i]
		if completedKeys[issue.IssueKey] || blockedKeys[issue.IssueKey] {
			continue
		}

		statusName := issueStatusName(issue)
		if closedSet[statusName] {
			continue
		}
		// Created が nil または since より前の場合は started に分類しない
		if issue.Created == nil {
			continue
		}
		if !inPeriod(*issue.Created, since, until) {
			continue
		}

		started = append(started, PeriodicIssueRef{
			IssueKey:  issue.IssueKey,
			Summary:   issue.Summary,
			Assignee:  toUserRef(issue.Assignee),
			StartedAt: issue.Created,
		})
	}

	// nil スライスを空スライスに正規化
	if completed == nil {
		completed = []PeriodicIssueRef{}
	}
	if started == nil {
		started = []PeriodicIssueRef{}
	}
	if blocked == nil {
		blocked = []PeriodicBlockedIssue{}
	}

	return
}

// issueStatusName は issue のステータス名を返す（nil 安全）。
func issueStatusName(issue *domain.Issue) string {
	if issue.Status == nil {
		return ""
	}
	return issue.Status.Name
}

// inPeriod は t が [since, until] 内かどうかを返す。
func inPeriod(t, since, until time.Time) bool {
	return !t.Before(since) && !t.After(until)
}

// blockedSignals は issue がブロッカーキーワードに一致する場合、マッチしたシグナルのリストを返す。
// 一致しない場合は空スライスを返す。
func blockedSignals(issue *domain.Issue, statusName string, keywords []string) []string {
	var signals []string
	for _, kw := range keywords {
		kwLower := strings.ToLower(kw)
		if strings.EqualFold(statusName, kw) {
			signals = append(signals, fmt.Sprintf("ステータス名がブロッカーキーワードに一致: %q", kw))
		} else if strings.Contains(strings.ToLower(issue.Summary), kwLower) {
			signals = append(signals, fmt.Sprintf("課題タイトルにブロッカーキーワードを含む: %q", kw))
		}
	}
	return signals
}

// buildPeriodicLLMHints は LLM 向けヒントを構築する。
func buildPeriodicLLMHints(projectKey string, completedN, startedN, blockedN int) digest.DigestLLMHints {
	hints := digest.DigestLLMHints{
		PrimaryEntities:      []string{fmt.Sprintf("project:%s", projectKey)},
		OpenQuestions:        []string{},
		SuggestedNextActions: []string{},
	}

	if blockedN > 0 {
		hints.OpenQuestions = append(hints.OpenQuestions,
			fmt.Sprintf("%d件のブロック課題があります", blockedN),
		)
	}

	if completedN > 0 {
		hints.SuggestedNextActions = append(hints.SuggestedNextActions,
			fmt.Sprintf("%d件の完了課題をレビューしてください", completedN),
		)
	}

	return hints
}

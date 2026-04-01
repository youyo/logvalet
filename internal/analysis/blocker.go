package analysis

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
	"github.com/youyo/logvalet/internal/domain"
)

// DefaultInProgressDays は「処理中」ステータス停滞判定のデフォルト閾値（日数）。
const DefaultInProgressDays = 14

// DefaultMaxCommentCount はコメント取得件数上限のデフォルト値。
const DefaultMaxCommentCount = 5

// シグナルコード定数
const (
	SignalHighPriorityUnassigned = "high_priority_unassigned"
	SignalLongInProgress         = "long_in_progress"
	SignalOverdueOpen            = "overdue_open"
	SignalBlockedByKeyword       = "blocked_by_keyword"
)

// defaultInProgressStatus はデフォルトの処理中ステータス名リスト。
var defaultInProgressStatusList = []string{"処理中"}

// defaultHighPriorityList はデフォルトの高優先度名リスト。
var defaultHighPriorityList = []string{"高", "最高"}

// blockerKeywords はコメントキーワード検出に使用するキーワード一覧（小文字）。
var blockerKeywords = []string{
	"ブロック", "待ち", "blocked", "pending", "依存", "待機", "対応待ち",
}

// BlockerConfig はブロッカー検出の設定。
type BlockerConfig struct {
	// InProgressDays は「処理中」ステータスの停滞閾値（日数）。0以下の場合 DefaultInProgressDays を使用。
	InProgressDays int
	// InProgressStatus は「処理中」とみなすステータス名リスト（デフォルト: ["処理中"]）。
	InProgressStatus []string
	// HighPriority は「高優先度」とみなす優先度名リスト（デフォルト: ["高", "最高"]）。
	HighPriority []string
	// ExcludeStatus は除外ステータス名（完了系）。
	ExcludeStatus []string
	// IncludeComments はコメントキーワード検出を有効化するか。
	IncludeComments bool
	// MaxCommentCount はコメント取得件数上限（0の場合 DefaultMaxCommentCount を使用）。
	MaxCommentCount int
}

// BlockerResult はブロッカー検出の結果。
type BlockerResult struct {
	Issues     []BlockerIssue        `json:"issues"`
	TotalCount int                   `json:"total_count"`
	BySeverity map[string]int        `json:"by_severity"`
	LLMHints   digest.DigestLLMHints `json:"llm_hints"`
}

// BlockerIssue は進行阻害と判定された個別課題。
type BlockerIssue struct {
	IssueKey string          `json:"issue_key"`
	Summary  string          `json:"summary"`
	Status   string          `json:"status"`
	Priority string          `json:"priority,omitempty"`
	Assignee *domain.UserRef `json:"assignee,omitempty"`
	DueDate  *time.Time      `json:"due_date,omitempty"`
	Signals  []BlockerSignal `json:"signals"`
	Severity string          `json:"severity"` // "HIGH" | "MEDIUM"
}

// BlockerSignal は個別の阻害要因。
type BlockerSignal struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// BlockerDetector は進行阻害課題を検出する。
type BlockerDetector struct {
	BaseAnalysisBuilder
}

// NewBlockerDetector は BlockerDetector を生成する。
func NewBlockerDetector(client backlog.Client, profile, space, baseURL string, opts ...Option) *BlockerDetector {
	return &BlockerDetector{
		BaseAnalysisBuilder: NewBaseAnalysisBuilder(client, profile, space, baseURL, opts...),
	}
}

// Detect は指定プロジェクトのブロッカー課題を検出する。
func (d *BlockerDetector) Detect(ctx context.Context, projectKeys []string, config BlockerConfig) (*AnalysisEnvelope, error) {
	// デフォルト値の設定
	inProgressDays := config.InProgressDays
	if inProgressDays <= 0 {
		inProgressDays = DefaultInProgressDays
	}

	inProgressSet := buildNameSet(config.InProgressStatus, defaultInProgressStatusList)
	highPrioritySet := buildNameSet(config.HighPriority, defaultHighPriorityList)
	excludeSet := buildExcludeSet(config.ExcludeStatus)

	maxCommentCount := config.MaxCommentCount
	if maxCommentCount <= 0 {
		maxCommentCount = DefaultMaxCommentCount
	}

	now := d.now()
	var blockerIssues []BlockerIssue
	var warnings []domain.Warning

	// プロジェクトごとに GetProject → ListIssues
	for _, projectKey := range projectKeys {
		issues, ws := d.fetchProjectIssues(ctx, projectKey)
		warnings = append(warnings, ws...)
		if issues == nil {
			continue
		}

		for i := range issues {
			signals := d.classifyBlocker(ctx, &issues[i], now, inProgressDays, inProgressSet, highPrioritySet, excludeSet, config.IncludeComments, maxCommentCount)
			if len(signals) == 0 {
				continue
			}

			severity := calcSeverity(signals)

			statusName := ""
			if issues[i].Status != nil {
				statusName = issues[i].Status.Name
			}
			priorityName := ""
			if issues[i].Priority != nil {
				priorityName = issues[i].Priority.Name
			}

			bi := BlockerIssue{
				IssueKey: issues[i].IssueKey,
				Summary:  issues[i].Summary,
				Status:   statusName,
				Priority: priorityName,
				Assignee: toUserRef(issues[i].Assignee),
				DueDate:  issues[i].DueDate,
				Signals:  signals,
				Severity: severity,
			}
			blockerIssues = append(blockerIssues, bi)
		}
	}

	// nil → 空スライス
	if blockerIssues == nil {
		blockerIssues = []BlockerIssue{}
	}

	// ソート: Severity 優先（HIGH → MEDIUM）、同一 severity 内は IssueKey 辞書順
	sort.Slice(blockerIssues, func(i, j int) bool {
		si := severityOrder(blockerIssues[i].Severity)
		sj := severityOrder(blockerIssues[j].Severity)
		if si != sj {
			return si < sj
		}
		return blockerIssues[i].IssueKey < blockerIssues[j].IssueKey
	})

	// by_severity カウント
	bySeverity := map[string]int{
		"HIGH":   0,
		"MEDIUM": 0,
	}
	for _, bi := range blockerIssues {
		bySeverity[bi.Severity]++
	}

	result := &BlockerResult{
		Issues:     blockerIssues,
		TotalCount: len(blockerIssues),
		BySeverity: bySeverity,
		LLMHints:   buildBlockerLLMHints(projectKeys, blockerIssues),
	}

	return d.newEnvelope("project_blockers", result, warnings), nil
}

// fetchProjectIssues はプロジェクトの課題一覧を取得する。エラー時は warnings を返す。
func (d *BlockerDetector) fetchProjectIssues(ctx context.Context, projectKey string) ([]domain.Issue, []domain.Warning) {
	project, err := d.client.GetProject(ctx, projectKey)
	if err != nil {
		return nil, []domain.Warning{{
			Code:      "project_fetch_failed",
			Message:   fmt.Sprintf("failed to get project %s: %v", projectKey, err),
			Component: "project",
			Retryable: true,
		}}
	}

	issues, err := d.client.ListIssues(ctx, backlog.ListIssuesOptions{
		ProjectIDs: []int{project.ID},
	})
	if err != nil {
		return nil, []domain.Warning{{
			Code:      "issues_fetch_failed",
			Message:   fmt.Sprintf("failed to list issues for project %s: %v", projectKey, err),
			Component: "issues",
			Retryable: true,
		}}
	}

	return issues, nil
}

// classifyBlocker は個別課題のシグナルを検出する。
func (d *BlockerDetector) classifyBlocker(
	ctx context.Context,
	issue *domain.Issue,
	now time.Time,
	inProgressDays int,
	inProgressSet map[string]bool,
	highPrioritySet map[string]bool,
	excludeSet map[string]bool,
	includeComments bool,
	maxCommentCount int,
) []BlockerSignal {
	// ExcludeStatus に含まれる課題はスキップ
	statusName := ""
	if issue.Status != nil {
		statusName = issue.Status.Name
	}
	if statusName != "" && excludeSet[statusName] {
		return nil
	}

	var signals []BlockerSignal

	// high_priority_unassigned
	if sig := detectHighPriorityUnassigned(issue, highPrioritySet); sig != nil {
		signals = append(signals, *sig)
	}

	// long_in_progress
	if sig := detectLongInProgress(issue, now, inProgressDays, inProgressSet); sig != nil {
		signals = append(signals, *sig)
	}

	// overdue_open
	if sig := detectOverdueOpen(issue, now); sig != nil {
		signals = append(signals, *sig)
	}

	// blocked_by_keyword（opt-in）
	if includeComments {
		if sig := d.detectCommentKeyword(ctx, issue, maxCommentCount); sig != nil {
			signals = append(signals, *sig)
		}
	}

	return signals
}

// detectHighPriorityUnassigned は優先度高かつ担当者なしを検出する。
func detectHighPriorityUnassigned(issue *domain.Issue, highPrioritySet map[string]bool) *BlockerSignal {
	if issue.Priority == nil || !highPrioritySet[issue.Priority.Name] {
		return nil
	}
	if issue.Assignee != nil {
		return nil
	}
	return &BlockerSignal{
		Code:    SignalHighPriorityUnassigned,
		Message: fmt.Sprintf("優先度「%s」で担当者未設定", issue.Priority.Name),
	}
}

// detectLongInProgress は処理中ステータスが長期停滞していることを検出する。
func detectLongInProgress(issue *domain.Issue, now time.Time, inProgressDays int, inProgressSet map[string]bool) *BlockerSignal {
	if issue.Status == nil || !inProgressSet[issue.Status.Name] {
		return nil
	}
	if issue.Updated == nil {
		return nil
	}
	days := int(now.Sub(*issue.Updated).Hours() / 24)
	if days < inProgressDays {
		return nil
	}
	return &BlockerSignal{
		Code:    SignalLongInProgress,
		Message: fmt.Sprintf("「%s」のまま%d日経過", issue.Status.Name, days),
	}
}

// detectOverdueOpen は期限超過かつ未完了を検出する。
func detectOverdueOpen(issue *domain.Issue, now time.Time) *BlockerSignal {
	if issue.DueDate == nil {
		return nil
	}
	if !issue.DueDate.Before(now) {
		return nil
	}
	days := int(now.Sub(*issue.DueDate).Hours() / 24)
	return &BlockerSignal{
		Code:    SignalOverdueOpen,
		Message: fmt.Sprintf("期限を%d日超過", days),
	}
}

// detectCommentKeyword は最新コメントにブロッカーキーワードが含まれるかを検出する。
func (d *BlockerDetector) detectCommentKeyword(ctx context.Context, issue *domain.Issue, maxCommentCount int) *BlockerSignal {
	comments, err := d.client.ListIssueComments(ctx, issue.IssueKey, backlog.ListCommentsOptions{
		Limit: maxCommentCount,
	})
	if err != nil || len(comments) == 0 {
		return nil
	}

	// 最新コメントのみ検索（解消済みの誤検出防止）
	latestComment := comments[0]
	content := latestComment.Content

	lowerContent := strings.ToLower(content)
	for _, keyword := range blockerKeywords {
		if strings.Contains(lowerContent, strings.ToLower(keyword)) {
			return &BlockerSignal{
				Code:    SignalBlockedByKeyword,
				Message: fmt.Sprintf("コメントに阻害キーワード: 「%s」", keyword),
			}
		}
	}
	return nil
}

// calcSeverity はシグナルから severity を計算する。
// HIGH シグナル（high_priority_unassigned, overdue_open）が1つでもあれば HIGH。
// それ以外は MEDIUM。
func calcSeverity(signals []BlockerSignal) string {
	highSignals := map[string]bool{
		SignalHighPriorityUnassigned: true,
		SignalOverdueOpen:            true,
	}
	for _, s := range signals {
		if highSignals[s.Code] {
			return "HIGH"
		}
	}
	return "MEDIUM"
}

// severityOrder は severity を数値順に変換する（ソート用）。
// HIGH=0, MEDIUM=1 (数値が小さいほど優先)
func severityOrder(severity string) int {
	switch severity {
	case "HIGH":
		return 0
	case "MEDIUM":
		return 1
	default:
		return 2
	}
}

// buildNameSet はスライスを set に変換する。スライスが空の場合はデフォルト値を使用する。
func buildNameSet(names []string, defaults []string) map[string]bool {
	if len(names) == 0 {
		names = defaults
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

// buildBlockerLLMHints はブロッカー検出結果から LLMHints を生成する。
func buildBlockerLLMHints(projectKeys []string, blockerIssues []BlockerIssue) digest.DigestLLMHints {
	primaryEntities := make([]string, 0, len(projectKeys))
	for _, pk := range projectKeys {
		primaryEntities = append(primaryEntities, fmt.Sprintf("project:%s", pk))
	}

	openQuestions := []string{}
	if len(blockerIssues) > 0 {
		openQuestions = append(openQuestions,
			fmt.Sprintf("%d件のブロッカー課題があります。対応を優先してください", len(blockerIssues)))
	}

	// HIGH severity のカウント
	highCount := 0
	for _, bi := range blockerIssues {
		if bi.Severity == "HIGH" {
			highCount++
		}
	}
	if highCount > 0 {
		openQuestions = append(openQuestions,
			fmt.Sprintf("%d件が HIGH severity です", highCount))
	}

	return digest.DigestLLMHints{
		PrimaryEntities:      primaryEntities,
		OpenQuestions:        openQuestions,
		SuggestedNextActions: []string{},
	}
}

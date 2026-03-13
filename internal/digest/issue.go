// Package digest は logvalet の各リソース向け DigestBuilder を提供する。
// spec §19 準拠。
package digest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// IssueDigestOptions は IssueDigestBuilder.Build() のオプション。
// spec §19 準拠。
type IssueDigestOptions struct {
	// MaxComments は取得するコメントの最大数。デフォルト 5。
	MaxComments int
	// IncludeActivity はアクティビティを含めるかどうか。
	IncludeActivity bool
}

// IssueDigestBuilder はインターフェース（spec §19）。
type IssueDigestBuilder interface {
	Build(ctx context.Context, issueKey string, opt IssueDigestOptions) (*domain.DigestEnvelope, error)
}

// DefaultIssueDigestBuilder は IssueDigestBuilder の標準実装。
// backlog.Client を使って必要なデータを収集し DigestEnvelope を構築する。
type DefaultIssueDigestBuilder struct {
	client  backlog.Client
	profile string
	space   string
	baseURL string
}

// NewDefaultIssueDigestBuilder は DefaultIssueDigestBuilder を生成する。
func NewDefaultIssueDigestBuilder(client backlog.Client, profile, space, baseURL string) *DefaultIssueDigestBuilder {
	return &DefaultIssueDigestBuilder{
		client:  client,
		profile: profile,
		space:   space,
		baseURL: baseURL,
	}
}

// IssueDigest は digest フィールドに格納される課題ダイジェスト構造体（spec §13.1）。
type IssueDigest struct {
	Issue    DigestIssue     `json:"issue"`
	Project  DigestProject   `json:"project"`
	Meta     DigestMeta      `json:"meta"`
	Comments []DigestComment `json:"comments"`
	Activity []interface{}   `json:"activity"`
	Summary  DigestSummary   `json:"summary"`
	LLMHints DigestLLMHints  `json:"llm_hints"`
}

// DigestIssue は digest 内の課題情報（spec §13.1 digest.issue）。
type DigestIssue struct {
	ID           int                    `json:"id"`
	Key          string                 `json:"key"`
	Summary      string                 `json:"summary"`
	Description  string                 `json:"description"`
	Status       *domain.IDName         `json:"status,omitempty"`
	Priority     *domain.IDName         `json:"priority,omitempty"`
	IssueType    *domain.IDName         `json:"issue_type,omitempty"`
	Assignee     *domain.UserRef        `json:"assignee,omitempty"`
	Reporter     *domain.UserRef        `json:"reporter,omitempty"`
	Categories   []string               `json:"categories"`
	Versions     []string               `json:"versions"`
	Milestones   []string               `json:"milestones"`
	CustomFields []domain.CustomField   `json:"custom_fields"`
	Created      *time.Time             `json:"created,omitempty"`
	Updated      *time.Time             `json:"updated,omitempty"`
	DueDate      *time.Time             `json:"due_date,omitempty"`
	StartDate    *time.Time             `json:"start_date,omitempty"`
}

// DigestProject は digest 内のプロジェクト情報（spec §13.1 digest.project）。
type DigestProject struct {
	ID   int    `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// DigestMeta は digest 内のメタ情報（spec §13.1 digest.meta）。
type DigestMeta struct {
	Statuses     []domain.Status                `json:"statuses"`
	Categories   []domain.Category              `json:"categories"`
	Versions     []domain.Version               `json:"versions"`
	CustomFields []domain.CustomFieldDefinition  `json:"custom_fields"`
}

// DigestComment は digest 内のコメント情報（spec §13.1 digest.comments）。
type DigestComment struct {
	ID      int64           `json:"id"`
	Content string          `json:"content"`
	Author  *domain.UserRef `json:"author,omitempty"`
	Created *time.Time      `json:"created,omitempty"`
}

// DigestSummary は決定論的サマリー（spec §13.1 digest.summary）。
type DigestSummary struct {
	Headline              string `json:"headline"`
	CommentCountIncluded  int    `json:"comment_count_included"`
	ActivityCountIncluded int    `json:"activity_count_included"`
	HasDescription        bool   `json:"has_description"`
	HasAssignee           bool   `json:"has_assignee"`
	StatusName            string `json:"status_name"`
	PriorityName          string `json:"priority_name"`
}

// DigestLLMHints は LLM 向けヒント情報（spec §13.1 digest.llm_hints）。
type DigestLLMHints struct {
	PrimaryEntities      []string `json:"primary_entities"`
	OpenQuestions        []string `json:"open_questions"`
	SuggestedNextActions []string `json:"suggested_next_actions"`
}

// Build は指定課題キーのダイジェストを構築する。
// 必須データ（課題・プロジェクト）の取得に失敗した場合はエラーを返す。
// オプションデータ（コメント・メタ情報）の取得失敗は warning として記録し、
// 部分成功として DigestEnvelope を返す（spec §19 partial success behavior）。
func (b *DefaultIssueDigestBuilder) Build(ctx context.Context, issueKey string, opt IssueDigestOptions) (*domain.DigestEnvelope, error) {
	if opt.MaxComments <= 0 {
		opt.MaxComments = 5
	}

	var warnings []domain.Warning

	// 1. 課題取得（必須）
	issue, err := b.client.GetIssue(ctx, issueKey)
	if err != nil {
		return nil, fmt.Errorf("GetIssue(%s): %w", issueKey, err)
	}

	// issueKey の prefix からプロジェクトキーを取得する（例: "PROJ-123" → "PROJ"）。
	// Client interface が projectKey(string) を要求するため、issueKey の prefix を利用する。
	projectKey := extractProjectKey(issueKey)

	// 2. プロジェクト取得（必須）
	project, err := b.client.GetProject(ctx, projectKey)
	if err != nil {
		return nil, fmt.Errorf("GetProject(%s): %w", projectKey, err)
	}

	// 3. コメント取得（オプション）
	var digestComments []DigestComment
	comments, err := b.client.ListIssueComments(ctx, issueKey, backlog.ListCommentsOptions{Limit: opt.MaxComments})
	if err != nil {
		if !errors.Is(err, backlog.ErrNotFound) || comments == nil {
			warnings = append(warnings, domain.Warning{
				Code:      "comments_fetch_failed",
				Message:   fmt.Sprintf("コメントの取得に失敗しました: %v", err),
				Component: "comments",
				Retryable: true,
			})
		}
	} else {
		for _, c := range comments {
			dc := DigestComment{
				ID:      c.ID,
				Content: c.Content,
				Created: c.Created,
			}
			if c.CreatedUser != nil {
				dc.Author = &domain.UserRef{
					ID:   c.CreatedUser.ID,
					Name: c.CreatedUser.Name,
				}
			}
			digestComments = append(digestComments, dc)
		}
	}
	if digestComments == nil {
		digestComments = []DigestComment{}
	}

	// 4. プロジェクトメタ情報取得（オプション）
	meta := DigestMeta{
		Statuses:     []domain.Status{},
		Categories:   []domain.Category{},
		Versions:     []domain.Version{},
		CustomFields: []domain.CustomFieldDefinition{},
	}

	statuses, err := b.client.ListProjectStatuses(ctx, projectKey)
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "statuses_fetch_failed",
			Message:   fmt.Sprintf("ステータス一覧の取得に失敗しました: %v", err),
			Component: "meta.statuses",
			Retryable: true,
		})
	} else if statuses != nil {
		meta.Statuses = statuses
	}

	categories, err := b.client.ListProjectCategories(ctx, projectKey)
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "categories_fetch_failed",
			Message:   fmt.Sprintf("カテゴリ一覧の取得に失敗しました: %v", err),
			Component: "meta.categories",
			Retryable: true,
		})
	} else if categories != nil {
		meta.Categories = categories
	}

	versions, err := b.client.ListProjectVersions(ctx, projectKey)
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "versions_fetch_failed",
			Message:   fmt.Sprintf("バージョン一覧の取得に失敗しました: %v", err),
			Component: "meta.versions",
			Retryable: true,
		})
	} else if versions != nil {
		meta.Versions = versions
	}

	customFields, err := b.client.ListProjectCustomFields(ctx, projectKey)
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "custom_fields_fetch_failed",
			Message:   fmt.Sprintf("カスタムフィールド定義の取得に失敗しました: %v", err),
			Component: "meta.custom_fields",
			Retryable: true,
		})
	} else if customFields != nil {
		meta.CustomFields = customFields
	}

	// 5. DigestIssue 組み立て
	di := buildDigestIssue(issue)

	// 6. DigestProject 組み立て
	dp := DigestProject{
		ID:   project.ID,
		Key:  project.ProjectKey,
		Name: project.Name,
	}

	// 7. DigestSummary 組み立て（決定論的）
	summary := buildDigestSummary(issue, len(digestComments))

	// 8. LLMHints 組み立て
	hints := buildLLMHints(issue, projectKey)

	digestData := &IssueDigest{
		Issue:    di,
		Project:  dp,
		Meta:     meta,
		Comments: digestComments,
		Activity: []interface{}{},
		Summary:  summary,
		LLMHints: hints,
	}

	if warnings == nil {
		warnings = []domain.Warning{}
	}

	envelope := &domain.DigestEnvelope{
		SchemaVersion: "1",
		Resource:      "issue",
		GeneratedAt:   time.Now().UTC(),
		Profile:       b.profile,
		Space:         b.space,
		BaseURL:       b.baseURL,
		Warnings:      warnings,
		Digest:        digestData,
	}

	return envelope, nil
}

// extractProjectKey は issueKey（例: "PROJ-123"）からプロジェクトキー（"PROJ"）を抽出する。
func extractProjectKey(issueKey string) string {
	for i, c := range issueKey {
		if c == '-' {
			return issueKey[:i]
		}
	}
	return issueKey
}

// buildDigestIssue は domain.Issue から DigestIssue を構築する。
func buildDigestIssue(issue *domain.Issue) DigestIssue {
	di := DigestIssue{
		ID:          issue.ID,
		Key:         issue.IssueKey,
		Summary:     issue.Summary,
		Description: issue.Description,
		Status:      issue.Status,
		Priority:    issue.Priority,
		IssueType:   issue.IssueType,
		Created:     issue.Created,
		Updated:     issue.Updated,
		DueDate:     issue.DueDate,
		StartDate:   issue.StartDate,
	}

	// Assignee を UserRef に変換
	if issue.Assignee != nil {
		di.Assignee = &domain.UserRef{
			ID:   issue.Assignee.ID,
			Name: issue.Assignee.Name,
		}
	}

	// Reporter を UserRef に変換
	if issue.Reporter != nil {
		di.Reporter = &domain.UserRef{
			ID:   issue.Reporter.ID,
			Name: issue.Reporter.Name,
		}
	}

	// カテゴリ名のスライスを構築
	categories := make([]string, 0, len(issue.Categories))
	for _, c := range issue.Categories {
		categories = append(categories, c.Name)
	}
	di.Categories = categories

	// バージョン名のスライスを構築
	versions := make([]string, 0, len(issue.Versions))
	for _, v := range issue.Versions {
		versions = append(versions, v.Name)
	}
	di.Versions = versions

	// マイルストーン名のスライスを構築
	milestones := make([]string, 0, len(issue.Milestones))
	for _, m := range issue.Milestones {
		milestones = append(milestones, m.Name)
	}
	di.Milestones = milestones

	// カスタムフィールド
	if issue.CustomFields != nil {
		di.CustomFields = issue.CustomFields
	} else {
		di.CustomFields = []domain.CustomField{}
	}

	return di
}

// buildDigestSummary は決定論的サマリーを構築する（spec §13.1 digest.summary）。
func buildDigestSummary(issue *domain.Issue, commentCount int) DigestSummary {
	statusName := ""
	if issue.Status != nil {
		statusName = issue.Status.Name
	}

	priorityName := ""
	if issue.Priority != nil {
		priorityName = issue.Priority.Name
	}

	assigneeName := "未割り当て"
	hasAssignee := issue.Assignee != nil
	if hasAssignee {
		assigneeName = issue.Assignee.Name
	}

	issueTypeName := "課題"
	if issue.IssueType != nil {
		issueTypeName = issue.IssueType.Name
	}

	headline := fmt.Sprintf("%s課題 %s（%s に割り当て）", issueTypeName, issue.IssueKey, assigneeName)
	if statusName != "" {
		headline = fmt.Sprintf("%s課題 %s - ステータス: %s（%s に割り当て）", issueTypeName, issue.IssueKey, statusName, assigneeName)
	}

	return DigestSummary{
		Headline:              headline,
		CommentCountIncluded:  commentCount,
		ActivityCountIncluded: 0,
		HasDescription:        issue.Description != "",
		HasAssignee:           hasAssignee,
		StatusName:            statusName,
		PriorityName:          priorityName,
	}
}

// buildLLMHints は LLM ヒントを構築する（spec §13.1 digest.llm_hints）。
func buildLLMHints(issue *domain.Issue, projectKey string) DigestLLMHints {
	entities := []string{issue.IssueKey, projectKey}

	// マイルストーンを primary_entities に追加
	for _, m := range issue.Milestones {
		entities = append(entities, m.Name)
	}

	return DigestLLMHints{
		PrimaryEntities:      entities,
		OpenQuestions:        []string{},
		SuggestedNextActions: []string{},
	}
}

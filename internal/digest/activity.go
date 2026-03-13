package digest

import (
	"context"
	"fmt"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// ActivityDigestOptions は ActivityDigestBuilder.Build() のオプション（spec §19）。
type ActivityDigestOptions struct {
	// Project はプロジェクトキーでフィルタ（未指定の場合はスペース全体）。
	Project string
	// Since は取得開始日時。
	Since *time.Time
	// Until は取得終了日時。
	Until *time.Time
	// Limit は取得上限数。デフォルト 20。
	Limit int
}

// ActivityDigestBuilder はインターフェース（spec §19）。
type ActivityDigestBuilder interface {
	Build(ctx context.Context, opt ActivityDigestOptions) (*domain.DigestEnvelope, error)
}

// DefaultActivityDigestBuilder は ActivityDigestBuilder の標準実装。
type DefaultActivityDigestBuilder struct {
	BaseDigestBuilder
}

// NewDefaultActivityDigestBuilder は DefaultActivityDigestBuilder を生成する。
func NewDefaultActivityDigestBuilder(client backlog.Client, profile, space, baseURL string) *DefaultActivityDigestBuilder {
	return &DefaultActivityDigestBuilder{BaseDigestBuilder{client: client, profile: profile, space: space, baseURL: baseURL}}
}

// ActivityDigest は digest フィールドに格納されるアクティビティダイジェスト構造体（spec §13.3）。
type ActivityDigest struct {
	Scope      ActivityScope                     `json:"scope"`
	Activities []domain.NormalizedActivity       `json:"activities"`
	Comments   []DigestComment                   `json:"comments"`
	Projects   map[string]ActivityProjectSummary `json:"projects"`
	Summary    ActivityDigestSummary             `json:"summary"`
	LLMHints   DigestLLMHints                    `json:"llm_hints"`
}

// ActivityScope はアクティビティダイジェストのスコープ情報。
type ActivityScope struct {
	Project string     `json:"project,omitempty"`
	Since   *time.Time `json:"since,omitempty"`
	Until   *time.Time `json:"until,omitempty"`
	Limit   int        `json:"limit"`
}

// ActivityProjectSummary はプロジェクト別サマリー。
type ActivityProjectSummary struct {
	ActivityCount int `json:"activity_count"`
	CommentCount  int `json:"comment_count"`
}

// ActivityDigestSummary はアクティビティダイジェストの決定論的サマリー。
type ActivityDigestSummary struct {
	Headline           string         `json:"headline"`
	TotalActivity      int            `json:"total_activity"`
	CommentCount       int            `json:"comment_count"`
	Types              map[string]int `json:"types"`
	RelatedIssueKeys   []string       `json:"related_issue_keys"`
	RelatedProjectKeys []string       `json:"related_project_keys"`
}

// Build はアクティビティダイジェストを構築する。
// アクティビティの取得に失敗した場合は partial success として空のダイジェストと warning を返す。
func (b *DefaultActivityDigestBuilder) Build(ctx context.Context, opt ActivityDigestOptions) (*domain.DigestEnvelope, error) {
	if opt.Limit <= 0 {
		opt.Limit = 20
	}

	var warnings []domain.Warning

	// 1. アクティビティ取得（オプション扱い: 失敗は warning として処理）
	var rawActivities []domain.Activity
	var fetchErr error

	if opt.Project != "" {
		listOpt := backlog.ListActivitiesOptions{
			Since:  opt.Since,
			Until:  opt.Until,
			Limit:  opt.Limit,
		}
		rawActivities, fetchErr = b.client.ListProjectActivities(ctx, opt.Project, listOpt)
	} else {
		listOpt := backlog.ListActivitiesOptions{
			Since: opt.Since,
			Until: opt.Until,
			Limit: opt.Limit,
		}
		rawActivities, fetchErr = b.client.ListSpaceActivities(ctx, listOpt)
	}

	if fetchErr != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "activities_fetch_failed",
			Message:   fmt.Sprintf("アクティビティの取得に失敗しました: %v", fetchErr),
			Component: "activities",
			Retryable: true,
		})
		rawActivities = []domain.Activity{}
	}

	// 2. アクティビティを正規化
	normalized := normalizeActivities(rawActivities)

	// 3. コメントを抽出（issue_commented タイプのアクティビティから）
	comments := extractCommentsFromActivities(rawActivities)

	// 4. プロジェクト別サマリーを集計
	projects := buildActivityProjectSummary(rawActivities)

	// 5. サマリーを構築
	summary := buildActivityDigestSummary(opt.Project, normalized, comments)

	// 6. LLMHints を構築
	hints := buildActivityLLMHints(summary)

	digestData := &ActivityDigest{
		Scope: ActivityScope{
			Project: opt.Project,
			Since:   opt.Since,
			Until:   opt.Until,
			Limit:   opt.Limit,
		},
		Activities: normalized,
		Comments:   comments,
		Projects:   projects,
		Summary:    summary,
		LLMHints:   hints,
	}

	return b.newEnvelope("activity", digestData, warnings), nil
}

// normalizeActivities は []domain.Activity を []domain.NormalizedActivity に変換する。
// spec §12 準拠: type 番号を文字列名に変換する。
func normalizeActivities(activities []domain.Activity) []domain.NormalizedActivity {
	result := make([]domain.NormalizedActivity, 0, len(activities))
	for _, a := range activities {
		na := domain.NormalizedActivity{
			ID:      a.ID,
			Type:    activityTypeName(a.Type),
			Created: a.Created,
		}
		na.Actor = toUserRef(a.CreatedUser)
		// Content から Issue 参照を抽出
		if a.Content != nil {
			if issueRef := extractIssueRef(a.Content); issueRef != nil {
				na.Issue = issueRef
			}
			if commentRef := extractCommentRef(a.Content); commentRef != nil {
				na.Comment = commentRef
			}
		}
		result = append(result, na)
	}
	return result
}

// extractCommentsFromActivities は issue_commented タイプのアクティビティからコメントを抽出する。
func extractCommentsFromActivities(activities []domain.Activity) []DigestComment {
	var comments []DigestComment
	for _, a := range activities {
		if a.Type != 3 { // 3 = issue_commented
			continue
		}
		if a.Content == nil {
			continue
		}
		commentMap, ok := a.Content["comment"].(map[string]interface{})
		if !ok {
			continue
		}

		dc := DigestComment{
			Created: a.Created,
		}

		if id, ok := commentMap["id"].(float64); ok {
			dc.ID = int64(id)
		}
		if content, ok := commentMap["content"].(string); ok {
			dc.Content = content
		}
		dc.Author = toUserRef(a.CreatedUser)
		comments = append(comments, dc)
	}
	if comments == nil {
		return []DigestComment{}
	}
	return comments
}

// buildActivityProjectSummary はアクティビティからプロジェクト別サマリーを集計する。
func buildActivityProjectSummary(activities []domain.Activity) map[string]ActivityProjectSummary {
	result := make(map[string]ActivityProjectSummary)
	for _, a := range activities {
		if a.Content == nil {
			continue
		}
		key, ok := a.Content["key"].(string)
		if !ok || key == "" {
			continue
		}
		// key から project key を抽出（例: "PROJ-123" → "PROJ"）
		projKey := extractProjectKey(key)
		if projKey == "" {
			continue
		}
		s := result[projKey]
		s.ActivityCount++
		if a.Type == 3 { // issue_commented
			s.CommentCount++
		}
		result[projKey] = s
	}
	return result
}

// buildActivityDigestSummary は決定論的なアクティビティサマリーを構築する。
func buildActivityDigestSummary(project string, activities []domain.NormalizedActivity, comments []DigestComment) ActivityDigestSummary {
	types := make(map[string]int)
	issueKeySet := make(map[string]struct{})
	projectKeySet := make(map[string]struct{})

	for _, a := range activities {
		types[a.Type]++
		if a.Issue != nil && a.Issue.Key != "" {
			issueKeySet[a.Issue.Key] = struct{}{}
			projKey := extractProjectKey(a.Issue.Key)
			if projKey != "" {
				projectKeySet[projKey] = struct{}{}
			}
		}
	}

	issueKeys := make([]string, 0, len(issueKeySet))
	for k := range issueKeySet {
		issueKeys = append(issueKeys, k)
	}
	projectKeys := make([]string, 0, len(projectKeySet))
	for k := range projectKeySet {
		projectKeys = append(projectKeys, k)
	}

	headline := fmt.Sprintf("アクティビティダイジェスト（%d件）", len(activities))
	if project != "" {
		headline = fmt.Sprintf("プロジェクト %s のアクティビティダイジェスト（%d件）", project, len(activities))
	}

	return ActivityDigestSummary{
		Headline:           headline,
		TotalActivity:      len(activities),
		CommentCount:       len(comments),
		Types:              types,
		RelatedIssueKeys:   issueKeys,
		RelatedProjectKeys: projectKeys,
	}
}

// buildActivityLLMHints は LLM ヒントを構築する。
func buildActivityLLMHints(summary ActivityDigestSummary) DigestLLMHints {
	entities := make([]string, 0)
	entities = append(entities, summary.RelatedIssueKeys...)
	entities = append(entities, summary.RelatedProjectKeys...)

	return DigestLLMHints{
		PrimaryEntities:      entities,
		OpenQuestions:        []string{},
		SuggestedNextActions: []string{},
	}
}

// extractIssueRef は activity.Content から ActivityIssueRef を抽出する。
func extractIssueRef(content map[string]interface{}) *domain.ActivityIssueRef {
	id, hasID := content["id"].(float64)
	key, hasKey := content["key"].(string)
	if !hasID && !hasKey {
		return nil
	}
	ref := &domain.ActivityIssueRef{}
	if hasID {
		ref.ID = int(id)
	}
	if hasKey {
		ref.Key = key
	}
	if summary, ok := content["summary"].(string); ok {
		ref.Summary = summary
	}
	return ref
}

// extractCommentRef は activity.Content から ActivityCommentRef を抽出する。
func extractCommentRef(content map[string]interface{}) *domain.ActivityCommentRef {
	commentMap, ok := content["comment"].(map[string]interface{})
	if !ok {
		return nil
	}
	ref := &domain.ActivityCommentRef{}
	if id, ok := commentMap["id"].(float64); ok {
		ref.ID = int64(id)
	}
	if c, ok := commentMap["content"].(string); ok {
		ref.Content = c
	}
	return ref
}

// activityTypeName は Backlog の activity type 番号を文字列名に変換する（spec §12）。
func activityTypeName(t int) string {
	switch t {
	case 1:
		return "issue_created"
	case 2:
		return "issue_updated"
	case 3:
		return "issue_commented"
	case 4:
		return "issue_deleted"
	case 5:
		return "wiki_created"
	case 6:
		return "wiki_updated"
	case 7:
		return "wiki_deleted"
	case 8:
		return "file_added"
	case 9:
		return "file_updated"
	case 10:
		return "file_deleted"
	case 11:
		return "svn_committed"
	case 12:
		return "git_pushed"
	case 13:
		return "git_created"
	case 14:
		return "issue_multi_updated"
	case 15:
		return "project_user_added"
	case 16:
		return "project_user_deleted"
	case 17:
		return "comment_notification"
	case 18:
		return "pr_created"
	case 19:
		return "pr_updated"
	case 20:
		return "pr_commented"
	case 21:
		return "pr_merged"
	default:
		return fmt.Sprintf("unknown_%d", t)
	}
}

package digest

import (
	"context"
	"fmt"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// UserDigestOptions は UserDigestBuilder.Build() のオプション（spec §19）。
type UserDigestOptions struct {
	// Since は取得開始日時。
	Since *time.Time
	// Until は取得終了日時。
	Until *time.Time
	// Limit は取得上限数。デフォルト 20。
	Limit int
	// Project はプロジェクトキーでフィルタ（オプション）。
	Project string
}

// UserDigestBuilder はインターフェース（spec §19）。
type UserDigestBuilder interface {
	Build(ctx context.Context, userID string, opt UserDigestOptions) (*domain.DigestEnvelope, error)
}

// DefaultUserDigestBuilder は UserDigestBuilder の標準実装。
// backlog.Client を使って必要なデータを収集し DigestEnvelope を構築する。
type DefaultUserDigestBuilder struct {
	BaseDigestBuilder
}

// NewDefaultUserDigestBuilder は DefaultUserDigestBuilder を生成する。
func NewDefaultUserDigestBuilder(client backlog.Client, profile, space, baseURL string) *DefaultUserDigestBuilder {
	return &DefaultUserDigestBuilder{BaseDigestBuilder{client: client, profile: profile, space: space, baseURL: baseURL}}
}

// UserDigest は digest フィールドに格納されるユーザーダイジェスト構造体（spec §13.4）。
type UserDigest struct {
	User       DigestUser                        `json:"user"`
	Scope      UserScope                         `json:"scope"`
	Activities []domain.NormalizedActivity       `json:"activities"`
	Comments   []DigestComment                   `json:"comments"`
	Projects   map[string]ActivityProjectSummary `json:"projects"`
	Summary    UserDigestSummary                 `json:"summary"`
	LLMHints   DigestLLMHints                    `json:"llm_hints"`
}

// DigestUser は UserDigest 内のユーザー参照（spec §13.4）。
type DigestUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// UserScope はユーザーダイジェストのスコープ情報（期間）。
type UserScope struct {
	Since *time.Time `json:"since,omitempty"`
	Until *time.Time `json:"until,omitempty"`
}

// UserDigestSummary はユーザーダイジェストの決定論的サマリー（spec §13.4）。
type UserDigestSummary struct {
	Headline           string         `json:"headline"`
	TotalActivity      int            `json:"total_activity"`
	CommentCount       int            `json:"comment_count"`
	Types              map[string]int `json:"types"`
	RelatedIssueKeys   []string       `json:"related_issue_keys"`
	RelatedProjectKeys []string       `json:"related_project_keys"`
}

// Build は指定ユーザーのダイジェストを構築する。
// ユーザー取得に失敗した場合はエラーを返す（必須）。
// アクティビティ・プロジェクト情報の取得失敗は warning として記録し、
// 部分成功として DigestEnvelope を返す（spec §13.4 / partial success behavior）。
func (b *DefaultUserDigestBuilder) Build(ctx context.Context, userID string, opt UserDigestOptions) (*domain.DigestEnvelope, error) {
	if opt.Limit <= 0 {
		opt.Limit = 20
	}

	var warnings []domain.Warning

	// 1. ユーザー取得（必須）
	user, err := b.client.GetUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("GetUser(%s): %w", userID, err)
	}

	// 2. アクティビティ取得（オプション扱い: 失敗は warning として処理）
	listOpt := backlog.ListUserActivitiesOptions{
		Since:   opt.Since,
		Until:   opt.Until,
		Limit:   opt.Limit,
		Project: opt.Project,
	}
	rawActivities, actErr := b.client.ListUserActivities(ctx, userID, listOpt)
	if actErr != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "activities_fetch_failed",
			Message:   fmt.Sprintf("アクティビティの取得に失敗しました: %v", actErr),
			Component: "activities",
			Retryable: true,
		})
		rawActivities = []domain.Activity{}
	}

	// 3. アクティビティを正規化
	normalized := normalizeActivities(rawActivities)

	// 4. コメントを抽出（issue_commented タイプのアクティビティから）
	comments := extractCommentsFromActivities(rawActivities)

	// 5. プロジェクト別サマリーを集計
	projects := buildActivityProjectSummary(rawActivities)

	// 6. サマリーを構築
	summary := buildUserDigestSummary(user, normalized, comments)

	// 7. LLMHints を構築
	hints := buildUserDigestLLMHints(user, summary)

	digestData := &UserDigest{
		User: DigestUser{
			ID:   user.ID,
			Name: user.Name,
		},
		Scope: UserScope{
			Since: opt.Since,
			Until: opt.Until,
		},
		Activities: normalized,
		Comments:   comments,
		Projects:   projects,
		Summary:    summary,
		LLMHints:   hints,
	}

	return b.newEnvelope("user", digestData, warnings), nil
}

// buildUserDigestSummary は決定論的なユーザーダイジェストサマリーを構築する（spec §13.4）。
func buildUserDigestSummary(user *domain.User, activities []domain.NormalizedActivity, comments []DigestComment) UserDigestSummary {
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

	headline := fmt.Sprintf("User activity digest for %s", user.Name)

	return UserDigestSummary{
		Headline:           headline,
		TotalActivity:      len(activities),
		CommentCount:       len(comments),
		Types:              types,
		RelatedIssueKeys:   issueKeys,
		RelatedProjectKeys: projectKeys,
	}
}

// buildUserDigestLLMHints は LLM ヒントを構築する（spec §13.4）。
func buildUserDigestLLMHints(user *domain.User, summary UserDigestSummary) DigestLLMHints {
	entities := []string{fmt.Sprintf("user:%d", user.ID), user.Name}
	entities = append(entities, summary.RelatedIssueKeys...)
	entities = append(entities, summary.RelatedProjectKeys...)

	return DigestLLMHints{
		PrimaryEntities:      entities,
		OpenQuestions:        []string{},
		SuggestedNextActions: []string{},
	}
}

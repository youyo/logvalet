package digest

import (
	"context"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// ProjectDigestOptions は ProjectDigestBuilder.Build() のオプション。
// 将来の拡張のためのプレースホルダー。
type ProjectDigestOptions struct{}

// ProjectDigestBuilder はインターフェース（spec §13.2）。
type ProjectDigestBuilder interface {
	Build(ctx context.Context, projectKey string, opt ProjectDigestOptions) (*domain.DigestEnvelope, error)
}

// DefaultProjectDigestBuilder は ProjectDigestBuilder の標準実装。
// backlog.Client を使って必要なデータを収集し DigestEnvelope を構築する。
type DefaultProjectDigestBuilder struct {
	BaseDigestBuilder
}

// NewDefaultProjectDigestBuilder は DefaultProjectDigestBuilder を生成する。
func NewDefaultProjectDigestBuilder(client backlog.Client, profile, space, baseURL string) *DefaultProjectDigestBuilder {
	return &DefaultProjectDigestBuilder{BaseDigestBuilder{client: client, profile: profile, space: space, baseURL: baseURL}}
}

// ProjectDigest は digest フィールドに格納されるプロジェクトダイジェスト構造体（spec §13.2）。
type ProjectDigest struct {
	Project        DigestProjectDetail   `json:"project"`
	Meta           DigestMeta            `json:"meta"`
	Teams          []domain.Team         `json:"teams"`
	RecentActivity []interface{}         `json:"recent_activity"`
	Summary        ProjectDigestSummary  `json:"summary"`
	LLMHints       DigestLLMHints        `json:"llm_hints"`
}

// DigestProjectDetail は Project Digest 内のプロジェクト詳細情報（spec §13.2 project）。
// DigestProject（issue.go で定義）より多くのフィールドを含む。
type DigestProjectDetail struct {
	ID       int    `json:"id"`
	Key      string `json:"key"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}

// ProjectDigestSummary は Project Digest の決定論的サマリー（spec §13.2 summary）。
type ProjectDigestSummary struct {
	Headline              string `json:"headline"`
	TeamCount             int    `json:"team_count"`
	ActivityCountIncluded int    `json:"activity_count_included"`
	StatusCount           int    `json:"status_count"`
	CategoryCount         int    `json:"category_count"`
	VersionCount          int    `json:"version_count"`
	IsArchived            bool   `json:"is_archived"`
}

// Build は指定プロジェクトキーのダイジェストを構築する。
// 必須データ（プロジェクト）の取得に失敗した場合はエラーを返す。
// オプションデータ（メタ情報・チーム・アクティビティ）の取得失敗は warning として記録し、
// 部分成功として DigestEnvelope を返す（spec §13.2 / partial success behavior）。
func (b *DefaultProjectDigestBuilder) Build(ctx context.Context, projectKey string, opt ProjectDigestOptions) (*domain.DigestEnvelope, error) {
	var warnings []domain.Warning

	// 1. プロジェクト取得（必須）
	project, err := b.client.GetProject(ctx, projectKey)
	if err != nil {
		return nil, fmt.Errorf("GetProject(%s): %w", projectKey, err)
	}

	// 2. プロジェクトメタ情報取得（並行・オプション）
	meta, metaWarnings := fetchProjectMeta(ctx, b.client, projectKey)
	warnings = append(warnings, metaWarnings...)

	// 3. チーム取得（オプション）
	var teams []domain.Team
	projectTeams, err := b.client.ListProjectTeams(ctx, projectKey)
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "teams_fetch_failed",
			Message:   fmt.Sprintf("チーム一覧の取得に失敗しました: %v", err),
			Component: "teams",
			Retryable: true,
		})
	} else if projectTeams != nil {
		teams = projectTeams
	}
	if teams == nil {
		teams = []domain.Team{}
	}

	// 4. 最近のアクティビティ取得（オプション）
	var recentActivity []interface{}
	activities, err := b.client.ListProjectActivities(ctx, projectKey, backlog.ListActivitiesOptions{})
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "activity_fetch_failed",
			Message:   fmt.Sprintf("アクティビティの取得に失敗しました: %v", err),
			Component: "recent_activity",
			Retryable: true,
		})
	} else {
		for _, a := range activities {
			recentActivity = append(recentActivity, a)
		}
	}
	if recentActivity == nil {
		recentActivity = []interface{}{}
	}

	// 5. DigestProjectDetail 組み立て
	dp := DigestProjectDetail{
		ID:       project.ID,
		Key:      project.ProjectKey,
		Name:     project.Name,
		Archived: project.Archived,
	}

	// 6. ProjectDigestSummary 組み立て（決定論的）
	summary := buildProjectDigestSummary(project, len(teams), len(activities), len(meta.Statuses), len(meta.Categories), len(meta.Versions))

	// 7. LLMHints 組み立て
	hints := buildProjectLLMHints(project)

	digestData := &ProjectDigest{
		Project:        dp,
		Meta:           meta,
		Teams:          teams,
		RecentActivity: recentActivity,
		Summary:        summary,
		LLMHints:       hints,
	}

	return b.newEnvelope("project", digestData, warnings), nil
}

// buildProjectDigestSummary は決定論的プロジェクトサマリーを構築する（spec §13.2 summary）。
func buildProjectDigestSummary(project *domain.Project, teamCount, activityCount, statusCount, categoryCount, versionCount int) ProjectDigestSummary {
	archivedText := ""
	if project.Archived {
		archivedText = "（アーカイブ済み）"
	}

	headline := fmt.Sprintf("プロジェクト %s - %s%s", project.ProjectKey, project.Name, archivedText)

	return ProjectDigestSummary{
		Headline:              headline,
		TeamCount:             teamCount,
		ActivityCountIncluded: activityCount,
		StatusCount:           statusCount,
		CategoryCount:         categoryCount,
		VersionCount:          versionCount,
		IsArchived:            project.Archived,
	}
}

// buildProjectLLMHints は LLM ヒントを構築する（spec §13.2 llm_hints）。
func buildProjectLLMHints(project *domain.Project) DigestLLMHints {
	entities := []string{project.ProjectKey, project.Name}

	return DigestLLMHints{
		PrimaryEntities:      entities,
		OpenQuestions:        []string{},
		SuggestedNextActions: []string{},
	}
}

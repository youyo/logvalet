package digest

import (
	"context"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// TeamDigestOptions は TeamDigestBuilder.Build() のオプション。
// 将来の拡張のためのプレースホルダー。
type TeamDigestOptions struct{}

// TeamDigestBuilder はインターフェース（spec §13.6）。
type TeamDigestBuilder interface {
	Build(ctx context.Context, teamID int, opt TeamDigestOptions) (*domain.DigestEnvelope, error)
}

// DefaultTeamDigestBuilder は TeamDigestBuilder の標準実装。
// backlog.Client を使って必要なデータを収集し DigestEnvelope を構築する。
type DefaultTeamDigestBuilder struct {
	BaseDigestBuilder
}

// NewDefaultTeamDigestBuilder は DefaultTeamDigestBuilder を生成する。
func NewDefaultTeamDigestBuilder(client backlog.Client, profile, space, baseURL string) *DefaultTeamDigestBuilder {
	return &DefaultTeamDigestBuilder{BaseDigestBuilder{client: client, profile: profile, space: space, baseURL: baseURL}}
}

// TeamDigest は digest フィールドに格納されるチームダイジェスト構造体（spec §13.6）。
type TeamDigest struct {
	Team     DigestTeam        `json:"team"`
	Projects []domain.Project  `json:"projects"`
	Summary  TeamDigestSummary `json:"summary"`
	LLMHints DigestLLMHints    `json:"llm_hints"`
}

// DigestTeam はチームダイジェスト内のチーム情報（spec §13.6）。
type DigestTeam struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TeamDigestSummary はチームダイジェストの決定論的サマリー（spec §13.6）。
type TeamDigestSummary struct {
	Headline     string `json:"headline"`
	ProjectCount int    `json:"project_count"`
}

// Build は指定チーム ID のダイジェストを構築する。
// チーム取得に失敗した場合または指定 teamID が見つからない場合はエラーを返す（必須）。
// プロジェクト情報の取得失敗は warning として記録し、
// 部分成功として DigestEnvelope を返す（spec §13.6 / partial success behavior）。
func (b *DefaultTeamDigestBuilder) Build(ctx context.Context, teamID int, opt TeamDigestOptions) (*domain.DigestEnvelope, error) {
	var warnings []domain.Warning

	// 1. チーム一覧を取得し、teamID に一致するチームを検索（必須）
	teams, err := b.client.ListTeams(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListTeams: %w", err)
	}

	var targetTeam *domain.Team
	for i := range teams {
		if teams[i].ID == teamID {
			targetTeam = &teams[i]
			break
		}
	}
	if targetTeam == nil {
		return nil, fmt.Errorf("team id %d not found: %w", teamID, backlog.ErrNotFound)
	}

	// 2. 全プロジェクト一覧を取得（オプション）
	projects, err := b.client.ListProjects(ctx)
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "projects_fetch_failed",
			Message:   fmt.Sprintf("プロジェクト一覧の取得に失敗しました: %v", err),
			Component: "projects",
			Retryable: true,
		})
		projects = nil
	}

	// 3. 各プロジェクトに対して ListProjectTeams を呼び、このチームが属するプロジェクトを収集（オプション）
	var teamProjects []domain.Project
	for _, proj := range projects {
		projTeams, err := b.client.ListProjectTeams(ctx, proj.ProjectKey)
		if err != nil {
			warnings = append(warnings, domain.Warning{
				Code:      "project_teams_fetch_failed",
				Message:   fmt.Sprintf("プロジェクト %s のチーム一覧取得に失敗しました: %v", proj.ProjectKey, err),
				Component: fmt.Sprintf("projects.%s.teams", proj.ProjectKey),
				Retryable: true,
			})
			continue
		}
		for _, t := range projTeams {
			if t.ID == teamID {
				teamProjects = append(teamProjects, proj)
				break
			}
		}
	}
	if teamProjects == nil {
		teamProjects = []domain.Project{}
	}

	// 4. TeamDigest 組み立て
	dt := DigestTeam{
		ID:   targetTeam.ID,
		Name: targetTeam.Name,
	}

	// 5. TeamDigestSummary 組み立て（決定論的）
	summary := buildTeamDigestSummary(targetTeam, len(teamProjects))

	// 6. LLMHints 組み立て
	hints := buildTeamDigestLLMHints(targetTeam, teamProjects)

	digestData := &TeamDigest{
		Team:     dt,
		Projects: teamProjects,
		Summary:  summary,
		LLMHints: hints,
	}

	return b.newEnvelope("team", digestData, warnings), nil
}

// buildTeamDigestSummary は決定論的チームサマリーを構築する（spec §13.6）。
func buildTeamDigestSummary(team *domain.Team, projectCount int) TeamDigestSummary {
	headline := fmt.Sprintf("チーム %s（%d プロジェクト）", team.Name, projectCount)
	return TeamDigestSummary{
		Headline:     headline,
		ProjectCount: projectCount,
	}
}

// buildTeamDigestLLMHints は LLM ヒントを構築する（spec §13.6）。
func buildTeamDigestLLMHints(team *domain.Team, projects []domain.Project) DigestLLMHints {
	entities := []string{fmt.Sprintf("team:%d", team.ID), team.Name}
	for _, p := range projects {
		entities = append(entities, p.ProjectKey)
	}
	return DigestLLMHints{
		PrimaryEntities:      entities,
		OpenQuestions:        []string{},
		SuggestedNextActions: []string{},
	}
}


package digest

import (
	"context"
	"errors"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

func TestTeamDigestBuilder_Build_success(t *testing.T) {
	mock := backlog.NewMockClient()

	teams := []domain.Team{
		{ID: 1, Name: "Backend Team"},
		{ID: 2, Name: "Frontend Team"},
	}
	projects := []domain.Project{
		{ID: 10, ProjectKey: "BACK", Name: "Backend Project", Archived: false},
		{ID: 20, ProjectKey: "FRONT", Name: "Frontend Project", Archived: false},
	}

	mock.ListTeamsFunc = func(ctx context.Context) ([]domain.Team, error) {
		return teams, nil
	}
	mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
		return projects, nil
	}
	// チーム1はBACKプロジェクトに所属
	mock.ListProjectTeamsFunc = func(ctx context.Context, projectKey string) ([]domain.Team, error) {
		if projectKey == "BACK" {
			return []domain.Team{{ID: 1, Name: "Backend Team"}}, nil
		}
		return []domain.Team{}, nil
	}

	builder := NewDefaultTeamDigestBuilder(mock, "default", "myspace", "https://myspace.backlog.com")
	envelope, err := builder.Build(context.Background(), 1, TeamDigestOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if envelope == nil {
		t.Fatal("envelope is nil")
	}
	if envelope.Resource != "team" {
		t.Errorf("Resource = %q, want %q", envelope.Resource, "team")
	}
	if len(envelope.Warnings) != 0 {
		t.Errorf("Warnings = %v, want empty", envelope.Warnings)
	}

	d, ok := envelope.Digest.(*TeamDigest)
	if !ok {
		t.Fatal("Digest is not *TeamDigest")
	}
	if d.Team.ID != 1 {
		t.Errorf("Team.ID = %d, want 1", d.Team.ID)
	}
	if d.Team.Name != "Backend Team" {
		t.Errorf("Team.Name = %q, want %q", d.Team.Name, "Backend Team")
	}
	if len(d.Projects) != 1 {
		t.Errorf("Projects count = %d, want 1", len(d.Projects))
	}
	if d.Projects[0].ProjectKey != "BACK" {
		t.Errorf("Projects[0].ProjectKey = %q, want %q", d.Projects[0].ProjectKey, "BACK")
	}
	if d.Summary.ProjectCount != 1 {
		t.Errorf("Summary.ProjectCount = %d, want 1", d.Summary.ProjectCount)
	}
	if d.Summary.Headline == "" {
		t.Error("Summary.Headline is empty")
	}
}

func TestTeamDigestBuilder_Build_team_not_found(t *testing.T) {
	mock := backlog.NewMockClient()

	teams := []domain.Team{
		{ID: 2, Name: "Frontend Team"},
	}
	mock.ListTeamsFunc = func(ctx context.Context) ([]domain.Team, error) {
		return teams, nil
	}

	builder := NewDefaultTeamDigestBuilder(mock, "default", "myspace", "https://myspace.backlog.com")
	_, err := builder.Build(context.Background(), 1, TeamDigestOptions{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, backlog.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestTeamDigestBuilder_Build_teams_fetch_failed(t *testing.T) {
	mock := backlog.NewMockClient()

	fetchErr := errors.New("API unavailable")
	mock.ListTeamsFunc = func(ctx context.Context) ([]domain.Team, error) {
		return nil, fetchErr
	}

	builder := NewDefaultTeamDigestBuilder(mock, "default", "myspace", "https://myspace.backlog.com")
	_, err := builder.Build(context.Background(), 1, TeamDigestOptions{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTeamDigestBuilder_Build_projects_fetch_failed(t *testing.T) {
	mock := backlog.NewMockClient()

	teams := []domain.Team{
		{ID: 1, Name: "Backend Team"},
	}
	mock.ListTeamsFunc = func(ctx context.Context) ([]domain.Team, error) {
		return teams, nil
	}
	mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
		return nil, errors.New("projects fetch failed")
	}

	builder := NewDefaultTeamDigestBuilder(mock, "default", "myspace", "https://myspace.backlog.com")
	envelope, err := builder.Build(context.Background(), 1, TeamDigestOptions{})

	// プロジェクト取得失敗は partial success（warning 付き空プロジェクト）
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if envelope == nil {
		t.Fatal("envelope is nil")
	}
	if len(envelope.Warnings) == 0 {
		t.Error("expected warnings, got none")
	}

	d, ok := envelope.Digest.(*TeamDigest)
	if !ok {
		t.Fatal("Digest is not *TeamDigest")
	}
	if len(d.Projects) != 0 {
		t.Errorf("Projects count = %d, want 0", len(d.Projects))
	}
}

func TestTeamDigestBuilder_Build_project_teams_fetch_failed(t *testing.T) {
	mock := backlog.NewMockClient()

	teams := []domain.Team{
		{ID: 1, Name: "Backend Team"},
	}
	projects := []domain.Project{
		{ID: 10, ProjectKey: "BACK", Name: "Backend Project", Archived: false},
	}
	mock.ListTeamsFunc = func(ctx context.Context) ([]domain.Team, error) {
		return teams, nil
	}
	mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
		return projects, nil
	}
	mock.ListProjectTeamsFunc = func(ctx context.Context, projectKey string) ([]domain.Team, error) {
		return nil, errors.New("ListProjectTeams failed")
	}

	builder := NewDefaultTeamDigestBuilder(mock, "default", "myspace", "https://myspace.backlog.com")
	envelope, err := builder.Build(context.Background(), 1, TeamDigestOptions{})

	// ListProjectTeams 失敗は partial success（warning あり、プロジェクト空）
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envelope.Warnings) == 0 {
		t.Error("expected warnings for ListProjectTeams failures, got none")
	}
	d, ok := envelope.Digest.(*TeamDigest)
	if !ok {
		t.Fatal("Digest is not *TeamDigest")
	}
	if len(d.Projects) != 0 {
		t.Errorf("Projects count = %d, want 0", len(d.Projects))
	}
}

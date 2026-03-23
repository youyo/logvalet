package backlog_test

import (
	"context"
	"errors"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

func TestMockClientGetMyself(t *testing.T) {
	t.Run("returns value from func", func(t *testing.T) {
		want := &domain.User{ID: 1, Name: "Test User"}
		mock := backlog.NewMockClient()
		mock.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
			return want, nil
		}
		got, err := mock.GetMyself(context.Background())
		if err != nil {
			t.Fatalf("GetMyself() error = %v", err)
		}
		if got.ID != want.ID {
			t.Errorf("GetMyself() ID = %d, want %d", got.ID, want.ID)
		}
		if mock.GetCallCount("GetMyself") != 1 {
			t.Errorf("GetCallCount(GetMyself) = %d, want 1", mock.GetCallCount("GetMyself"))
		}
	})

	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.GetMyself(context.Background())
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("GetMyself() error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientGetIssue(t *testing.T) {
	t.Run("returns issue from func", func(t *testing.T) {
		want := &domain.Issue{IssueKey: "PROJ-123", Summary: "Test Issue"}
		mock := backlog.NewMockClient()
		mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
			if issueKey != "PROJ-123" {
				return nil, backlog.ErrNotFound
			}
			return want, nil
		}
		got, err := mock.GetIssue(context.Background(), "PROJ-123")
		if err != nil {
			t.Fatalf("GetIssue() error = %v", err)
		}
		if got.IssueKey != "PROJ-123" {
			t.Errorf("GetIssue() IssueKey = %q, want %q", got.IssueKey, "PROJ-123")
		}
	})

	t.Run("call count increments", func(t *testing.T) {
		mock := backlog.NewMockClient()
		mock.GetIssueFunc = func(ctx context.Context, issueKey string) (*domain.Issue, error) {
			return &domain.Issue{IssueKey: issueKey}, nil
		}
		_, _ = mock.GetIssue(context.Background(), "A-1")
		_, _ = mock.GetIssue(context.Background(), "A-2")
		if mock.GetCallCount("GetIssue") != 2 {
			t.Errorf("GetCallCount(GetIssue) = %d, want 2", mock.GetCallCount("GetIssue"))
		}
	})
}

func TestMockClientListIssues(t *testing.T) {
	t.Run("returns issues from func", func(t *testing.T) {
		mock := backlog.NewMockClient()
		mock.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
			return []domain.Issue{{IssueKey: "A-1"}, {IssueKey: "A-2"}}, nil
		}
		got, err := mock.ListIssues(context.Background(), backlog.ListIssuesOptions{ProjectIDs: []int{1}})
		if err != nil {
			t.Fatalf("ListIssues() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("ListIssues() len = %d, want 2", len(got))
		}
	})
}

func TestMockClientCallCountThreadSafe(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return &domain.User{ID: 1}, nil
	}
	// 並列呼び出し
	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = mock.GetMyself(context.Background())
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	if mock.GetCallCount("GetMyself") != 10 {
		t.Errorf("GetCallCount(GetMyself) = %d, want 10", mock.GetCallCount("GetMyself"))
	}
}

func TestMockClientGetTeam(t *testing.T) {
	t.Run("returns TeamWithMembers from func", func(t *testing.T) {
		want := &domain.TeamWithMembers{
			ID:   173843,
			Name: "Test Team",
			Members: []domain.User{
				{ID: 10, Name: "User Ten"},
			},
		}
		mock := backlog.NewMockClient()
		mock.GetTeamFunc = func(ctx context.Context, teamID int) (*domain.TeamWithMembers, error) {
			if teamID != 173843 {
				return nil, backlog.ErrNotFound
			}
			return want, nil
		}
		got, err := mock.GetTeam(context.Background(), 173843)
		if err != nil {
			t.Fatalf("GetTeam() error = %v", err)
		}
		if got.ID != 173843 {
			t.Errorf("ID = %d, want 173843", got.ID)
		}
		if len(got.Members) != 1 {
			t.Fatalf("len(Members) = %d, want 1", len(got.Members))
		}
		if mock.GetCallCount("GetTeam") != 1 {
			t.Errorf("GetCallCount(GetTeam) = %d, want 1", mock.GetCallCount("GetTeam"))
		}
	})

	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.GetTeam(context.Background(), 173843)
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("GetTeam() error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientListTeams_withMembers(t *testing.T) {
	t.Run("returns TeamWithMembers slice from func", func(t *testing.T) {
		want := []domain.TeamWithMembers{
			{
				ID:   173843,
				Name: "ヘプタゴン",
				Members: []domain.User{
					{ID: 10, Name: "Alice"},
					{ID: 11, Name: "Bob"},
				},
			},
		}
		mock := backlog.NewMockClient()
		mock.ListTeamsFunc = func(ctx context.Context) ([]domain.TeamWithMembers, error) {
			return want, nil
		}
		got, err := mock.ListTeams(context.Background())
		if err != nil {
			t.Fatalf("ListTeams() error = %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		if got[0].ID != 173843 {
			t.Errorf("ID = %d, want 173843", got[0].ID)
		}
		if len(got[0].Members) != 2 {
			t.Errorf("len(Members) = %d, want 2", len(got[0].Members))
		}
		if mock.GetCallCount("ListTeams") != 1 {
			t.Errorf("GetCallCount(ListTeams) = %d, want 1", mock.GetCallCount("ListTeams"))
		}
	})

	t.Run("returns ErrNotFound when func not set", func(t *testing.T) {
		mock := backlog.NewMockClient()
		_, err := mock.ListTeams(context.Background())
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("ListTeams() error = %v, want ErrNotFound", err)
		}
	})
}

func TestMockClientAllMethodsDefaultToErrNotFound(t *testing.T) {
	mock := backlog.NewMockClient()
	ctx := context.Background()

	t.Run("ListUsers", func(t *testing.T) {
		_, err := mock.ListUsers(ctx)
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
	t.Run("GetIssue", func(t *testing.T) {
		_, err := mock.GetIssue(ctx, "A-1")
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
	t.Run("ListIssues", func(t *testing.T) {
		_, err := mock.ListIssues(ctx, backlog.ListIssuesOptions{})
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
	t.Run("GetSpace", func(t *testing.T) {
		_, err := mock.GetSpace(ctx)
		if !errors.Is(err, backlog.ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

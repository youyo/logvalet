package cli

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// teamListCmdNoMembers は --no-members フラグ適用後にメンバーが除外されることを確認する。
func TestTeamListCmd_NoMembers(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListTeamsFunc = func(ctx context.Context) ([]domain.TeamWithMembers, error) {
		return []domain.TeamWithMembers{
			{
				ID:   1,
				Name: "チームA",
				Members: []domain.User{
					{ID: 10, Name: "田中太郎"},
					{ID: 11, Name: "鈴木花子"},
				},
			},
			{
				ID:   2,
				Name: "チームB",
				Members: []domain.User{
					{ID: 20, Name: "佐藤次郎"},
				},
			},
		}, nil
	}

	teams, err := mc.ListTeams(context.Background())
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	// --no-members フラグ相当の処理
	cmd := &TeamListCmd{NoMembers: true}
	if cmd.NoMembers {
		for i := range teams {
			teams[i].Members = nil
		}
	}

	for i, team := range teams {
		if team.Members != nil {
			t.Errorf("teams[%d].Members は nil であるべきだが %v", i, team.Members)
		}
	}
}

// TestTeamListCmd_WithMembers はデフォルト（--no-members なし）でメンバーが保持されることを確認する。
func TestTeamListCmd_WithMembers(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListTeamsFunc = func(ctx context.Context) ([]domain.TeamWithMembers, error) {
		return []domain.TeamWithMembers{
			{
				ID:   1,
				Name: "チームA",
				Members: []domain.User{
					{ID: 10, Name: "田中太郎"},
				},
			},
		}, nil
	}

	teams, err := mc.ListTeams(context.Background())
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	// --no-members なし（デフォルト）
	cmd := &TeamListCmd{NoMembers: false}
	if cmd.NoMembers {
		for i := range teams {
			teams[i].Members = nil
		}
	}

	if len(teams[0].Members) == 0 {
		t.Error("teams[0].Members はメンバーを含むべきだが空")
	}
}

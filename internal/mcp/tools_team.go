package mcp

import (
	"context"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// RegisterTeamTools はチーム関連の MCP tools を ToolRegistry に登録する。
func RegisterTeamTools(r *ToolRegistry) {
	// logvalet_team_list
	r.Register(gomcp.NewTool("logvalet_team_list",
		gomcp.WithDescription("List all teams in the space"),
		gomcp.WithNumber("count", gomcp.Description("Max number of teams (max 100)")),
		gomcp.WithNumber("offset", gomcp.Description("Offset for pagination")),
		gomcp.WithBoolean("no_members", gomcp.Description("If true, exclude member information from response")),
		readOnlyAnnotation("チーム一覧取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		opt := backlog.ListTeamsOptions{}
		if count, ok := intArg(args, "count"); ok && count > 0 {
			opt.Count = count
		}
		if offset, ok := intArg(args, "offset"); ok && offset > 0 {
			opt.Offset = offset
		}

		teams, err := client.ListTeams(ctx, opt)
		if err != nil {
			return nil, err
		}

		// no_members=true の場合、[]domain.Team に射影（members キーを除外）
		if noMembers, ok := boolArg(args, "no_members"); ok && noMembers {
			out := make([]domain.Team, 0, len(teams))
			for _, t := range teams {
				out = append(out, domain.Team{ID: t.ID, Name: t.Name})
			}
			return out, nil
		}

		return teams, nil
	})

	// logvalet_team_get
	r.Register(gomcp.NewTool("logvalet_team_get",
		gomcp.WithDescription("Get team details by team ID"),
		gomcp.WithNumber("team_id", gomcp.Required(), gomcp.Description("Team ID (numeric)")),
		readOnlyAnnotation("チーム詳細取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		teamID, ok := intArg(args, "team_id")
		if !ok || teamID == 0 {
			return nil, fmt.Errorf("team_id is required")
		}
		return client.GetTeam(ctx, teamID)
	})
}

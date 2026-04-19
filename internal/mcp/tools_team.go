package mcp

import (
	"context"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
)

// RegisterTeamTools はチーム関連の MCP tools を ToolRegistry に登録する。
func RegisterTeamTools(r *ToolRegistry) {
	// logvalet_team_list
	r.Register(gomcp.NewTool("logvalet_team_list",
		gomcp.WithDescription("List all teams in the space"),
		readOnlyAnnotation("チーム一覧取得"),
	), func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		return client.ListTeams(ctx)
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

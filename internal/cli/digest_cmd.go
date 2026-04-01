package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/youyo/logvalet/internal/digest"
)

// DigestCmd は lv digest コマンドグループ。
// unified / weekly / daily の各サブコマンドを含む。
type DigestCmd struct {
	Unified DigestUnifiedCmd `cmd:"" name:"unified" help:"generate unified digest (issues + activities)"`
	Weekly  DigestWeeklyCmd  `cmd:"" help:"generate weekly periodic digest"`
	Daily   DigestDailyCmd   `cmd:"" help:"generate daily periodic digest"`
}

// DigestUnifiedCmd は lv digest unified コマンド（旧 DigestCmd の機能を継承）。
// 課題・アクティビティを統合した期間スコープ指定のダイジェストを生成する。
type DigestUnifiedCmd struct {
	// Project はプロジェクトキー（複数指定可）。
	Project []string `short:"k" help:"project key (multiple allowed)"`
	// User はユーザー指定（me, 数値ID, ユーザー名。複数指定可）。
	User []string `help:"user (me, numeric ID, or user name, multiple allowed)"`
	// Team はチームID またはチーム名（複数指定可）。メンバー全員の課題・アクティビティを取得する。
	Team []string `help:"team ID or name (multiple allowed)"`
	// Issue は課題キー（複数指定可）。
	Issue []string `help:"issue key (multiple allowed)"`
	// Since は期間開始（today, this-week, this-month, YYYY-MM-DD）。必須。
	Since string `help:"period start (today, this-week, this-month, YYYY-MM-DD)" required:""`
	// Until は期間終了（today, this-week, this-month, YYYY-MM-DD）。省略時は today。
	Until string `help:"period end (today, this-week, this-month, YYYY-MM-DD)"`
	// DueDate は期限日フィルタ。
	DueDate string `help:"due date filter (today, overdue, this-week, this-month, YYYY-MM-DD, YYYY-MM-DD:YYYY-MM-DD)"`
	// StartDate は開始日フィルタ。
	StartDate string `help:"start date filter (today, this-week, this-month, YYYY-MM-DD, YYYY-MM-DD:YYYY-MM-DD)"`
}

// Run は digest unified コマンドの実行。
func (c *DigestUnifiedCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	// 1. 期間解決
	since, until, err := resolvePeriod(c.Since, c.Until)
	if err != nil {
		return fmt.Errorf("failed to resolve period: %w", err)
	}

	scope := digest.UnifiedDigestScope{
		IssueKeys: c.Issue,
		Since:     since,
		Until:     until,
	}

	// 2a. --due-date 解決
	if c.DueDate != "" {
		dueSince, dueUntil, err := resolveDueDate(c.DueDate)
		if err != nil {
			return fmt.Errorf("failed to resolve due date: %w", err)
		}
		scope.DueDateSince = dueSince
		scope.DueDateUntil = dueUntil
	}

	// 2b. --start-date 解決
	if c.StartDate != "" {
		startSince, startUntil, err := resolveStartDate(c.StartDate)
		if err != nil {
			return fmt.Errorf("failed to resolve start date: %w", err)
		}
		scope.StartDateSince = startSince
		scope.StartDateUntil = startUntil
	}

	// 2. Project → ProjectIDs 解決
	for _, key := range c.Project {
		proj, err := rc.Client.GetProject(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to resolve project key %q: %w", key, err)
		}
		scope.ProjectKeys = append(scope.ProjectKeys, proj.ProjectKey)
		scope.ProjectIDs = append(scope.ProjectIDs, proj.ID)
	}

	// 3. User → UserIDs 解決
	for _, u := range c.User {
		ids, err := resolveAssignee(ctx, u, rc.Client)
		if err != nil {
			return fmt.Errorf("failed to resolve user %q: %w", u, err)
		}
		scope.UserIDs = append(scope.UserIDs, ids...)
	}
	if len(scope.UserIDs) > 0 {
		scope.UserIDs = uniqueInts(scope.UserIDs)
	}

	// 4. Team → TeamIDs 解決（名前 or 数値ID → int ID）
	teamIDs, err := resolveTeamIDs(ctx, c.Team, rc.Client)
	if err != nil {
		return fmt.Errorf("failed to resolve teams: %w", err)
	}
	scope.TeamIDs = teamIDs

	// 5. Team のメンバーを UserIDs にも追加（issue list の assigneeId に使用するため）
	for _, teamID := range teamIDs {
		team, err := rc.Client.GetTeam(ctx, teamID)
		if err != nil {
			return fmt.Errorf("failed to get team (ID=%d): %w", teamID, err)
		}
		for _, m := range team.Members {
			scope.UserIDs = append(scope.UserIDs, m.ID)
		}
	}
	if len(scope.UserIDs) > 0 {
		scope.UserIDs = uniqueInts(scope.UserIDs)
	}

	// 6. Build
	b := digest.NewUnifiedDigestBuilder(rc.Client, rc.Config.Profile, rc.Config.Space, rc.Config.BaseURL)
	envelope, err := b.Build(ctx, scope)
	if err != nil {
		return err
	}

	return rc.Renderer.Render(os.Stdout, envelope)
}

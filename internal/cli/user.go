package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/render"
)

// UserCmd は user コマンド群のルート。
type UserCmd struct {
	Me       UserMeCmd       `cmd:"" help:"display current authenticated user"`
	List     UserListCmd     `cmd:"" help:"list users"`
	Get      UserGetCmd      `cmd:"" help:"get user"`
	Activity UserActivityCmd `cmd:"" help:"get user activities"`
	Workload UserWorkloadCmd `cmd:"" help:"calculate user workload for a project"`
}

// UserMeCmd は user me コマンド。認証済みユーザー自身の情報を表示する。
type UserMeCmd struct{}

// Run は Kong から呼び出されるエントリポイント。
func (c *UserMeCmd) Run(g *GlobalFlags) error {
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	return c.run(context.Background(), rc.Client, rc.Renderer, os.Stdout)
}

// run はテスト可能な内部実装。GetMyself を呼んでレンダリングする。
func (c *UserMeCmd) run(ctx context.Context, client backlog.Client, renderer render.Renderer, out io.Writer) error {
	user, err := client.GetMyself(ctx)
	if err != nil {
		return fmt.Errorf("user me: %w", err)
	}
	return renderer.Render(out, user)
}

// UserListCmd は user list コマンド（spec §14.14）。
type UserListCmd struct {
	ListFlags
}

func (c *UserListCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	users, err := rc.Client.ListUsers(ctx)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, users)
}

// UserGetCmd は user get コマンド（spec §14.15）。
// UserID は数値 ID または userKey（文字列）を受け付ける。
type UserGetCmd struct {
	// UserID はユーザーID（数値）またはユーザーキー（文字列）。
	UserID string `arg:"" required:"" help:"user ID (numeric) or user key (string)"`
}

func (c *UserGetCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	user, err := rc.Client.GetUser(ctx, c.UserID)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, user)
}

// UserActivityCmd は user activity コマンド（spec §14.16）。
// Since/Until/Limit は DigestFlags から継承する。
type UserActivityCmd struct {
	DigestFlags
	// UserID はユーザーID（数値）またはユーザーキー（文字列）。
	UserID string `arg:"" required:"" help:"user ID (numeric) or user key (string)"`
	// Project はプロジェクトキーでフィルタ。
	Project string `help:"filter by project key" env:"LOGVALET_PROJECT"`
	// ActivityType はアクティビティタイプでフィルタ（オプション拡張）。
	ActivityType string `name:"type" help:"filter by activity type (e.g., issue_created, issue_commented)"`
}

func (c *UserActivityCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	opt := backlog.ListUserActivitiesOptions{
		Count: c.Limit,
	}
	activities, err := rc.Client.ListUserActivities(ctx, c.UserID, opt)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, activities)
}


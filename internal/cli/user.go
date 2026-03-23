package cli

import (
	"context"
	"os"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
)

// UserCmd は user コマンド群のルート。
type UserCmd struct {
	List     UserListCmd     `cmd:"" help:"ユーザー一覧を取得する"`
	Get      UserGetCmd      `cmd:"" help:"ユーザーを取得する"`
	Activity UserActivityCmd `cmd:"" help:"ユーザーのアクティビティを取得する"`
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
	UserID string `arg:"" required:"" help:"ユーザーID（数値）またはユーザーキー（文字列）"`
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
	UserID string `arg:"" required:"" help:"ユーザーID（数値）またはユーザーキー（文字列）"`
	// Project はプロジェクトキーでフィルタ。
	Project string `help:"プロジェクトキーでフィルタ" env:"LOGVALET_PROJECT"`
	// ActivityType はアクティビティタイプでフィルタ（オプション拡張）。
	ActivityType string `name:"type" help:"アクティビティタイプでフィルタ（例: issue_created, issue_commented）"`
}

func (c *UserActivityCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	opt := backlog.ListUserActivitiesOptions{
		Limit: c.Limit,
	}
	if c.Since != "" {
		t, parseErr := time.Parse(time.RFC3339, c.Since)
		if parseErr == nil {
			opt.Since = &t
		}
	}
	if c.Until != "" {
		t, parseErr := time.Parse(time.RFC3339, c.Until)
		if parseErr == nil {
			opt.Until = &t
		}
	}
	activities, err := rc.Client.ListUserActivities(ctx, c.UserID, opt)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, activities)
}


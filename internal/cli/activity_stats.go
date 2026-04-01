package cli

import (
	"context"
	"os"
	"time"

	"github.com/youyo/logvalet/internal/analysis"
)

// ActivityStatsCmd は activity stats コマンド。
// アクティビティの統計（タイプ別・アクター別・時間帯別・パターン）を集計する。
type ActivityStatsCmd struct {
	// Scope はアクティビティスコープ（project/user/space）。
	Scope string `help:"activity scope (project/user/space)" default:"space" enum:"project,user,space"`
	// ProjectKey はプロジェクトキー（scope=project 時に使用）。
	ProjectKey string `help:"project key (required when scope=project)" short:"k"`
	// UserID はユーザーID（scope=user 時に使用）。
	UserID string `help:"user ID (required when scope=user)"`
	// Since は取得開始日時（ISO 8601 形式）。
	Since string `help:"start date/time (ISO 8601)"`
	// Until は取得終了日時（ISO 8601 形式）。
	Until string `help:"end date/time (ISO 8601)"`
	// TopN は top_active_actors/types の表示件数（デフォルト: 5）。
	TopN int `help:"number of top actors/types to include" default:"5" name:"top-n"`
}

// Run は activity stats コマンドを実行する。
func (c *ActivityStatsCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	// scope に応じた ScopeKey を決定
	scopeKey := c.ProjectKey
	if c.Scope == "user" {
		scopeKey = c.UserID
	}

	opt := analysis.ActivityStatsOptions{
		Scope:    c.Scope,
		ScopeKey: scopeKey,
		TopN:     c.TopN,
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

	builder := analysis.NewActivityStatsBuilder(
		rc.Client,
		rc.Config.Profile,
		rc.Config.Space,
		rc.Config.BaseURL,
	)

	envelope, err := builder.Build(ctx, opt)
	if err != nil {
		return err
	}

	return rc.Renderer.Render(os.Stdout, envelope)
}

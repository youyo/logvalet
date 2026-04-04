package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/youyo/logvalet/internal/backlog"
)

// WatchingCmd はウォッチコマンド群のルート。
type WatchingCmd struct {
	List       WatchingListCmd       `cmd:"" help:"list watchings for a user"`
	Count      WatchingCountCmd      `cmd:"" help:"count watchings for a user"`
	Get        WatchingGetCmd        `cmd:"" help:"get watching detail"`
	Add        WatchingAddCmd        `cmd:"" help:"add watching for an issue"`
	Update     WatchingUpdateCmd     `cmd:"" help:"update watching note"`
	Delete     WatchingDeleteCmd     `cmd:"" help:"delete watching"`
	MarkAsRead WatchingMarkAsReadCmd `cmd:"" name:"mark-as-read" help:"mark watching as read"`
}

// WatchingListCmd は watching list コマンド。
// lv watching list USER-ID [options]
// USER-ID は数値 ID または "me"（認証ユーザーに自動解決）。
type WatchingListCmd struct {
	UserID string `arg:"" required:"" help:"user ID or 'me' for authenticated user"`
	Count  int    `help:"max number of items" default:"20"`
	Offset int    `help:"offset" default:"0"`
	Order  string `help:"sort order (asc|desc)" default:"desc"`
	Sort   string `help:"sort key (created|updated|issueUpdated)" default:""`
}

func (c *WatchingListCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	userID, err := resolveUserID(ctx, c.UserID, rc.Client)
	if err != nil {
		return err
	}
	opt := backlog.ListWatchingsOptions{
		Count:  c.Count,
		Offset: c.Offset,
		Order:  c.Order,
		Sort:   c.Sort,
	}
	watchings, err := rc.Client.ListWatchings(ctx, userID, opt)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, watchings)
}

// WatchingCountCmd は watching count コマンド。
// lv watching count USER-ID
// USER-ID は数値 ID または "me"（認証ユーザーに自動解決）。
type WatchingCountCmd struct {
	UserID string `arg:"" required:"" help:"user ID or 'me' for authenticated user"`
}

func (c *WatchingCountCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	userID, err := resolveUserID(ctx, c.UserID, rc.Client)
	if err != nil {
		return err
	}
	count, err := rc.Client.CountWatchings(ctx, userID, backlog.ListWatchingsOptions{})
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, map[string]int{"count": count})
}

// resolveUserID は "me" を GetMyself で解決し、数値文字列はそのまま int に変換する。
func resolveUserID(ctx context.Context, input string, client backlog.Client) (int, error) {
	if input == "me" {
		user, err := client.GetMyself(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to resolve 'me': %w", err)
		}
		return user.ID, nil
	}
	id, err := strconv.Atoi(input)
	if err != nil {
		return 0, fmt.Errorf("user-id must be a numeric ID or 'me': %q", input)
	}
	return id, nil
}

// WatchingGetCmd は watching get コマンド。
// lv watching get WATCHING-ID
type WatchingGetCmd struct {
	WatchingID int64 `arg:"" required:"" help:"watching ID"`
}

func (c *WatchingGetCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	watching, err := rc.Client.GetWatching(ctx, c.WatchingID)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, watching)
}

// WatchingAddCmd は watching add コマンド。
// lv watching add ISSUE-ID-OR-KEY [--note NOTE] [--dry-run]
type WatchingAddCmd struct {
	WriteFlags
	IssueIDOrKey string `arg:"" required:"" help:"issue ID or key (e.g., PROJ-123)"`
	Note         string `help:"note for watching" default:""`
}

func (c *WatchingAddCmd) Run(g *GlobalFlags) error {
	if c.DryRun {
		params := map[string]interface{}{
			"issue_id_or_key": c.IssueIDOrKey,
			"note":            c.Note,
		}
		data, err := formatDryRun("add_watching", params)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	}

	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	req := backlog.AddWatchingRequest{
		IssueIDOrKey: c.IssueIDOrKey,
		Note:         c.Note,
	}
	watching, err := rc.Client.AddWatching(ctx, req)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, watching)
}

// WatchingUpdateCmd は watching update コマンド。
// lv watching update WATCHING-ID --note NOTE [--dry-run]
type WatchingUpdateCmd struct {
	WriteFlags
	WatchingID int64  `arg:"" required:"" help:"watching ID"`
	Note       string `required:"" help:"new note for watching"`
}

func (c *WatchingUpdateCmd) Run(g *GlobalFlags) error {
	if c.DryRun {
		params := map[string]interface{}{
			"watching_id": c.WatchingID,
			"note":        c.Note,
		}
		data, err := formatDryRun("update_watching", params)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	}

	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	req := backlog.UpdateWatchingRequest{
		Note: c.Note,
	}
	watching, err := rc.Client.UpdateWatching(ctx, c.WatchingID, req)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, watching)
}

// WatchingDeleteCmd は watching delete コマンド。
// lv watching delete WATCHING-ID [--dry-run]
type WatchingDeleteCmd struct {
	WriteFlags
	WatchingID int64 `arg:"" required:"" help:"watching ID"`
}

func (c *WatchingDeleteCmd) Run(g *GlobalFlags) error {
	if c.DryRun {
		params := map[string]interface{}{
			"watching_id": c.WatchingID,
		}
		data, err := formatDryRun("delete_watching", params)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	}

	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	watching, err := rc.Client.DeleteWatching(ctx, c.WatchingID)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, watching)
}

// WatchingMarkAsReadCmd は watching mark-as-read コマンド。
// lv watching mark-as-read WATCHING-ID [--dry-run]
type WatchingMarkAsReadCmd struct {
	WriteFlags
	WatchingID int64 `arg:"" required:"" help:"watching ID"`
}

func (c *WatchingMarkAsReadCmd) Run(g *GlobalFlags) error {
	if c.DryRun {
		params := map[string]interface{}{
			"watching_id": c.WatchingID,
		}
		data, err := formatDryRun("mark_watching_as_read", params)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	}

	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	if err := rc.Client.MarkWatchingAsRead(ctx, c.WatchingID); err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, map[string]string{"result": "ok"})
}

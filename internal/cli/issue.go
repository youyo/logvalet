package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/digest"
)

// IssueCmd は issue コマンド群のルート。
type IssueCmd struct {
	Get     IssueGetCmd     `cmd:"" help:"課題を取得する"`
	List    IssueListCmd    `cmd:"" help:"課題一覧を取得する"`
	Digest  IssueDigestCmd  `cmd:"" help:"課題のダイジェストを生成する"`
	Create  IssueCreateCmd  `cmd:"" help:"課題を作成する"`
	Update  IssueUpdateCmd  `cmd:"" help:"課題を更新する"`
	Comment IssueCommentCmd `cmd:"" help:"コメントを操作する"`
}

// IssueGetCmd は issue get コマンド。
type IssueGetCmd struct {
	IssueIDOrKey string `arg:"" required:"" help:"課題ID または 課題キー (例: PROJ-123)"`
}

func (c *IssueGetCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	issue, err := rc.Client.GetIssue(ctx, c.IssueIDOrKey)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, issue)
}

// IssueListCmd は issue list コマンド。
type IssueListCmd struct {
	ListFlags
	ProjectKey []string `short:"k" help:"プロジェクトキー"`
}

func (c *IssueListCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	opt := backlog.ListIssuesOptions{
		Limit:  c.Count,
		Offset: c.Offset,
	}
	if len(c.ProjectKey) > 0 {
		opt.ProjectKey = c.ProjectKey[0]
	}
	issues, err := rc.Client.ListIssues(ctx, opt)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, issues)
}

// IssueDigestCmd は issue digest コマンド（spec §14.3）。
type IssueDigestCmd struct {
	IssueKey   string `arg:"" required:"" help:"課題キー (例: PROJ-123)"`
	Comments   int    `short:"c" help:"取得するコメント数" default:"5" env:"LOGVALET_COMMENTS"`
	NoActivity bool   `help:"アクティビティを含めない" env:"LOGVALET_NO_ACTIVITY"`
}

func (c *IssueDigestCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	builder := digest.NewDefaultIssueDigestBuilder(rc.Client, rc.Config.Profile, rc.Config.Space, rc.Config.BaseURL)
	envelope, err := builder.Build(ctx, c.IssueKey, digest.IssueDigestOptions{
		MaxComments:     c.Comments,
		IncludeActivity: !c.NoActivity,
	})
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, envelope)
}

// IssueCreateCmd は issue create コマンド（spec §14.4）。
type IssueCreateCmd struct {
	WriteFlags
	ProjectKey      string   `required:"" help:"プロジェクトキー"`
	Summary         string   `required:"" help:"課題のサマリー"`
	IssueType       string   `required:"" help:"課題種別 (名前またはID)"`
	Description     string   `help:"課題の説明"`
	DescriptionFile string   `help:"課題の説明をファイルから読み込む"`
	Priority        string   `help:"優先度"`
	Assignee        string   `help:"担当者"`
	Category        []string `help:"カテゴリ（複数指定可）"`
	Version         []string `help:"バージョン（複数指定可）"`
	Milestone       []string `help:"マイルストーン（複数指定可）"`
	DueDate         string   `help:"期限日 (YYYY-MM-DD)"`
	StartDate       string   `help:"開始日 (YYYY-MM-DD)"`
}

// Run は issue create コマンドの実行（spec §14.4）。
// バリデーション → dry-run プレビュー → API 呼び出し（未実装）
func (c *IssueCreateCmd) Run(g *GlobalFlags) error {
	// --description と --description-file は排他
	if err := validateDescriptionFlags(c.Description, c.DescriptionFile); err != nil {
		return err
	}

	// description-file からの読み込み
	description, err := resolveContent(c.Description, c.DescriptionFile)
	if err != nil {
		return err
	}

	if c.DryRun {
		params := map[string]interface{}{
			"project_key": c.ProjectKey,
			"summary":     c.Summary,
			"issue_type":  c.IssueType,
			"description": nilIfEmpty(description),
			"priority":    nilIfEmpty(c.Priority),
			"assignee":    nilIfEmpty(c.Assignee),
			"category":    c.Category,
			"version":     c.Version,
			"milestone":   c.Milestone,
			"due_date":    nilIfEmpty(c.DueDate),
			"start_date":  nilIfEmpty(c.StartDate),
		}
		data, err := formatDryRun("create_issue", params)
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
	issue, err := rc.Client.CreateIssue(ctx, backlog.CreateIssueRequest{
		ProjectKey:  c.ProjectKey,
		Summary:     c.Summary,
		IssueType:   c.IssueType,
		Description: description,
		Priority:    c.Priority,
		Assignee:    c.Assignee,
	})
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, issue)
}

// IssueUpdateCmd は issue update コマンド（spec §14.5）。
type IssueUpdateCmd struct {
	WriteFlags
	IssueIDOrKey    string   `arg:"" required:"" help:"課題ID または 課題キー"`
	Summary         *string  `help:"課題のサマリー"`
	Description     *string  `help:"課題の説明"`
	DescriptionFile string   `help:"課題の説明をファイルから読み込む"`
	Status          *string  `help:"ステータス"`
	Priority        *string  `help:"優先度"`
	Assignee        *string  `help:"担当者"`
	Category        []string `help:"カテゴリ（複数指定可）"`
	Version         []string `help:"バージョン（複数指定可）"`
	Milestone       []string `help:"マイルストーン（複数指定可）"`
	DueDate         *string  `help:"期限日 (YYYY-MM-DD)"`
	StartDate       *string  `help:"開始日 (YYYY-MM-DD)"`
}

// Run は issue update コマンドの実行（spec §14.5）。
// 少なくとも1つの更新フィールドが必要。
func (c *IssueUpdateCmd) Run(g *GlobalFlags) error {
	// --description と --description-file は排他
	var descStr *string
	if c.DescriptionFile != "" {
		descStr = &c.DescriptionFile // 排他チェック用に non-nil とみなす
	}
	descText := ""
	if c.Description != nil {
		descText = *c.Description
	}
	if err := validateDescriptionFlags(descText, c.DescriptionFile); err != nil {
		return err
	}

	// 少なくとも1つのフィールドが必要
	if err := validateAtLeastOneUpdateFlag(
		c.Summary, c.Description, c.Status, c.Priority, c.Assignee,
		c.DueDate, c.StartDate, descStr,
		c.Category, c.Version, c.Milestone,
	); err != nil {
		return err
	}

	// description-file からの読み込み
	var resolvedDescription *string
	if c.DescriptionFile != "" {
		content, err := resolveContent("", c.DescriptionFile)
		if err != nil {
			return err
		}
		resolvedDescription = &content
	} else {
		resolvedDescription = c.Description
	}

	if c.DryRun {
		params := map[string]interface{}{
			"issue_key":   c.IssueIDOrKey,
			"summary":     ptrOrNil(c.Summary),
			"description": ptrOrNil(resolvedDescription),
			"status":      ptrOrNil(c.Status),
			"priority":    ptrOrNil(c.Priority),
			"assignee":    ptrOrNil(c.Assignee),
			"due_date":    ptrOrNil(c.DueDate),
			"start_date":  ptrOrNil(c.StartDate),
		}
		data, err := formatDryRun("update_issue", params)
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
	issue, err := rc.Client.UpdateIssue(ctx, c.IssueIDOrKey, backlog.UpdateIssueRequest{
		Summary:     c.Summary,
		Description: resolvedDescription,
		Status:      c.Status,
	})
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, issue)
}

// IssueCommentCmd は issue comment コマンド群。
type IssueCommentCmd struct {
	List   IssueCommentListCmd   `cmd:"" help:"コメント一覧を取得する"`
	Add    IssueCommentAddCmd    `cmd:"" help:"コメントを追加する"`
	Update IssueCommentUpdateCmd `cmd:"" help:"コメントを更新する"`
}

// IssueCommentListCmd は issue comment list コマンド（spec §14.6）。
type IssueCommentListCmd struct {
	ListFlags
	IssueIDOrKey string `arg:"" required:"" help:"課題ID または 課題キー"`
}

func (c *IssueCommentListCmd) Run(g *GlobalFlags) error {
	ctx := context.Background()
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}
	opt := backlog.ListCommentsOptions{
		Limit:  c.Count,
		Offset: c.Offset,
	}
	comments, err := rc.Client.ListIssueComments(ctx, c.IssueIDOrKey, opt)
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, comments)
}

// IssueCommentAddCmd は issue comment add コマンド（spec §14.7）。
type IssueCommentAddCmd struct {
	WriteFlags
	IssueIDOrKey string `arg:"" required:"" help:"課題ID または 課題キー"`
	Content      string `help:"コメント本文（--content-file と排他）"`
	ContentFile  string `help:"コメント本文をファイルから読み込む（--content と排他）"`
}

// Run は issue comment add コマンドの実行（spec §14.7）。
func (c *IssueCommentAddCmd) Run(g *GlobalFlags) error {
	// --content と --content-file は排他、かつどちらか必須
	if err := validateContentFlags(c.Content, c.ContentFile); err != nil {
		return err
	}

	// content-file からの読み込み
	content, err := resolveContent(c.Content, c.ContentFile)
	if err != nil {
		return err
	}

	if c.DryRun {
		params := map[string]interface{}{
			"issue_key": c.IssueIDOrKey,
			"content":   content,
		}
		data, err := formatDryRun("add_comment", params)
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
	comment, err := rc.Client.AddIssueComment(ctx, c.IssueIDOrKey, backlog.AddCommentRequest{
		Content: content,
	})
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, comment)
}

// IssueCommentUpdateCmd は issue comment update コマンド（spec §14.8）。
type IssueCommentUpdateCmd struct {
	WriteFlags
	IssueIDOrKey string `arg:"" required:"" help:"課題ID または 課題キー"`
	CommentID    int    `arg:"" required:"" help:"コメントID"`
	Content      string `help:"コメント本文（--content-file と排他）"`
	ContentFile  string `help:"コメント本文をファイルから読み込む（--content と排他）"`
}

// Run は issue comment update コマンドの実行（spec §14.8）。
func (c *IssueCommentUpdateCmd) Run(g *GlobalFlags) error {
	// --content と --content-file は排他、かつどちらか必須
	if err := validateContentFlags(c.Content, c.ContentFile); err != nil {
		return err
	}

	// content-file からの読み込み
	content, err := resolveContent(c.Content, c.ContentFile)
	if err != nil {
		return err
	}

	if c.DryRun {
		params := map[string]interface{}{
			"issue_key":  c.IssueIDOrKey,
			"comment_id": c.CommentID,
			"content":    content,
		}
		data, err := formatDryRun("update_comment", params)
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
	comment, err := rc.Client.UpdateIssueComment(ctx, c.IssueIDOrKey, int64(c.CommentID), backlog.UpdateCommentRequest{
		Content: content,
	})
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, comment)
}

// ---- ヘルパー ----

// nilIfEmpty は空文字列の場合に nil を返す。dry-run 出力用。
func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ptrOrNil はポインタが nil の場合に nil を返す。dry-run 出力用。
func ptrOrNil(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}

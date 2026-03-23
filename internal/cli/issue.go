package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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
	Assignee   string   `help:"担当者 (me, 数値ID, またはユーザー名)"`
	Status     string   `help:"ステータス (open, 名前, カンマ区切り, 数値ID)。名前/open は --project-key 必須"`
	DueDate    string   `help:"期限日フィルタ (today, overdue, YYYY-MM-DD)"`
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
	// projectKey → projectId 変換
	for _, key := range c.ProjectKey {
		proj, err := rc.Client.GetProject(ctx, key)
		if err != nil {
			return fmt.Errorf("プロジェクトキー %q の解決に失敗: %w", key, err)
		}
		opt.ProjectIDs = append(opt.ProjectIDs, proj.ID)
	}
	// --assignee 解決
	if c.Assignee != "" {
		assigneeIDs, err := resolveAssignee(ctx, c.Assignee, rc.Client)
		if err != nil {
			return fmt.Errorf("担当者の解決に失敗: %w", err)
		}
		opt.AssigneeIDs = assigneeIDs
	}
	// --status 解決
	if c.Status != "" {
		statusIDs, err := resolveStatuses(ctx, c.Status, c.ProjectKey, rc.Client)
		if err != nil {
			return fmt.Errorf("ステータスの解決に失敗: %w", err)
		}
		opt.StatusIDs = statusIDs
	}
	// --due-date 解決
	if c.DueDate != "" {
		since, until, err := resolveDueDate(c.DueDate)
		if err != nil {
			return fmt.Errorf("期限日の解決に失敗: %w", err)
		}
		opt.DueDateSince = since
		opt.DueDateUntil = until
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
	Comments   int    `help:"取得するコメント数" default:"5" env:"LOGVALET_COMMENTS"`
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
	IssueType       string   `help:"課題種別 (名前またはID)。未指定時はプロジェクトのデフォルト種別"`
	Description     string   `help:"課題の説明"`
	DescriptionFile string   `help:"課題の説明をファイルから読み込む"`
	Priority        string   `help:"優先度 (名前またはID)。未指定時はデフォルト優先度"`
	Assignee        string   `help:"担当者のユーザーID"`
	Category        []string `help:"カテゴリ (名前またはID、複数指定可)"`
	Version         []string `name:"versions" help:"バージョン (名前またはID、複数指定可)"`
	Milestone       []string `help:"マイルストーン (名前またはID、複数指定可)"`
	DueDate         string   `help:"期限日 (YYYY-MM-DD)"`
	StartDate       string   `help:"開始日 (YYYY-MM-DD)"`
	ParentIssueID   int      `help:"親課題のID"`
	NotifiedUserID  []int    `help:"通知先ユーザーID (複数指定可)"`
}

// Run は issue create コマンドの実行（spec §14.4）。
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
			"project_key":      c.ProjectKey,
			"summary":          c.Summary,
			"issue_type":       nilIfEmpty(c.IssueType),
			"description":      nilIfEmpty(description),
			"priority":         nilIfEmpty(c.Priority),
			"assignee":         nilIfEmpty(c.Assignee),
			"category":         c.Category,
			"version":          c.Version,
			"milestone":        c.Milestone,
			"due_date":         nilIfEmpty(c.DueDate),
			"start_date":       nilIfEmpty(c.StartDate),
			"parent_issue_id":  c.ParentIssueID,
			"notified_user_id": c.NotifiedUserID,
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

	// 1. projectKey → projectId
	proj, err := rc.Client.GetProject(ctx, c.ProjectKey)
	if err != nil {
		return fmt.Errorf("プロジェクトキー %q の解決に失敗: %w", c.ProjectKey, err)
	}

	// 2. issueType 解決（未指定時はデフォルト = 先頭要素）
	issueTypes, err := rc.Client.ListProjectIssueTypes(ctx, c.ProjectKey)
	if err != nil {
		return fmt.Errorf("課題種別の取得に失敗: %w", err)
	}
	var issueTypeID int
	if c.IssueType == "" {
		if len(issueTypes) == 0 {
			return fmt.Errorf("プロジェクト %q に課題種別が存在しません", c.ProjectKey)
		}
		issueTypeID = issueTypes[0].ID
	} else {
		issueTypeID, err = resolveNameOrID(c.IssueType, issueTypes)
		if err != nil {
			return fmt.Errorf("課題種別の解決に失敗: %w", err)
		}
	}

	// 3. priority 解決（未指定時はデフォルト = 「中」）
	priorities, err := rc.Client.ListPriorities(ctx)
	if err != nil {
		return fmt.Errorf("優先度の取得に失敗: %w", err)
	}
	var priorityID int
	if c.Priority == "" {
		if len(priorities) == 0 {
			return fmt.Errorf("優先度の一覧が空です")
		}
		// 「中」(Normal) を名前で検索、なければ先頭要素にフォールバック
		priorityID = priorities[0].ID
		for _, p := range priorities {
			if strings.EqualFold(p.Name, "中") || strings.EqualFold(p.Name, "Normal") {
				priorityID = p.ID
				break
			}
		}
	} else {
		priorityID, err = resolveNameOrID(c.Priority, priorities)
		if err != nil {
			return fmt.Errorf("優先度の解決に失敗: %w", err)
		}
	}

	// 4. assignee（数値直接入力）
	var assigneeID int
	if c.Assignee != "" {
		assigneeID, err = strconv.Atoi(c.Assignee)
		if err != nil {
			return fmt.Errorf("担当者IDは数値で指定してください: %q", c.Assignee)
		}
	}

	// 5. categories 名前→ID
	var categoryIDs []int
	if len(c.Category) > 0 {
		cats, err := rc.Client.ListProjectCategories(ctx, c.ProjectKey)
		if err != nil {
			return fmt.Errorf("カテゴリの取得に失敗: %w", err)
		}
		categoryIDs, err = resolveNamesOrIDs(c.Category, toIDNamesFromCategories(cats))
		if err != nil {
			return fmt.Errorf("カテゴリの解決に失敗: %w", err)
		}
	}

	// 6. versions / milestones — 同じ API (ListProjectVersions)
	var versionIDs []int
	var milestoneIDs []int
	if len(c.Version) > 0 || len(c.Milestone) > 0 {
		vers, err := rc.Client.ListProjectVersions(ctx, c.ProjectKey)
		if err != nil {
			return fmt.Errorf("バージョンの取得に失敗: %w", err)
		}
		verIDNames := toIDNamesFromVersions(vers)
		if len(c.Version) > 0 {
			versionIDs, err = resolveNamesOrIDs(c.Version, verIDNames)
			if err != nil {
				return fmt.Errorf("バージョンの解決に失敗: %w", err)
			}
		}
		if len(c.Milestone) > 0 {
			milestoneIDs, err = resolveNamesOrIDs(c.Milestone, verIDNames)
			if err != nil {
				return fmt.Errorf("マイルストーンの解決に失敗: %w", err)
			}
		}
	}

	// 7. dates → parseDate
	dueDate, err := parseDate(c.DueDate)
	if err != nil {
		return fmt.Errorf("期限日の解析に失敗: %w", err)
	}
	startDate, err := parseDate(c.StartDate)
	if err != nil {
		return fmt.Errorf("開始日の解析に失敗: %w", err)
	}

	// 8. API 呼び出し
	issue, err := rc.Client.CreateIssue(ctx, backlog.CreateIssueRequest{
		ProjectID:       proj.ID,
		Summary:         c.Summary,
		IssueTypeID:     issueTypeID,
		Description:     description,
		PriorityID:      priorityID,
		AssigneeID:      assigneeID,
		CategoryIDs:     categoryIDs,
		VersionIDs:      versionIDs,
		MilestoneIDs:    milestoneIDs,
		DueDate:         dueDate,
		StartDate:       startDate,
		ParentIssueID:   c.ParentIssueID,
		NotifiedUserIDs: c.NotifiedUserID,
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
	Status          *string  `help:"ステータス (名前またはID)"`
	Priority        *string  `help:"優先度 (名前またはID)"`
	Assignee        *string  `help:"担当者のユーザーID"`
	IssueType       *string  `help:"課題種別 (名前またはID)"`
	Category        []string `help:"カテゴリ（複数指定可）"`
	Version         []string `name:"versions" help:"バージョン（複数指定可）"`
	Milestone       []string `help:"マイルストーン（複数指定可）"`
	DueDate         *string  `help:"期限日 (YYYY-MM-DD)"`
	StartDate       *string  `help:"開始日 (YYYY-MM-DD)"`
	NotifiedUserID  []int    `help:"通知先ユーザーID (複数指定可)"`
	Comment         *string  `help:"更新時のコメント"`
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

	// 少なくとも1つのフィールドが必要（IssueType, Comment, NotifiedUserID も含む）
	hasNotifiedUserID := len(c.NotifiedUserID) > 0
	var notifiedUserIDSlice []string
	if hasNotifiedUserID {
		notifiedUserIDSlice = []string{"_"} // 非空スライスとして扱う
	}
	if err := validateAtLeastOneUpdateFlag(
		c.Summary, c.Description, c.Status, c.Priority, c.Assignee,
		c.DueDate, c.StartDate, descStr,
		c.Category, c.Version, c.Milestone, notifiedUserIDSlice,
	); err != nil {
		// IssueType や Comment もチェック
		if c.IssueType == nil && c.Comment == nil {
			return err
		}
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
			"issue_key":        c.IssueIDOrKey,
			"summary":          ptrOrNil(c.Summary),
			"description":      ptrOrNil(resolvedDescription),
			"status":           ptrOrNil(c.Status),
			"priority":         ptrOrNil(c.Priority),
			"assignee":         ptrOrNil(c.Assignee),
			"issue_type":       ptrOrNil(c.IssueType),
			"due_date":         ptrOrNil(c.DueDate),
			"start_date":       ptrOrNil(c.StartDate),
			"notified_user_id": c.NotifiedUserID,
			"comment":          ptrOrNil(c.Comment),
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

	// issueKey からプロジェクトキーを抽出
	projectKey := extractProjectKey(c.IssueIDOrKey)

	// Status の名前→ID 変換
	var statusID *int
	if c.Status != nil {
		statuses, err := rc.Client.ListProjectStatuses(ctx, projectKey)
		if err != nil {
			return fmt.Errorf("ステータスの取得に失敗: %w", err)
		}
		id, err := resolveNameOrID(*c.Status, toIDNamesFromStatuses(statuses))
		if err != nil {
			return fmt.Errorf("ステータスの解決に失敗: %w", err)
		}
		statusID = &id
	}

	// Priority の名前→ID 変換
	var priorityID *int
	if c.Priority != nil {
		priorities, err := rc.Client.ListPriorities(ctx)
		if err != nil {
			return fmt.Errorf("優先度の取得に失敗: %w", err)
		}
		id, err := resolveNameOrID(*c.Priority, priorities)
		if err != nil {
			return fmt.Errorf("優先度の解決に失敗: %w", err)
		}
		priorityID = &id
	}

	// IssueType の名前→ID 変換
	var issueTypeID *int
	if c.IssueType != nil {
		issueTypes, err := rc.Client.ListProjectIssueTypes(ctx, projectKey)
		if err != nil {
			return fmt.Errorf("課題種別の取得に失敗: %w", err)
		}
		id, err := resolveNameOrID(*c.IssueType, issueTypes)
		if err != nil {
			return fmt.Errorf("課題種別の解決に失敗: %w", err)
		}
		issueTypeID = &id
	}

	// Assignee（数値直接入力）
	var assigneeID *int
	if c.Assignee != nil {
		id, err := strconv.Atoi(*c.Assignee)
		if err != nil {
			return fmt.Errorf("担当者IDは数値で指定してください: %q", *c.Assignee)
		}
		assigneeID = &id
	}

	// Categories の名前→ID 変換
	var categoryIDs []int
	if len(c.Category) > 0 {
		cats, err := rc.Client.ListProjectCategories(ctx, projectKey)
		if err != nil {
			return fmt.Errorf("カテゴリの取得に失敗: %w", err)
		}
		categoryIDs, err = resolveNamesOrIDs(c.Category, toIDNamesFromCategories(cats))
		if err != nil {
			return fmt.Errorf("カテゴリの解決に失敗: %w", err)
		}
	}

	// Versions / Milestones の名前→ID 変換
	var versionIDs []int
	var milestoneIDs []int
	if len(c.Version) > 0 || len(c.Milestone) > 0 {
		vers, err := rc.Client.ListProjectVersions(ctx, projectKey)
		if err != nil {
			return fmt.Errorf("バージョンの取得に失敗: %w", err)
		}
		verIDNames := toIDNamesFromVersions(vers)
		if len(c.Version) > 0 {
			versionIDs, err = resolveNamesOrIDs(c.Version, verIDNames)
			if err != nil {
				return fmt.Errorf("バージョンの解決に失敗: %w", err)
			}
		}
		if len(c.Milestone) > 0 {
			milestoneIDs, err = resolveNamesOrIDs(c.Milestone, verIDNames)
			if err != nil {
				return fmt.Errorf("マイルストーンの解決に失敗: %w", err)
			}
		}
	}

	// DueDate / StartDate の parseDate
	var dueDate *time.Time
	if c.DueDate != nil {
		dueDate, err = parseDate(*c.DueDate)
		if err != nil {
			return fmt.Errorf("期限日の解析に失敗: %w", err)
		}
	}
	var startDate *time.Time
	if c.StartDate != nil {
		startDate, err = parseDate(*c.StartDate)
		if err != nil {
			return fmt.Errorf("開始日の解析に失敗: %w", err)
		}
	}

	issue, err := rc.Client.UpdateIssue(ctx, c.IssueIDOrKey, backlog.UpdateIssueRequest{
		Summary:         c.Summary,
		Description:     resolvedDescription,
		StatusID:        statusID,
		PriorityID:      priorityID,
		AssigneeID:      assigneeID,
		IssueTypeID:     issueTypeID,
		CategoryIDs:     categoryIDs,
		VersionIDs:      versionIDs,
		MilestoneIDs:    milestoneIDs,
		DueDate:         dueDate,
		StartDate:       startDate,
		NotifiedUserIDs: c.NotifiedUserID,
		Comment:         c.Comment,
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
	IssueIDOrKey   string `arg:"" required:"" help:"課題ID または 課題キー"`
	Content        string `help:"コメント本文（--content-file と排他）"`
	ContentFile    string `help:"コメント本文をファイルから読み込む（--content と排他）"`
	NotifiedUserID []int  `help:"通知先ユーザーID (複数指定可)"`
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
			"issue_key":        c.IssueIDOrKey,
			"content":          content,
			"notified_user_id": c.NotifiedUserID,
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
		Content:         content,
		NotifiedUserIDs: c.NotifiedUserID,
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


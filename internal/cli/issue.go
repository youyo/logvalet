package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// IssueCmd は issue コマンド群のルート。
type IssueCmd struct {
	Get        IssueGetCmd        `cmd:"" help:"get issue"`
	List       IssueListCmd       `cmd:"" help:"list issues"`
	Create     IssueCreateCmd     `cmd:"" help:"create issue"`
	Update     IssueUpdateCmd     `cmd:"" help:"update issue"`
	Comment    IssueCommentCmd    `cmd:"" help:"manage comments"`
	Attachment IssueAttachmentCmd `cmd:"" help:"manage attachments"`
}

// IssueGetCmd は issue get コマンド。
type IssueGetCmd struct {
	IssueIDOrKey string `arg:"" required:"" help:"issue ID or key (e.g., PROJ-123)"`
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
	ProjectKey []string `short:"k" help:"project key (required if --status is open/named)"`
	Assignee   string   `help:"assignee (me, numeric ID, or user name)"`
	Status     string   `help:"status (not-closed, open, name, comma-separated, numeric ID). open/named requires -k"`
	DueDate    string   `help:"due date filter (today, overdue, this-week, this-month, YYYY-MM-DD, YYYY-MM-DD:YYYY-MM-DD). auto-paginate if specified"`
	StartDate  string   `help:"start date filter (today, this-week, this-month, YYYY-MM-DD, YYYY-MM-DD:YYYY-MM-DD). auto-paginate if specified"`
	Sort       string   `help:"sort key (dueDate, created, updated, priority, status, assignee)"`
	Order      string   `help:"sort order (asc, desc)" default:"desc" enum:"asc,desc,"`
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
			return fmt.Errorf("failed to resolve project key %q: %w", key, err)
		}
		opt.ProjectIDs = append(opt.ProjectIDs, proj.ID)
	}
	// --assignee 解決
	if c.Assignee != "" {
		assigneeIDs, err := resolveAssignee(ctx, c.Assignee, rc.Client)
		if err != nil {
			return fmt.Errorf("failed to resolve assignee: %w", err)
		}
		opt.AssigneeIDs = assigneeIDs
	}
	// --status 解決
	if c.Status != "" {
		statusIDs, err := resolveStatuses(ctx, c.Status, c.ProjectKey, rc.Client)
		if err != nil {
			return fmt.Errorf("failed to resolve status: %w", err)
		}
		opt.StatusIDs = statusIDs
	}
	// --due-date 解決
	if c.DueDate != "" {
		since, until, err := resolveDueDate(c.DueDate)
		if err != nil {
			return fmt.Errorf("failed to resolve due date: %w", err)
		}
		opt.DueDateSince = since
		opt.DueDateUntil = until
	}
	// --start-date 解決
	if c.StartDate != "" {
		since, until, err := resolveStartDate(c.StartDate)
		if err != nil {
			return fmt.Errorf("failed to resolve start date: %w", err)
		}
		opt.StartDateSince = since
		opt.StartDateUntil = until
	}
	// sort/order 設定
	opt.Sort = c.Sort
	opt.Order = c.Order
	var issues []domain.Issue
	if c.DueDate != "" || c.StartDate != "" {
		issues, err = fetchAllIssues(ctx, rc.Client, opt)
	} else {
		issues, err = rc.Client.ListIssues(ctx, opt)
	}
	if err != nil {
		return err
	}
	return rc.Renderer.Render(os.Stdout, issues)
}

// IssueCreateCmd は issue create コマンド（spec §14.4）。
type IssueCreateCmd struct {
	WriteFlags
	ProjectKey      string   `required:"" help:"project key"`
	Summary         string   `required:"" help:"issue summary"`
	IssueType       string   `help:"issue type (name or ID). defaults to project's default type"`
	Description     string   `help:"issue description"`
	DescriptionFile string   `help:"read issue description from file"`
	Priority        string   `help:"priority (name or ID). defaults to normal priority"`
	Assignee        string   `help:"assignee user ID"`
	Category        []string `help:"category (name or ID, multiple allowed)"`
	Version         []string `name:"versions" help:"version (name or ID, multiple allowed)"`
	Milestone       []string `help:"milestone (name or ID, multiple allowed)"`
	DueDate         string   `help:"due date (YYYY-MM-DD)"`
	StartDate       string   `help:"start date (YYYY-MM-DD)"`
	ParentIssueID   int      `help:"parent issue ID"`
	NotifiedUserID  []int    `help:"notify user IDs (multiple allowed)"`
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
		return fmt.Errorf("failed to resolve project key %q: %w", c.ProjectKey, err)
	}

	// 2. issueType 解決（未指定時はデフォルト = 先頭要素）
	issueTypes, err := rc.Client.ListProjectIssueTypes(ctx, c.ProjectKey)
	if err != nil {
		return fmt.Errorf("failed to get issue types: %w", err)
	}
	var issueTypeID int
	if c.IssueType == "" {
		if len(issueTypes) == 0 {
			return fmt.Errorf("project %q has no issue types", c.ProjectKey)
		}
		issueTypeID = issueTypes[0].ID
	} else {
		issueTypeID, err = resolveNameOrID(c.IssueType, issueTypes)
		if err != nil {
			return fmt.Errorf("failed to resolve issue type: %w", err)
		}
	}

	// 3. priority 解決（未指定時はデフォルト = 「中」）
	priorities, err := rc.Client.ListPriorities(ctx)
	if err != nil {
		return fmt.Errorf("failed to get priorities: %w", err)
	}
	var priorityID int
	if c.Priority == "" {
		if len(priorities) == 0 {
			return fmt.Errorf("priorities list is empty")
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
			return fmt.Errorf("failed to resolve priority: %w", err)
		}
	}

	// 4. assignee（数値直接入力）
	var assigneeID int
	if c.Assignee != "" {
		assigneeID, err = strconv.Atoi(c.Assignee)
		if err != nil {
			return fmt.Errorf("assignee ID must be numeric: %q", c.Assignee)
		}
	}

	// 5. categories 名前→ID
	var categoryIDs []int
	if len(c.Category) > 0 {
		cats, err := rc.Client.ListProjectCategories(ctx, c.ProjectKey)
		if err != nil {
			return fmt.Errorf("failed to get categories: %w", err)
		}
		categoryIDs, err = resolveNamesOrIDs(c.Category, toIDNamesFromCategories(cats))
		if err != nil {
			return fmt.Errorf("failed to resolve categories: %w", err)
		}
	}

	// 6. versions / milestones — 同じ API (ListProjectVersions)
	var versionIDs []int
	var milestoneIDs []int
	if len(c.Version) > 0 || len(c.Milestone) > 0 {
		vers, err := rc.Client.ListProjectVersions(ctx, c.ProjectKey)
		if err != nil {
			return fmt.Errorf("failed to get versions: %w", err)
		}
		verIDNames := toIDNamesFromVersions(vers)
		if len(c.Version) > 0 {
			versionIDs, err = resolveNamesOrIDs(c.Version, verIDNames)
			if err != nil {
				return fmt.Errorf("failed to resolve versions: %w", err)
			}
		}
		if len(c.Milestone) > 0 {
			milestoneIDs, err = resolveNamesOrIDs(c.Milestone, verIDNames)
			if err != nil {
				return fmt.Errorf("failed to resolve milestones: %w", err)
			}
		}
	}

	// 7. dates → parseDate
	dueDate, err := parseDate(c.DueDate)
	if err != nil {
		return fmt.Errorf("failed to parse due date: %w", err)
	}
	startDate, err := parseDate(c.StartDate)
	if err != nil {
		return fmt.Errorf("failed to parse start date: %w", err)
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
	IssueIDOrKey    string   `arg:"" required:"" help:"issue ID or key"`
	Summary         *string  `help:"issue summary"`
	Description     *string  `help:"issue description"`
	DescriptionFile string   `help:"read issue description from file"`
	Status          *string  `help:"status (name or ID)"`
	Priority        *string  `help:"priority (name or ID)"`
	Assignee        *string  `help:"assignee user ID"`
	IssueType       *string  `help:"issue type (name or ID)"`
	Category        []string `help:"category (multiple allowed)"`
	Version         []string `name:"versions" help:"version (multiple allowed)"`
	Milestone       []string `help:"milestone (multiple allowed)"`
	DueDate         *string  `help:"due date (YYYY-MM-DD)"`
	StartDate       *string  `help:"start date (YYYY-MM-DD)"`
	NotifiedUserID  []int    `help:"notify user IDs (multiple allowed)"`
	Comment         *string  `help:"comment when updating"`
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
			return fmt.Errorf("failed to get statuses: %w", err)
		}
		id, err := resolveNameOrID(*c.Status, toIDNamesFromStatuses(statuses))
		if err != nil {
			return fmt.Errorf("failed to resolve status: %w", err)
		}
		statusID = &id
	}

	// Priority の名前→ID 変換
	var priorityID *int
	if c.Priority != nil {
		priorities, err := rc.Client.ListPriorities(ctx)
		if err != nil {
			return fmt.Errorf("failed to get priorities: %w", err)
		}
		id, err := resolveNameOrID(*c.Priority, priorities)
		if err != nil {
			return fmt.Errorf("failed to resolve priority: %w", err)
		}
		priorityID = &id
	}

	// IssueType の名前→ID 変換
	var issueTypeID *int
	if c.IssueType != nil {
		issueTypes, err := rc.Client.ListProjectIssueTypes(ctx, projectKey)
		if err != nil {
			return fmt.Errorf("failed to get issue types: %w", err)
		}
		id, err := resolveNameOrID(*c.IssueType, issueTypes)
		if err != nil {
			return fmt.Errorf("failed to resolve issue type: %w", err)
		}
		issueTypeID = &id
	}

	// Assignee（数値直接入力）
	var assigneeID *int
	if c.Assignee != nil {
		id, err := strconv.Atoi(*c.Assignee)
		if err != nil {
			return fmt.Errorf("assignee ID must be numeric: %q", *c.Assignee)
		}
		assigneeID = &id
	}

	// Categories の名前→ID 変換
	var categoryIDs []int
	if len(c.Category) > 0 {
		cats, err := rc.Client.ListProjectCategories(ctx, projectKey)
		if err != nil {
			return fmt.Errorf("failed to get categories: %w", err)
		}
		categoryIDs, err = resolveNamesOrIDs(c.Category, toIDNamesFromCategories(cats))
		if err != nil {
			return fmt.Errorf("failed to resolve categories: %w", err)
		}
	}

	// Versions / Milestones の名前→ID 変換
	var versionIDs []int
	var milestoneIDs []int
	if len(c.Version) > 0 || len(c.Milestone) > 0 {
		vers, err := rc.Client.ListProjectVersions(ctx, projectKey)
		if err != nil {
			return fmt.Errorf("failed to get versions: %w", err)
		}
		verIDNames := toIDNamesFromVersions(vers)
		if len(c.Version) > 0 {
			versionIDs, err = resolveNamesOrIDs(c.Version, verIDNames)
			if err != nil {
				return fmt.Errorf("failed to resolve versions: %w", err)
			}
		}
		if len(c.Milestone) > 0 {
			milestoneIDs, err = resolveNamesOrIDs(c.Milestone, verIDNames)
			if err != nil {
				return fmt.Errorf("failed to resolve milestones: %w", err)
			}
		}
	}

	// DueDate / StartDate の parseDate
	var dueDate *time.Time
	if c.DueDate != nil {
		dueDate, err = parseDate(*c.DueDate)
		if err != nil {
			return fmt.Errorf("failed to parse due date: %w", err)
		}
	}
	var startDate *time.Time
	if c.StartDate != nil {
		startDate, err = parseDate(*c.StartDate)
		if err != nil {
			return fmt.Errorf("failed to parse start date: %w", err)
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
	List   IssueCommentListCmd   `cmd:"" help:"list comments"`
	Add    IssueCommentAddCmd    `cmd:"" help:"add comment"`
	Update IssueCommentUpdateCmd `cmd:"" help:"update comment"`
}

// IssueCommentListCmd は issue comment list コマンド（spec §14.6）。
type IssueCommentListCmd struct {
	ListFlags
	IssueIDOrKey string `arg:"" required:"" help:"issue ID or key"`
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
	IssueIDOrKey   string `arg:"" required:"" help:"issue ID or key"`
	Content        string `help:"comment body (mutually exclusive with --content-file)"`
	ContentFile    string `help:"read comment body from file (mutually exclusive with --content)"`
	NotifiedUserID []int  `help:"notify user IDs (multiple allowed)"`
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
	IssueIDOrKey string `arg:"" required:"" help:"issue ID or key"`
	CommentID    int    `arg:"" required:"" help:"comment ID"`
	Content      string `help:"comment body (mutually exclusive with --content-file)"`
	ContentFile  string `help:"read comment body from file (mutually exclusive with --content)"`
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

// fetchAllIssues は自動ページングで全件取得する。最大 10,000 件で打ち切る。
func fetchAllIssues(ctx context.Context, client backlog.Client, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
	const maxTotal = 10000
	if opt.Limit <= 0 {
		opt.Limit = 100 // デフォルトページサイズ
	}
	var all []domain.Issue
	for {
		page, err := client.ListIssues(ctx, opt)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < opt.Limit || len(all) >= maxTotal {
			break
		}
		opt.Offset += opt.Limit
	}
	if len(all) > maxTotal {
		all = all[:maxTotal]
	}
	return all, nil
}

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


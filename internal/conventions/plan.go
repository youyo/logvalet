package conventions

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	"gopkg.in/yaml.v3"
)

// Action は Plan の 1 項目が行う操作。
type Action string

const (
	ActionCreate    Action = "create"
	ActionUpdate    Action = "update"
	ActionUnchanged Action = "unchanged"
	ActionSkip      Action = "skip"
)

// ResourceKind は Plan の 1 項目が対象とするリソース種別。
type ResourceKind string

const (
	KindProject   ResourceKind = "project"
	KindIssueType ResourceKind = "issue_type"
	KindStatus    ResourceKind = "status"
	KindCategory  ResourceKind = "category"
	KindIssue     ResourceKind = "issue"
)

// FieldChange は 1 フィールドの変更内容。
type FieldChange struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// PlanItem は Plan の 1 項目。
type PlanItem struct {
	Resource ResourceKind  `json:"resource"`
	Action   Action        `json:"action"`
	Name     string        `json:"name"`
	Changes  []FieldChange `json:"changes,omitempty"`
	Reason   string        `json:"reason,omitempty"`

	// 実行に必要な内部情報（JSON には出さない）。
	targetID               int
	issueKey               string
	createProjectRequest   *backlog.CreateProjectRequest
	createIssueTypeRequest *backlog.AddIssueTypeRequest
	updateIssueTypeRequest *backlog.UpdateIssueTypeRequest
	createStatusRequest    *backlog.AddStatusRequest
	updateStatusRequest    *backlog.UpdateStatusRequest
	createCategoryRequest  *backlog.AddCategoryRequest
	updateCategoryRequest  *backlog.UpdateCategoryRequest
	createIssueRequest     *backlog.CreateIssueRequest
	updateIssueRequest     *backlog.UpdateIssueRequest
	issueTypeName          string
	categoryName           string
	assigneeName           string
}

// PlanSummary は Plan の集計。
type PlanSummary struct {
	Create    int `json:"create"`
	Update    int `json:"update"`
	Unchanged int `json:"unchanged"`
	Skip      int `json:"skip"`
}

// Plan は conventions を Backlog に適用するための差分計画。
type Plan struct {
	ProjectKey string      `json:"project_key"`
	Items      []PlanItem  `json:"items"`
	Summary    PlanSummary `json:"summary"`
}

// PlanOptions は BuildPlan のオプション。
type PlanOptions struct {
	// ProjectKey は conventions.yaml の project.key を上書きする（空なら上書きしない）。
	ProjectKey string
	// Create はプロジェクトが存在しないときに作成する計画を立てる。
	Create bool
}

// BuildPlan は conventions と Backlog の現状から差分計画を生成する。読み取りのみを行う。
func BuildPlan(ctx context.Context, client backlog.Client, c *Conventions, opt PlanOptions) (*Plan, error) {
	violations := Validate(c)
	if HasError(violations) {
		return nil, fmt.Errorf("conventions に error violation があります: %s", formatViolations(violations))
	}
	if client == nil {
		return nil, errors.New("Backlog client が nil です")
	}

	projectKey := strings.TrimSpace(opt.ProjectKey)
	if projectKey == "" {
		projectKey = strings.TrimSpace(c.Project.Key)
	}
	project, err := client.GetProject(ctx, projectKey)
	if err != nil {
		if !errors.Is(err, backlog.ErrNotFound) {
			return nil, fmt.Errorf("プロジェクト %q の取得に失敗しました: %w", projectKey, err)
		}
		if !opt.Create {
			return nil, fmt.Errorf("プロジェクト %q が見つかりません: %w", projectKey, err)
		}
		return buildCreatePlan(c, projectKey), nil
	}
	if project == nil {
		return nil, fmt.Errorf("プロジェクト %q の取得結果が nil です", projectKey)
	}

	return buildExistingPlan(ctx, client, c, projectKey, project)
}

func buildCreatePlan(c *Conventions, projectKey string) *Plan {
	plan := &Plan{ProjectKey: projectKey}
	addPlanItem(plan, PlanItem{
		Resource: KindProject,
		Action:   ActionCreate,
		Name:     projectKey,
		createProjectRequest: &backlog.CreateProjectRequest{
			Key:  projectKey,
			Name: strings.TrimSpace(c.Project.Name),
		},
	})
	for _, issueType := range c.IssueTypes {
		name := strings.TrimSpace(issueType.Name)
		addPlanItem(plan, PlanItem{
			Resource: KindIssueType,
			Action:   ActionCreate,
			Name:     name,
			createIssueTypeRequest: &backlog.AddIssueTypeRequest{
				Name:                name,
				Color:               issueType.Color,
				TemplateSummary:     issueType.TemplateSummary,
				TemplateDescription: issueType.TemplateDescription,
			},
		})
	}
	for _, status := range c.Statuses {
		name := strings.TrimSpace(status.Name)
		addPlanItem(plan, PlanItem{
			Resource: KindStatus,
			Action:   ActionCreate,
			Name:     name,
			createStatusRequest: &backlog.AddStatusRequest{
				Name:  name,
				Color: status.Color,
			},
		})
	}
	for _, engagement := range c.Engagements {
		name := strings.TrimSpace(engagement.Name)
		addPlanItem(plan, PlanItem{
			Resource:              KindCategory,
			Action:                ActionCreate,
			Name:                  name,
			createCategoryRequest: &backlog.AddCategoryRequest{Name: name},
		})
	}

	issueTypeIDs := issueTypeIDs(c, nil)
	ruleDescription := mustRuleDescription(c)
	addPlanItem(plan, PlanItem{
		Resource:      KindIssue,
		Action:        ActionCreate,
		Name:          RuleIssueSummary,
		issueTypeName: IssueTypeRule,
		createIssueRequest: &backlog.CreateIssueRequest{
			Summary:     RuleIssueSummary,
			IssueTypeID: issueTypeIDs[IssueTypeRule],
			Description: ruleDescription,
		},
	})

	engagementType := issueTypeByName(c.IssueTypes, IssueTypeEngagement)
	for _, engagement := range c.Engagements {
		lead := strings.TrimSpace(engagement.Lead)
		if lead == "" {
			addPlanItem(plan, PlanItem{
				Resource: KindIssue,
				Action:   ActionSkip,
				Name:     engagementIssueSummary(engagement.Name),
				Reason:   "Lead が未指定のため案件親課題を作成しません",
			})
			continue
		}
		startDate := parseDateOrNil(engagement.StartDate)
		dueDate := parseDateOrNil(engagement.DueDate)
		addPlanItem(plan, PlanItem{
			Resource:      KindIssue,
			Action:        ActionCreate,
			Name:          engagementIssueSummary(engagement.Name),
			issueTypeName: IssueTypeEngagement,
			categoryName:  strings.TrimSpace(engagement.Name),
			assigneeName:  lead,
			createIssueRequest: &backlog.CreateIssueRequest{
				Summary:     engagementIssueSummary(engagement.Name),
				IssueTypeID: issueTypeIDs[IssueTypeEngagement],
				Description: engagementType.TemplateDescription,
				StartDate:   startDate,
				DueDate:     dueDate,
			},
		})
	}
	return plan
}

func buildExistingPlan(ctx context.Context, client backlog.Client, c *Conventions, projectKey string, project *domain.Project) (*Plan, error) {
	issueTypes, err := listIssueTypes(ctx, client, projectKey)
	if err != nil {
		return nil, fmt.Errorf("課題種別の取得に失敗しました: %w", err)
	}
	statuses, err := listStatuses(ctx, client, projectKey)
	if err != nil {
		return nil, fmt.Errorf("状態の取得に失敗しました: %w", err)
	}
	categories, err := listCategories(ctx, client, projectKey)
	if err != nil {
		return nil, fmt.Errorf("カテゴリの取得に失敗しました: %w", err)
	}
	issues, err := client.ListIssues(ctx, backlog.ListIssuesOptions{ProjectIDs: []int{project.ID}})
	if errors.Is(err, backlog.ErrNotFound) {
		issues = nil
	} else if err != nil {
		return nil, fmt.Errorf("プロジェクト課題の取得に失敗しました: %w", err)
	}

	issueTypeIndex, err := uniqueIssueTypes(issueTypes)
	if err != nil {
		return nil, err
	}
	statusIndex, err := uniqueStatuses(statuses)
	if err != nil {
		return nil, err
	}
	categoryIndex, err := uniqueCategories(categories)
	if err != nil {
		return nil, err
	}

	users := []domain.User(nil)
	needUsers := false
	for _, engagement := range c.Engagements {
		if strings.TrimSpace(engagement.Lead) != "" {
			needUsers = true
			break
		}
	}
	if needUsers {
		users, err = client.ListProjectUsers(ctx, projectKey, backlog.ListProjectUsersOptions{})
		if errors.Is(err, backlog.ErrNotFound) {
			users = nil
		} else if err != nil {
			return nil, fmt.Errorf("プロジェクトメンバーの取得に失敗しました: %w", err)
		}
	}

	ruleIssues := make([]domain.Issue, 0)
	for _, issue := range issues {
		if issue.IssueType != nil && sameName(issue.IssueType.Name, IssueTypeRule) {
			ruleIssues = append(ruleIssues, issue)
		}
	}
	if len(ruleIssues) > 1 {
		return nil, fmt.Errorf("issue %q（規約課題）が複数あります（%d 件）", IssueTypeRule, len(ruleIssues))
	}

	plan := &Plan{ProjectKey: projectKey}
	addPlanItem(plan, PlanItem{Resource: KindProject, Action: ActionUnchanged, Name: projectKey, targetID: project.ID})
	for _, desired := range c.IssueTypes {
		name := strings.TrimSpace(desired.Name)
		existing, ok := issueTypeIndex[name]
		if !ok {
			addPlanItem(plan, PlanItem{
				Resource: KindIssueType,
				Action:   ActionCreate,
				Name:     name,
				createIssueTypeRequest: &backlog.AddIssueTypeRequest{
					Name:                name,
					Color:               desired.Color,
					TemplateSummary:     desired.TemplateSummary,
					TemplateDescription: desired.TemplateDescription,
				},
			})
			continue
		}
		changes := issueTypeChanges(existing, desired)
		item := PlanItem{Resource: KindIssueType, Name: name, targetID: existing.ID}
		if len(changes) == 0 {
			item.Action = ActionUnchanged
		} else {
			item.Action = ActionUpdate
			item.Changes = changes
			item.updateIssueTypeRequest = issueTypeUpdateRequest(desired, changes)
		}
		addPlanItem(plan, item)
	}

	customStatusCount := 0
	for _, status := range statuses {
		if status.ID >= 5 {
			customStatusCount++
		}
	}
	for _, desired := range c.Statuses {
		name := strings.TrimSpace(desired.Name)
		existing, ok := statusIndex[name]
		if !ok {
			item := PlanItem{Resource: KindStatus, Name: name}
			if customStatusCount >= 8 {
				item.Action = ActionSkip
				item.Reason = "カスタム状態は最大 8 個までです"
			} else {
				item.Action = ActionCreate
				item.createStatusRequest = &backlog.AddStatusRequest{Name: name, Color: desired.Color}
			}
			addPlanItem(plan, item)
			continue
		}
		item := PlanItem{Resource: KindStatus, Action: ActionUnchanged, Name: name, targetID: existing.ID}
		if existing.Color != desired.Color {
			item.Action = ActionUpdate
			item.Changes = []FieldChange{{Field: "color", From: humanValue(existing.Color), To: humanValue(desired.Color)}}
			color := desired.Color
			item.updateStatusRequest = &backlog.UpdateStatusRequest{Color: &color}
		}
		addPlanItem(plan, item)
	}

	for _, desired := range c.Engagements {
		name := strings.TrimSpace(desired.Name)
		existing, ok := categoryIndex[name]
		if !ok {
			addPlanItem(plan, PlanItem{Resource: KindCategory, Action: ActionCreate, Name: name, createCategoryRequest: &backlog.AddCategoryRequest{Name: name}})
		} else {
			addPlanItem(plan, PlanItem{Resource: KindCategory, Action: ActionUnchanged, Name: name, targetID: existing.ID})
		}
	}

	ruleDescription := mustRuleDescription(c)
	ruleTypeID := issueTypeID(issueTypeIndex, IssueTypeRule)
	if len(ruleIssues) == 0 {
		addPlanItem(plan, PlanItem{
			Resource:      KindIssue,
			Action:        ActionCreate,
			Name:          RuleIssueSummary,
			issueTypeName: IssueTypeRule,
			createIssueRequest: &backlog.CreateIssueRequest{
				ProjectID:   project.ID,
				Summary:     RuleIssueSummary,
				IssueTypeID: ruleTypeID,
				Description: ruleDescription,
			},
		})
	} else {
		existing := ruleIssues[0]
		item := PlanItem{Resource: KindIssue, Name: RuleIssueSummary, targetID: existing.ID, issueKey: existing.IssueKey}
		if existing.Description == ruleDescription {
			item.Action = ActionUnchanged
		} else {
			item.Action = ActionUpdate
			item.Changes = []FieldChange{{Field: "description", From: humanValue(existing.Description), To: humanValue(ruleDescription)}}
			description := ruleDescription
			item.updateIssueRequest = &backlog.UpdateIssueRequest{Description: &description}
		}
		addPlanItem(plan, item)
	}

	engagementTypeID := issueTypeID(issueTypeIndex, IssueTypeEngagement)
	engagementType, hasEngagementType := issueTypeIndex[normalize(IssueTypeEngagement)]
	for _, desired := range c.Engagements {
		name := strings.TrimSpace(desired.Name)
		lead := strings.TrimSpace(desired.Lead)
		if lead == "" {
			addPlanItem(plan, PlanItem{Resource: KindIssue, Action: ActionSkip, Name: engagementIssueSummary(name), Reason: "Lead が未指定のため案件親課題を作成しません"})
			continue
		}
		matches := matchingUsers(users, lead)
		if len(matches) == 0 {
			addPlanItem(plan, PlanItem{Resource: KindIssue, Action: ActionSkip, Name: engagementIssueSummary(name), Reason: fmt.Sprintf("Lead %q がプロジェクトメンバーに見つかりません", lead)})
			continue
		}
		if len(matches) > 1 {
			addPlanItem(plan, PlanItem{Resource: KindIssue, Action: ActionSkip, Name: engagementIssueSummary(name), Reason: fmt.Sprintf("Lead %q が複数のメンバーに一致します", lead)})
			continue
		}
		user := matches[0]
		parentMatches := matchingEngagementIssues(issues, name)
		if len(parentMatches) > 1 {
			return nil, fmt.Errorf("案件親課題 %q が複数あります（%d 件）", engagementIssueSummary(name), len(parentMatches))
		}
		category, hasCategory := categoryIndex[name]
		if len(parentMatches) == 0 {
			item := PlanItem{
				Resource:      KindIssue,
				Action:        ActionCreate,
				Name:          engagementIssueSummary(name),
				issueTypeName: IssueTypeEngagement,
				categoryName:  name,
				assigneeName:  lead,
				createIssueRequest: &backlog.CreateIssueRequest{
					ProjectID:   project.ID,
					Summary:     engagementIssueSummary(name),
					IssueTypeID: engagementTypeID,
					Description: engagementTemplateDescription(hasEngagementType, engagementType),
					AssigneeID:  user.ID,
					CategoryIDs: categoryIDs(hasCategory, category.ID),
					StartDate:   parseDateOrNil(desired.StartDate),
					DueDate:     parseDateOrNil(desired.DueDate),
				},
			}
			addPlanItem(plan, item)
			continue
		}

		existing := parentMatches[0]
		changes := engagementChanges(existing, desired, categoryName(category, hasCategory, name), lead)
		item := PlanItem{Resource: KindIssue, Name: engagementIssueSummary(name), targetID: existing.ID, issueKey: existing.IssueKey, categoryName: name, assigneeName: lead}
		if len(changes) == 0 {
			item.Action = ActionUnchanged
		} else {
			item.Action = ActionUpdate
			item.Changes = changes
			item.updateIssueRequest = engagementUpdateRequest(desired, category, hasCategory, user.ID, changes)
		}
		addPlanItem(plan, item)
	}

	return plan, nil
}

func addPlanItem(plan *Plan, item PlanItem) {
	plan.Items = append(plan.Items, item)
	switch item.Action {
	case ActionCreate:
		plan.Summary.Create++
	case ActionUpdate:
		plan.Summary.Update++
	case ActionUnchanged:
		plan.Summary.Unchanged++
	case ActionSkip:
		plan.Summary.Skip++
	}
}

func listIssueTypes(ctx context.Context, client backlog.Client, key string) ([]domain.IssueType, error) {
	items, err := client.ListProjectIssueTypes(ctx, key)
	if errors.Is(err, backlog.ErrNotFound) {
		return nil, nil
	}
	return items, err
}

func listStatuses(ctx context.Context, client backlog.Client, key string) ([]domain.Status, error) {
	items, err := client.ListProjectStatuses(ctx, key)
	if errors.Is(err, backlog.ErrNotFound) {
		return nil, nil
	}
	return items, err
}

func listCategories(ctx context.Context, client backlog.Client, key string) ([]domain.Category, error) {
	items, err := client.ListProjectCategories(ctx, key)
	if errors.Is(err, backlog.ErrNotFound) {
		return nil, nil
	}
	return items, err
}

func uniqueIssueTypes(items []domain.IssueType) (map[string]domain.IssueType, error) {
	result := make(map[string]domain.IssueType, len(items))
	for _, item := range items {
		name := normalize(item.Name)
		if _, exists := result[name]; exists {
			return nil, fmt.Errorf("issue_type %q（課題種別）が複数あります", name)
		}
		result[name] = item
	}
	return result, nil
}

func uniqueStatuses(items []domain.Status) (map[string]domain.Status, error) {
	result := make(map[string]domain.Status, len(items))
	for _, item := range items {
		name := normalize(item.Name)
		if _, exists := result[name]; exists {
			return nil, fmt.Errorf("status %q（状態）が複数あります", name)
		}
		result[name] = item
	}
	return result, nil
}

func uniqueCategories(items []domain.Category) (map[string]domain.Category, error) {
	result := make(map[string]domain.Category, len(items))
	for _, item := range items {
		name := normalize(item.Name)
		if _, exists := result[name]; exists {
			return nil, fmt.Errorf("category %q（カテゴリ）が複数あります", name)
		}
		result[name] = item
	}
	return result, nil
}

func issueTypeChanges(existing domain.IssueType, desired IssueType) []FieldChange {
	changes := make([]FieldChange, 0, 3)
	if existing.Color != desired.Color {
		changes = append(changes, FieldChange{Field: "color", From: humanValue(existing.Color), To: humanValue(desired.Color)})
	}
	if existing.TemplateSummary != desired.TemplateSummary {
		changes = append(changes, FieldChange{Field: "template_summary", From: humanValue(existing.TemplateSummary), To: humanValue(desired.TemplateSummary)})
	}
	if existing.TemplateDescription != desired.TemplateDescription {
		changes = append(changes, FieldChange{Field: "template_description", From: humanValue(existing.TemplateDescription), To: humanValue(desired.TemplateDescription)})
	}
	return changes
}

func issueTypeUpdateRequest(desired IssueType, changes []FieldChange) *backlog.UpdateIssueTypeRequest {
	request := &backlog.UpdateIssueTypeRequest{}
	for _, change := range changes {
		switch change.Field {
		case "color":
			value := desired.Color
			request.Color = &value
		case "template_summary":
			value := desired.TemplateSummary
			request.TemplateSummary = &value
		case "template_description":
			value := desired.TemplateDescription
			request.TemplateDescription = &value
		}
	}
	return request
}

func engagementChanges(existing domain.Issue, desired Engagement, category string, lead string) []FieldChange {
	changes := make([]FieldChange, 0, 4)
	fromAssignee := ""
	if existing.Assignee != nil {
		fromAssignee = existing.Assignee.Name
	}
	if normalize(fromAssignee) != normalize(lead) {
		changes = append(changes, FieldChange{Field: "assignee", From: humanValue(fromAssignee), To: humanValue(lead)})
	}
	fromStart := formatDate(existing.StartDate)
	if fromStart != desired.StartDate {
		changes = append(changes, FieldChange{Field: "start_date", From: humanValue(fromStart), To: humanValue(desired.StartDate)})
	}
	fromDue := formatDate(existing.DueDate)
	if fromDue != desired.DueDate {
		changes = append(changes, FieldChange{Field: "due_date", From: humanValue(fromDue), To: humanValue(desired.DueDate)})
	}
	fromCategory := ""
	if len(existing.Categories) > 0 {
		categoryNames := make([]string, 0, len(existing.Categories))
		for _, existingCategory := range existing.Categories {
			categoryNames = append(categoryNames, strings.TrimSpace(existingCategory.Name))
		}
		fromCategory = strings.Join(categoryNames, ", ")
	}
	if normalize(fromCategory) != normalize(category) {
		changes = append(changes, FieldChange{Field: "category", From: humanValue(fromCategory), To: humanValue(category)})
	}
	return changes
}

func engagementUpdateRequest(desired Engagement, category domain.Category, hasCategory bool, assigneeID int, changes []FieldChange) *backlog.UpdateIssueRequest {
	request := &backlog.UpdateIssueRequest{}
	for _, change := range changes {
		switch change.Field {
		case "assignee":
			request.AssigneeID = &assigneeID
		case "start_date":
			request.StartDate = parseDateOrNil(desired.StartDate)
		case "due_date":
			request.DueDate = parseDateOrNil(desired.DueDate)
		case "category":
			if hasCategory {
				request.CategoryIDs = []int{category.ID}
			} else {
				request.CategoryIDs = nil
			}
		}
	}
	return request
}

func matchingUsers(users []domain.User, name string) []domain.User {
	matches := make([]domain.User, 0, 1)
	for _, user := range users {
		if sameName(user.Name, name) {
			matches = append(matches, user)
		}
	}
	return matches
}

func matchingEngagementIssues(issues []domain.Issue, name string) []domain.Issue {
	matches := make([]domain.Issue, 0, 1)
	summary := engagementIssueSummary(name)
	for _, issue := range issues {
		if issue.IssueType != nil && sameName(issue.IssueType.Name, IssueTypeEngagement) && sameName(issue.Summary, summary) {
			matches = append(matches, issue)
		}
	}
	return matches
}

func issueTypeIDs(c *Conventions, existing map[string]domain.IssueType) map[string]int {
	ids := map[string]int{}
	for _, issueType := range c.IssueTypes {
		name := strings.TrimSpace(issueType.Name)
		if existing != nil {
			if item, ok := existing[normalize(name)]; ok {
				ids[name] = item.ID
				continue
			}
		}
		ids[name] = 0
	}
	return ids
}

func issueTypeByName(items []IssueType, name string) IssueType {
	for _, item := range items {
		if sameName(item.Name, name) {
			return item
		}
	}
	return IssueType{}
}

func issueTypeID(items map[string]domain.IssueType, name string) int {
	if item, ok := items[normalize(name)]; ok {
		return item.ID
	}
	return 0
}

func engagementTemplateDescription(ok bool, issueType domain.IssueType) string {
	if !ok {
		return ""
	}
	return issueType.TemplateDescription
}

func categoryIDs(ok bool, id int) []int {
	if !ok {
		return nil
	}
	return []int{id}
}

func categoryName(category domain.Category, ok bool, fallback string) string {
	if !ok {
		return fallback
	}
	return strings.TrimSpace(category.Name)
}

func formatDate(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02")
}

func parseDateOrNil(value string) *time.Time {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return nil
	}
	return &parsed
}

func humanValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(none)"
	}
	if strings.ContainsAny(value, "\r\n") {
		return "(変更あり)"
	}
	return value
}

func normalize(value string) string { return strings.TrimSpace(value) }

func sameName(left, right string) bool { return normalize(left) == normalize(right) }

func engagementIssueSummary(name string) string { return "[案件] " + strings.TrimSpace(name) }

func mustRuleDescription(c *Conventions) string {
	yamlSource, err := yaml.Marshal(c)
	if err != nil {
		return ""
	}
	return BuildRuleIssueDescription(yamlSource)
}

func formatViolations(violations []Violation) string {
	parts := make([]string, 0)
	for _, violation := range violations {
		if violation.Severity != SeverityError {
			continue
		}
		if violation.Path == "" {
			parts = append(parts, violation.Message)
		} else {
			parts = append(parts, violation.Path+": "+violation.Message)
		}
	}
	return strings.Join(parts, "; ")
}

package conventions

import (
	"context"
	"errors"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
)

// ItemStatus は Plan の 1 項目の実行結果。
type ItemStatus string

const (
	StatusApplied    ItemStatus = "applied"
	StatusFailed     ItemStatus = "failed"
	StatusSkipped    ItemStatus = "skipped"
	StatusNotReached ItemStatus = "not_reached"
)

// ItemResult は Plan の 1 項目の実行結果。
type ItemResult struct {
	PlanItem
	Status ItemStatus `json:"status"`
	Error  string     `json:"error,omitempty"`
}

// ApplyResult は Plan の実行結果。
type ApplyResult struct {
	ProjectKey string       `json:"project_key"`
	Items      []ItemResult `json:"items"`
	Summary    ApplySummary `json:"summary"`
}

// ApplySummary は Plan の実行結果の集計。
type ApplySummary struct {
	Applied    int `json:"applied"`
	Failed     int `json:"failed"`
	Skipped    int `json:"skipped"`
	NotReached int `json:"not_reached"`
}

// Apply は Plan を順に実行する。失敗した時点で止め、残りは not_reached にする。
// 実行結果と、最初に発生したエラー（無ければ nil）を返す。
func Apply(ctx context.Context, client backlog.Client, p *Plan) (*ApplyResult, error) {
	if client == nil {
		return nil, errors.New("Backlog client が nil です")
	}
	if p == nil {
		return nil, errors.New("Plan が nil です")
	}

	result := &ApplyResult{
		ProjectKey: p.ProjectKey,
		Items:      make([]ItemResult, 0, len(p.Items)),
	}
	runtime := newApplyRuntime(p)

	for i, item := range p.Items {
		status, err := applyPlanItem(ctx, client, p.ProjectKey, item, &runtime)
		itemResult := ItemResult{PlanItem: item, Status: status}
		if err != nil {
			itemResult.Status = StatusFailed
			itemResult.Error = err.Error()
			result.Items = append(result.Items, itemResult)
			result.Summary.Failed++
			for _, notReached := range p.Items[i+1:] {
				result.Items = append(result.Items, ItemResult{PlanItem: notReached, Status: StatusNotReached})
				result.Summary.NotReached++
			}
			return result, err
		}

		result.Items = append(result.Items, itemResult)
		switch status {
		case StatusApplied:
			result.Summary.Applied++
		case StatusSkipped:
			result.Summary.Skipped++
		}
	}

	return result, nil
}

type applyRuntime struct {
	projectID           int
	projectCreatePlan   bool
	projectLookupCalled bool
	issueTypeIDs        map[string]int
	categoryIDs         map[string]int
}

func newApplyRuntime(p *Plan) applyRuntime {
	runtime := applyRuntime{
		issueTypeIDs: make(map[string]int),
		categoryIDs:  make(map[string]int),
	}
	for _, item := range p.Items {
		if item.Resource != KindProject {
			continue
		}
		switch item.Action {
		case ActionCreate:
			runtime.projectCreatePlan = true
		case ActionUnchanged:
			runtime.projectID = item.targetID
		}
		break
	}
	return runtime
}

func applyPlanItem(ctx context.Context, client backlog.Client, projectKey string, item PlanItem, runtime *applyRuntime) (ItemStatus, error) {
	switch item.Action {
	case ActionUnchanged:
		return StatusApplied, nil
	case ActionSkip:
		return StatusSkipped, nil
	case ActionCreate, ActionUpdate:
		// handled below
	default:
		return StatusFailed, fmt.Errorf("未対応の action %q です", item.Action)
	}

	switch item.Resource {
	case KindProject:
		if item.Action != ActionCreate {
			return StatusFailed, fmt.Errorf("project の action %q は未対応です", item.Action)
		}
		if item.createProjectRequest == nil {
			return StatusFailed, errors.New("project の create request がありません")
		}
		project, err := client.CreateProject(ctx, *item.createProjectRequest)
		if err != nil {
			return StatusFailed, err
		}
		if project != nil {
			runtime.projectID = project.ID
		}
		return StatusApplied, nil

	case KindIssueType:
		return applyIssueType(ctx, client, projectKey, item, runtime)
	case KindStatus:
		return applyStatus(ctx, client, projectKey, item)
	case KindCategory:
		return applyCategory(ctx, client, projectKey, item, runtime)
	case KindIssue:
		return applyIssue(ctx, client, projectKey, item, runtime)
	default:
		return StatusFailed, fmt.Errorf("未対応の resource %q です", item.Resource)
	}
}

func applyIssueType(ctx context.Context, client backlog.Client, projectKey string, item PlanItem, runtime *applyRuntime) (ItemStatus, error) {
	switch item.Action {
	case ActionCreate:
		if item.createIssueTypeRequest == nil {
			return StatusFailed, errors.New("issue_type の create request がありません")
		}
		created, err := client.AddIssueType(ctx, projectKey, *item.createIssueTypeRequest)
		if err != nil {
			return StatusFailed, err
		}
		if created != nil {
			runtime.issueTypeIDs[normalize(item.Name)] = created.ID
		}
		return StatusApplied, nil
	case ActionUpdate:
		if item.updateIssueTypeRequest == nil {
			return StatusFailed, errors.New("issue_type の update request がありません")
		}
		_, err := client.UpdateIssueType(ctx, projectKey, item.targetID, *item.updateIssueTypeRequest)
		if err != nil {
			return StatusFailed, err
		}
		return StatusApplied, nil
	default:
		return StatusFailed, fmt.Errorf("issue_type の action %q は未対応です", item.Action)
	}
}

func applyStatus(ctx context.Context, client backlog.Client, projectKey string, item PlanItem) (ItemStatus, error) {
	switch item.Action {
	case ActionCreate:
		if item.createStatusRequest == nil {
			return StatusFailed, errors.New("status の create request がありません")
		}
		_, err := client.AddStatus(ctx, projectKey, *item.createStatusRequest)
		if err != nil {
			return StatusFailed, err
		}
		return StatusApplied, nil
	case ActionUpdate:
		if item.updateStatusRequest == nil {
			return StatusFailed, errors.New("status の update request がありません")
		}
		_, err := client.UpdateStatus(ctx, projectKey, item.targetID, *item.updateStatusRequest)
		if err != nil {
			return StatusFailed, err
		}
		return StatusApplied, nil
	default:
		return StatusFailed, fmt.Errorf("status の action %q は未対応です", item.Action)
	}
}

func applyCategory(ctx context.Context, client backlog.Client, projectKey string, item PlanItem, runtime *applyRuntime) (ItemStatus, error) {
	switch item.Action {
	case ActionCreate:
		if item.createCategoryRequest == nil {
			return StatusFailed, errors.New("category の create request がありません")
		}
		created, err := client.AddCategory(ctx, projectKey, *item.createCategoryRequest)
		if err != nil {
			return StatusFailed, err
		}
		if created != nil {
			runtime.categoryIDs[normalize(item.Name)] = created.ID
		}
		return StatusApplied, nil
	case ActionUpdate:
		if item.updateCategoryRequest == nil {
			return StatusFailed, errors.New("category の update request がありません")
		}
		updated, err := client.UpdateCategory(ctx, projectKey, item.targetID, *item.updateCategoryRequest)
		if err != nil {
			return StatusFailed, err
		}
		if updated != nil {
			runtime.categoryIDs[normalize(item.Name)] = updated.ID
		}
		return StatusApplied, nil
	default:
		return StatusFailed, fmt.Errorf("category の action %q は未対応です", item.Action)
	}
}

func applyIssue(ctx context.Context, client backlog.Client, projectKey string, item PlanItem, runtime *applyRuntime) (ItemStatus, error) {
	switch item.Action {
	case ActionCreate:
		if item.createIssueRequest == nil {
			return StatusFailed, errors.New("issue の create request がありません")
		}
		req := *item.createIssueRequest
		if err := resolveCreateIssueRequest(ctx, client, projectKey, item, &req, runtime); err != nil {
			return StatusFailed, err
		}
		_, err := client.CreateIssue(ctx, req)
		if err != nil {
			return StatusFailed, err
		}
		return StatusApplied, nil
	case ActionUpdate:
		if item.updateIssueRequest == nil {
			return StatusFailed, errors.New("issue の update request がありません")
		}
		req := *item.updateIssueRequest
		if err := resolveUpdateIssueRequest(item, &req, runtime); err != nil {
			return StatusFailed, err
		}
		_, err := client.UpdateIssue(ctx, item.issueKey, req)
		if err != nil {
			return StatusFailed, err
		}
		return StatusApplied, nil
	default:
		return StatusFailed, fmt.Errorf("issue の action %q は未対応です", item.Action)
	}
}

func resolveCreateIssueRequest(ctx context.Context, client backlog.Client, projectKey string, item PlanItem, req *backlog.CreateIssueRequest, runtime *applyRuntime) error {
	if req.ProjectID == 0 {
		if runtime.projectID == 0 && !runtime.projectCreatePlan && !runtime.projectLookupCalled {
			runtime.projectLookupCalled = true
			project, err := client.GetProject(ctx, projectKey)
			if err != nil {
				return err
			}
			if project != nil {
				runtime.projectID = project.ID
			}
		}
		req.ProjectID = runtime.projectID
	}
	if req.ProjectID <= 0 {
		return fmt.Errorf("issue %q の project ID を解決できません", item.Name)
	}

	if req.IssueTypeID == 0 && item.issueTypeName != "" {
		req.IssueTypeID = runtime.issueTypeIDs[normalize(item.issueTypeName)]
	}
	if req.IssueTypeID <= 0 {
		return fmt.Errorf("issue %q の issue type ID を解決できません", item.Name)
	}

	if len(req.CategoryIDs) == 0 && item.categoryName != "" {
		categoryID := runtime.categoryIDs[normalize(item.categoryName)]
		if categoryID <= 0 {
			return fmt.Errorf("issue %q の category ID を解決できません", item.Name)
		}
		req.CategoryIDs = []int{categoryID}
	}
	for _, categoryID := range req.CategoryIDs {
		if categoryID <= 0 {
			return fmt.Errorf("issue %q の category ID が不正です", item.Name)
		}
	}
	return nil
}

func resolveUpdateIssueRequest(item PlanItem, req *backlog.UpdateIssueRequest, runtime *applyRuntime) error {
	if len(req.CategoryIDs) == 0 && item.categoryName != "" {
		categoryID := runtime.categoryIDs[normalize(item.categoryName)]
		if categoryID <= 0 {
			return fmt.Errorf("issue %q の category ID を解決できません", item.Name)
		}
		req.CategoryIDs = []int{categoryID}
	}
	for _, categoryID := range req.CategoryIDs {
		if categoryID <= 0 {
			return fmt.Errorf("issue %q の category ID が不正です", item.Name)
		}
	}
	return nil
}

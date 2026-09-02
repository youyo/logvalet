package conventions

import (
	"testing"
)

func TestRenderPlanGolden(t *testing.T) {
	plan := &Plan{
		ProjectKey: "SANDBOX",
		Items: []PlanItem{
			{Resource: KindIssueType, Action: ActionCreate, Name: "案件"},
			{Resource: KindStatus, Action: ActionCreate, Name: "レビュー中"},
			{Resource: KindCategory, Action: ActionUnchanged, Name: "開発チーム"},
			{Resource: KindCategory, Action: ActionCreate, Name: "顧客A 基盤更改"},
			{Resource: KindIssue, Action: ActionUpdate, Name: "[案件] 運用保守", Changes: []FieldChange{{Field: "assignee", From: "(none)", To: "鈴木 花子"}}},
			{Resource: KindIssue, Action: ActionSkip, Name: "[案件] 新規案件", Reason: "Lead が未指定のため案件親課題を作成しません"},
		},
		Summary: PlanSummary{Create: 3, Update: 1, Unchanged: 1, Skip: 1},
	}

	want := "project SANDBOX\n" +
		"  issue_type  + 案件\n" +
		"  status      + レビュー中\n" +
		"  category    = 開発チーム\n" +
		"  category    + 顧客A 基盤更改\n" +
		"  issue       ~ [案件] 運用保守 assignee: (none) -> 鈴木 花子\n" +
		"  issue       ! [案件] 新規案件 Lead が未指定のため案件親課題を作成しません\n" +
		"plan: 3 create, 1 update, 1 unchanged, 1 skip\n"
	if got := RenderPlan(plan); got != want {
		t.Errorf("RenderPlan() =\n%s\nwant\n%s", got, want)
	}
}

func TestRenderResultGolden(t *testing.T) {
	result := &ApplyResult{
		ProjectKey: "SANDBOX",
		Items: []ItemResult{
			{PlanItem: PlanItem{Resource: KindIssueType, Action: ActionCreate, Name: "案件"}, Status: StatusApplied},
			{PlanItem: PlanItem{Resource: KindStatus, Action: ActionUpdate, Name: "レビュー中", Changes: []FieldChange{{Field: "color", From: "#old", To: "#new"}}}, Status: StatusFailed, Error: "permission denied"},
			{PlanItem: PlanItem{Resource: KindCategory, Action: ActionSkip, Name: "開発", Reason: "上限"}, Status: StatusSkipped},
			{PlanItem: PlanItem{Resource: KindIssue, Action: ActionCreate, Name: "[案件] 未到達"}, Status: StatusNotReached},
		},
		Summary: ApplySummary{Applied: 1, Failed: 1, Skipped: 1, NotReached: 1},
	}

	want := "project SANDBOX\n" +
		"  issue_type  + 案件 applied\n" +
		"  status      ~ レビュー中 color: #old -> #new failed: permission denied\n" +
		"  category    ! 開発 上限 skipped\n" +
		"  issue       + [案件] 未到達 not_reached\n" +
		"applied: 1, failed: 1, skipped: 1, not_reached: 1\n"
	if got := RenderResult(result); got != want {
		t.Errorf("RenderResult() =\n%s\nwant\n%s", got, want)
	}
}

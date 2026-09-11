package conventions

import (
	"fmt"
	"strings"
)

// RenderPlan は Plan を人間向けのテキストに整形する。dry-run の stderr 出力に使う。
func RenderPlan(p *Plan) string {
	if p == nil {
		return ""
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "project %s\n", p.ProjectKey)
	for _, item := range p.Items {
		builder.WriteString(renderPlanItem(item))
		builder.WriteByte('\n')
	}
	fmt.Fprintf(&builder, "plan: %d create, %d update, %d unchanged, %d skip\n", p.Summary.Create, p.Summary.Update, p.Summary.Unchanged, p.Summary.Skip)
	return builder.String()
}

// RenderResult は実行結果を人間向けのテキストに整形する。
func RenderResult(r *ApplyResult) string {
	if r == nil {
		return ""
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "project %s\n", r.ProjectKey)
	for _, item := range r.Items {
		builder.WriteString(renderPlanItem(item.PlanItem))
		builder.WriteByte(' ')
		builder.WriteString(renderItemStatus(item.Status, item.Error))
		builder.WriteByte('\n')
	}
	fmt.Fprintf(&builder, "applied: %d, failed: %d, skipped: %d, not_reached: %d\n", r.Summary.Applied, r.Summary.Failed, r.Summary.Skipped, r.Summary.NotReached)
	return builder.String()
}

func renderPlanItem(item PlanItem) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "  %-12s%s %s", item.Resource, actionSymbol(item.Action), item.Name)
	if item.Action == ActionUpdate {
		for _, change := range item.Changes {
			fmt.Fprintf(&builder, " %s: %s -> %s", change.Field, change.From, change.To)
		}
	}
	if item.Action == ActionSkip && item.Reason != "" {
		builder.WriteByte(' ')
		builder.WriteString(item.Reason)
	}
	return builder.String()
}

func actionSymbol(action Action) string {
	switch action {
	case ActionCreate:
		return "+"
	case ActionUpdate:
		return "~"
	case ActionUnchanged:
		return "="
	case ActionSkip:
		return "!"
	default:
		return "?"
	}
}

func renderItemStatus(status ItemStatus, errText string) string {
	switch status {
	case StatusApplied:
		return string(StatusApplied)
	case StatusFailed:
		if errText == "" {
			return string(StatusFailed)
		}
		return string(StatusFailed) + ": " + errText
	case StatusSkipped:
		return string(StatusSkipped)
	case StatusNotReached:
		return string(StatusNotReached)
	default:
		return string(status)
	}
}

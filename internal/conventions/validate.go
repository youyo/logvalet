package conventions

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Severity は違反の重大度。
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Violation は規約の検証で見つかった問題。
type Violation struct {
	Severity Severity `json:"severity"`
	Path     string   `json:"path"`
	Message  string   `json:"message"`
}

var projectKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// Validate は conventions を検証し、見つかった問題を返す。純粋関数。
func Validate(c *Conventions) []Violation {
	if c == nil {
		return []Violation{{Severity: SeverityError, Message: "規約が nil です"}}
	}

	violations := make([]Violation, 0)
	add := func(severity Severity, path, message string) {
		violations = append(violations, Violation{Severity: severity, Path: path, Message: message})
	}

	if c.SchemaVersion != SchemaVersion {
		add(SeverityError, "schema_version", fmt.Sprintf("schema_version は %d である必要があります", SchemaVersion))
	}
	projectKey := strings.TrimSpace(c.Project.Key)
	if projectKey == "" || !projectKeyPattern.MatchString(c.Project.Key) {
		add(SeverityError, "project.key", "project.key は英大文字で始まる英大文字・数字・アンダースコアで指定してください")
	}
	if strings.TrimSpace(c.Project.Name) == "" {
		add(SeverityWarning, "project.name", "project.name が未指定です")
	}

	issueTypeNames := make(map[string]struct{}, len(c.IssueTypes))
	for _, issueType := range c.IssueTypes {
		name := strings.TrimSpace(issueType.Name)
		issueTypeNames[name] = struct{}{}
	}
	if _, ok := issueTypeNames[IssueTypeRule]; !ok {
		add(SeverityError, "issue_types", fmt.Sprintf("issue_types に「%s」がありません", IssueTypeRule))
	}
	if _, ok := issueTypeNames[IssueTypeEngagement]; !ok {
		add(SeverityError, "issue_types", fmt.Sprintf("issue_types に「%s」がありません", IssueTypeEngagement))
	}
	seenIssueTypes := make(map[string]int, len(c.IssueTypes))
	for i, issueType := range c.IssueTypes {
		path := fmt.Sprintf("issue_types[%d]", i)
		name := strings.TrimSpace(issueType.Name)
		if name == "" {
			add(SeverityError, path+".name", "課題種別名が未指定です")
		}
		if _, exists := seenIssueTypes[name]; exists {
			add(SeverityError, path+".name", "課題種別名が重複しています")
		} else {
			seenIssueTypes[name] = i
		}
		if !IsValidIssueTypeColor(issueType.Color) {
			add(SeverityError, path+".color", "課題種別の色が allowlist にありません")
		}
	}

	seenStatuses := make(map[string]int, len(c.Statuses))
	for i, status := range c.Statuses {
		path := fmt.Sprintf("statuses[%d]", i)
		name := strings.TrimSpace(status.Name)
		if name == "" {
			add(SeverityError, path+".name", "状態名が未指定です")
		}
		if _, exists := seenStatuses[name]; exists {
			add(SeverityError, path+".name", "状態名が重複しています")
		} else {
			seenStatuses[name] = i
		}
		if isDefaultStatus(name) {
			add(SeverityError, path+".name", "既定状態と同じ名前は指定できません")
		}
		if !IsValidStatusColor(status.Color) {
			add(SeverityError, path+".color", "状態の色が allowlist にありません")
		}
	}

	initiativeNames := make(map[string]struct{}, len(c.Initiatives))
	seenInitiatives := make(map[string]int, len(c.Initiatives))
	for i, initiative := range c.Initiatives {
		path := fmt.Sprintf("initiatives[%d]", i)
		name := strings.TrimSpace(initiative.Name)
		initiativeNames[name] = struct{}{}
		if name == "" {
			add(SeverityError, path+".name", "Initiative 名が未指定です")
		}
		if _, exists := seenInitiatives[name]; exists {
			add(SeverityError, path+".name", "Initiative 名が重複しています")
		} else {
			seenInitiatives[name] = i
		}
	}

	if len(c.Engagements) > 0 && len(c.Initiatives) == 0 {
		add(SeverityError, "engagements", "案件がある場合は Initiative が必要です")
	}
	seenEngagements := make(map[string]int, len(c.Engagements))
	for i, engagement := range c.Engagements {
		path := fmt.Sprintf("engagements[%d]", i)
		name := strings.TrimSpace(engagement.Name)
		initiative := strings.TrimSpace(engagement.Initiative)
		if name == "" {
			add(SeverityError, path+".name", "案件名が未指定です")
		}
		if _, exists := seenEngagements[name]; exists {
			add(SeverityError, path+".name", "案件名が重複しています")
		} else {
			seenEngagements[name] = i
		}
		if initiative == "" {
			add(SeverityError, path+".initiative", "案件の Initiative が未指定です")
		} else if _, exists := initiativeNames[initiative]; !exists {
			add(SeverityError, path+".initiative", "指定された Initiative が存在しません")
		}
		if strings.TrimSpace(engagement.Lead) == "" {
			add(SeverityWarning, path+".lead", "案件の Lead が未指定です")
		}
		validateEngagementDates(engagement, path, add)
	}

	if strings.TrimSpace(c.Priority.High) == "" {
		add(SeverityWarning, "priority.high", "priority.high が未指定です")
	}
	if strings.TrimSpace(c.Priority.Normal) == "" {
		add(SeverityWarning, "priority.normal", "priority.normal が未指定です")
	}
	if strings.TrimSpace(c.Priority.Low) == "" {
		add(SeverityWarning, "priority.low", "priority.low が未指定です")
	}
	if c.ClosePolicy.LowUntouchedDays != nil && *c.ClosePolicy.LowUntouchedDays <= 0 {
		add(SeverityError, "close_policy.low_untouched_days", "low_untouched_days は 1 以上で指定してください")
	}
	if len(c.Statuses) >= 9 {
		add(SeverityError, "statuses", "状態は 8 件以下で指定してください")
	}

	return violations
}

// HasError は violation に error が 1 件でも含まれるかを返す。
func HasError(vs []Violation) bool {
	for _, violation := range vs {
		if violation.Severity == SeverityError {
			return true
		}
	}
	return false
}

func isDefaultStatus(name string) bool {
	for _, defaultStatus := range DefaultStatuses {
		if name == strings.TrimSpace(defaultStatus) {
			return true
		}
	}
	return false
}

func validateEngagementDates(engagement Engagement, path string, add func(Severity, string, string)) {
	const dateLayout = "2006-01-02"
	startValid := engagement.StartDate == ""
	if !startValid {
		_, err := time.Parse(dateLayout, engagement.StartDate)
		startValid = err == nil
		if !startValid {
			add(SeverityError, path+".start_date", "start_date は YYYY-MM-DD 形式で指定してください")
		}
	}
	dueValid := engagement.DueDate == ""
	if !dueValid {
		_, err := time.Parse(dateLayout, engagement.DueDate)
		dueValid = err == nil
		if !dueValid {
			add(SeverityError, path+".due_date", "due_date は YYYY-MM-DD 形式で指定してください")
		}
	}
	if !startValid || !dueValid || engagement.StartDate == "" || engagement.DueDate == "" {
		return
	}
	start, _ := time.Parse(dateLayout, engagement.StartDate)
	due, _ := time.Parse(dateLayout, engagement.DueDate)
	if due.Before(start) {
		add(SeverityError, path+".due_date", "due_date は start_date 以降で指定してください")
	}
}

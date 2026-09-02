package conventions

import (
	"reflect"
	"testing"
)

func validConventions() *Conventions {
	return &Conventions{
		SchemaVersion: SchemaVersion,
		Project:       Project{Key: "DEMO", Name: "デモ"},
		Priority:      Priority{High: "高", Normal: "中", Low: "低"},
		Statuses:      []Status{{Name: "保留", Color: StatusColors[0]}},
		IssueTypes: []IssueType{
			{Name: IssueTypeRule, Color: IssueTypeColors[0]},
			{Name: IssueTypeEngagement, Color: IssueTypeColors[1]},
		},
		Initiatives: []Initiative{{Name: "重点テーマ"}},
		Engagements: []Engagement{{
			Name:       "顧客基盤更改",
			Lead:       "alice",
			Initiative: "重点テーマ",
			StartDate:  "2026-09-01",
			DueDate:    "2026-09-30",
		}},
	}
}

func TestValidate_ValidConventions(t *testing.T) {
	violations := Validate(validConventions())
	if len(violations) != 0 {
		t.Fatalf("有効な conventions に違反があります: %#v", violations)
	}
}

func TestValidate_AllRules(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		path     string
		mutate   func(*Conventions)
	}{
		{name: "1 schema_version", severity: SeverityError, path: "schema_version", mutate: func(c *Conventions) { c.SchemaVersion = 2 }},
		{name: "2 project.key", severity: SeverityError, path: "project.key", mutate: func(c *Conventions) { c.Project.Key = "demo-1" }},
		{name: "3 project.name", severity: SeverityWarning, path: "project.name", mutate: func(c *Conventions) { c.Project.Name = "  " }},
		{name: "4 規約", severity: SeverityError, path: "issue_types", mutate: func(c *Conventions) { c.IssueTypes = c.IssueTypes[1:] }},
		{name: "5 案件", severity: SeverityError, path: "issue_types", mutate: func(c *Conventions) { c.IssueTypes = c.IssueTypes[:1] }},
		{name: "6 issue type name", severity: SeverityError, path: "issue_types[2].name", mutate: func(c *Conventions) { c.IssueTypes = append(c.IssueTypes, IssueType{Color: IssueTypeColors[2]}) }},
		{name: "7 issue type duplicate", severity: SeverityError, path: "issue_types[2].name", mutate: func(c *Conventions) {
			c.IssueTypes = append(c.IssueTypes, IssueType{Name: IssueTypeEngagement, Color: IssueTypeColors[2]})
		}},
		{name: "8 issue type color", severity: SeverityError, path: "issue_types[2].color", mutate: func(c *Conventions) { c.IssueTypes = append(c.IssueTypes, IssueType{Name: "バグ", Color: "#ffffff"}) }},
		{name: "9 status name", severity: SeverityError, path: "statuses[0].name", mutate: func(c *Conventions) { c.Statuses[0].Name = " \t" }},
		{name: "10 status duplicate", severity: SeverityError, path: "statuses[1].name", mutate: func(c *Conventions) { c.Statuses = append(c.Statuses, Status{Name: "保留", Color: StatusColors[1]}) }},
		{name: "11 default status", severity: SeverityError, path: "statuses[0].name", mutate: func(c *Conventions) { c.Statuses[0].Name = DefaultStatuses[0] }},
		{name: "12 status color", severity: SeverityError, path: "statuses[0].color", mutate: func(c *Conventions) { c.Statuses[0].Color = "#ffffff" }},
		{name: "13 initiative name", severity: SeverityError, path: "initiatives[1].name", mutate: func(c *Conventions) { c.Initiatives = append(c.Initiatives, Initiative{}) }},
		{name: "14 initiative duplicate", severity: SeverityError, path: "initiatives[1].name", mutate: func(c *Conventions) { c.Initiatives = append(c.Initiatives, Initiative{Name: "重点テーマ"}) }},
		{name: "15 engagements without initiatives", severity: SeverityError, path: "engagements", mutate: func(c *Conventions) { c.Initiatives = nil }},
		{name: "16 engagement name", severity: SeverityError, path: "engagements[1].name", mutate: func(c *Conventions) {
			c.Engagements = append(c.Engagements, Engagement{Lead: "alice", Initiative: "重点テーマ"})
		}},
		{name: "17 engagement duplicate", severity: SeverityError, path: "engagements[1].name", mutate: func(c *Conventions) {
			c.Engagements = append(c.Engagements, Engagement{Name: "顧客基盤更改", Lead: "alice", Initiative: "重点テーマ"})
		}},
		{name: "18 engagement initiative empty", severity: SeverityError, path: "engagements[0].initiative", mutate: func(c *Conventions) { c.Engagements[0].Initiative = "  " }},
		{name: "19 engagement initiative missing", severity: SeverityError, path: "engagements[0].initiative", mutate: func(c *Conventions) { c.Engagements[0].Initiative = "存在しないテーマ" }},
		{name: "20 engagement lead", severity: SeverityWarning, path: "engagements[0].lead", mutate: func(c *Conventions) { c.Engagements[0].Lead = "  " }},
		{name: "21 invalid date", severity: SeverityError, path: "engagements[0].start_date", mutate: func(c *Conventions) { c.Engagements[0].StartDate = "2026/09/01" }},
		{name: "22 due before start", severity: SeverityError, path: "engagements[0].due_date", mutate: func(c *Conventions) { c.Engagements[0].DueDate = "2026-08-31" }},
		{name: "23 priority", severity: SeverityWarning, path: "priority.high", mutate: func(c *Conventions) { c.Priority.High = "\n" }},
		{name: "24 close policy", severity: SeverityError, path: "close_policy.low_untouched_days", mutate: func(c *Conventions) { days := 0; c.ClosePolicy.LowUntouchedDays = &days }},
		{name: "25 status count", severity: SeverityError, path: "statuses", mutate: func(c *Conventions) {
			for i := 0; i < 8; i++ {
				c.Statuses = append(c.Statuses, Status{Name: "保留" + string(rune('A'+i)), Color: StatusColors[i%len(StatusColors)]})
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validConventions()
			tt.mutate(c)
			violations := Validate(c)
			if !hasViolation(violations, tt.path, tt.severity) {
				t.Fatalf("%s の違反がありません: %#v", tt.name, violations)
			}
		})
	}
}

func TestValidate_DoesNotCheckDueDateOrderWhenDateInvalid(t *testing.T) {
	c := validConventions()
	c.Engagements[0].StartDate = "2026/09/01"
	c.Engagements[0].DueDate = "2026-08-01"

	violations := Validate(c)
	if hasViolation(violations, "engagements[0].due_date", SeverityError) {
		t.Fatalf("不正な日付に対する日付順序違反が出ています: %#v", violations)
	}
}

func TestValidate_OrderIsDeterministic(t *testing.T) {
	c := validConventions()
	c.SchemaVersion = 2
	c.Project.Key = "demo"
	c.Project.Name = " "
	c.Statuses[0] = Status{Name: "", Color: "#ffffff"}
	c.Engagements[0].Lead = " "
	c.Priority = Priority{}
	days := 0
	c.ClosePolicy.LowUntouchedDays = &days

	wantPaths := []string{
		"schema_version",
		"project.key",
		"project.name",
		"statuses[0].name",
		"statuses[0].color",
		"engagements[0].lead",
		"priority.high",
		"priority.normal",
		"priority.low",
		"close_policy.low_untouched_days",
	}
	first := Validate(c)
	second := Validate(c)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("同じ入力で違反順序が変わりました:\nfirst: %#v\nsecond: %#v", first, second)
	}
	if len(first) != len(wantPaths) {
		t.Fatalf("違反数 = %d, want %d: %#v", len(first), len(wantPaths), first)
	}
	for i, path := range wantPaths {
		if first[i].Path != path {
			t.Fatalf("違反 %d の path = %q, want %q", i, first[i].Path, path)
		}
	}
}

func TestHasError(t *testing.T) {
	if HasError([]Violation{{Severity: SeverityWarning}}) {
		t.Fatal("warning のみで HasError が true になりました")
	}
	if !HasError([]Violation{{Severity: SeverityError}}) {
		t.Fatal("error で HasError が false になりました")
	}
}

func hasViolation(violations []Violation, path string, severity Severity) bool {
	for _, violation := range violations {
		if violation.Path == path && violation.Severity == severity {
			return true
		}
	}
	return false
}

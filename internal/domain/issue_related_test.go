package domain

import (
	"encoding/json"
	"testing"
)

// issue #63 記載の実測スキーマ相当（フラット配列 + 末尾 type フィールド、実測値 "RELATES"）。
const relatedIssueFlatJSON = `{
	"id": 4,
	"projectId": 1,
	"issueKey": "TEST-4",
	"summary": "related summary",
	"description": "",
	"status": {"id": 1, "name": "未対応"},
	"category": [],
	"versions": [],
	"milestone": [],
	"type": "RELATES"
}`

func TestRelatedIssue_UnmarshalJSON_Flat(t *testing.T) {
	var ri RelatedIssue
	if err := json.Unmarshal([]byte(relatedIssueFlatJSON), &ri); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if ri.ID != 4 {
		t.Errorf("ID = %d, want 4", ri.ID)
	}
	if ri.IssueKey != "TEST-4" {
		t.Errorf("IssueKey = %q, want TEST-4", ri.IssueKey)
	}
	if ri.Summary != "related summary" {
		t.Errorf("Summary = %q, want %q", ri.Summary, "related summary")
	}
	if ri.Status == nil || ri.Status.Name != "未対応" {
		t.Errorf("Status = %+v, want Name=未対応", ri.Status)
	}
	if ri.Type != "RELATES" {
		t.Errorf("Type = %q, want RELATES", ri.Type)
	}
}

func TestRelatedIssue_UnmarshalJSON_UnknownType(t *testing.T) {
	src := `{"id": 5, "issueKey": "TEST-5", "summary": "s", "category": [], "versions": [], "milestone": [], "type": "PRECEDES"}`
	var ri RelatedIssue
	if err := json.Unmarshal([]byte(src), &ri); err != nil {
		t.Fatalf("Unmarshal failed for unknown type: %v", err)
	}
	if ri.Type != "PRECEDES" {
		t.Errorf("Type = %q, want PRECEDES (unknown values must be preserved as-is)", ri.Type)
	}
}

func TestRelatedIssue_UnmarshalJSON_MissingType(t *testing.T) {
	src := `{"id": 6, "issueKey": "TEST-6", "summary": "s", "category": [], "versions": [], "milestone": []}`
	var ri RelatedIssue
	if err := json.Unmarshal([]byte(src), &ri); err != nil {
		t.Fatalf("Unmarshal failed when type is missing: %v", err)
	}
	if ri.Type != "" {
		t.Errorf("Type = %q, want empty string when type field is absent", ri.Type)
	}
	if ri.ID != 6 {
		t.Errorf("ID = %d, want 6", ri.ID)
	}
}

func TestRelatedIssue_MarshalJSON_Flat(t *testing.T) {
	ri := RelatedIssue{
		Issue: Issue{
			ID:       7,
			IssueKey: "TEST-7",
			Summary:  "s",
		},
		Type: "RELATES",
	}
	b, err := json.Marshal(ri)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("re-unmarshal failed: %v", err)
	}
	if _, ok := m["type"]; !ok {
		t.Errorf("marshaled JSON missing top-level %q key: %s", "type", b)
	}
	if _, ok := m["issueKey"]; !ok {
		t.Errorf("marshaled JSON missing top-level %q key (Issue fields must be flattened): %s", "issueKey", b)
	}
	if v, _ := m["type"].(string); v != "RELATES" {
		t.Errorf("type = %v, want RELATES", m["type"])
	}
}

// 未公開 API のため将来的にネスト形状 {"relatedIssue": {...}, "type": "..."} へ変更される
// 可能性を排除できず、両形状を受容できることを確認する。
func TestRelatedIssue_UnmarshalJSON_Nested(t *testing.T) {
	src := `{
		"relatedIssue": {
			"id": 8,
			"issueKey": "TEST-8",
			"summary": "nested summary",
			"category": [],
			"versions": [],
			"milestone": []
		},
		"type": "RELATES"
	}`
	var ri RelatedIssue
	if err := json.Unmarshal([]byte(src), &ri); err != nil {
		t.Fatalf("Unmarshal failed for nested form: %v", err)
	}
	if ri.ID != 8 {
		t.Errorf("ID = %d, want 8", ri.ID)
	}
	if ri.IssueKey != "TEST-8" {
		t.Errorf("IssueKey = %q, want TEST-8", ri.IssueKey)
	}
	if ri.Summary != "nested summary" {
		t.Errorf("Summary = %q, want %q", ri.Summary, "nested summary")
	}
	if ri.Type != "RELATES" {
		t.Errorf("Type = %q, want RELATES", ri.Type)
	}
}

func TestRelatedIssue_UnmarshalJSON_Array(t *testing.T) {
	src := `[` + relatedIssueFlatJSON + `]`
	var list []RelatedIssue
	if err := json.Unmarshal([]byte(src), &list); err != nil {
		t.Fatalf("Unmarshal array failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if list[0].IssueKey != "TEST-4" || list[0].Type != "RELATES" {
		t.Errorf("list[0] = %+v, want IssueKey=TEST-4 Type=RELATES", list[0])
	}
}

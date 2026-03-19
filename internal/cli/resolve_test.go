package cli

import (
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/domain"
)

var testIDNames = []domain.IDName{
	{ID: 1, Name: "課題"},
	{ID: 2, Name: "バグ"},
	{ID: 3, Name: "Task"},
}

func TestResolveNameOrID_byName(t *testing.T) {
	id, err := resolveNameOrID("課題", testIDNames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 1 {
		t.Errorf("id = %d, want 1", id)
	}
}

func TestResolveNameOrID_byID(t *testing.T) {
	id, err := resolveNameOrID("2", testIDNames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 2 {
		t.Errorf("id = %d, want 2", id)
	}
}

func TestResolveNameOrID_notFound(t *testing.T) {
	_, err := resolveNameOrID("不明", testIDNames)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolveNameOrID_empty(t *testing.T) {
	_, err := resolveNameOrID("", testIDNames)
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}

func TestResolveNameOrID_caseInsensitive(t *testing.T) {
	items := []domain.IDName{
		{ID: 1, Name: "Bug"},
		{ID: 2, Name: "Task"},
	}
	id, err := resolveNameOrID("bug", items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 1 {
		t.Errorf("id = %d, want 1", id)
	}
}

func TestResolveNameOrID_duplicateName(t *testing.T) {
	items := []domain.IDName{
		{ID: 1, Name: "課題"},
		{ID: 2, Name: "課題"},
	}
	_, err := resolveNameOrID("課題", items)
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}
}

func TestResolveNamesOrIDs_multiple(t *testing.T) {
	items := []domain.IDName{
		{ID: 1, Name: "課題"},
		{ID: 2, Name: "バグ"},
		{ID: 3, Name: "Task"},
	}
	ids, err := resolveNamesOrIDs([]string{"課題", "2", "Task"}, items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("len(ids) = %d, want 3", len(ids))
	}
	if ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Errorf("ids = %v, want [1, 2, 3]", ids)
	}
}

func TestResolveNamesOrIDs_error(t *testing.T) {
	items := []domain.IDName{
		{ID: 1, Name: "課題"},
	}
	_, err := resolveNamesOrIDs([]string{"課題", "不明"}, items)
	if err == nil {
		t.Fatal("expected error for unknown name, got nil")
	}
}

func TestParseDate_valid(t *testing.T) {
	result, err := parseDate("2026-03-19")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil time, got nil")
	}
	expected := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("result = %v, want %v", result, expected)
	}
}

func TestParseDate_invalid(t *testing.T) {
	_, err := parseDate("invalid")
	if err == nil {
		t.Fatal("expected error for invalid date, got nil")
	}
}

func TestParseDate_empty(t *testing.T) {
	result, err := parseDate("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestExtractProjectKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"PROJ-123", "PROJ"},
		{"HEP_ISSUES-456", "HEP_ISSUES"},
		{"PROJECT-1", "PROJECT"},
		{"NODIGIT", "NODIGIT"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractProjectKey(tt.input)
			if got != tt.want {
				t.Errorf("extractProjectKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

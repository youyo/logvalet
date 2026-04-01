package analysis

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// T1: TestNewAnalysisEnvelope_JSONShape は AnalysisEnvelope の JSON 構造を検証する。
func TestNewAnalysisEnvelope_JSONShape(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	client := backlog.NewMockClient()

	b := NewBaseAnalysisBuilder(client, "default", "heptagon", "https://heptagon.backlog.com", WithClock(func() time.Time { return fixedNow }))

	data := map[string]string{"key": "value"}
	env := b.newEnvelope("test_resource", data, nil)

	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// schema_version
	if v, ok := m["schema_version"]; !ok {
		t.Error("missing field: schema_version")
	} else if v != "1" {
		t.Errorf("schema_version = %v, want %q", v, "1")
	}

	// resource
	if v, ok := m["resource"]; !ok {
		t.Error("missing field: resource")
	} else if v != "test_resource" {
		t.Errorf("resource = %v, want %q", v, "test_resource")
	}

	// generated_at
	if _, ok := m["generated_at"]; !ok {
		t.Error("missing field: generated_at")
	}

	// analysis
	if _, ok := m["analysis"]; !ok {
		t.Error("missing field: analysis")
	}

	// warnings は空配列（nil ではない）
	warningsRaw, ok := m["warnings"]
	if !ok {
		t.Fatal("missing field: warnings")
	}
	warningsArr, ok := warningsRaw.([]interface{})
	if !ok {
		t.Fatalf("warnings is not an array: %T", warningsRaw)
	}
	if len(warningsArr) != 0 {
		t.Errorf("warnings length = %d, want 0", len(warningsArr))
	}
}

// T2: TestBaseAnalysisBuilder_NewEnvelope は newEnvelope が profile, space, base_url を正しくセットすることを検証する。
func TestBaseAnalysisBuilder_NewEnvelope(t *testing.T) {
	fixedNow := time.Date(2026, 4, 1, 9, 30, 0, 0, time.UTC)
	client := backlog.NewMockClient()

	b := NewBaseAnalysisBuilder(client, "prod", "myspace", "https://myspace.backlog.com", WithClock(func() time.Time { return fixedNow }))

	env := b.newEnvelope("issue_context", "dummy", []domain.Warning{
		{Code: "test_warn", Message: "test warning", Component: "test", Retryable: false},
	})

	if env.Profile != "prod" {
		t.Errorf("Profile = %q, want %q", env.Profile, "prod")
	}
	if env.Space != "myspace" {
		t.Errorf("Space = %q, want %q", env.Space, "myspace")
	}
	if env.BaseURL != "https://myspace.backlog.com" {
		t.Errorf("BaseURL = %q, want %q", env.BaseURL, "https://myspace.backlog.com")
	}
	if !env.GeneratedAt.Equal(fixedNow) {
		t.Errorf("GeneratedAt = %v, want %v", env.GeneratedAt, fixedNow)
	}
	if len(env.Warnings) != 1 {
		t.Errorf("Warnings length = %d, want 1", len(env.Warnings))
	}
}

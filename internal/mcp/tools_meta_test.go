package mcp_test

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// ===== B9: logvalet_meta_version =====

// TestMetaVersion_Normal は project_key 指定で ListProjectVersions が呼ばれることを確認する。
func TestMetaVersion_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedProjectKey string
	mock.ListProjectVersionsFunc = func(ctx context.Context, projectKey string) ([]domain.Version, error) {
		capturedProjectKey = projectKey
		return []domain.Version{{ID: 1, Name: "v1.0"}, {ID: 2, Name: "v2.0"}}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_meta_version", map[string]any{"project_key": "PROJ"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedProjectKey != "PROJ" {
		t.Errorf("projectKey = %q, want %q", capturedProjectKey, "PROJ")
	}
	if mock.GetCallCount("ListProjectVersions") != 1 {
		t.Errorf("expected ListProjectVersions called 1 time, got %d", mock.GetCallCount("ListProjectVersions"))
	}
}

// TestMetaVersion_MissingProjectKey は project_key 未指定で IsError=true になることを確認する。
func TestMetaVersion_MissingProjectKey(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_meta_version", map[string]any{})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// ===== B10: logvalet_meta_custom_field =====

// TestMetaCustomField_Normal は project_key 指定で ListProjectCustomFields が呼ばれることを確認する。
func TestMetaCustomField_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedProjectKey string
	mock.ListProjectCustomFieldsFunc = func(ctx context.Context, projectKey string) ([]domain.CustomFieldDefinition, error) {
		capturedProjectKey = projectKey
		return []domain.CustomFieldDefinition{{ID: 1, Name: "カスタムフィールド1"}}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_meta_custom_field", map[string]any{"project_key": "PROJ"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedProjectKey != "PROJ" {
		t.Errorf("projectKey = %q, want %q", capturedProjectKey, "PROJ")
	}
	if mock.GetCallCount("ListProjectCustomFields") != 1 {
		t.Errorf("expected ListProjectCustomFields called 1 time, got %d", mock.GetCallCount("ListProjectCustomFields"))
	}
}

// TestMetaCustomField_MissingProjectKey は project_key 未指定で IsError=true になることを確認する。
func TestMetaCustomField_MissingProjectKey(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_meta_custom_field", map[string]any{})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

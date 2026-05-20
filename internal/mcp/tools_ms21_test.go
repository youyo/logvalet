package mcp_test

import (
	"context"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/youyo/logvalet/internal/backlog"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// ============================================================================
// M21: RegisterWithSpaces / RegisterWithSpacesWrite が InputSchema.Properties に
//      spaces / all_spaces を自動注入すること
// ============================================================================

// TestRegisterWithSpaces_InjectsSpacesAndAllSpaces は RegisterWithSpaces が
// tool.InputSchema.Properties に "spaces" と "all_spaces" を注入することを検証する。
func TestRegisterWithSpaces_InjectsSpacesAndAllSpaces(t *testing.T) {
	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", nil, nil)

	tool := gomcp.NewTool("inject_test", gomcp.WithDescription("test"))
	reg.RegisterWithSpaces(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		return nil, nil
	})

	st := s.GetTool("inject_test")
	if st == nil {
		t.Fatal("tool not registered")
	}
	props := st.Tool.InputSchema.Properties
	if _, ok := props["spaces"]; !ok {
		t.Error("expected 'spaces' in InputSchema.Properties")
	}
	if _, ok := props["all_spaces"]; !ok {
		t.Error("expected 'all_spaces' in InputSchema.Properties")
	}
}

// TestRegisterWithSpaces_PreservesExistingProperties は注入が既存 Properties を上書きしないことを検証する。
func TestRegisterWithSpaces_PreservesExistingProperties(t *testing.T) {
	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", nil, nil)

	tool := gomcp.NewTool("preserve_test",
		gomcp.WithDescription("preserve"),
		gomcp.WithString("mode", gomcp.Description("mode")),
	)
	reg.RegisterWithSpaces(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		return nil, nil
	})

	st := s.GetTool("preserve_test")
	if st == nil {
		t.Fatal("tool not registered")
	}
	props := st.Tool.InputSchema.Properties
	if _, ok := props["mode"]; !ok {
		t.Error("expected existing 'mode' property to be preserved")
	}
	if _, ok := props["spaces"]; !ok {
		t.Error("expected 'spaces' to be injected alongside existing properties")
	}
	if _, ok := props["all_spaces"]; !ok {
		t.Error("expected 'all_spaces' to be injected alongside existing properties")
	}
}

// TestRegisterWithSpacesWrite_InjectsSpaces は RegisterWithSpacesWrite が
// tool.InputSchema.Properties に "spaces" を注入することを検証する。
func TestRegisterWithSpacesWrite_InjectsSpaces(t *testing.T) {
	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", nil, nil)

	tool := gomcp.NewTool("write_inject_test", gomcp.WithDescription("test write"))
	reg.RegisterWithSpacesWrite(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		return nil, nil
	})

	st := s.GetTool("write_inject_test")
	if st == nil {
		t.Fatal("tool not registered")
	}
	props := st.Tool.InputSchema.Properties
	if _, ok := props["spaces"]; !ok {
		t.Error("expected 'spaces' in InputSchema.Properties for write tool")
	}
}

// TestRegisterWithSpaces_SpacesPropertySchema は注入した "spaces" が配列型スキーマを持つことを検証する。
func TestRegisterWithSpaces_SpacesPropertySchema(t *testing.T) {
	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(true))
	reg := mcpinternal.NewToolRegistryWithMultiSpace(s, nil, "", nil, nil)

	tool := gomcp.NewTool("schema_test", gomcp.WithDescription("schema"))
	reg.RegisterWithSpaces(tool, func(_ context.Context, _ backlog.Client, _ map[string]any) (any, error) {
		return nil, nil
	})

	st := s.GetTool("schema_test")
	props := st.Tool.InputSchema.Properties

	spacesProp, ok := props["spaces"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'spaces' to be map[string]any, got %T", props["spaces"])
	}
	if spacesProp["type"] != "array" {
		t.Errorf("expected spaces.type=array, got %v", spacesProp["type"])
	}

	allSpacesProp, ok := props["all_spaces"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'all_spaces' to be map[string]any, got %T", props["all_spaces"])
	}
	if allSpacesProp["type"] != "boolean" {
		t.Errorf("expected all_spaces.type=boolean, got %v", allSpacesProp["type"])
	}
}

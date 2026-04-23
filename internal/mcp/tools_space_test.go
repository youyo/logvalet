package mcp_test

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// ===== B7: logvalet_space_digest =====

// TestSpaceDigest_Normal はパラメータなしで DefaultSpaceDigestBuilder.Build が呼ばれることを確認する。
func TestSpaceDigest_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetSpaceFunc = func(ctx context.Context) (*domain.Space, error) {
		return &domain.Space{SpaceKey: "test-space", Name: "テストスペース"}, nil
	}
	mock.ListSpaceActivitiesFunc = func(ctx context.Context, opt backlog.ListActivitiesOptions) ([]domain.Activity, error) {
		return []domain.Activity{}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_space_digest", map[string]any{})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if mock.GetCallCount("GetSpace") != 1 {
		t.Errorf("expected GetSpace called 1 time, got %d", mock.GetCallCount("GetSpace"))
	}
}

// ===== B8: logvalet_space_disk_usage =====

// TestSpaceDiskUsage_Normal は GetSpaceDiskUsage が呼ばれることを確認する。
func TestSpaceDiskUsage_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetSpaceDiskUsageFunc = func(ctx context.Context) (*domain.DiskUsage, error) {
		return &domain.DiskUsage{Capacity: 1024 * 1024 * 1024, Issue: 512 * 1024}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_space_disk_usage", map[string]any{})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if mock.GetCallCount("GetSpaceDiskUsage") != 1 {
		t.Errorf("expected GetSpaceDiskUsage called 1 time, got %d", mock.GetCallCount("GetSpaceDiskUsage"))
	}
}

// TestSpaceDiskUsage_Error は GetSpaceDiskUsage がエラーの場合に IsError=true になることを確認する。
func TestSpaceDiskUsage_Error(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetSpaceDiskUsageFunc = func(ctx context.Context) (*domain.DiskUsage, error) {
		return nil, backlog.ErrAPI
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_space_disk_usage", map[string]any{})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

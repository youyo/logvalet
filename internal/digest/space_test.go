package digest

import (
	"context"
	"errors"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

func TestSpaceDigestBuilder_Build_success(t *testing.T) {
	mock := backlog.NewMockClient()

	space := &domain.Space{
		SpaceKey:           "myspace",
		Name:               "My Space",
		OwnerID:            1,
		Lang:               "ja",
		Timezone:           "Asia/Tokyo",
		ReportSendTime:     "08:00",
		TextFormattingRule: "markdown",
	}
	diskUsage := &domain.DiskUsage{
		Capacity:   10737418240, // 10 GB
		Issue:      1073741824,  // 1 GB
		Wiki:       536870912,   // 512 MB
		File:       268435456,   // 256 MB
		Subversion: 0,
		Git:        0,
		GitLFS:     0,
	}

	mock.GetSpaceFunc = func(ctx context.Context) (*domain.Space, error) {
		return space, nil
	}
	mock.GetSpaceDiskUsageFunc = func(ctx context.Context) (*domain.DiskUsage, error) {
		return diskUsage, nil
	}

	builder := NewDefaultSpaceDigestBuilder(mock, "default", "myspace", "https://myspace.backlog.com")
	envelope, err := builder.Build(context.Background(), SpaceDigestOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if envelope == nil {
		t.Fatal("envelope is nil")
	}
	if envelope.Resource != "space" {
		t.Errorf("Resource = %q, want %q", envelope.Resource, "space")
	}
	if len(envelope.Warnings) != 0 {
		t.Errorf("Warnings = %v, want empty", envelope.Warnings)
	}

	d, ok := envelope.Digest.(*SpaceDigest)
	if !ok {
		t.Fatal("Digest is not *SpaceDigest")
	}
	if d.Space.SpaceKey != "myspace" {
		t.Errorf("Space.SpaceKey = %q, want %q", d.Space.SpaceKey, "myspace")
	}
	if d.Space.Name != "My Space" {
		t.Errorf("Space.Name = %q, want %q", d.Space.Name, "My Space")
	}
	if d.DiskUsage == nil {
		t.Error("DiskUsage is nil, want non-nil")
	}
	if d.DiskUsage.Capacity != 10737418240 {
		t.Errorf("DiskUsage.Capacity = %d, want 10737418240", d.DiskUsage.Capacity)
	}
	if !d.Summary.HasDiskUsage {
		t.Error("Summary.HasDiskUsage = false, want true")
	}
	if d.Summary.Headline == "" {
		t.Error("Summary.Headline is empty")
	}
}

func TestSpaceDigestBuilder_Build_space_fetch_failed(t *testing.T) {
	mock := backlog.NewMockClient()

	fetchErr := errors.New("GetSpace API error")
	mock.GetSpaceFunc = func(ctx context.Context) (*domain.Space, error) {
		return nil, fetchErr
	}

	builder := NewDefaultSpaceDigestBuilder(mock, "default", "myspace", "https://myspace.backlog.com")
	_, err := builder.Build(context.Background(), SpaceDigestOptions{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSpaceDigestBuilder_Build_disk_usage_fetch_failed(t *testing.T) {
	mock := backlog.NewMockClient()

	space := &domain.Space{
		SpaceKey: "myspace",
		Name:     "My Space",
		OwnerID:  1,
		Lang:     "ja",
		Timezone: "Asia/Tokyo",
	}
	mock.GetSpaceFunc = func(ctx context.Context) (*domain.Space, error) {
		return space, nil
	}
	mock.GetSpaceDiskUsageFunc = func(ctx context.Context) (*domain.DiskUsage, error) {
		return nil, errors.New("disk usage fetch failed")
	}

	builder := NewDefaultSpaceDigestBuilder(mock, "default", "myspace", "https://myspace.backlog.com")
	envelope, err := builder.Build(context.Background(), SpaceDigestOptions{})

	// ディスク使用量取得失敗は partial success（warning 付き nil DiskUsage）
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envelope.Warnings) == 0 {
		t.Error("expected warnings for disk usage fetch failure, got none")
	}

	d, ok := envelope.Digest.(*SpaceDigest)
	if !ok {
		t.Fatal("Digest is not *SpaceDigest")
	}
	if d.DiskUsage != nil {
		t.Errorf("DiskUsage = %v, want nil", d.DiskUsage)
	}
	if d.Summary.HasDiskUsage {
		t.Error("Summary.HasDiskUsage = true, want false")
	}
}

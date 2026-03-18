package digest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

func TestDocumentDigestBuilder_Build_success(t *testing.T) {
	now := time.Now().UTC()
	doc := &domain.Document{
		ID:        "test-doc-uuid-001",
		ProjectID: 10,
		Title:     "テストドキュメント",
		Plain:     "本文テキスト",
		Created:   &now,
		Updated:   &now,
		CreatedUser: &domain.User{
			ID:   1,
			Name: "Alice",
		},
	}
	project := &domain.Project{
		ID:         10,
		ProjectKey: "PROJ",
		Name:       "テストプロジェクト",
	}
	attachments := []domain.Attachment{
		{ID: 1, Name: "file.txt", Size: 1024},
	}

	mock := backlog.NewMockClient()
	mock.GetDocumentFunc = func(ctx context.Context, documentID string) (*domain.Document, error) {
		if documentID != "test-doc-uuid-001" {
			return nil, errors.New("unexpected documentID")
		}
		return doc, nil
	}
	mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
		return []domain.Project{*project}, nil
	}
	mock.ListDocumentAttachmentsFunc = func(ctx context.Context, documentID string) ([]domain.Attachment, error) {
		return attachments, nil
	}

	builder := NewDefaultDocumentDigestBuilder(mock, "default", "myspace", "https://example.backlog.com")
	envelope, err := builder.Build(context.Background(), "test-doc-uuid-001", DocumentDigestOptions{})
	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}

	if envelope.Resource != "document" {
		t.Errorf("Resource = %q, want %q", envelope.Resource, "document")
	}
	if len(envelope.Warnings) != 0 {
		t.Errorf("Warnings = %d, want 0", len(envelope.Warnings))
	}

	d, ok := envelope.Digest.(*DocumentDigest)
	if !ok {
		t.Fatalf("Digest is not *DocumentDigest")
	}
	if d.Document.ID != "test-doc-uuid-001" {
		t.Errorf("Document.ID = %q, want %q", d.Document.ID, "test-doc-uuid-001")
	}
	if d.Document.Title != "テストドキュメント" {
		t.Errorf("Document.Title = %q, want %q", d.Document.Title, "テストドキュメント")
	}
	if d.Project.Key != "PROJ" {
		t.Errorf("Project.Key = %q, want %q", d.Project.Key, "PROJ")
	}
	if len(d.Attachments) != 1 {
		t.Errorf("Attachments count = %d, want 1", len(d.Attachments))
	}
	if !d.Summary.HasContent {
		t.Error("Summary.HasContent = false, want true")
	}
	if d.Summary.AttachmentCount != 1 {
		t.Errorf("Summary.AttachmentCount = %d, want 1", d.Summary.AttachmentCount)
	}
	if d.Document.CreatedUser == nil {
		t.Error("Document.CreatedUser = nil, want non-nil")
	}
}

func TestDocumentDigestBuilder_Build_document_not_found(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetDocumentFunc = func(ctx context.Context, documentID string) (*domain.Document, error) {
		return nil, backlog.ErrNotFound
	}

	builder := NewDefaultDocumentDigestBuilder(mock, "default", "myspace", "https://example.backlog.com")
	_, err := builder.Build(context.Background(), "nonexistent-uuid", DocumentDigestOptions{})
	if err == nil {
		t.Fatal("Build() should return error when document not found")
	}
}

func TestDocumentDigestBuilder_Build_attachments_fetch_failed(t *testing.T) {
	now := time.Now().UTC()
	doc := &domain.Document{
		ID:        "test-doc-uuid-001",
		ProjectID: 10,
		Title:     "テスト",
		Plain:     "",
		Created:   &now,
	}
	project := &domain.Project{
		ID:         10,
		ProjectKey: "PROJ",
		Name:       "プロジェクト",
	}

	mock := backlog.NewMockClient()
	mock.GetDocumentFunc = func(ctx context.Context, documentID string) (*domain.Document, error) {
		return doc, nil
	}
	mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
		return []domain.Project{*project}, nil
	}
	mock.ListDocumentAttachmentsFunc = func(ctx context.Context, documentID string) ([]domain.Attachment, error) {
		return nil, errors.New("network error")
	}

	builder := NewDefaultDocumentDigestBuilder(mock, "default", "myspace", "https://example.backlog.com")
	envelope, err := builder.Build(context.Background(), "test-doc-uuid-001", DocumentDigestOptions{})
	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}

	// 添付ファイル取得失敗は warning として記録し partial success を返す
	if len(envelope.Warnings) == 0 {
		t.Error("Warnings should not be empty when attachments fetch failed")
	}

	d, ok := envelope.Digest.(*DocumentDigest)
	if !ok {
		t.Fatalf("Digest is not *DocumentDigest")
	}
	if len(d.Attachments) != 0 {
		t.Errorf("Attachments count = %d, want 0 (empty slice)", len(d.Attachments))
	}
}

func TestDocumentDigestBuilder_Build_project_fetch_failed(t *testing.T) {
	now := time.Now().UTC()
	doc := &domain.Document{
		ID:        "test-doc-uuid-001",
		ProjectID: 10,
		Title:     "テスト",
		Created:   &now,
	}

	mock := backlog.NewMockClient()
	mock.GetDocumentFunc = func(ctx context.Context, documentID string) (*domain.Document, error) {
		return doc, nil
	}
	mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
		return nil, errors.New("projects fetch failed")
	}
	mock.ListDocumentAttachmentsFunc = func(ctx context.Context, documentID string) ([]domain.Attachment, error) {
		return []domain.Attachment{}, nil
	}

	builder := NewDefaultDocumentDigestBuilder(mock, "default", "myspace", "https://example.backlog.com")
	envelope, err := builder.Build(context.Background(), "test-doc-uuid-001", DocumentDigestOptions{})
	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}

	// プロジェクト取得失敗は warning として記録する
	if len(envelope.Warnings) == 0 {
		t.Error("Warnings should not be empty when project fetch failed")
	}

	d, ok := envelope.Digest.(*DocumentDigest)
	if !ok {
		t.Fatalf("Digest is not *DocumentDigest")
	}
	// プロジェクト情報は空になる
	if d.Project.Key != "" {
		t.Errorf("Project.Key = %q, want empty (project not resolved)", d.Project.Key)
	}
}

func TestDocumentDigestBuilder_Build_project_id_not_matched(t *testing.T) {
	now := time.Now().UTC()
	doc := &domain.Document{
		ID:        "test-doc-uuid-001",
		ProjectID: 999, // どのプロジェクトにもマッチしない
		Title:     "テスト",
		Created:   &now,
	}
	otherProject := &domain.Project{
		ID:         10,
		ProjectKey: "OTHER",
		Name:       "別プロジェクト",
	}

	mock := backlog.NewMockClient()
	mock.GetDocumentFunc = func(ctx context.Context, documentID string) (*domain.Document, error) {
		return doc, nil
	}
	mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
		return []domain.Project{*otherProject}, nil
	}
	mock.ListDocumentAttachmentsFunc = func(ctx context.Context, documentID string) ([]domain.Attachment, error) {
		return []domain.Attachment{}, nil
	}

	builder := NewDefaultDocumentDigestBuilder(mock, "default", "myspace", "https://example.backlog.com")
	envelope, err := builder.Build(context.Background(), "test-doc-uuid-001", DocumentDigestOptions{})
	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}

	// ID マッチ失敗は warning として記録する
	if len(envelope.Warnings) == 0 {
		t.Error("Warnings should not be empty when project ID not matched")
	}
}

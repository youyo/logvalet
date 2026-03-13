package digest

import (
	"context"
	"fmt"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// DocumentDigestOptions は DocumentDigestBuilder.Build() のオプション。
// 将来の拡張用プレースホルダー。
type DocumentDigestOptions struct{}

// DocumentDigestBuilder はインターフェース（spec §13.5）。
type DocumentDigestBuilder interface {
	Build(ctx context.Context, documentID int64, opt DocumentDigestOptions) (*domain.DigestEnvelope, error)
}

// DefaultDocumentDigestBuilder は DocumentDigestBuilder の標準実装。
// backlog.Client を使って必要なデータを収集し DigestEnvelope を構築する。
type DefaultDocumentDigestBuilder struct {
	client  backlog.Client
	profile string
	space   string
	baseURL string
}

// NewDefaultDocumentDigestBuilder は DefaultDocumentDigestBuilder を生成する。
func NewDefaultDocumentDigestBuilder(client backlog.Client, profile, space, baseURL string) *DefaultDocumentDigestBuilder {
	return &DefaultDocumentDigestBuilder{
		client:  client,
		profile: profile,
		space:   space,
		baseURL: baseURL,
	}
}

// DocumentDigest は digest フィールドに格納されるドキュメントダイジェスト構造体（spec §13.5）。
type DocumentDigest struct {
	Document    DigestDocumentDetail  `json:"document"`
	Project     DigestProject         `json:"project"`
	Attachments []domain.Attachment   `json:"attachments"`
	Summary     DocumentDigestSummary `json:"summary"`
	LLMHints    DigestLLMHints        `json:"llm_hints"`
}

// DigestDocumentDetail は Document Digest 内のドキュメント詳細情報（spec §13.5 document）。
type DigestDocumentDetail struct {
	ID          int64           `json:"id"`
	ProjectID   int             `json:"project_id"`
	Title       string          `json:"title"`
	Content     string          `json:"content,omitempty"`
	Created     *time.Time      `json:"created,omitempty"`
	Updated     *time.Time      `json:"updated,omitempty"`
	CreatedUser *domain.UserRef `json:"created_user,omitempty"`
}

// DocumentDigestSummary は Document Digest の決定論的サマリー（spec §13.5 summary）。
type DocumentDigestSummary struct {
	Headline        string `json:"headline"`
	AttachmentCount int    `json:"attachment_count"`
	HasContent      bool   `json:"has_content"`
	ContentLength   int    `json:"content_length"`
}

// Build は指定ドキュメント ID のダイジェストを構築する。
// 必須データ（ドキュメント）の取得に失敗した場合はエラーを返す。
// オプションデータ（プロジェクト・添付ファイル）の取得失敗は warning として記録し、
// 部分成功として DigestEnvelope を返す（spec §13.5 / partial success behavior）。
//
// 注意: Backlog API の GetProject は projectKey(string) を引数に取るが、
// domain.Document には ProjectID(int) しか含まれない。そのため ListProjects で全件取得し
// ID マッチでプロジェクト情報を解決する。
func (b *DefaultDocumentDigestBuilder) Build(ctx context.Context, documentID int64, opt DocumentDigestOptions) (*domain.DigestEnvelope, error) {
	var warnings []domain.Warning

	// 1. ドキュメント取得（必須）
	doc, err := b.client.GetDocument(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("GetDocument(%d): %w", documentID, err)
	}

	// 2. プロジェクト取得（オプション）
	// Document には ProjectID(int) しかないため ListProjects で ID マッチ
	dp := DigestProject{}
	projects, err := b.client.ListProjects(ctx)
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "project_fetch_failed",
			Message:   fmt.Sprintf("プロジェクト一覧の取得に失敗しました: %v", err),
			Component: "project",
			Retryable: true,
		})
	} else {
		matched := false
		for _, p := range projects {
			if p.ID == doc.ProjectID {
				dp = DigestProject{
					ID:   p.ID,
					Key:  p.ProjectKey,
					Name: p.Name,
				}
				matched = true
				break
			}
		}
		if !matched {
			warnings = append(warnings, domain.Warning{
				Code:      "project_not_matched",
				Message:   fmt.Sprintf("プロジェクト ID %d に対応するプロジェクトが見つかりませんでした", doc.ProjectID),
				Component: "project",
				Retryable: false,
			})
		}
	}

	// 3. 添付ファイル取得（オプション）
	var attachments []domain.Attachment
	atts, err := b.client.ListDocumentAttachments(ctx, documentID)
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "attachments_fetch_failed",
			Message:   fmt.Sprintf("添付ファイル一覧の取得に失敗しました: %v", err),
			Component: "attachments",
			Retryable: true,
		})
		attachments = []domain.Attachment{}
	} else if atts != nil {
		attachments = atts
	} else {
		attachments = []domain.Attachment{}
	}

	// 4. DigestDocumentDetail 組み立て
	dd := buildDigestDocumentDetail(doc)

	// 5. DocumentDigestSummary 組み立て（決定論的）
	summary := buildDocumentDigestSummary(doc, len(attachments))

	// 6. LLMHints 組み立て
	hints := buildDocumentLLMHints(doc, dp)

	digestData := &DocumentDigest{
		Document:    dd,
		Project:     dp,
		Attachments: attachments,
		Summary:     summary,
		LLMHints:    hints,
	}

	if warnings == nil {
		warnings = []domain.Warning{}
	}

	envelope := &domain.DigestEnvelope{
		SchemaVersion: "1",
		Resource:      "document",
		GeneratedAt:   time.Now().UTC(),
		Profile:       b.profile,
		Space:         b.space,
		BaseURL:       b.baseURL,
		Warnings:      warnings,
		Digest:        digestData,
	}

	return envelope, nil
}

// buildDigestDocumentDetail は domain.Document から DigestDocumentDetail を構築する。
func buildDigestDocumentDetail(doc *domain.Document) DigestDocumentDetail {
	dd := DigestDocumentDetail{
		ID:        doc.ID,
		ProjectID: doc.ProjectID,
		Title:     doc.Title,
		Content:   doc.Content,
		Created:   doc.Created,
		Updated:   doc.Updated,
	}

	if doc.CreatedUser != nil {
		dd.CreatedUser = &domain.UserRef{
			ID:   doc.CreatedUser.ID,
			Name: doc.CreatedUser.Name,
		}
	}

	return dd
}

// buildDocumentDigestSummary は決定論的ドキュメントサマリーを構築する（spec §13.5 summary）。
func buildDocumentDigestSummary(doc *domain.Document, attachmentCount int) DocumentDigestSummary {
	headline := fmt.Sprintf("ドキュメント「%s」（ID: %d）", doc.Title, doc.ID)

	return DocumentDigestSummary{
		Headline:        headline,
		AttachmentCount: attachmentCount,
		HasContent:      doc.Content != "",
		ContentLength:   len([]rune(doc.Content)),
	}
}

// buildDocumentLLMHints は LLM ヒントを構築する（spec §13.5 llm_hints）。
func buildDocumentLLMHints(doc *domain.Document, project DigestProject) DigestLLMHints {
	entities := []string{fmt.Sprintf("document:%d", doc.ID), doc.Title}
	if project.Key != "" {
		entities = append(entities, project.Key)
	}

	return DigestLLMHints{
		PrimaryEntities:      entities,
		OpenQuestions:        []string{},
		SuggestedNextActions: []string{},
	}
}

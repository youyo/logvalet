package digest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	"golang.org/x/sync/errgroup"
)

// BaseDigestBuilder は全 DigestBuilder に共通するフィールドと helper を提供する
type BaseDigestBuilder struct {
	client  backlog.Client
	profile string
	space   string
	baseURL string
}

// newEnvelope は DigestEnvelope を組み立てる共通 helper
func (b *BaseDigestBuilder) newEnvelope(resource string, digest any, warnings []domain.Warning) *domain.DigestEnvelope {
	if warnings == nil {
		warnings = []domain.Warning{}
	}
	return &domain.DigestEnvelope{
		SchemaVersion: "1",
		Resource:      resource,
		GeneratedAt:   time.Now().UTC(),
		Profile:       b.profile,
		Space:         b.space,
		BaseURL:       b.baseURL,
		Warnings:      warnings,
		Digest:        digest,
	}
}

// toUserRef は domain.User を domain.UserRef に変換する（nil 安全）
func toUserRef(user *domain.User) *domain.UserRef {
	if user == nil {
		return nil
	}
	return &domain.UserRef{ID: user.ID, Name: user.Name}
}

// extractProjectKey は issueKey（例: "PROJ-123"）からプロジェクトキー（"PROJ"）を抽出する。
func extractProjectKey(issueKey string) string {
	for i, c := range issueKey {
		if c == '-' {
			return issueKey[:i]
		}
	}
	return issueKey
}

// fetchProjectMeta はプロジェクトのメタ情報を errgroup で並行取得する
// 各 API 呼び出しが独立しているため並行化する
func fetchProjectMeta(ctx context.Context, client backlog.Client, projectKey string) (DigestMeta, []domain.Warning) {
	meta := DigestMeta{
		Statuses:     []domain.Status{},
		Categories:   []domain.Category{},
		Versions:     []domain.Version{},
		CustomFields: []domain.CustomFieldDefinition{},
	}
	var mu sync.Mutex
	var warnings []domain.Warning

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		statuses, err := client.ListProjectStatuses(gctx, projectKey)
		if err != nil {
			mu.Lock()
			warnings = append(warnings, domain.Warning{
				Code:      "statuses_fetch_failed",
				Message:   fmt.Sprintf("ステータス一覧の取得に失敗しました: %v", err),
				Component: "meta.statuses",
				Retryable: true,
			})
			mu.Unlock()
			return nil // 部分失敗は warning に留める
		}
		if statuses != nil {
			mu.Lock()
			meta.Statuses = statuses
			mu.Unlock()
		}
		return nil
	})

	g.Go(func() error {
		categories, err := client.ListProjectCategories(gctx, projectKey)
		if err != nil {
			mu.Lock()
			warnings = append(warnings, domain.Warning{
				Code:      "categories_fetch_failed",
				Message:   fmt.Sprintf("カテゴリ一覧の取得に失敗しました: %v", err),
				Component: "meta.categories",
				Retryable: true,
			})
			mu.Unlock()
			return nil
		}
		if categories != nil {
			mu.Lock()
			meta.Categories = categories
			mu.Unlock()
		}
		return nil
	})

	g.Go(func() error {
		versions, err := client.ListProjectVersions(gctx, projectKey)
		if err != nil {
			mu.Lock()
			warnings = append(warnings, domain.Warning{
				Code:      "versions_fetch_failed",
				Message:   fmt.Sprintf("バージョン一覧の取得に失敗しました: %v", err),
				Component: "meta.versions",
				Retryable: true,
			})
			mu.Unlock()
			return nil
		}
		if versions != nil {
			mu.Lock()
			meta.Versions = versions
			mu.Unlock()
		}
		return nil
	})

	g.Go(func() error {
		customFields, err := client.ListProjectCustomFields(gctx, projectKey)
		if err != nil {
			mu.Lock()
			warnings = append(warnings, domain.Warning{
				Code:      "custom_fields_fetch_failed",
				Message:   fmt.Sprintf("カスタムフィールド一覧の取得に失敗しました: %v", err),
				Component: "meta.custom_fields",
				Retryable: true,
			})
			mu.Unlock()
			return nil
		}
		if customFields != nil {
			mu.Lock()
			meta.CustomFields = customFields
			mu.Unlock()
		}
		return nil
	})

	_ = g.Wait()
	return meta, warnings
}

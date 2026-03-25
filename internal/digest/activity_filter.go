package digest

import (
	"context"
	"fmt"
	"time"
)

const defaultActivityFetchLimit = 10000
const activityPageSize = 100

// FetchActivitiesOptions は FetchActivitiesWithDateFilter のオプション。
type FetchActivitiesOptions struct {
	// Since は取得開始日時（inclusive）。nil の場合は制限なし。
	Since *time.Time
	// Until は取得終了日時（inclusive）。nil の場合は制限なし。
	Until *time.Time
	// Limit は収集する最大件数。0 の場合は defaultActivityFetchLimit (10000) を使用する。
	Limit int
}

// ActivityPageFetcher は1ページ分の activities を取得する関数。
// count はページサイズ、maxID は取得する activity の ID の上限（0 は初回）。
type ActivityPageFetcher func(ctx context.Context, count int, maxID int) ([]interface{}, error)

// FetchActivitiesWithDateFilter は activities をページングしつつ日付でフィルタして全件取得する。
//
// ロジック:
//  1. fetcher(ctx, 100, 0) で最新100件取得（maxID=0 は初回）
//  2. 各 activity の created を確認:
//     - created > until → スキップ（continue）
//     - created < since → ループ終了（break）
//     - 範囲内 → 収集
//  3. len(page) == 100 かつ since に未到達 → 次ページ取得（maxID = 最後の activity の ID - 1）
//  4. 上限件数（デフォルト 10,000）で打ち切り
func FetchActivitiesWithDateFilter(ctx context.Context, fetcher ActivityPageFetcher, opts FetchActivitiesOptions) ([]interface{}, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultActivityFetchLimit
	}

	result := make([]interface{}, 0)
	maxID := 0

	for {
		// コンテキストのキャンセルチェック
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		page, err := fetcher(ctx, activityPageSize, maxID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch activities (maxID=%d): %w", maxID, err)
		}
		if len(page) == 0 {
			break
		}

		reachedSince := false
		for _, item := range page {
			created, err := extractActivityCreated(item)
			if err != nil {
				// created フィールドが取得できない場合はスキップ
				continue
			}

			// until より新しい → スキップ
			if opts.Until != nil && created.After(*opts.Until) {
				continue
			}

			// since より古い → ループ終了
			if opts.Since != nil && created.Before(*opts.Since) {
				reachedSince = true
				break
			}

			result = append(result, item)

			// 上限チェック
			if len(result) >= limit {
				return result, nil
			}
		}

		// since に到達した場合 or ページが100件未満 → 次ページなし
		if reachedSince || len(page) < activityPageSize {
			break
		}

		// 次ページの maxID を設定（最後の activity の ID - 1）
		lastID, err := extractActivityID(page[len(page)-1])
		if err != nil {
			// ID が取得できない場合は終了
			break
		}
		maxID = lastID - 1
		if maxID <= 0 {
			break
		}
	}

	return result, nil
}

// extractActivityCreated は activity（interface{}）から created フィールドを time.Time として取得する。
func extractActivityCreated(item interface{}) (time.Time, error) {
	m, ok := item.(map[string]interface{})
	if !ok {
		return time.Time{}, fmt.Errorf("activity is not map[string]interface{}: %T", item)
	}
	createdRaw, ok := m["created"]
	if !ok {
		return time.Time{}, fmt.Errorf("missing created field")
	}

	switch v := createdRaw.(type) {
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			// フォールバック: RFC3339Nano
			t, err = time.Parse(time.RFC3339Nano, v)
			if err != nil {
				return time.Time{}, fmt.Errorf("failed to parse created %q: %w", v, err)
			}
		}
		return t, nil
	case time.Time:
		return v, nil
	case *time.Time:
		if v == nil {
			return time.Time{}, fmt.Errorf("created is nil")
		}
		return *v, nil
	default:
		return time.Time{}, fmt.Errorf("unsupported type for created: %T", createdRaw)
	}
}

// extractActivityID は activity（interface{}）から id フィールドを int として取得する。
func extractActivityID(item interface{}) (int, error) {
	m, ok := item.(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("activity is not map[string]interface{}: %T", item)
	}
	idRaw, ok := m["id"]
	if !ok {
		return 0, fmt.Errorf("missing id field")
	}
	switch v := idRaw.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("unsupported type for id: %T", idRaw)
	}
}

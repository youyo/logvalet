package backlog_test

import (
	"context"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// makeActivities は指定された日付の Activity スライスを生成するヘルパー。
// IDs は newest-first (降順) で割り当てる。
func makeActivities(dates []time.Time, baseID int64) []domain.Activity {
	acts := make([]domain.Activity, len(dates))
	for i, d := range dates {
		d := d // ループ変数コピー
		acts[i] = domain.Activity{
			ID:      baseID - int64(i),
			Type:    1,
			Created: &d,
		}
	}
	return acts
}

// TestFetchUserActivities_Pagination は since/until 指定時に2ページにわたってアクティビティが
// 取得されることを確認する。
// バッチサイズ100件 × 2ページ構成。最初のページは満杯（100件）なので次ページに進む。
func TestFetchUserActivities_Pagination(t *testing.T) {
	// 5月のアクティビティ100件（第1ページ）
	may2026 := make([]time.Time, 100)
	for i := range may2026 {
		may2026[i] = time.Date(2026, 5, 31-i/4, 12, 0, 0, 0, time.UTC)
	}
	page1 := makeActivities(may2026, 200)

	// 4月のアクティビティ50件（第2ページ）
	apr2026 := make([]time.Time, 50)
	for i := range apr2026 {
		apr2026[i] = time.Date(2026, 4, 30-i/2, 12, 0, 0, 0, time.UTC)
	}
	page2 := makeActivities(apr2026, 100)

	mock := backlog.NewMockClient()
	callCount := 0
	mock.ListUserActivitiesFunc = func(ctx context.Context, userID string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if opt.MaxId == 0 {
			return page1, nil
		}
		return page2, nil
	}

	since := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 5, 31, 23, 59, 59, 0, time.UTC)

	got, err := backlog.FetchUserActivities(context.Background(), mock, "user1", &since, &until, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2回以上 ListUserActivities が呼ばれること（ページネーションの確認）
	if callCount < 2 {
		t.Errorf("expected at least 2 API calls for pagination, got %d", callCount)
	}

	// 4月のアクティビティが含まれること
	var aprCount int
	for _, a := range got {
		if a.Created != nil && a.Created.Month() == time.April {
			aprCount++
		}
	}
	if aprCount == 0 {
		t.Error("expected April activities to be included via pagination, but got none")
	}
}

// TestFetchUserActivities_LimitWithNoDateFilter は since/until 未指定時に limit 件で停止することを確認する。
func TestFetchUserActivities_LimitWithNoDateFilter(t *testing.T) {
	// モックは常に100件を返す（opt.Count を無視）
	now := time.Now()
	bigBatch := make([]domain.Activity, 100)
	for i := range bigBatch {
		t := now.Add(-time.Duration(i) * time.Hour)
		bigBatch[i] = domain.Activity{
			ID:      int64(1000 - i),
			Type:    1,
			Created: &t,
		}
	}

	mock := backlog.NewMockClient()
	mock.ListUserActivitiesFunc = func(ctx context.Context, userID string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		return bigBatch, nil
	}

	limit := 10
	got, err := backlog.FetchUserActivities(context.Background(), mock, "user1", nil, nil, limit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != limit {
		t.Errorf("expected %d activities (limit), got %d", limit, len(got))
	}
}

// TestFetchUserActivities_SinceFilter は since より古いアクティビティが除外されることを確認する。
func TestFetchUserActivities_SinceFilter(t *testing.T) {
	since := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	// 4月と5月のアクティビティを混在させる（少数、全件1ページに収まる）
	dates := []time.Time{
		time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC), // since 以降: 含まれる
		time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),  // since 当日: 含まれる
		time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC), // since より前: 除外される
		time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),  // since より前: 除外される
	}
	// 4件は batchSize=100 より少ないので1ページで終了する
	activities := makeActivities(dates, 10)

	mock := backlog.NewMockClient()
	mock.ListUserActivitiesFunc = func(ctx context.Context, userID string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		return activities, nil
	}

	got, err := backlog.FetchUserActivities(context.Background(), mock, "user1", &since, nil, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// since 以前のアクティビティが除外されること
	for _, a := range got {
		if a.Created != nil && a.Created.Before(since) {
			t.Errorf("activity created at %v should have been filtered out (since=%v)", a.Created, since)
		}
	}

	// 期待される数: 2件（5月10日と5月1日）
	if len(got) != 2 {
		t.Errorf("expected 2 activities after since filter, got %d", len(got))
	}
}

// TestFetchUserActivities_UntilFilter は until より新しいアクティビティがスキップされることを確認する。
func TestFetchUserActivities_UntilFilter(t *testing.T) {
	until := time.Date(2026, 4, 30, 23, 59, 59, 0, time.UTC)

	dates := []time.Time{
		time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),   // until より後: スキップ
		time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC),  // until 以前: 含まれる
		time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),   // until 以前: 含まれる
	}
	activities := makeActivities(dates, 10)

	mock := backlog.NewMockClient()
	mock.ListUserActivitiesFunc = func(ctx context.Context, userID string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		return activities, nil
	}

	got, err := backlog.FetchUserActivities(context.Background(), mock, "user1", nil, &until, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// until より後のアクティビティが含まれないこと
	for _, a := range got {
		if a.Created != nil && a.Created.After(until) {
			t.Errorf("activity created at %v should have been filtered out (until=%v)", a.Created, until)
		}
	}

	// 期待される数: 2件（4月30日と4月1日）
	if len(got) != 2 {
		t.Errorf("expected 2 activities after until filter, got %d", len(got))
	}
}

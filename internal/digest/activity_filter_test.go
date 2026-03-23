package digest_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/digest"
)

// makeActivity はテスト用 activity データを生成する。
// created は RFC3339 形式の文字列を指定する。
func makeActivity(id int, created string) interface{} {
	return map[string]interface{}{
		"id":      float64(id),
		"created": created,
		"type":    float64(1),
	}
}

// makeActivities は連続した id と created を持つ activity スライスを生成する。
// baseTime から offset 分ずつ引いた時刻を created に使用する。
func makeActivities(startID int, count int, baseTime time.Time, step time.Duration) []interface{} {
	result := make([]interface{}, count)
	for i := 0; i < count; i++ {
		t := baseTime.Add(-time.Duration(i) * step)
		result[i] = makeActivity(startID-i, t.Format(time.RFC3339))
	}
	return result
}

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(fmt.Sprintf("invalid time: %s", s))
	}
	return t
}

// TestFetchActivitiesWithDateFilter_normal は10件のうち since〜until 範囲内の3件を取得するケース。
func TestFetchActivitiesWithDateFilter_normal(t *testing.T) {
	since := mustTime("2026-03-10T00:00:00Z")
	until := mustTime("2026-03-20T23:59:59Z")

	// id 降順（新しい順）: 10〜1
	// created: 2026-03-25, 24, 23, 22, 21, 20, 15, 10, 05, 01
	activities := []interface{}{
		makeActivity(10, "2026-03-25T00:00:00Z"), // > until: skip
		makeActivity(9, "2026-03-24T00:00:00Z"),  // > until: skip
		makeActivity(8, "2026-03-23T00:00:00Z"),  // > until: skip
		makeActivity(7, "2026-03-22T00:00:00Z"),  // > until: skip
		makeActivity(6, "2026-03-21T00:00:00Z"),  // > until: skip
		makeActivity(5, "2026-03-20T00:00:00Z"),  // in range
		makeActivity(4, "2026-03-15T00:00:00Z"),  // in range
		makeActivity(3, "2026-03-10T00:00:00Z"),  // in range
		makeActivity(2, "2026-03-05T00:00:00Z"),  // < since: stop
		makeActivity(1, "2026-03-01T00:00:00Z"),  // < since: stop
	}

	callCount := 0
	fetcher := func(ctx context.Context, count int, maxID int) ([]interface{}, error) {
		callCount++
		if callCount > 1 {
			return nil, nil
		}
		return activities, nil
	}

	opts := digest.FetchActivitiesOptions{
		Since: &since,
		Until: &until,
	}
	result, err := digest.FetchActivitiesWithDateFilter(context.Background(), fetcher, opts)
	if err != nil {
		t.Fatalf("FetchActivitiesWithDateFilter() エラー: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("len(result) = %d, want 3", len(result))
	}
}

// TestFetchActivitiesWithDateFilter_skipFuture は until より新しいものをスキップするケース。
func TestFetchActivitiesWithDateFilter_skipFuture(t *testing.T) {
	since := mustTime("2026-03-01T00:00:00Z")
	until := mustTime("2026-03-10T00:00:00Z")

	activities := []interface{}{
		makeActivity(5, "2026-03-20T00:00:00Z"), // > until: skip
		makeActivity(4, "2026-03-15T00:00:00Z"), // > until: skip
		makeActivity(3, "2026-03-10T00:00:00Z"), // == until: in range
		makeActivity(2, "2026-03-05T00:00:00Z"), // in range
		makeActivity(1, "2026-03-01T00:00:00Z"), // == since: in range
	}

	fetcher := func(ctx context.Context, count int, maxID int) ([]interface{}, error) {
		return activities, nil
	}

	opts := digest.FetchActivitiesOptions{
		Since: &since,
		Until: &until,
	}
	result, err := digest.FetchActivitiesWithDateFilter(context.Background(), fetcher, opts)
	if err != nil {
		t.Fatalf("FetchActivitiesWithDateFilter() エラー: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("len(result) = %d, want 3 (future items should be skipped)", len(result))
	}
}

// TestFetchActivitiesWithDateFilter_stopPast は since より古いもので終了するケース。
func TestFetchActivitiesWithDateFilter_stopPast(t *testing.T) {
	since := mustTime("2026-03-10T00:00:00Z")
	until := mustTime("2026-03-20T00:00:00Z")

	activities := []interface{}{
		makeActivity(5, "2026-03-18T00:00:00Z"), // in range
		makeActivity(4, "2026-03-15T00:00:00Z"), // in range
		makeActivity(3, "2026-03-09T00:00:00Z"), // < since: stop here
		makeActivity(2, "2026-03-05T00:00:00Z"), // < since: not reached
		makeActivity(1, "2026-03-01T00:00:00Z"), // < since: not reached
	}

	fetcher := func(ctx context.Context, count int, maxID int) ([]interface{}, error) {
		return activities, nil
	}

	opts := digest.FetchActivitiesOptions{
		Since: &since,
		Until: &until,
	}
	result, err := digest.FetchActivitiesWithDateFilter(context.Background(), fetcher, opts)
	if err != nil {
		t.Fatalf("FetchActivitiesWithDateFilter() エラー: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("len(result) = %d, want 2 (loop should stop at past items)", len(result))
	}
}

// TestFetchActivitiesWithDateFilter_multiPage は2ページにまたがる取得のケース。
func TestFetchActivitiesWithDateFilter_multiPage(t *testing.T) {
	since := mustTime("2026-03-01T00:00:00Z")
	until := mustTime("2026-03-31T00:00:00Z")

	// ページ1: 100件（全て範囲内だが since には未到達）
	page1 := makeActivities(200, 100, mustTime("2026-03-31T00:00:00Z"), 6*time.Hour)
	// ページ2: 50件（一部 since 未満）
	page2 := makeActivities(100, 50, mustTime("2026-03-06T00:00:00Z"), 24*time.Hour)

	callCount := 0
	var receivedMaxIDs []int

	fetcher := func(ctx context.Context, count int, maxID int) ([]interface{}, error) {
		callCount++
		receivedMaxIDs = append(receivedMaxIDs, maxID)
		if callCount == 1 {
			return page1, nil
		}
		return page2, nil
	}

	opts := digest.FetchActivitiesOptions{
		Since: &since,
		Until: &until,
	}
	result, err := digest.FetchActivitiesWithDateFilter(context.Background(), fetcher, opts)
	if err != nil {
		t.Fatalf("FetchActivitiesWithDateFilter() エラー: %v", err)
	}

	// ページ1の100件 + ページ2の一部が含まれるはず
	if len(result) == 0 {
		t.Error("result が空（複数ページから収集されるはず）")
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2", callCount)
	}
	// 2回目の呼び出しは maxID > 0 のはず
	if len(receivedMaxIDs) >= 2 && receivedMaxIDs[1] == 0 {
		t.Error("2回目の fetcher 呼び出しの maxID が 0 のまま（ページング未実装）")
	}
}

// TestFetchActivitiesWithDateFilter_maxLimit は上限で打ち切るケース。
func TestFetchActivitiesWithDateFilter_maxLimit(t *testing.T) {
	since := mustTime("2020-01-01T00:00:00Z")
	until := mustTime("2030-12-31T00:00:00Z")

	callCount := 0
	fetcher := func(ctx context.Context, count int, maxID int) ([]interface{}, error) {
		callCount++
		// 常に100件返す（無限に続く）
		baseTime := mustTime("2025-01-01T00:00:00Z").Add(-time.Duration(callCount-1) * 100 * 24 * time.Hour)
		return makeActivities(callCount*100, 100, baseTime, time.Hour), nil
	}

	limit := 250
	opts := digest.FetchActivitiesOptions{
		Since: &since,
		Until: &until,
		Limit: limit,
	}
	result, err := digest.FetchActivitiesWithDateFilter(context.Background(), fetcher, opts)
	if err != nil {
		t.Fatalf("FetchActivitiesWithDateFilter() エラー: %v", err)
	}
	if len(result) > limit {
		t.Errorf("len(result) = %d, want <= %d (limit not respected)", len(result), limit)
	}
}

// TestFetchActivitiesWithDateFilter_empty は0件の場合に空スライスを返すケース。
func TestFetchActivitiesWithDateFilter_empty(t *testing.T) {
	since := mustTime("2026-03-01T00:00:00Z")
	until := mustTime("2026-03-31T00:00:00Z")

	fetcher := func(ctx context.Context, count int, maxID int) ([]interface{}, error) {
		return []interface{}{}, nil
	}

	opts := digest.FetchActivitiesOptions{
		Since: &since,
		Until: &until,
	}
	result, err := digest.FetchActivitiesWithDateFilter(context.Background(), fetcher, opts)
	if err != nil {
		t.Fatalf("FetchActivitiesWithDateFilter() エラー: %v", err)
	}
	if result == nil {
		t.Error("result が nil（空スライスを期待）")
	}
	if len(result) != 0 {
		t.Errorf("len(result) = %d, want 0", len(result))
	}
}

// TestFetchActivitiesWithDateFilter_nilSinceUntil は since/until が nil の場合に全件返すケース。
func TestFetchActivitiesWithDateFilter_nilSinceUntil(t *testing.T) {
	activities := []interface{}{
		makeActivity(3, "2026-03-20T00:00:00Z"),
		makeActivity(2, "2026-03-10T00:00:00Z"),
		makeActivity(1, "2026-03-01T00:00:00Z"),
	}

	fetcher := func(ctx context.Context, count int, maxID int) ([]interface{}, error) {
		return activities, nil
	}

	opts := digest.FetchActivitiesOptions{
		Since: nil,
		Until: nil,
	}
	result, err := digest.FetchActivitiesWithDateFilter(context.Background(), fetcher, opts)
	if err != nil {
		t.Fatalf("FetchActivitiesWithDateFilter() エラー: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("len(result) = %d, want 3 (nil since/until should return all)", len(result))
	}
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	"github.com/youyo/logvalet/internal/render"
)

func makeActivity(id int64, created time.Time) domain.Activity {
	return domain.Activity{
		ID:      id,
		Type:    1,
		Created: &created,
	}
}

func newJSONRenderer(t *testing.T) render.Renderer {
	t.Helper()
	r, err := render.NewRenderer("json", false, "")
	if err != nil {
		t.Fatalf("render.NewRenderer エラー: %v", err)
	}
	return r
}

// TestUserActivityCmd_run_NoFilter_RespectsLimit は Since/Until 未指定時に Limit が適用されることを確認する。
func TestUserActivityCmd_run_NoFilter_RespectsLimit(t *testing.T) {
	base := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	all := []domain.Activity{
		makeActivity(3, base.Add(2*time.Hour)),
		makeActivity(2, base.Add(time.Hour)),
		makeActivity(1, base),
	}

	mc := backlog.NewMockClient()
	mc.ListUserActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		return all, nil
	}

	var out bytes.Buffer
	cmd := &UserActivityCmd{UserID: "1234"}
	cmd.Limit = 2
	if err := cmd.run(context.Background(), mc, newJSONRenderer(t), &out); err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	var result []domain.Activity
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("JSON パースエラー: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("アクティビティ数: 期待 2, 実際 %d", len(result))
	}
}

// TestUserActivityCmd_run_Since_ExcludesOlder は Since より古いアクティビティが除外されることを確認する。
func TestUserActivityCmd_run_Since_ExcludesOlder(t *testing.T) {
	base := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	// newest first
	activities := []domain.Activity{
		makeActivity(4, base.Add(2*time.Hour)),  // in range
		makeActivity(3, base.Add(time.Hour)),    // in range
		makeActivity(2, base),                   // on boundary (Since==base) → include
		makeActivity(1, base.Add(-time.Hour)),   // older than Since → exclude
	}

	mc := backlog.NewMockClient()
	mc.ListUserActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		return activities, nil
	}

	var out bytes.Buffer
	cmd := &UserActivityCmd{UserID: "1234"}
	cmd.Since = base.Format(time.RFC3339)
	cmd.Limit = 100
	if err := cmd.run(context.Background(), mc, newJSONRenderer(t), &out); err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	var result []domain.Activity
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("JSON パースエラー: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("アクティビティ数: 期待 3 (Since以降), 実際 %d", len(result))
	}
}

// TestUserActivityCmd_run_Until_SkipsNewer は Until より新しいアクティビティがスキップされることを確認する。
func TestUserActivityCmd_run_Until_SkipsNewer(t *testing.T) {
	base := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	activities := []domain.Activity{
		makeActivity(4, base.Add(2*time.Hour)), // after Until → skip
		makeActivity(3, base.Add(time.Hour)),   // after Until → skip
		makeActivity(2, base),                  // on boundary (Until==base) → include
		makeActivity(1, base.Add(-time.Hour)),  // before Until → include
	}

	mc := backlog.NewMockClient()
	mc.ListUserActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		return activities, nil
	}

	var out bytes.Buffer
	cmd := &UserActivityCmd{UserID: "1234"}
	cmd.Until = base.Format(time.RFC3339)
	cmd.Limit = 100
	if err := cmd.run(context.Background(), mc, newJSONRenderer(t), &out); err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	var result []domain.Activity
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("JSON パースエラー: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("アクティビティ数: 期待 2 (Until以前), 実際 %d", len(result))
	}
}

// TestUserActivityCmd_run_SinceAndUntil_FiltersRange は Since〜Until の範囲外が除外されることを確認する。
func TestUserActivityCmd_run_SinceAndUntil_FiltersRange(t *testing.T) {
	since := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 30, 23, 59, 59, 0, time.UTC)

	activities := []domain.Activity{
		makeActivity(5, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)),  // after Until
		makeActivity(4, time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)), // in range
		makeActivity(3, time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)), // in range
		makeActivity(2, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)),   // on Since boundary → include
		makeActivity(1, time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC)), // before Since
	}

	mc := backlog.NewMockClient()
	mc.ListUserActivitiesFunc = func(_ context.Context, _ string, _ backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		return activities, nil
	}

	var out bytes.Buffer
	cmd := &UserActivityCmd{UserID: "1234"}
	cmd.Since = since.Format(time.RFC3339)
	cmd.Until = until.Format(time.RFC3339)
	cmd.Limit = 100
	if err := cmd.run(context.Background(), mc, newJSONRenderer(t), &out); err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	var result []domain.Activity
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("JSON パースエラー: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("アクティビティ数: 期待 3 (4月分), 実際 %d", len(result))
	}
}

// TestUserActivityCmd_run_Pagination_MakesSecondCall は100件返された場合に2回目のAPIコールが行われることを確認する。
func TestUserActivityCmd_run_Pagination_MakesSecondCall(t *testing.T) {
	since := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	// 1ページ目: 100件（すべてSince以降）
	firstBatch := make([]domain.Activity, 100)
	for i := range firstBatch {
		id := int64(200 - i)
		ts := since.Add(time.Duration(100-i) * time.Hour)
		firstBatch[i] = makeActivity(id, ts)
	}
	// 2ページ目: Since境界付近の2件 + Since以前の1件（停止条件）
	secondBatch := []domain.Activity{
		makeActivity(99, since.Add(time.Hour)),  // in range
		makeActivity(98, since),                  // in range (boundary)
		makeActivity(97, since.Add(-time.Hour)), // before Since → stop
	}

	callCount := 0
	mc := backlog.NewMockClient()
	mc.ListUserActivitiesFunc = func(_ context.Context, _ string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if opt.MaxId == 0 {
			return firstBatch, nil
		}
		return secondBatch, nil
	}

	var out bytes.Buffer
	cmd := &UserActivityCmd{UserID: "1234"}
	cmd.Since = since.Format(time.RFC3339)
	cmd.Limit = 100
	if err := cmd.run(context.Background(), mc, newJSONRenderer(t), &out); err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	if callCount < 2 {
		t.Errorf("APIコール数: 期待 2以上 (ページネーション), 実際 %d", callCount)
	}

	var result []domain.Activity
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("JSON パースエラー: %v", err)
	}
	// 1ページ目100件 + 2ページ目の範囲内2件
	if len(result) != 102 {
		t.Errorf("アクティビティ数: 期待 102, 実際 %d", len(result))
	}
}

// TestUserActivityCmd_run_InvalidSince は無効な Since フォーマットでエラーが返されることを確認する。
func TestUserActivityCmd_run_InvalidSince(t *testing.T) {
	mc := backlog.NewMockClient()
	var out bytes.Buffer
	cmd := &UserActivityCmd{UserID: "1234"}
	cmd.Since = "not-a-date"
	cmd.Limit = 100
	err := cmd.run(context.Background(), mc, newJSONRenderer(t), &out)
	if err == nil {
		t.Fatal("無効な Since でエラーが返されなかった")
	}
}

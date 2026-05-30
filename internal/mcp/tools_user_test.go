package mcp_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	mcpinternal "github.com/youyo/logvalet/internal/mcp"
)

// ===== B1: logvalet_user_me =====

// TestUserMe_Normal は GetMyself が呼ばれてユーザーが返ることを確認する。
func TestUserMe_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return &domain.User{ID: 42, UserID: "testuser", Name: "テストユーザー"}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_user_me", map[string]any{})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if mock.GetCallCount("GetMyself") != 1 {
		t.Errorf("expected GetMyself called 1 time, got %d", mock.GetCallCount("GetMyself"))
	}
}

// TestUserMe_Error は GetMyself がエラーを返した場合に IsError=true になることを確認する。
func TestUserMe_Error(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return nil, backlog.ErrNotFound
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_user_me", map[string]any{})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// ===== B2: logvalet_user_activity =====

// TestUserActivity_Normal は user_id と limit が正しく渡されることを確認する。
func TestUserActivity_Normal(t *testing.T) {
	mock := backlog.NewMockClient()
	var capturedUserID string
	var capturedOpt backlog.ListUserActivitiesOptions
	now := time.Now()
	mock.ListUserActivitiesFunc = func(ctx context.Context, userID string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		capturedUserID = userID
		capturedOpt = opt
		return []domain.Activity{{ID: 1, Type: 1, Created: &now}}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_user_activity", map[string]any{
		"user_id": "12345",
		"limit":   float64(10),
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if capturedUserID != "12345" {
		t.Errorf("userID = %q, want %q", capturedUserID, "12345")
	}
	if capturedOpt.Count != 10 {
		t.Errorf("Count = %d, want 10", capturedOpt.Count)
	}
}

// TestUserActivity_Me は user_id="me" のとき GetMyself → ListUserActivities が呼ばれることを確認する。
func TestUserActivity_Me(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return &domain.User{ID: 99, UserID: "me_user", Name: "自分"}, nil
	}
	var capturedUserID string
	now := time.Now()
	mock.ListUserActivitiesFunc = func(ctx context.Context, userID string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		capturedUserID = userID
		return []domain.Activity{{ID: 2, Type: 2, Created: &now}}, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_user_activity", map[string]any{"user_id": "me"})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if mock.GetCallCount("GetMyself") != 1 {
		t.Errorf("expected GetMyself called 1 time, got %d", mock.GetCallCount("GetMyself"))
	}
	if capturedUserID != "99" {
		t.Errorf("userID = %q, want %q", capturedUserID, "99")
	}
}

// TestUserActivity_MissingUserID は user_id 未指定で IsError=true になることを確認する。
func TestUserActivity_MissingUserID(t *testing.T) {
	mock := backlog.NewMockClient()

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_user_activity", map[string]any{})

	if !result.IsError {
		t.Fatal("expected tool error but got none")
	}
}

// ===== B2-pagination: logvalet_user_activity ページネーション =====

// TestUserActivity_SincePaginates は最新20件が全部5月のとき
// since=4月 指定でページネーションして4月のデータが返ることを確認する。
// 現在のハンドラーは単一バッチ(Count=limit)しか取得しないためこのテストは失敗する（Red）。
func TestUserActivity_SincePaginates(t *testing.T) {
	// 第1ページ: 5月のアクティビティ100件（満杯バッチ = ページネーション継続の条件）
	may2026 := make([]domain.Activity, 100)
	for i := range may2026 {
		d := time.Date(2026, 5, 31-i/4, 12, 0, 0, 0, time.UTC)
		may2026[i] = domain.Activity{
			ID:      int64(200 - i),
			Type:    1,
			Created: &d,
		}
	}

	// 第2ページ: 4月のアクティビティ10件
	apr2026 := make([]domain.Activity, 10)
	for i := range apr2026 {
		d := time.Date(2026, 4, 30-i, 12, 0, 0, 0, time.UTC)
		apr2026[i] = domain.Activity{
			ID:      int64(100 - i),
			Type:    1,
			Created: &d,
		}
	}

	mock := backlog.NewMockClient()
	callCount := 0
	mock.ListUserActivitiesFunc = func(ctx context.Context, userID string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		callCount++
		if opt.MaxId == 0 {
			return may2026, nil
		}
		return apr2026, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_user_activity", map[string]any{
		"user_id": "user1",
		"since":   "2026-04-01",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	// callCount >= 2 でないとページネーションが起きていない（Red: 現在の実装は1回のみ）
	if callCount < 2 {
		t.Errorf("expected at least 2 API calls for pagination (since=April), got %d — handler does not paginate", callCount)
	}
}

// TestUserActivity_UntilEndOfDay は until=2026-04-30 指定時に
// 4月30日の昼に作成されたアクティビティが含まれることを確認する。
// 現在の実装は until を 2026-04-30T00:00:00 としてフィルタするため
// 4月30日12:00の活動を .After(*until) で除外してしまう（Red）。
func TestUserActivity_UntilEndOfDay(t *testing.T) {
	// 4月30日の昼のアクティビティ（当日の活動）
	midday := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	// 5月1日の活動（until 以後: 除外されるべき）
	may1 := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	activities := []domain.Activity{
		{ID: 2, Type: 1, Created: &may1},   // until より後: 除外されるべき
		{ID: 1, Type: 1, Created: &midday}, // until の当日昼: 含まれるべき
	}

	mock := backlog.NewMockClient()
	mock.ListUserActivitiesFunc = func(ctx context.Context, userID string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		return activities, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_user_activity", map[string]any{
		"user_id": "user1",
		"until":   "2026-04-30",
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected non-empty result content")
	}

	// TextContent から JSON テキストを取得して ID=1 が含まれることを確認
	// 現在の実装は parseDateStr("2026-04-30") → 2026-04-30T00:00:00 として
	// midday.After(2026-04-30T00:00:00) = true で除外してしまうため失敗（Red）
	textContent, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected gomcp.TextContent, got %T", result.Content[0])
	}

	// JSON パースして activities 配列を取得し、ID=1 が含まれることを確認
	var got []domain.Activity
	if err := json.Unmarshal([]byte(textContent.Text), &got); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	foundID1 := false
	foundID2 := false
	for _, a := range got {
		if a.ID == 1 {
			foundID1 = true
		}
		if a.ID == 2 {
			foundID2 = true
		}
	}
	if !foundID1 {
		t.Errorf("expected activity ID=1 (midday Apr 30) to be included (until end-of-day), but it was filtered out — got %d activities", len(got))
	}
	if foundID2 {
		t.Errorf("expected activity ID=2 (May 1) to be excluded (after until=Apr 30), but it was included")
	}
}

// TestUserActivity_LimitWithoutDateFilter は since/until 未指定時に limit 件で停止することを確認する。
// 現在の実装はフィルタなし時にバッチをそのまま返す（limitを超えても truncate しない可能性）ため
// モックが opt.Count を無視して100件返したとき、limit=5 の指定が機能しなければ失敗（Red）。
func TestUserActivity_LimitWithoutDateFilter(t *testing.T) {
	// モックは常に100件を返す（opt.Count を無視する）
	now := time.Now()
	bigBatch := make([]domain.Activity, 100)
	for i := range bigBatch {
		d := now.Add(-time.Duration(i) * time.Hour)
		bigBatch[i] = domain.Activity{
			ID:      int64(1000 - i),
			Type:    1,
			Created: &d,
		}
	}

	mock := backlog.NewMockClient()
	mock.ListUserActivitiesFunc = func(ctx context.Context, userID string, opt backlog.ListUserActivitiesOptions) ([]domain.Activity, error) {
		// opt.Count を無視して常に100件返す
		return bigBatch, nil
	}

	s := mcpinternal.NewServer(mock, "test", mcpinternal.ServerConfig{})
	result := callTool(t, s, "logvalet_user_activity", map[string]any{
		"user_id": "user1",
		"limit":   float64(5),
	})

	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected non-empty result content")
	}

	textContent, ok := result.Content[0].(gomcp.TextContent)
	if !ok {
		t.Fatalf("expected gomcp.TextContent, got %T", result.Content[0])
	}

	// JSON パースして activities 配列の件数が limit=5 以内であることを確認
	// 現在の実装: フィルタなしの場合 activities をそのまま返す（100件）→ Red
	var got []domain.Activity
	if err := json.Unmarshal([]byte(textContent.Text), &got); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(got) > 5 {
		t.Errorf("expected at most 5 activities (limit=5), got %d — handler does not enforce limit when API returns more", len(got))
	}
}

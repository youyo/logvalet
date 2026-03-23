package cli

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// ---- テスト用ステータス一覧 ----

var testStatuses = []domain.Status{
	{ID: 1, Name: "未対応"},
	{ID: 2, Name: "処理中"},
	{ID: 3, Name: "処理済み"},
	{ID: 4, Name: "完了"},
}

// ---- resolveAssignee テスト ----

// C1: "me" → GetMyself → [42]
func TestResolveAssignee_me(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return &domain.User{ID: 42, Name: "テストユーザー"}, nil
	}
	ids, err := resolveAssignee(context.Background(), "me", mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(ids) != 1 || ids[0] != 42 {
		t.Fatalf("期待 [42], 実際 %v", ids)
	}
}

// C2: 数値文字列 → [99]
func TestResolveAssignee_numeric(t *testing.T) {
	mc := backlog.NewMockClient()
	ids, err := resolveAssignee(context.Background(), "99", mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(ids) != 1 || ids[0] != 99 {
		t.Fatalf("期待 [99], 実際 %v", ids)
	}
}

// C2b: 名前文字列 → ListUsers → case-insensitive 完全一致 → [50]
func TestResolveAssignee_name(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListUsersFunc = func(ctx context.Context) ([]domain.User, error) {
		return []domain.User{
			{ID: 50, Name: "田中太郎"},
			{ID: 51, Name: "鈴木花子"},
		}, nil
	}
	ids, err := resolveAssignee(context.Background(), "田中太郎", mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(ids) != 1 || ids[0] != 50 {
		t.Fatalf("期待 [50], 実際 %v", ids)
	}
}

// E1: 名前に一致しない → エラー
func TestResolveAssignee_name_not_found(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListUsersFunc = func(ctx context.Context) ([]domain.User, error) {
		return []domain.User{
			{ID: 50, Name: "田中太郎"},
		}, nil
	}
	_, err := resolveAssignee(context.Background(), "存在しないユーザー", mc)
	if err == nil {
		t.Fatal("エラーが期待されたが nil")
	}
}

// E1b: 同名ユーザーが複数いる → エラー
func TestResolveAssignee_name_multiple(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListUsersFunc = func(ctx context.Context) ([]domain.User, error) {
		return []domain.User{
			{ID: 50, Name: "田中"},
			{ID: 51, Name: "田中"},
		}, nil
	}
	_, err := resolveAssignee(context.Background(), "田中", mc)
	if err == nil {
		t.Fatal("エラーが期待されたが nil")
	}
}

// E5: GetMyself エラー伝播
func TestResolveAssignee_getMyself_error(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return nil, errors.New("API エラー")
	}
	_, err := resolveAssignee(context.Background(), "me", mc)
	if err == nil {
		t.Fatal("エラーが期待されたが nil")
	}
}

// ---- resolveStatuses テスト ----

// C3: "open" → 完了以外のステータス ID → [1,2,3]
func TestResolveStatuses_open(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return testStatuses, nil
	}
	ids, err := resolveStatuses(context.Background(), "open", []string{"PROJ"}, mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !intSliceEqual(ids, []int{1, 2, 3}) {
		t.Fatalf("期待 [1,2,3], 実際 %v", ids)
	}
}

// C4: 名前 "未対応" → [1]
func TestResolveStatuses_name(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return testStatuses, nil
	}
	ids, err := resolveStatuses(context.Background(), "未対応", []string{"PROJ"}, mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !intSliceEqual(ids, []int{1}) {
		t.Fatalf("期待 [1], 実際 %v", ids)
	}
}

// C5: カンマ区切り "未対応,処理中" → [1,2]
func TestResolveStatuses_comma(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return testStatuses, nil
	}
	ids, err := resolveStatuses(context.Background(), "未対応,処理中", []string{"PROJ"}, mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !intSliceEqual(ids, []int{1, 2}) {
		t.Fatalf("期待 [1,2], 実際 %v", ids)
	}
}

// C6: 数値単体 → [1]
func TestResolveStatuses_numeric(t *testing.T) {
	mc := backlog.NewMockClient()
	ids, err := resolveStatuses(context.Background(), "1", []string{}, mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !intSliceEqual(ids, []int{1}) {
		t.Fatalf("期待 [1], 実際 %v", ids)
	}
}

// E2: "open" + projectKeys=[] → エラー
func TestResolveStatuses_open_no_project(t *testing.T) {
	mc := backlog.NewMockClient()
	_, err := resolveStatuses(context.Background(), "open", []string{}, mc)
	if err == nil {
		t.Fatal("エラーが期待されたが nil")
	}
}

// E3: 名前 + projectKeys=[] → エラー
func TestResolveStatuses_name_no_project(t *testing.T) {
	mc := backlog.NewMockClient()
	_, err := resolveStatuses(context.Background(), "未対応", []string{}, mc)
	if err == nil {
		t.Fatal("エラーが期待されたが nil")
	}
}

// X1: 完了ステータスなし → 全 ID が返る
func TestResolveStatuses_custom_no_closed(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return []domain.Status{
			{ID: 100, Name: "ToDo"},
			{ID: 200, Name: "InProgress"},
		}, nil
	}
	ids, err := resolveStatuses(context.Background(), "open", []string{"PROJ"}, mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !intSliceEqual(ids, []int{100, 200}) {
		t.Fatalf("期待 [100,200], 実際 %v", ids)
	}
}

// X2: 全て完了ステータス → [-1] (フォールバックで0件保証)
func TestResolveStatuses_only_closed(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return []domain.Status{
			{ID: 4, Name: "完了"},
		}, nil
	}
	ids, err := resolveStatuses(context.Background(), "open", []string{"PROJ"}, mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !intSliceEqual(ids, []int{-1}) {
		t.Fatalf("期待 [-1], 実際 %v", ids)
	}
}

// X3: カンマ区切り数値+名前の混在、重複除去
func TestResolveStatuses_comma_mixed(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return testStatuses, nil
	}
	ids, err := resolveStatuses(context.Background(), "1,未対応", []string{"PROJ"}, mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	// 1と未対応(=1)で重複 → [1]
	if !intSliceEqual(ids, []int{1}) {
		t.Fatalf("期待 [1], 実際 %v", ids)
	}
}

// X4: 複数プロジェクト open → マージ・重複除去
func TestResolveStatuses_multi_project_open(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		if projectKey == "A" {
			return []domain.Status{
				{ID: 1, Name: "未対応"},
				{ID: 2, Name: "処理中"},
				{ID: 4, Name: "完了"},
			}, nil
		}
		// B は A と同じステータスセット（ID が重複）
		return []domain.Status{
			{ID: 1, Name: "未対応"},
			{ID: 2, Name: "処理中"},
			{ID: 4, Name: "完了"},
		}, nil
	}
	ids, err := resolveStatuses(context.Background(), "open", []string{"A", "B"}, mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !intSliceEqual(ids, []int{1, 2}) {
		t.Fatalf("期待 [1,2], 実際 %v", ids)
	}
}

// ---- resolveDueDate テスト ----

// C7: "today" → Since=Until=今日
func TestResolveDueDate_today(t *testing.T) {
	since, until, err := resolveDueDate("today")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if since == nil || until == nil {
		t.Fatal("since/until が nil")
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	if !since.Equal(today) {
		t.Fatalf("since 期待 %v, 実際 %v", today, *since)
	}
	if !until.Equal(today) {
		t.Fatalf("until 期待 %v, 実際 %v", today, *until)
	}
}

// C8: "overdue" → Until=昨日, Since=nil
func TestResolveDueDate_overdue(t *testing.T) {
	since, until, err := resolveDueDate("overdue")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if since != nil {
		t.Fatalf("since は nil 期待, 実際 %v", *since)
	}
	if until == nil {
		t.Fatal("until が nil")
	}
	now := time.Now()
	yesterday := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, time.Local)
	if !until.Equal(yesterday) {
		t.Fatalf("until 期待 %v, 実際 %v", yesterday, *until)
	}
}

// C9: "2026-12-31" → Since=Until=2026-12-31
func TestResolveDueDate_date(t *testing.T) {
	since, until, err := resolveDueDate("2026-12-31")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if since == nil || until == nil {
		t.Fatal("since/until が nil")
	}
	expected := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	if !since.Equal(expected) {
		t.Fatalf("since 期待 %v, 実際 %v", expected, *since)
	}
	if !until.Equal(expected) {
		t.Fatalf("until 期待 %v, 実際 %v", expected, *until)
	}
}

// C10: "" → nil, nil, nil
func TestResolveDueDate_empty(t *testing.T) {
	since, until, err := resolveDueDate("")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if since != nil || until != nil {
		t.Fatalf("nil 期待: since=%v, until=%v", since, until)
	}
}

// E4: 不正な文字列 → エラー
func TestResolveDueDate_invalid(t *testing.T) {
	_, _, err := resolveDueDate("abc")
	if err == nil {
		t.Fatal("エラーが期待されたが nil")
	}
}

// N1: resolveAssignee で UserID マッチ → [50]
func TestResolveAssignee_userID(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListUsersFunc = func(ctx context.Context) ([]domain.User, error) {
		return []domain.User{
			{ID: 50, UserID: "taro.tanaka", Name: "田中太郎"},
			{ID: 51, UserID: "hanako.suzuki", Name: "鈴木花子"},
		}, nil
	}
	ids, err := resolveAssignee(context.Background(), "taro.tanaka", mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(ids) != 1 || ids[0] != 50 {
		t.Fatalf("期待 [50], 実際 %v", ids)
	}
}

// N2: resolveStatuses 複数プロジェクトで名前解決 → [1, 10] (break なし全走査)
func TestResolveStatuses_multiProject_name(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		if projectKey == "A" {
			return []domain.Status{
				{ID: 1, Name: "未対応"},
				{ID: 2, Name: "処理中"},
			}, nil
		}
		// B プロジェクトでは "未対応" が ID=10
		return []domain.Status{
			{ID: 10, Name: "未対応"},
			{ID: 20, Name: "処理中"},
		}, nil
	}
	ids, err := resolveStatuses(context.Background(), "未対応", []string{"A", "B"}, mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	// 全プロジェクトを走査して両方の "未対応" を収集
	if !intSliceEqual(ids, []int{1, 10}) {
		t.Fatalf("期待 [1,10], 実際 %v", ids)
	}
}

// N3: resolveStatuses open で全ステータスが完了 → [-1] (フォールバック)
func TestResolveStatuses_open_allClosed_fallback(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		return []domain.Status{
			{ID: 4, Name: "完了"},
		}, nil
	}
	ids, err := resolveStatuses(context.Background(), "open", []string{"PROJ"}, mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !intSliceEqual(ids, []int{-1}) {
		t.Fatalf("期待 [-1], 実際 %v", ids)
	}
}

// ---- ヘルパー ----

// intSliceEqual は順序付きで 2 つの int スライスを比較する。
func intSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

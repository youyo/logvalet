package cli

import (
	"context"
	"errors"
	"strings"
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
	mc.ListTeamsFunc = func(ctx context.Context) ([]domain.TeamWithMembers, error) {
		return nil, nil
	}
	ids, err := resolveAssignee(context.Background(), "田中太郎", mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(ids) != 1 || ids[0] != 50 {
		t.Fatalf("期待 [50], 実際 %v", ids)
	}
}

// E1: 名前に一致しない → チーム名でもフォールバックして一致しない → エラー
func TestResolveAssignee_name_not_found(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListUsersFunc = func(ctx context.Context) ([]domain.User, error) {
		return []domain.User{
			{ID: 50, Name: "田中太郎"},
		}, nil
	}
	mc.ListTeamsFunc = func(ctx context.Context) ([]domain.TeamWithMembers, error) {
		return []domain.TeamWithMembers{
			{ID: 1, Name: "開発チーム"},
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

// A2: チーム名完全一致 → メンバー全員のID
func TestResolveAssignee_team_name_exact(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListUsersFunc = func(ctx context.Context) ([]domain.User, error) {
		return []domain.User{{ID: 10, Name: "田中太郎"}}, nil
	}
	mc.ListTeamsFunc = func(ctx context.Context) ([]domain.TeamWithMembers, error) {
		return []domain.TeamWithMembers{
			{ID: 100, Name: "株式会社ヘプタゴン全体"},
		}, nil
	}
	mc.GetTeamFunc = func(ctx context.Context, teamID int) (*domain.TeamWithMembers, error) {
		return &domain.TeamWithMembers{
			ID:   100,
			Name: "株式会社ヘプタゴン全体",
			Members: []domain.User{
				{ID: 201, Name: "メンバーA"},
				{ID: 202, Name: "メンバーB"},
			},
		}, nil
	}
	ids, err := resolveAssignee(context.Background(), "株式会社ヘプタゴン全体", mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !intSliceEqual(ids, []int{201, 202}) {
		t.Fatalf("期待 [201,202], 実際 %v", ids)
	}
}

// A3: ユーザー名もチーム名も一致しない → エラー（ユーザー名+チーム名の一覧）
func TestResolveAssignee_not_found_shows_both_lists(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListUsersFunc = func(ctx context.Context) ([]domain.User, error) {
		return []domain.User{{ID: 10, Name: "田中太郎"}}, nil
	}
	mc.ListTeamsFunc = func(ctx context.Context) ([]domain.TeamWithMembers, error) {
		return []domain.TeamWithMembers{
			{ID: 100, Name: "開発チーム"},
		}, nil
	}
	_, err := resolveAssignee(context.Background(), "存在しない", mc)
	if err == nil {
		t.Fatal("エラーが期待されたが nil")
	}
	// エラーメッセージにユーザー名とチーム名が含まれる
	if !strings.Contains(err.Error(), "田中太郎") {
		t.Errorf("エラーにユーザー名が含まれていない: %v", err)
	}
	if !strings.Contains(err.Error(), "開発チーム") {
		t.Errorf("エラーにチーム名が含まれていない: %v", err)
	}
}

// A4: チーム名部分一致 → メンバーID
func TestResolveAssignee_team_name_partial(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListUsersFunc = func(ctx context.Context) ([]domain.User, error) {
		return []domain.User{{ID: 10, Name: "田中太郎"}}, nil
	}
	mc.ListTeamsFunc = func(ctx context.Context) ([]domain.TeamWithMembers, error) {
		return []domain.TeamWithMembers{
			{ID: 100, Name: "株式会社ヘプタゴン全体"},
		}, nil
	}
	mc.GetTeamFunc = func(ctx context.Context, teamID int) (*domain.TeamWithMembers, error) {
		return &domain.TeamWithMembers{
			ID:   100,
			Name: "株式会社ヘプタゴン全体",
			Members: []domain.User{
				{ID: 201, Name: "メンバーA"},
				{ID: 202, Name: "メンバーB"},
			},
		}, nil
	}
	// "ヘプタゴン" で部分一致 → "株式会社ヘプタゴン全体" にマッチ
	ids, err := resolveAssignee(context.Background(), "ヘプタゴン", mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !intSliceEqual(ids, []int{201, 202}) {
		t.Fatalf("期待 [201,202], 実際 %v", ids)
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
	mc.ListTeamsFunc = func(ctx context.Context) ([]domain.TeamWithMembers, error) {
		return nil, nil
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

// ---- resolveStatuses not-closed テスト ----

// NC1: "not-closed", projectKeys=[] → [1,2,3], err=nil
func TestResolveStatuses_notClosed(t *testing.T) {
	mc := backlog.NewMockClient()
	ids, err := resolveStatuses(context.Background(), "not-closed", []string{}, mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !intSliceEqual(ids, []int{1, 2, 3}) {
		t.Fatalf("期待 [1,2,3], 実際 %v", ids)
	}
}

// NC2: "not-closed", projectKeys=["PROJ"] → [1,2,3], err=nil (API 呼び出しなし)
func TestResolveStatuses_notClosed_withProject(t *testing.T) {
	mc := backlog.NewMockClient()
	// ListProjectStatuses が呼ばれないことを確認するため、呼ばれたらエラーにする
	mc.ListProjectStatusesFunc = func(ctx context.Context, projectKey string) ([]domain.Status, error) {
		t.Error("not-closed で ListProjectStatuses は呼ばれてはいけない")
		return nil, nil
	}
	ids, err := resolveStatuses(context.Background(), "not-closed", []string{"PROJ"}, mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !intSliceEqual(ids, []int{1, 2, 3}) {
		t.Fatalf("期待 [1,2,3], 実際 %v", ids)
	}
}

// ---- resolveDueDate 範囲テスト ----

// DD1: "2026-03-01:2026-03-31" → Since=03-01, Until=03-31
func TestResolveDueDate_range_both(t *testing.T) {
	since, until, err := resolveDueDate("2026-03-01:2026-03-31")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if since == nil {
		t.Fatal("since が nil")
	}
	if until == nil {
		t.Fatal("until が nil")
	}
	expectedSince := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	expectedUntil := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	if !since.Equal(expectedSince) {
		t.Fatalf("since 期待 %v, 実際 %v", expectedSince, *since)
	}
	if !until.Equal(expectedUntil) {
		t.Fatalf("until 期待 %v, 実際 %v", expectedUntil, *until)
	}
}

// DD2: "2026-03-01:" → Since=03-01, Until=nil
func TestResolveDueDate_range_sinceOnly(t *testing.T) {
	since, until, err := resolveDueDate("2026-03-01:")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if since == nil {
		t.Fatal("since が nil")
	}
	if until != nil {
		t.Fatalf("until は nil 期待, 実際 %v", *until)
	}
	expectedSince := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if !since.Equal(expectedSince) {
		t.Fatalf("since 期待 %v, 実際 %v", expectedSince, *since)
	}
}

// DD3: ":2026-03-31" → Since=nil, Until=03-31
func TestResolveDueDate_range_untilOnly(t *testing.T) {
	since, until, err := resolveDueDate(":2026-03-31")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if since != nil {
		t.Fatalf("since は nil 期待, 実際 %v", *since)
	}
	if until == nil {
		t.Fatal("until が nil")
	}
	expectedUntil := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	if !until.Equal(expectedUntil) {
		t.Fatalf("until 期待 %v, 実際 %v", expectedUntil, *until)
	}
}

// DD4: ":" → エラー（両側空）
func TestResolveDueDate_range_empty(t *testing.T) {
	_, _, err := resolveDueDate(":")
	if err == nil {
		t.Fatal("エラーが期待されたが nil")
	}
}

// DD5: "invalid:2026-03-31" → エラー
func TestResolveDueDate_range_invalidDate(t *testing.T) {
	_, _, err := resolveDueDate("invalid:2026-03-31")
	if err == nil {
		t.Fatal("エラーが期待されたが nil")
	}
}

// DD6: "this-week" → Since=今週月曜, Until=今週日曜
func TestResolveDueDate_thisWeek(t *testing.T) {
	since, until, err := resolveDueDate("this-week")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if since == nil {
		t.Fatal("since が nil")
	}
	if until == nil {
		t.Fatal("until が nil")
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	expectedSince := weekStart(today)
	expectedUntil := expectedSince.AddDate(0, 0, 6)
	if !since.Equal(expectedSince) {
		t.Fatalf("since 期待 %v, 実際 %v", expectedSince, *since)
	}
	if !until.Equal(expectedUntil) {
		t.Fatalf("until 期待 %v, 実際 %v", expectedUntil, *until)
	}
}

// DD7: "this-month" → Since=今月1日, Until=今月末日
func TestResolveDueDate_thisMonth(t *testing.T) {
	since, until, err := resolveDueDate("this-month")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if since == nil {
		t.Fatal("since が nil")
	}
	if until == nil {
		t.Fatal("until が nil")
	}
	now := time.Now()
	expectedSince := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	// 翌月1日の前日 = 今月末日
	expectedUntil := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.Local)
	if !since.Equal(expectedSince) {
		t.Fatalf("since 期待 %v, 実際 %v", expectedSince, *since)
	}
	if !until.Equal(expectedUntil) {
		t.Fatalf("until 期待 %v, 実際 %v", expectedUntil, *until)
	}
}

// E1: "abc:def" → エラー
func TestResolveDueDate_range_invalidFormat(t *testing.T) {
	_, _, err := resolveDueDate("abc:def")
	if err == nil {
		t.Fatal("エラーが期待されたが nil")
	}
}

// ---- weekStart ヘルパーテスト ----

func TestWeekStart(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "WS1: 月曜日 2026-03-23",
			input:    time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "WS2: 水曜日 2026-03-25",
			input:    time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "WS3: 日曜日 2026-03-29",
			input:    time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "WS4: 土曜日 2026-03-28",
			input:    time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := weekStart(tt.input)
			if !got.Equal(tt.expected) {
				t.Fatalf("期待 %v, 実際 %v", tt.expected, got)
			}
		})
	}
}

// ---- fetchAllIssues テスト ----

// PG1: 1回目100件、2回目50件 → 全150件返す
func TestFetchAllIssues_multiPage(t *testing.T) {
	mc := backlog.NewMockClient()
	callCount := 0
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		callCount++
		if callCount == 1 {
			// 1ページ目: 100件
			issues := make([]domain.Issue, 100)
			for i := range issues {
				issues[i] = domain.Issue{ID: i + 1}
			}
			return issues, nil
		}
		// 2ページ目: 50件
		issues := make([]domain.Issue, 50)
		for i := range issues {
			issues[i] = domain.Issue{ID: 100 + i + 1}
		}
		return issues, nil
	}

	opt := backlog.ListIssuesOptions{Limit: 100, Offset: 0}
	all, err := fetchAllIssues(context.Background(), mc, opt)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(all) != 150 {
		t.Fatalf("期待 150件, 実際 %d件", len(all))
	}
	if callCount != 2 {
		t.Fatalf("期待 2回呼び出し, 実際 %d回", callCount)
	}
}

// PG2: 50件 → 1回で完了
func TestFetchAllIssues_singlePage(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		issues := make([]domain.Issue, 50)
		for i := range issues {
			issues[i] = domain.Issue{ID: i + 1}
		}
		return issues, nil
	}

	opt := backlog.ListIssuesOptions{Limit: 100, Offset: 0}
	all, err := fetchAllIssues(context.Background(), mc, opt)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(all) != 50 {
		t.Fatalf("期待 50件, 実際 %d件", len(all))
	}
	if mc.GetCallCount("ListIssues") != 1 {
		t.Fatalf("期待 1回呼び出し, 実際 %d回", mc.GetCallCount("ListIssues"))
	}
}

// PG3: 0件 → 即完了、空スライス
func TestFetchAllIssues_empty(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		return []domain.Issue{}, nil
	}

	opt := backlog.ListIssuesOptions{Limit: 100, Offset: 0}
	all, err := fetchAllIssues(context.Background(), mc, opt)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("期待 0件, 実際 %d件", len(all))
	}
	if mc.GetCallCount("ListIssues") != 1 {
		t.Fatalf("期待 1回呼び出し, 実際 %d回", mc.GetCallCount("ListIssues"))
	}
}

// PG4: 10,000件で打ち切りテスト（Limit=100 で100回呼び出し後、100件返してもループが打ち切られる）
func TestFetchAllIssues_maxLimit(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		// 常に100件を返し続ける
		issues := make([]domain.Issue, 100)
		for i := range issues {
			issues[i] = domain.Issue{ID: opt.Offset + i + 1}
		}
		return issues, nil
	}

	opt := backlog.ListIssuesOptions{Limit: 100, Offset: 0}
	all, err := fetchAllIssues(context.Background(), mc, opt)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(all) != 10000 {
		t.Fatalf("期待 10000件, 実際 %d件", len(all))
	}
	// 10000 / 100 = 100回呼び出し
	if mc.GetCallCount("ListIssues") != 100 {
		t.Fatalf("期待 100回呼び出し, 実際 %d回", mc.GetCallCount("ListIssues"))
	}
}

// E2: 2ページ目でエラー → エラー伝播
func TestFetchAllIssues_apiError(t *testing.T) {
	mc := backlog.NewMockClient()
	callCount := 0
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		callCount++
		if callCount == 1 {
			issues := make([]domain.Issue, 100)
			for i := range issues {
				issues[i] = domain.Issue{ID: i + 1}
			}
			return issues, nil
		}
		return nil, errors.New("API エラー")
	}

	opt := backlog.ListIssuesOptions{Limit: 100, Offset: 0}
	_, err := fetchAllIssues(context.Background(), mc, opt)
	if err == nil {
		t.Fatal("エラーが期待されたが nil")
	}
}

// ZL1: Limit=0 で呼び出し → 無限ループせずに正常に結果を返す（Limit が 100 に正規化される）
func TestFetchAllIssues_zeroLimit(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListIssuesFunc = func(ctx context.Context, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
		// Limit が 100 に正規化されているはずなので、3件返す（1ページで完了）
		if opt.Limit != 100 {
			t.Errorf("Limit が 100 に正規化されていない: %d", opt.Limit)
		}
		issues := make([]domain.Issue, 3)
		for i := range issues {
			issues[i] = domain.Issue{ID: i + 1}
		}
		return issues, nil
	}

	// Limit=0 で呼び出す
	opt := backlog.ListIssuesOptions{Limit: 0, Offset: 0}
	all, err := fetchAllIssues(context.Background(), mc, opt)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("期待 3件, 実際 %d件", len(all))
	}
	if mc.GetCallCount("ListIssues") != 1 {
		t.Fatalf("期待 1回呼び出し, 実際 %d回", mc.GetCallCount("ListIssues"))
	}
}

// ---- resolvePeriod テスト ----

// B4: ("this-week", "") → since=今週月曜, until=nil
func TestResolvePeriod_thisWeek(t *testing.T) {
	since, until, err := resolvePeriod("this-week", "")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if since == nil {
		t.Fatal("since が nil")
	}
	if until != nil {
		t.Fatalf("until は nil 期待, 実際 %v", *until)
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	expectedSince := weekStart(today)
	if !since.Equal(expectedSince) {
		t.Fatalf("since 期待 %v, 実際 %v", expectedSince, *since)
	}
}

// B5: ("this-month", "") → since=今月1日, until=nil
func TestResolvePeriod_thisMonth(t *testing.T) {
	since, until, err := resolvePeriod("this-month", "")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if since == nil {
		t.Fatal("since が nil")
	}
	if until != nil {
		t.Fatalf("until は nil 期待, 実際 %v", *until)
	}
	now := time.Now()
	expectedSince := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	if !since.Equal(expectedSince) {
		t.Fatalf("since 期待 %v, 実際 %v", expectedSince, *since)
	}
}

// B6: ("2026-03-01", "2026-03-31") → 指定通り
func TestResolvePeriod_bothDates(t *testing.T) {
	since, until, err := resolvePeriod("2026-03-01", "2026-03-31")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if since == nil {
		t.Fatal("since が nil")
	}
	if until == nil {
		t.Fatal("until が nil")
	}
	expectedSince := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	expectedUntil := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	if !since.Equal(expectedSince) {
		t.Fatalf("since 期待 %v, 実際 %v", expectedSince, *since)
	}
	if !until.Equal(expectedUntil) {
		t.Fatalf("until 期待 %v, 実際 %v", expectedUntil, *until)
	}
}

// B7: ("this-week", "this-week") → since=今週月曜, until=今週日曜
func TestResolvePeriod_sinceThisWeekUntilThisWeek(t *testing.T) {
	since, until, err := resolvePeriod("this-week", "this-week")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if since == nil {
		t.Fatal("since が nil")
	}
	if until == nil {
		t.Fatal("until が nil")
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	expectedSince := weekStart(today)
	expectedUntil := expectedSince.AddDate(0, 0, 6)
	if !since.Equal(expectedSince) {
		t.Fatalf("since 期待 %v, 実際 %v", expectedSince, *since)
	}
	if !until.Equal(expectedUntil) {
		t.Fatalf("until 期待 %v, 実際 %v", expectedUntil, *until)
	}
}

// B8: ("", "") → nil, nil, nil
func TestResolvePeriod_empty(t *testing.T) {
	since, until, err := resolvePeriod("", "")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if since != nil {
		t.Fatalf("since は nil 期待, 実際 %v", *since)
	}
	if until != nil {
		t.Fatalf("until は nil 期待, 実際 %v", *until)
	}
}

// ---- resolveTeamIDs テスト ----

// T1: 数値文字列 "173843" → [173843]
func TestResolveTeamIDs_numeric(t *testing.T) {
	mc := backlog.NewMockClient()
	ids, err := resolveTeamIDs(context.Background(), []string{"173843"}, mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !intSliceEqual(ids, []int{173843}) {
		t.Fatalf("期待 [173843], 実際 %v", ids)
	}
}

// T2: チーム名 "ヘプタゴン" → 名前一致の ID
func TestResolveTeamIDs_name(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListTeamsFunc = func(ctx context.Context) ([]domain.TeamWithMembers, error) {
		return []domain.TeamWithMembers{
			{ID: 173843, Name: "ヘプタゴン"},
			{ID: 221464, Name: "他チーム"},
		}, nil
	}
	ids, err := resolveTeamIDs(context.Background(), []string{"ヘプタゴン"}, mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !intSliceEqual(ids, []int{173843}) {
		t.Fatalf("期待 [173843], 実際 %v", ids)
	}
}

// T3: 存在しない名前 → エラー（利用可能なチーム名一覧を含む）
func TestResolveTeamIDs_notFound(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListTeamsFunc = func(ctx context.Context) ([]domain.TeamWithMembers, error) {
		return []domain.TeamWithMembers{
			{ID: 173843, Name: "ヘプタゴン"},
		}, nil
	}
	_, err := resolveTeamIDs(context.Background(), []string{"存在しない"}, mc)
	if err == nil {
		t.Fatal("エラーが期待されたが nil")
	}
	if !strings.Contains(err.Error(), "ヘプタゴン") {
		t.Fatalf("エラーメッセージに利用可能なチーム名が含まれない: %v", err)
	}
}

// T4: 複数指定 ["173843", "221464"] → [173843, 221464]
func TestResolveTeamIDs_multiple(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListTeamsFunc = func(ctx context.Context) ([]domain.TeamWithMembers, error) {
		return []domain.TeamWithMembers{
			{ID: 173843, Name: "ヘプタゴン"},
			{ID: 221464, Name: "別チーム"},
		}, nil
	}
	ids, err := resolveTeamIDs(context.Background(), []string{"173843", "221464"}, mc)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !intSliceEqual(ids, []int{173843, 221464}) {
		t.Fatalf("期待 [173843, 221464], 実際 %v", ids)
	}
}

// T5: 名前で複数一致 → エラー
func TestResolveTeamIDs_multipleMatch(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.ListTeamsFunc = func(ctx context.Context) ([]domain.TeamWithMembers, error) {
		return []domain.TeamWithMembers{
			{ID: 1, Name: "チームA"},
			{ID: 2, Name: "チームA"},
		}, nil
	}
	_, err := resolveTeamIDs(context.Background(), []string{"チームA"}, mc)
	if err == nil {
		t.Fatal("エラーが期待されたが nil")
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

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// closedStatusNames は完了とみなすステータス名一覧（case-insensitive 比較）。
var closedStatusNames = []string{"完了", "Closed", "Done", "Completed"}

// isClosedStatus はステータス名が完了ステータスかを判定する。
func isClosedStatus(name string) bool {
	for _, closed := range closedStatusNames {
		if strings.EqualFold(name, closed) {
			return true
		}
	}
	return false
}

// uniqueInts は重複を除去した int スライスを返す（入力順を保持）。
func uniqueInts(ids []int) []int {
	seen := make(map[int]bool, len(ids))
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

// resolveAssignee は --assignee フラグの値を AssigneeIDs に変換する。
// "me" → GetMyself、数値文字列 → そのまま ID、それ以外 → ListUsers で名前検索。
func resolveAssignee(ctx context.Context, input string, client backlog.Client) ([]int, error) {
	if input == "me" {
		user, err := client.GetMyself(ctx)
		if err != nil {
			return nil, err
		}
		return []int{user.ID}, nil
	}
	if id, err := strconv.Atoi(input); err == nil {
		return []int{id}, nil
	}
	// 名前検索
	users, err := client.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	var matched []domain.User
	for _, u := range users {
		if strings.EqualFold(u.Name, input) || strings.EqualFold(u.UserID, input) {
			matched = append(matched, u)
		}
	}
	switch len(matched) {
	case 0:
		names := make([]string, len(users))
		for i, u := range users {
			names[i] = u.Name
		}
		return nil, fmt.Errorf("担当者 %q が見つかりません。利用可能: [%s]", input, strings.Join(names, ", "))
	case 1:
		return []int{matched[0].ID}, nil
	default:
		return nil, fmt.Errorf("担当者 %q に複数一致しました", input)
	}
}

// resolveStatuses は --status フラグの値を StatusIDs に変換する。
// "open" → 完了以外のステータス（projectKeys 必須）
// カンマ区切り → 各要素を数値または名前で解決
// 単一数値 → そのまま ID（projectKeys 不要）
func resolveStatuses(ctx context.Context, input string, projectKeys []string, client backlog.Client) ([]int, error) {
	if input == "open" {
		if len(projectKeys) == 0 {
			return nil, fmt.Errorf("--status open には --project-key (-k) が必須です")
		}
		var ids []int
		for _, key := range projectKeys {
			statuses, err := client.ListProjectStatuses(ctx, key)
			if err != nil {
				return nil, fmt.Errorf("プロジェクト %q のステータス取得に失敗: %w", key, err)
			}
			for _, s := range statuses {
				if !isClosedStatus(s.Name) {
					ids = append(ids, s.ID)
				}
			}
		}
		result := uniqueInts(ids)
		if len(result) == 0 {
			// 全ステータスが完了の場合、存在しないIDで0件を保証
			return []int{-1}, nil
		}
		return result, nil
	}

	// カンマ区切りまたは単一値の処理
	parts := strings.Split(input, ",")
	var ids []int
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// 数値なら直接 ID
		if id, err := strconv.Atoi(part); err == nil {
			ids = append(ids, id)
			continue
		}
		// 名前 → projectKeys 必須
		if len(projectKeys) == 0 {
			return nil, fmt.Errorf("ステータス名 %q の解決には --project-key (-k) が必須です", part)
		}
		// 各プロジェクトのステータスで名前解決
		resolved := false
		for _, key := range projectKeys {
			statuses, err := client.ListProjectStatuses(ctx, key)
			if err != nil {
				return nil, fmt.Errorf("プロジェクト %q のステータス取得に失敗: %w", key, err)
			}
			id, err := resolveNameOrID(part, toIDNamesFromStatuses(statuses))
			if err != nil {
				continue
			}
			ids = append(ids, id)
			resolved = true
		}
		if !resolved {
			return nil, fmt.Errorf("ステータス %q が見つかりません", part)
		}
	}
	return uniqueInts(ids), nil
}

// resolveDueDate は --due-date フラグの値を DueDateSince / DueDateUntil に変換する。
// "" → nil, nil, nil
// "today" → Since=Until=今日
// "overdue" → Since=nil, Until=昨日
// "YYYY-MM-DD" → Since=Until=指定日
func resolveDueDate(input string) (*time.Time, *time.Time, error) {
	if input == "" {
		return nil, nil, nil
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	switch input {
	case "today":
		return &today, &today, nil
	case "overdue":
		yesterday := today.AddDate(0, 0, -1)
		return nil, &yesterday, nil
	default:
		t, err := parseDate(input)
		if err != nil {
			return nil, nil, fmt.Errorf("期限日は today, overdue, YYYY-MM-DD で指定してください: %q", input)
		}
		return t, t, nil
	}
}

// resolveNameOrID は入力文字列を ID に変換する。
// 数値ならそのまま ID として返却、文字列なら case-insensitive 完全一致検索。
// 一致 0 件 → エラー（利用可能一覧を含む）
// 一致 2 件以上 → エラー
func resolveNameOrID(input string, items []domain.IDName) (int, error) {
	if input == "" {
		return 0, fmt.Errorf("入力が空です")
	}

	// 数値として解析できる場合はそのまま ID として返す
	if id, err := strconv.Atoi(input); err == nil {
		return id, nil
	}

	// case-insensitive 完全一致検索
	var matched []domain.IDName
	for _, item := range items {
		if strings.EqualFold(item.Name, input) {
			matched = append(matched, item)
		}
	}

	switch len(matched) {
	case 0:
		names := make([]string, len(items))
		for i, item := range items {
			names[i] = item.Name
		}
		return 0, fmt.Errorf("%q が見つかりません。利用可能: [%s]", input, strings.Join(names, ", "))
	case 1:
		return matched[0].ID, nil
	default:
		return 0, fmt.Errorf("%q に複数一致しました", input)
	}
}

// resolveNamesOrIDs は複数入力を一括変換する。
func resolveNamesOrIDs(inputs []string, items []domain.IDName) ([]int, error) {
	ids := make([]int, 0, len(inputs))
	for _, input := range inputs {
		id, err := resolveNameOrID(input, items)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// parseDate は "YYYY-MM-DD" 形式の文字列を *time.Time に変換する。
func parseDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, fmt.Errorf("日付の形式が不正です（YYYY-MM-DD 形式で指定してください）: %q", s)
	}
	return &t, nil
}

// toIDNamesFromCategories は Category スライスを IDName スライスに変換する。
func toIDNamesFromCategories(items []domain.Category) []domain.IDName {
	result := make([]domain.IDName, len(items))
	for i, item := range items {
		result[i] = domain.IDName{ID: item.ID, Name: item.Name}
	}
	return result
}

// toIDNamesFromVersions は Version スライスを IDName スライスに変換する。
func toIDNamesFromVersions(items []domain.Version) []domain.IDName {
	result := make([]domain.IDName, len(items))
	for i, item := range items {
		result[i] = domain.IDName{ID: item.ID, Name: item.Name}
	}
	return result
}

// toIDNamesFromStatuses は Status スライスを IDName スライスに変換する。
func toIDNamesFromStatuses(items []domain.Status) []domain.IDName {
	result := make([]domain.IDName, len(items))
	for i, item := range items {
		result[i] = domain.IDName{ID: item.ID, Name: item.Name}
	}
	return result
}

// extractProjectKey は issueKey（例: "HEP_ISSUES-123"）からプロジェクトキーを抽出する。
func extractProjectKey(issueKey string) string {
	idx := strings.LastIndex(issueKey, "-")
	if idx <= 0 {
		return issueKey
	}
	return issueKey[:idx]
}

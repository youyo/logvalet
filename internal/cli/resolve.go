package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/youyo/logvalet/internal/domain"
)

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

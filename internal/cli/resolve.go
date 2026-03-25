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
// "me" → GetMyself、数値文字列 → そのまま ID、それ以外 → ListUsers で名前検索 →
// 一致なしの場合は ListTeams でチーム名（部分一致）検索 → GetTeam でメンバー展開。
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
	// ユーザー名検索
	users, err := client.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	var matchedUsers []domain.User
	for _, u := range users {
		if strings.EqualFold(u.Name, input) || strings.EqualFold(u.UserID, input) {
			matchedUsers = append(matchedUsers, u)
		}
	}
	switch len(matchedUsers) {
	case 1:
		return []int{matchedUsers[0].ID}, nil
	default:
		if len(matchedUsers) > 1 {
			return nil, fmt.Errorf("担当者 %q に複数一致しました", input)
		}
	}

	// ユーザー名一致なし → チーム名フォールバック
	teams, err := client.ListTeams(ctx)
	if err != nil {
		return nil, fmt.Errorf("チーム一覧の取得に失敗: %w", err)
	}
	var matchedTeams []domain.TeamWithMembers
	for _, t := range teams {
		if strings.Contains(strings.ToLower(t.Name), strings.ToLower(input)) {
			matchedTeams = append(matchedTeams, t)
		}
	}
	switch len(matchedTeams) {
	case 0:
		// どちらも一致なし → エラー（ユーザー名+チーム名の一覧）
		userNames := make([]string, len(users))
		for i, u := range users {
			userNames[i] = u.Name
		}
		teamNames := make([]string, len(teams))
		for i, t := range teams {
			teamNames[i] = t.Name
		}
		return nil, fmt.Errorf("担当者 %q が見つかりません。利用可能ユーザー: [%s]、利用可能チーム: [%s]",
			input, strings.Join(userNames, ", "), strings.Join(teamNames, ", "))
	case 1:
		team, err := client.GetTeam(ctx, matchedTeams[0].ID)
		if err != nil {
			return nil, fmt.Errorf("チーム (ID=%d) の取得に失敗: %w", matchedTeams[0].ID, err)
		}
		ids := make([]int, len(team.Members))
		for i, m := range team.Members {
			ids[i] = m.ID
		}
		return ids, nil
	default:
		return nil, fmt.Errorf("チーム名 %q に複数一致しました", input)
	}
}

// resolveStatuses は --status フラグの値を StatusIDs に変換する。
// "not-closed" → Backlog 標準の未完了ステータス [1,2,3]（projectKeys 不要）
// "open" → 完了以外のステータス（projectKeys 必須）
// カンマ区切り → 各要素を数値または名前で解決
// 単一数値 → そのまま ID（projectKeys 不要）
func resolveStatuses(ctx context.Context, input string, projectKeys []string, client backlog.Client) ([]int, error) {
	if input == "not-closed" {
		// Backlog 標準ステータス: 1=未対応, 2=処理中, 3=処理済み（4=完了を除外）
		return []int{1, 2, 3}, nil
	}
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
// "this-week" → Since=今週月曜, Until=今週日曜
// "this-month" → Since=今月1日, Until=今月末日
// "YYYY-MM-DD" → Since=Until=指定日
// "YYYY-MM-DD:YYYY-MM-DD" → Since=左側, Until=右側
// "YYYY-MM-DD:" → Since のみ
// ":YYYY-MM-DD" → Until のみ
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
	case "this-week":
		monday := weekStart(today)
		sunday := monday.AddDate(0, 0, 6)
		return &monday, &sunday, nil
	case "this-month":
		firstDay := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
		lastDay := time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, today.Location())
		return &firstDay, &lastDay, nil
	default:
		// コロン区切りの範囲指定を試みる
		if strings.Contains(input, ":") {
			since, until, err := parseDateRange(input)
			if err != nil {
				return nil, nil, fmt.Errorf("期限日は today, overdue, this-week, this-month, YYYY-MM-DD, YYYY-MM-DD:YYYY-MM-DD で指定してください: %q", input)
			}
			return since, until, nil
		}
		t, err := parseDate(input)
		if err != nil {
			return nil, nil, fmt.Errorf("期限日は today, overdue, this-week, this-month, YYYY-MM-DD, YYYY-MM-DD:YYYY-MM-DD で指定してください: %q", input)
		}
		return t, t, nil
	}
}

// resolveStartDate は --start-date フラグの値を StartDateSince / StartDateUntil に変換する。
// "" → nil, nil, nil
// "today" → Since=Until=今日
// "this-week" → Since=今週月曜, Until=今週日曜
// "this-month" → Since=今月1日, Until=今月末日
// "YYYY-MM-DD" → Since=Until=指定日
// "YYYY-MM-DD:YYYY-MM-DD" → Since=左側, Until=右側
// "YYYY-MM-DD:" → Since のみ
// ":YYYY-MM-DD" → Until のみ
// "overdue" → エラー（startDate では非対応）
func resolveStartDate(input string) (*time.Time, *time.Time, error) {
	if input == "" {
		return nil, nil, nil
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	switch input {
	case "today":
		return &today, &today, nil
	case "overdue":
		return nil, nil, fmt.Errorf("開始日は today, this-week, this-month, YYYY-MM-DD, YYYY-MM-DD:YYYY-MM-DD で指定してください: %q", input)
	case "this-week":
		monday := weekStart(today)
		sunday := monday.AddDate(0, 0, 6)
		return &monday, &sunday, nil
	case "this-month":
		firstDay := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
		lastDay := time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, today.Location())
		return &firstDay, &lastDay, nil
	default:
		// コロン区切りの範囲指定を試みる
		if strings.Contains(input, ":") {
			since, until, err := parseDateRange(input)
			if err != nil {
				return nil, nil, fmt.Errorf("開始日は today, this-week, this-month, YYYY-MM-DD, YYYY-MM-DD:YYYY-MM-DD で指定してください: %q", input)
			}
			return since, until, nil
		}
		t, err := parseDate(input)
		if err != nil {
			return nil, nil, fmt.Errorf("開始日は today, this-week, this-month, YYYY-MM-DD, YYYY-MM-DD:YYYY-MM-DD で指定してください: %q", input)
		}
		return t, t, nil
	}
}

// resolvePeriod は --since / --until フラグの値をそれぞれ *time.Time に変換する。
// "" → nil
// "today" → 今日
// "this-week" → since の場合は今週月曜、until の場合は今週日曜
// "this-month" → since の場合は今月1日、until の場合は今月末日
// "YYYY-MM-DD" → その日付
func resolvePeriod(since, until string) (*time.Time, *time.Time, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	var sinceTime *time.Time
	if since != "" {
		switch since {
		case "today":
			sinceTime = &today
		case "this-week":
			monday := weekStart(today)
			sinceTime = &monday
		case "this-month":
			firstDay := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
			sinceTime = &firstDay
		default:
			t, err := parseDate(since)
			if err != nil {
				return nil, nil, fmt.Errorf("since の日付形式が不正です: %w", err)
			}
			sinceTime = t
		}
	}

	var untilTime *time.Time
	if until != "" {
		switch until {
		case "today":
			untilTime = &today
		case "this-week":
			monday := weekStart(today)
			sunday := monday.AddDate(0, 0, 6)
			untilTime = &sunday
		case "this-month":
			lastDay := time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, today.Location())
			untilTime = &lastDay
		default:
			t, err := parseDate(until)
			if err != nil {
				return nil, nil, fmt.Errorf("until の日付形式が不正です: %w", err)
			}
			untilTime = t
		}
	}

	return sinceTime, untilTime, nil
}

// weekStart は月曜始まりの週開始日（月曜日）を返す。
// Go の time.Weekday(): Sunday=0, Monday=1, ..., Saturday=6
func weekStart(t time.Time) time.Time {
	weekday := t.Weekday()
	var offset int
	if weekday == time.Sunday {
		offset = -6
	} else {
		offset = -int(weekday - time.Monday)
	}
	return t.AddDate(0, 0, offset)
}

// parseDateRange はコロン区切りの日付範囲文字列をパースする。
// "A:B" → Since=A, Until=B
// "A:" → Since=A, Until=nil
// ":B" → Since=nil, Until=B
// ":" → エラー（両側空）
func parseDateRange(input string) (*time.Time, *time.Time, error) {
	parts := strings.SplitN(input, ":", 2)
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("コロン区切りの範囲指定が不正です: %q", input)
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if left == "" && right == "" {
		return nil, nil, fmt.Errorf("範囲指定 ':' の両側が空です")
	}
	var since, until *time.Time
	if left != "" {
		t, err := parseDate(left)
		if err != nil {
			return nil, nil, err
		}
		since = t
	}
	if right != "" {
		t, err := parseDate(right)
		if err != nil {
			return nil, nil, err
		}
		until = t
	}
	return since, until, nil
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

// resolveTeamIDs は --team フラグの値（[]string）を TeamID の []int に変換する。
// 数値文字列 → そのまま ID
// 文字列 → ListTeams() で全チーム取得、名前で case-insensitive 部分一致
// 一致なし → エラー（利用可能なチーム名一覧を表示）
// 複数一致 → エラー
func resolveTeamIDs(ctx context.Context, inputs []string, client backlog.Client) ([]int, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	// 全て数値かチェック、非数値があれば ListTeams で名前解決が必要
	ids := make([]int, 0, len(inputs))
	var nameInputs []string
	nameInputIdx := make(map[string]int) // 名前 → inputs スライス中のインデックス

	for _, input := range inputs {
		if id, err := strconv.Atoi(input); err == nil {
			ids = append(ids, id)
		} else {
			nameInputs = append(nameInputs, input)
			nameInputIdx[input] = len(ids) // 後でマージするため位置を記録
		}
	}

	if len(nameInputs) == 0 {
		return ids, nil
	}

	// 名前解決が必要: ListTeams を呼び出す
	teams, err := client.ListTeams(ctx)
	if err != nil {
		return nil, fmt.Errorf("チーム一覧の取得に失敗: %w", err)
	}

	// チーム名の一覧を作成（エラーメッセージ用）
	teamNames := make([]string, len(teams))
	for i, t := range teams {
		teamNames[i] = t.Name
	}

	// 各名前入力を解決し、元の入力順序を維持するため一時マップに格納
	resolvedNames := make(map[string]int)
	for _, name := range nameInputs {
		var matched []domain.TeamWithMembers
		for _, t := range teams {
			if strings.Contains(strings.ToLower(t.Name), strings.ToLower(name)) {
				matched = append(matched, t)
			}
		}
		switch len(matched) {
		case 0:
			return nil, fmt.Errorf("チーム %q が見つかりません。利用可能: [%s]", name, strings.Join(teamNames, ", "))
		case 1:
			resolvedNames[name] = matched[0].ID
		default:
			// 完全一致を試みる
			var exactMatched []domain.TeamWithMembers
			for _, t := range matched {
				if strings.EqualFold(t.Name, name) {
					exactMatched = append(exactMatched, t)
				}
			}
			if len(exactMatched) == 1 {
				resolvedNames[name] = exactMatched[0].ID
			} else {
				return nil, fmt.Errorf("チーム %q に複数一致しました", name)
			}
		}
	}

	// 元の入力順序で結果を組み立てる
	result := make([]int, 0, len(inputs))
	for _, input := range inputs {
		if id, err := strconv.Atoi(input); err == nil {
			result = append(result, id)
		} else {
			result = append(result, resolvedNames[input])
		}
	}
	return result, nil
}

// extractProjectKey は issueKey（例: "HEP_ISSUES-123"）からプロジェクトキーを抽出する。
func extractProjectKey(issueKey string) string {
	idx := strings.LastIndex(issueKey, "-")
	if idx <= 0 {
		return issueKey
	}
	return issueKey[:idx]
}

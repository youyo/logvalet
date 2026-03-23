package digest

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	"golang.org/x/sync/errgroup"
)

// UnifiedDigestScope は UnifiedDigestBuilder.Build() のスコープ指定。
type UnifiedDigestScope struct {
	// ProjectKeys はプロジェクトキーのスライス（複数指定可）。
	ProjectKeys []string
	// ProjectIDs はプロジェクトIDのスライス。ProjectKeys から解決後に格納する。
	ProjectIDs []int
	// UserIDs はユーザーIDのスライス（複数指定可）。
	UserIDs []int
	// TeamIDs はチームIDのスライス（複数指定可）。メンバーは内部で展開する。
	TeamIDs []int
	// IssueKeys は課題キーのスライス（複数指定可）。
	IssueKeys []string
	// Since は期間開始日時（inclusive）。nil の場合は制限なし。
	Since *time.Time
	// Until は期間終了日時（inclusive）。nil の場合は制限なし。
	Until *time.Time
}

// UnifiedDigestBuilder は統一 digest コマンドの Builder。
type UnifiedDigestBuilder struct {
	BaseDigestBuilder
}

// NewUnifiedDigestBuilder は UnifiedDigestBuilder を生成する。
func NewUnifiedDigestBuilder(client backlog.Client, profile, space, baseURL string) *UnifiedDigestBuilder {
	return &UnifiedDigestBuilder{BaseDigestBuilder{client: client, profile: profile, space: space, baseURL: baseURL}}
}

// ---- 出力型定義 ----

// UnifiedDigestScopeOutput は digest 出力の scope フィールド。
type UnifiedDigestScopeOutput struct {
	Projects []DigestProject `json:"projects"`
	Users    []DigestUser    `json:"users"`
	Teams    []DigestTeamRef `json:"teams"`
	Issues   []string        `json:"issues"`
	Since    string          `json:"since,omitempty"`
	Until    string          `json:"until,omitempty"`
}

// DigestUser は digest 内のユーザー参照。
type DigestUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// DigestTeamRef は digest 内のチーム参照。
type DigestTeamRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// UnifiedDigestSummary は統一 digest のサマリー。
type UnifiedDigestSummary struct {
	IssueCount    int                       `json:"issue_count"`
	ActivityCount int                       `json:"activity_count"`
	ByStatus      map[string]int            `json:"by_status"`
	ByUser        map[string]UserDigestStat `json:"by_user"`
	Period        DigestPeriod              `json:"period"`
}

// UserDigestStat はユーザーごとの課題/アクティビティ統計。
type UserDigestStat struct {
	Issues     int `json:"issues"`
	Activities int `json:"activities"`
}

// DigestPeriod は digest の期間情報。
type DigestPeriod struct {
	Since string `json:"since"`
	Until string `json:"until"`
}

// UnifiedDigest は UnifiedDigestBuilder の出力 digest 構造体。
type UnifiedDigest struct {
	Scope      UnifiedDigestScopeOutput `json:"scope"`
	Issues     []domain.Issue           `json:"issues"`
	Activities []interface{}            `json:"activities"`
	Summary    UnifiedDigestSummary     `json:"summary"`
	LLMHints   DigestLLMHints           `json:"llm_hints"`
}

// ---- Build ----

// Build は UnifiedDigestScope に従い DigestEnvelope を構築する。
// issues と activities は部分失敗を許容（warning 付き空スライス）。
func (b *UnifiedDigestBuilder) Build(ctx context.Context, scope UnifiedDigestScope) (*domain.DigestEnvelope, error) {
	var warnings []domain.Warning
	var mu sync.Mutex

	// 1. TeamIDs からメンバーを展開して UserIDs にマージ
	userIDs := make([]int, len(scope.UserIDs))
	copy(userIDs, scope.UserIDs)

	teamRefs := make([]DigestTeamRef, 0, len(scope.TeamIDs))
	for _, teamID := range scope.TeamIDs {
		team, err := b.client.GetTeam(ctx, teamID)
		if err != nil {
			mu.Lock()
			warnings = append(warnings, domain.Warning{
				Code:      "team_fetch_failed",
				Message:   fmt.Sprintf("チーム (ID=%d) の取得に失敗: %v", teamID, err),
				Component: fmt.Sprintf("scope.teams.%d", teamID),
				Retryable: true,
			})
			mu.Unlock()
			continue
		}
		teamRefs = append(teamRefs, DigestTeamRef{ID: team.ID, Name: team.Name})
		for _, m := range team.Members {
			userIDs = append(userIDs, m.ID)
		}
	}
	userIDs = uniqueIntSlice(userIDs)

	// 2. ProjectKeys → ProjectIDs 解決
	projectRefs := make([]DigestProject, 0, len(scope.ProjectKeys))
	projectIDs := make([]int, len(scope.ProjectIDs))
	copy(projectIDs, scope.ProjectIDs)

	for _, key := range scope.ProjectKeys {
		proj, err := b.client.GetProject(ctx, key)
		if err != nil {
			mu.Lock()
			warnings = append(warnings, domain.Warning{
				Code:      "project_fetch_failed",
				Message:   fmt.Sprintf("プロジェクト %q の解決に失敗: %v", key, err),
				Component: fmt.Sprintf("scope.projects.%s", key),
				Retryable: true,
			})
			mu.Unlock()
			continue
		}
		projectRefs = append(projectRefs, DigestProject{ID: proj.ID, Key: proj.ProjectKey, Name: proj.Name})
		projectIDs = append(projectIDs, proj.ID)
	}

	// 3. UserIDs → UserRefs 解決
	userRefs := make([]DigestUser, 0, len(userIDs))
	for _, uid := range userIDs {
		user, err := b.client.GetUser(ctx, strconv.Itoa(uid))
		if err != nil {
			// user の名前解決失敗は warning のみ（IDは分かっているので続行）
			mu.Lock()
			warnings = append(warnings, domain.Warning{
				Code:      "user_fetch_failed",
				Message:   fmt.Sprintf("ユーザー (ID=%d) の取得に失敗: %v", uid, err),
				Component: fmt.Sprintf("scope.users.%d", uid),
				Retryable: true,
			})
			mu.Unlock()
			userRefs = append(userRefs, DigestUser{ID: uid, Name: fmt.Sprintf("user:%d", uid)})
			continue
		}
		userRefs = append(userRefs, DigestUser{ID: user.ID, Name: user.Name})
	}

	// 4. issues 取得（自動ページング）
	var issues []domain.Issue
	issueOpt := backlog.ListIssuesOptions{
		ProjectIDs:   projectIDs,
		AssigneeIDs:  userIDs,
		UpdatedSince: scope.Since,
		UpdatedUntil: scope.Until,
		Limit:        100,
	}

	if len(scope.IssueKeys) > 0 {
		// issueKeys が指定されている場合は個別取得
		for _, key := range scope.IssueKeys {
			issue, err := b.client.GetIssue(ctx, key)
			if err != nil {
				mu.Lock()
				warnings = append(warnings, domain.Warning{
					Code:      "issue_fetch_failed",
					Message:   fmt.Sprintf("課題 %q の取得に失敗: %v", key, err),
					Component: fmt.Sprintf("issues.%s", key),
					Retryable: true,
				})
				mu.Unlock()
				continue
			}
			issues = append(issues, *issue)
		}
	} else {
		// ページング取得
		fetchedIssues, err := fetchAllIssuesPaged(ctx, b.client, issueOpt)
		if err != nil {
			warnings = append(warnings, domain.Warning{
				Code:      "issues_fetch_failed",
				Message:   fmt.Sprintf("課題一覧の取得に失敗: %v", err),
				Component: "issues",
				Retryable: true,
			})
			issues = []domain.Issue{}
		} else {
			issues = fetchedIssues
		}
	}
	if issues == nil {
		issues = []domain.Issue{}
	}

	// 5. activities 取得
	var activities []interface{}
	var actErr error

	if len(userIDs) > 0 {
		// ユーザーごとに並列取得してマージ
		activities, actErr = b.fetchUserActivities(ctx, userIDs, scope.Since, scope.Until)
	} else if len(scope.IssueKeys) > 0 {
		// issue scope → プロジェクトのアクティビティ
		activities, actErr = b.fetchIssueProjectActivities(ctx, scope.IssueKeys, scope.Since, scope.Until)
	} else if len(projectIDs) > 0 {
		// プロジェクトごとに並列取得してマージ
		activities, actErr = b.fetchProjectActivities(ctx, scope.ProjectKeys, scope.Since, scope.Until)
	} else {
		// スペース全体
		activities, actErr = b.fetchSpaceActivities(ctx, scope.Since, scope.Until)
	}

	if actErr != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "activities_fetch_failed",
			Message:   fmt.Sprintf("アクティビティの取得に失敗: %v", actErr),
			Component: "activities",
			Retryable: true,
		})
		activities = []interface{}{}
	}
	if activities == nil {
		activities = []interface{}{}
	}

	// 6. summary 構築
	summary := buildUnifiedSummary(issues, activities, scope.Since, scope.Until)

	// 7. LLMHints 構築
	hints := buildUnifiedLLMHints(projectRefs, userRefs)

	// 8. scope 出力構築
	issueKeysOutput := make([]string, len(scope.IssueKeys))
	copy(issueKeysOutput, scope.IssueKeys)

	scopeOut := UnifiedDigestScopeOutput{
		Projects: projectRefs,
		Users:    userRefs,
		Teams:    teamRefs,
		Issues:   issueKeysOutput,
	}
	if scope.Since != nil {
		scopeOut.Since = scope.Since.Format("2006-01-02")
	}
	if scope.Until != nil {
		scopeOut.Until = scope.Until.Format("2006-01-02")
	}

	digestData := &UnifiedDigest{
		Scope:      scopeOut,
		Issues:     issues,
		Activities: activities,
		Summary:    summary,
		LLMHints:   hints,
	}

	return b.newEnvelope("digest", digestData, warnings), nil
}

// ---- アクティビティ取得ヘルパー ----

// fetchUserActivities は複数ユーザーのアクティビティを並列取得してマージする。
func (b *UnifiedDigestBuilder) fetchUserActivities(ctx context.Context, userIDs []int, since, until *time.Time) ([]interface{}, error) {
	type result struct {
		activities []interface{}
		err        error
	}

	results := make([]result, len(userIDs))
	g, gctx := errgroup.WithContext(ctx)

	for i, uid := range userIDs {
		i, uid := i, uid
		g.Go(func() error {
			fetcher := func(ctx context.Context, count int, maxID int) ([]interface{}, error) {
				activities, err := b.client.ListUserActivities(ctx, strconv.Itoa(uid), backlog.ListUserActivitiesOptions{
					Limit: count,
				})
				if err != nil {
					return nil, err
				}
				return activitiesToInterface(activities), nil
			}
			acts, err := FetchActivitiesWithDateFilter(gctx, fetcher, FetchActivitiesOptions{
				Since: since,
				Until: until,
			})
			results[i] = result{activities: acts, err: err}
			return nil // エラーは results に格納
		})
	}

	_ = g.Wait()

	var all []interface{}
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		all = append(all, r.activities...)
	}
	return all, nil
}

// fetchProjectActivities は複数プロジェクトのアクティビティを並列取得してマージする。
func (b *UnifiedDigestBuilder) fetchProjectActivities(ctx context.Context, projectKeys []string, since, until *time.Time) ([]interface{}, error) {
	type result struct {
		activities []interface{}
		err        error
	}

	results := make([]result, len(projectKeys))
	g, gctx := errgroup.WithContext(ctx)

	for i, key := range projectKeys {
		i, key := i, key
		g.Go(func() error {
			fetcher := func(ctx context.Context, count int, maxID int) ([]interface{}, error) {
				activities, err := b.client.ListProjectActivities(ctx, key, backlog.ListActivitiesOptions{
					Limit: count,
				})
				if err != nil {
					return nil, err
				}
				return activitiesToInterface(activities), nil
			}
			acts, err := FetchActivitiesWithDateFilter(gctx, fetcher, FetchActivitiesOptions{
				Since: since,
				Until: until,
			})
			results[i] = result{activities: acts, err: err}
			return nil
		})
	}

	_ = g.Wait()

	var all []interface{}
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		all = append(all, r.activities...)
	}
	return all, nil
}

// fetchSpaceActivities はスペース全体のアクティビティを取得する。
func (b *UnifiedDigestBuilder) fetchSpaceActivities(ctx context.Context, since, until *time.Time) ([]interface{}, error) {
	fetcher := func(ctx context.Context, count int, maxID int) ([]interface{}, error) {
		activities, err := b.client.ListSpaceActivities(ctx, backlog.ListActivitiesOptions{
			Limit: count,
		})
		if err != nil {
			return nil, err
		}
		return activitiesToInterface(activities), nil
	}
	return FetchActivitiesWithDateFilter(ctx, fetcher, FetchActivitiesOptions{
		Since: since,
		Until: until,
	})
}

// fetchIssueProjectActivities は issue scope のとき、各 issueKey のプロジェクトアクティビティを取得する。
func (b *UnifiedDigestBuilder) fetchIssueProjectActivities(ctx context.Context, issueKeys []string, since, until *time.Time) ([]interface{}, error) {
	// issueKey からプロジェクトキーセットを抽出
	projectKeySet := make(map[string]struct{})
	for _, key := range issueKeys {
		pk := extractProjectKey(key)
		if pk != "" {
			projectKeySet[pk] = struct{}{}
		}
	}
	keys := make([]string, 0, len(projectKeySet))
	for k := range projectKeySet {
		keys = append(keys, k)
	}
	return b.fetchProjectActivities(ctx, keys, since, until)
}

// ---- summary / hints 構築ヘルパー ----

// buildUnifiedSummary は課題とアクティビティからサマリーを構築する。
func buildUnifiedSummary(issues []domain.Issue, activities []interface{}, since, until *time.Time) UnifiedDigestSummary {
	byStatus := make(map[string]int)
	for _, issue := range issues {
		if issue.Status != nil {
			byStatus[issue.Status.Name]++
		}
	}

	sinceStr := ""
	untilStr := ""
	if since != nil {
		sinceStr = since.Format("2006-01-02")
	}
	if until != nil {
		untilStr = until.Format("2006-01-02")
	}

	return UnifiedDigestSummary{
		IssueCount:    len(issues),
		ActivityCount: len(activities),
		ByStatus:      byStatus,
		ByUser:        map[string]UserDigestStat{},
		Period: DigestPeriod{
			Since: sinceStr,
			Until: untilStr,
		},
	}
}

// buildUnifiedLLMHints は LLM ヒントを構築する。
func buildUnifiedLLMHints(projects []DigestProject, users []DigestUser) DigestLLMHints {
	entities := make([]string, 0, len(projects)+len(users))
	for _, p := range projects {
		entities = append(entities, p.Key)
	}
	for _, u := range users {
		entities = append(entities, u.Name)
	}
	return DigestLLMHints{
		PrimaryEntities:      entities,
		OpenQuestions:        []string{},
		SuggestedNextActions: []string{},
	}
}

// ---- ユーティリティ ----

// fetchAllIssuesPaged は自動ページングで全課題を取得する（最大 10,000 件）。
func fetchAllIssuesPaged(ctx context.Context, client backlog.Client, opt backlog.ListIssuesOptions) ([]domain.Issue, error) {
	const maxTotal = 10000
	if opt.Limit <= 0 {
		opt.Limit = 100
	}
	var all []domain.Issue
	for {
		page, err := client.ListIssues(ctx, opt)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < opt.Limit || len(all) >= maxTotal {
			break
		}
		opt.Offset += opt.Limit
	}
	if len(all) > maxTotal {
		all = all[:maxTotal]
	}
	return all, nil
}

// activitiesToInterface は []domain.Activity を []interface{} に変換する。
// FetchActivitiesWithDateFilter は interface{} スライスを期待するため変換が必要。
// domain.Activity の全フィールド（id, type, content, createdUser, created）を保持する。
func activitiesToInterface(activities []domain.Activity) []interface{} {
	result := make([]interface{}, 0, len(activities))
	for _, a := range activities {
		m := map[string]interface{}{
			"id":   int(a.ID),
			"type": a.Type,
		}
		if a.Created != nil {
			m["created"] = a.Created
		}
		if a.Content != nil {
			m["content"] = a.Content
		}
		if a.CreatedUser != nil {
			m["createdUser"] = a.CreatedUser
		}
		result = append(result, m)
	}
	return result
}

// uniqueIntSlice は重複を除去した int スライスを返す（入力順を保持）。
func uniqueIntSlice(ids []int) []int {
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

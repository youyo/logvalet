package analysis

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// DefaultTopN は top_active_actors/types のデフォルト表示件数。
const DefaultTopN = 5

// ActivityStatsOptions は ActivityStatsBuilder.Build() のオプション。
type ActivityStatsOptions struct {
	// Scope はアクティビティ取得のスコープ（"project" / "user" / "space"）。
	// 空文字列の場合は "space" として動作する。
	Scope string
	// ScopeKey はプロジェクトキー（scope="project"）またはユーザーID（scope="user"）。
	ScopeKey string
	// Since は集計開始日時（nil: now - 7days）。
	Since *time.Time
	// Until は集計終了日時（nil: now）。
	Until *time.Time
	// TopN は top_active_actors/types の表示件数（0: デフォルト 5）。
	TopN int
}

// ActivityStats は activity_stats の analysis フィールドの型。
type ActivityStats struct {
	Scope           string            `json:"scope"`
	ScopeKey        string            `json:"scope_key,omitempty"`
	Since           time.Time         `json:"since"`
	Until           time.Time         `json:"until"`
	TotalCount      int               `json:"total_count"`
	ByType          map[string]int    `json:"by_type"`
	ByActor         map[string]int    `json:"by_actor"`
	ByDate          map[string]int    `json:"by_date"`
	ByHour          map[int]int       `json:"by_hour"`
	TopActiveActors []ActorStat       `json:"top_active_actors"`
	TopActiveTypes  []TypeStat        `json:"top_active_types"`
	DateRange       ActivityDateRange `json:"date_range"`
	Patterns        ActivityPatterns  `json:"patterns"`
}

// ActorStat はアクター別アクティビティ統計。
type ActorStat struct {
	Name  string  `json:"name"`
	Count int     `json:"count"`
	Ratio float64 `json:"ratio"`
}

// TypeStat はタイプ別アクティビティ統計。
type TypeStat struct {
	Type  string  `json:"type"`
	Count int     `json:"count"`
	Ratio float64 `json:"ratio"`
}

// ActivityDateRange は集計期間。
type ActivityDateRange struct {
	Since time.Time `json:"since"`
	Until time.Time `json:"until"`
}

// ActivityPatterns はアクティビティパターン分析。
type ActivityPatterns struct {
	PeakHour           int     `json:"peak_hour"`
	PeakDayOfWeek      string  `json:"peak_day_of_week"`
	ActorConcentration float64 `json:"actor_concentration"`
	TypeConcentration  float64 `json:"type_concentration"`
}

// ActivityStatsBuilder はアクティビティ統計を集計する。
type ActivityStatsBuilder struct {
	BaseAnalysisBuilder
}

// NewActivityStatsBuilder は ActivityStatsBuilder を生成する。
func NewActivityStatsBuilder(client backlog.Client, profile, space, baseURL string, opts ...Option) *ActivityStatsBuilder {
	return &ActivityStatsBuilder{
		BaseAnalysisBuilder: NewBaseAnalysisBuilder(client, profile, space, baseURL, opts...),
	}
}

// Build は指定オプションでアクティビティ統計を構築して AnalysisEnvelope を返す。
func (b *ActivityStatsBuilder) Build(ctx context.Context, opt ActivityStatsOptions) (*AnalysisEnvelope, error) {
	now := b.now()

	// デフォルト値の解決
	topN := opt.TopN
	if topN <= 0 {
		topN = DefaultTopN
	}

	scope := opt.Scope
	if scope == "" {
		scope = "space"
	}

	since := now.AddDate(0, 0, -7)
	if opt.Since != nil {
		since = *opt.Since
	}
	until := now
	if opt.Until != nil {
		until = *opt.Until
	}

	var warnings []domain.Warning
	var activities []domain.Activity

	switch scope {
	case "project":
		fetcher := func(ctx context.Context, maxId int) ([]domain.Activity, error) {
			listOpt := backlog.ListActivitiesOptions{Count: 100}
			if maxId > 0 {
				listOpt.MaxId = maxId
			}
			return b.client.ListProjectActivities(ctx, opt.ScopeKey, listOpt)
		}
		acts, err := b.fetchActivitiesForStats(ctx, fetcher, since, until)
		if err != nil {
			warnings = append(warnings, domain.Warning{
				Code:      "activities_fetch_failed",
				Message:   fmt.Sprintf("failed to list project activities for %s: %v", opt.ScopeKey, err),
				Component: "activities",
				Retryable: true,
			})
			activities = []domain.Activity{}
		} else {
			activities = acts
		}

	case "user":
		fetcher := func(ctx context.Context, maxId int) ([]domain.Activity, error) {
			listOpt := backlog.ListUserActivitiesOptions{Count: 100}
			if maxId > 0 {
				listOpt.MaxId = maxId
			}
			return b.client.ListUserActivities(ctx, opt.ScopeKey, listOpt)
		}
		acts, err := b.fetchActivitiesForStats(ctx, fetcher, since, until)
		if err != nil {
			warnings = append(warnings, domain.Warning{
				Code:      "activities_fetch_failed",
				Message:   fmt.Sprintf("failed to list user activities for %s: %v", opt.ScopeKey, err),
				Component: "activities",
				Retryable: true,
			})
			activities = []domain.Activity{}
		} else {
			activities = acts
		}

	default: // "space" or empty
		fetcher := func(ctx context.Context, maxId int) ([]domain.Activity, error) {
			listOpt := backlog.ListActivitiesOptions{Count: 100}
			if maxId > 0 {
				listOpt.MaxId = maxId
			}
			return b.client.ListSpaceActivities(ctx, listOpt)
		}
		acts, err := b.fetchActivitiesForStats(ctx, fetcher, since, until)
		if err != nil {
			warnings = append(warnings, domain.Warning{
				Code:      "activities_fetch_failed",
				Message:   fmt.Sprintf("failed to list space activities: %v", err),
				Component: "activities",
				Retryable: true,
			})
			activities = []domain.Activity{}
		} else {
			activities = acts
		}
	}

	stats := buildActivityStats(activities, scope, opt.ScopeKey, since, until, topN)
	return b.newEnvelope("activity_stats", stats, warnings), nil
}

// fetchActivitiesForStats は maxId ベースでページングしつつ since/until でローカルフィルタする。
func (b *ActivityStatsBuilder) fetchActivitiesForStats(
	ctx context.Context,
	fetcher func(ctx context.Context, maxId int) ([]domain.Activity, error),
	since, until time.Time,
) ([]domain.Activity, error) {
	var result []domain.Activity
	maxId := 0

	for {
		batch, err := fetcher(ctx, maxId)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}

		reachedSince := false
		for _, a := range batch {
			if a.Created != nil {
				if a.Created.Before(since) {
					reachedSince = true
					break
				}
				if a.Created.After(until) {
					continue
				}
			}
			result = append(result, a)
		}

		if reachedSince || len(batch) < 100 {
			break
		}

		lastID := batch[len(batch)-1].ID
		maxId = int(lastID) - 1
		if maxId <= 0 {
			break
		}
	}

	return result, nil
}

// buildActivityStats は activities から ActivityStats を構築する。
func buildActivityStats(activities []domain.Activity, scope, scopeKey string, since, until time.Time, topN int) *ActivityStats {
	total := len(activities)

	byType := map[string]int{}
	byActor := map[string]int{}

	for i := range activities {
		a := &activities[i]

		// by_type
		typeName := activityTypeNameForAnalysis(a.Type)
		byType[typeName]++

		// by_actor
		actor := "unknown"
		if a.CreatedUser != nil && a.CreatedUser.Name != "" {
			actor = a.CreatedUser.Name
		}
		byActor[actor]++
	}

	byDate := buildByDate(activities)
	byHour := buildByHour(activities)

	topActors := buildTopActors(byActor, total, topN)
	topTypes := buildTopTypes(byType, total, topN)

	patterns := buildPatterns(activities, byActor, byType, total)

	return &ActivityStats{
		Scope:           scope,
		ScopeKey:        scopeKey,
		Since:           since,
		Until:           until,
		TotalCount:      total,
		ByType:          byType,
		ByActor:         byActor,
		ByDate:          byDate,
		ByHour:          byHour,
		TopActiveActors: topActors,
		TopActiveTypes:  topTypes,
		DateRange: ActivityDateRange{
			Since: since,
			Until: until,
		},
		Patterns: patterns,
	}
}

// buildByDate は日付別カウントマップを返す（キー: "YYYY-MM-DD" UTC）。
func buildByDate(activities []domain.Activity) map[string]int {
	result := map[string]int{}
	for i := range activities {
		a := &activities[i]
		if a.Created == nil {
			continue
		}
		key := a.Created.UTC().Format("2006-01-02")
		result[key]++
	}
	return result
}

// buildByHour は時間帯別カウントマップを返す（キー: 0-23）。
func buildByHour(activities []domain.Activity) map[int]int {
	result := map[int]int{}
	for i := range activities {
		a := &activities[i]
		if a.Created == nil {
			continue
		}
		h := a.Created.UTC().Hour()
		result[h]++
	}
	return result
}

// buildTopActors はアクター別カウントからトップ N のアクター統計を返す。
func buildTopActors(byActor map[string]int, total, topN int) []ActorStat {
	if len(byActor) == 0 {
		return []ActorStat{}
	}

	actors := make([]ActorStat, 0, len(byActor))
	for name, count := range byActor {
		ratio := 0.0
		if total > 0 {
			ratio = float64(count) / float64(total)
		}
		actors = append(actors, ActorStat{Name: name, Count: count, Ratio: ratio})
	}

	// count 降順 → name 昇順でソート（安定性）
	sortActorStats(actors)

	if topN < len(actors) {
		actors = actors[:topN]
	}
	return actors
}

// buildTopTypes はタイプ別カウントからトップ N のタイプ統計を返す。
func buildTopTypes(byType map[string]int, total, topN int) []TypeStat {
	if len(byType) == 0 {
		return []TypeStat{}
	}

	types := make([]TypeStat, 0, len(byType))
	for typeName, count := range byType {
		ratio := 0.0
		if total > 0 {
			ratio = float64(count) / float64(total)
		}
		types = append(types, TypeStat{Type: typeName, Count: count, Ratio: ratio})
	}

	// count 降順 → type 昇順でソート
	sortTypeStats(types)

	if topN < len(types) {
		types = types[:topN]
	}
	return types
}

// buildPatterns はアクティビティパターンを分析して ActivityPatterns を返す。
func buildPatterns(activities []domain.Activity, byActor, byType map[string]int, total int) ActivityPatterns {
	if total == 0 {
		return ActivityPatterns{}
	}

	byHour := buildByHour(activities)
	peakHour := calcPeakHour(byHour)
	peakDOW := calcPeakDayOfWeek(activities)
	actorConc := calcConcentration(byActor, total)
	typeConc := calcConcentration(byType, total)

	return ActivityPatterns{
		PeakHour:           peakHour,
		PeakDayOfWeek:      peakDOW,
		ActorConcentration: actorConc,
		TypeConcentration:  typeConc,
	}
}

// calcConcentration は counts における上位3件の合計 / total を返す（0.0〜1.0）。
func calcConcentration(counts map[string]int, total int) float64 {
	if total == 0 || len(counts) == 0 {
		return 0.0
	}

	vals := make([]int, 0, len(counts))
	for _, v := range counts {
		vals = append(vals, v)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(vals)))

	top3 := 0
	for i := 0; i < 3 && i < len(vals); i++ {
		top3 += vals[i]
	}

	return float64(top3) / float64(total)
}

// calcPeakHour は byHour マップから最もアクティビティが多い時間帯を返す。
func calcPeakHour(byHour map[int]int) int {
	if len(byHour) == 0 {
		return 0
	}
	peak := -1
	maxCount := -1
	for h, c := range byHour {
		if c > maxCount || (c == maxCount && h < peak) {
			maxCount = c
			peak = h
		}
	}
	return peak
}

// calcPeakDayOfWeek は activities から最もアクティビティが多い曜日を "Mon" 等の形式で返す。
func calcPeakDayOfWeek(activities []domain.Activity) string {
	bydow := map[time.Weekday]int{}
	for i := range activities {
		a := &activities[i]
		if a.Created == nil {
			continue
		}
		bydow[a.Created.UTC().Weekday()]++
	}

	if len(bydow) == 0 {
		return ""
	}

	var peakDOW time.Weekday
	maxCount := -1
	for dow, c := range bydow {
		if c > maxCount || (c == maxCount && int(dow) < int(peakDOW)) {
			maxCount = c
			peakDOW = dow
		}
	}

	// time.Weekday.String() は "Monday" 形式なので "Mon" に変換
	return peakDOW.String()[:3]
}

// sortActorStats は ActorStat を count 降順 → name 昇順でソートする。
func sortActorStats(actors []ActorStat) {
	sort.SliceStable(actors, func(i, j int) bool {
		if actors[i].Count != actors[j].Count {
			return actors[i].Count > actors[j].Count
		}
		return actors[i].Name < actors[j].Name
	})
}

// sortTypeStats は TypeStat を count 降順 → type 昇順でソートする。
func sortTypeStats(types []TypeStat) {
	sort.SliceStable(types, func(i, j int) bool {
		if types[i].Count != types[j].Count {
			return types[i].Count > types[j].Count
		}
		return types[i].Type < types[j].Type
	})
}

// activityTypeNameForAnalysis は Backlog の activity type 番号を文字列名に変換する（spec §12）。
// digest/activity.go の activityTypeName のコピー（analysis パッケージ内プライベート関数のため）。
func activityTypeNameForAnalysis(t int) string {
	switch t {
	case 1:
		return "issue_created"
	case 2:
		return "issue_updated"
	case 3:
		return "issue_commented"
	case 4:
		return "issue_deleted"
	case 5:
		return "wiki_created"
	case 6:
		return "wiki_updated"
	case 7:
		return "wiki_deleted"
	case 8:
		return "file_added"
	case 9:
		return "file_updated"
	case 10:
		return "file_deleted"
	case 11:
		return "svn_committed"
	case 12:
		return "git_pushed"
	case 13:
		return "git_created"
	case 14:
		return "issue_multi_updated"
	case 15:
		return "project_user_added"
	case 16:
		return "project_user_deleted"
	case 17:
		return "comment_notification"
	case 18:
		return "pr_created"
	case 19:
		return "pr_updated"
	case 20:
		return "pr_commented"
	case 21:
		return "pr_merged"
	default:
		return fmt.Sprintf("unknown_%d", t)
	}
}

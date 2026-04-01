package analysis

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	"golang.org/x/sync/errgroup"
)

// DefaultMaxActivityPages は ListProjectActivities ページネーションのデフォルト上限。
const DefaultMaxActivityPages = 5

// DefaultActivityPageLimit は1回の ListProjectActivities リクエストで取得する最大件数。
const DefaultActivityPageLimit = 100

// TimelineEventKind はタイムラインイベントの種類。
type TimelineEventKind string

const (
	// TimelineEventKindComment はコメントイベント。
	TimelineEventKindComment TimelineEventKind = "comment"
	// TimelineEventKindUpdate は更新イベント（issue_updated, issue_multi_updated）。
	TimelineEventKindUpdate TimelineEventKind = "update"
	// TimelineEventKindCreated は課題作成イベント（issue_created）。
	TimelineEventKindCreated TimelineEventKind = "created"
)

// timelineKindPriority はソート時の kind 優先度（小さいほど前）。
var timelineKindPriority = map[TimelineEventKind]int{
	TimelineEventKindCreated: 0,
	TimelineEventKindUpdate:  1,
	TimelineEventKindComment: 2,
}

// TimelineChange は課題フィールドの変更情報。
type TimelineChange struct {
	Field    string `json:"field"`
	OldValue string `json:"old_value,omitempty"`
	NewValue string `json:"new_value,omitempty"`
}

// TimelineEvent は統一タイムラインイベント型。
type TimelineEvent struct {
	ID           int64             `json:"id"`
	Kind         TimelineEventKind `json:"kind"`
	ActivityType string            `json:"activity_type,omitempty"`
	Timestamp    *time.Time        `json:"timestamp,omitempty"`
	Actor        *domain.UserRef   `json:"actor,omitempty"`
	Content      string            `json:"content,omitempty"`
	Changes      []TimelineChange  `json:"changes,omitempty"`
}

// TimelineMeta はタイムラインのメタ情報。
type TimelineMeta struct {
	TotalEvents      int `json:"total_events"`
	CommentCount     int `json:"comment_count"`
	UpdateCount      int `json:"update_count"`
	ParticipantCount int `json:"participant_count"`
}

// CommentTimeline は課題のタイムライン分析結果。
type CommentTimeline struct {
	IssueKey     string          `json:"issue_key"`
	IssueSummary string          `json:"issue_summary"`
	Events       []TimelineEvent `json:"events"`
	Meta         TimelineMeta    `json:"meta"`
}

// CommentTimelineOptions は CommentTimelineBuilder.Build() のオプション。
type CommentTimelineOptions struct {
	// MaxComments は表示するコメント最大数（0 = 全件）。
	MaxComments int
	// IncludeUpdates は更新履歴を含めるか（nil = デフォルト true）。
	IncludeUpdates *bool
	// MaxActivityPages は ListProjectActivities のページネーション上限（デフォルト: 5）。
	MaxActivityPages int
	// Since は取得開始時刻（nil = 制限なし）。
	Since *time.Time
	// Until は取得終了時刻（nil = 制限なし）。
	Until *time.Time
}

// CommentTimelineBuilder は CommentTimeline を構築する。
type CommentTimelineBuilder struct {
	BaseAnalysisBuilder
}

// NewCommentTimelineBuilder は CommentTimelineBuilder を生成する。
func NewCommentTimelineBuilder(client backlog.Client, profile, space, baseURL string, opts ...Option) *CommentTimelineBuilder {
	return &CommentTimelineBuilder{
		BaseAnalysisBuilder: NewBaseAnalysisBuilder(client, profile, space, baseURL, opts...),
	}
}

// Build は issueKey の CommentTimeline を構築する。
// GetIssue の失敗は error を返す（必須）。
// ListIssueComments / ListProjectActivities の失敗は warnings に追加して部分結果を返す。
func (b *CommentTimelineBuilder) Build(ctx context.Context, issueKey string, opt CommentTimelineOptions) (*AnalysisEnvelope, error) {
	// デフォルト値の補完
	includeUpdates := true
	if opt.IncludeUpdates != nil {
		includeUpdates = *opt.IncludeUpdates
	}
	if opt.MaxActivityPages <= 0 {
		opt.MaxActivityPages = DefaultMaxActivityPages
	}

	// 1. 対象課題取得（必須）
	issue, err := b.client.GetIssue(ctx, issueKey)
	if err != nil {
		return nil, fmt.Errorf("get issue %s: %w", issueKey, err)
	}

	projectKey := extractProjectKey(issueKey)

	var (
		comments   []domain.Comment
		activities []domain.Activity
		mu         sync.Mutex
		warnings   []domain.Warning
	)

	g, gctx := errgroup.WithContext(ctx)

	// goroutine 1: ListIssueComments
	g.Go(func() error {
		cs, err := b.client.ListIssueComments(gctx, issueKey, backlog.ListCommentsOptions{})
		if err != nil {
			mu.Lock()
			warnings = append(warnings, domain.Warning{
				Code:      "comments_fetch_failed",
				Message:   fmt.Sprintf("failed to list comments for %s: %v", issueKey, err),
				Component: "events",
				Retryable: true,
			})
			mu.Unlock()
			return nil
		}
		mu.Lock()
		comments = cs
		mu.Unlock()
		return nil
	})

	// goroutine 2: ListProjectActivities（ページネーション）
	if includeUpdates {
		g.Go(func() error {
			result, truncated, fetchErr := b.fetchActivities(gctx, projectKey, opt)
			mu.Lock()
			defer mu.Unlock()
			if fetchErr != nil {
				warnings = append(warnings, domain.Warning{
					Code:      "activities_fetch_failed",
					Message:   fmt.Sprintf("failed to list project activities for %s: %v", projectKey, fetchErr),
					Component: "events",
					Retryable: true,
				})
				return nil
			}
			activities = result
			if truncated {
				warnings = append(warnings, domain.Warning{
					Code:      "activity_pagination_truncated",
					Message:   fmt.Sprintf("activity fetch truncated at %d pages (%d activities)", opt.MaxActivityPages, len(result)),
					Component: "events",
					Retryable: false,
				})
			}
			return nil
		})
	}

	_ = g.Wait()

	// 3. イベントをマージ・ソート
	events, changeWarnings := mergeAndSortEvents(issue, comments, activities, opt, includeUpdates)
	mu.Lock()
	warnings = append(warnings, changeWarnings...)
	mu.Unlock()

	// 4. meta を構築
	meta := buildTimelineMeta(events)

	ct := &CommentTimeline{
		IssueKey:     issue.IssueKey,
		IssueSummary: issue.Summary,
		Events:       events,
		Meta:         meta,
	}

	return b.newEnvelope("issue_timeline", ct, warnings), nil
}

// fetchActivities は ListProjectActivities をページネーションで取得する。
// 返値: (activities, truncated, error)
func (b *CommentTimelineBuilder) fetchActivities(ctx context.Context, projectKey string, opt CommentTimelineOptions) ([]domain.Activity, bool, error) {
	var result []domain.Activity
	offset := 0
	truncated := false

	for page := 0; page < opt.MaxActivityPages; page++ {
		batch, err := b.client.ListProjectActivities(ctx, projectKey, backlog.ListActivitiesOptions{
			Limit:  DefaultActivityPageLimit,
			Offset: offset,
		})
		if err != nil {
			return nil, false, err
		}

		result = append(result, batch...)
		offset += len(batch)

		// 取得件数が limit 未満 → これ以上ない
		if len(batch) < DefaultActivityPageLimit {
			break
		}

		// まだページが続くが上限に達した
		if page == opt.MaxActivityPages-1 {
			truncated = true
		}
	}

	return result, truncated, nil
}

// mergeAndSortEvents はコメントとアクティビティを統合し時系列昇順にソートする。
// 返値: (events, changeWarnings)
func mergeAndSortEvents(issue *domain.Issue, comments []domain.Comment, activities []domain.Activity, opt CommentTimelineOptions, includeUpdates bool) ([]TimelineEvent, []domain.Warning) {
	var events []TimelineEvent
	var warnings []domain.Warning

	// コメントイベントの変換
	commentEvents := buildCommentEvents(comments, opt)
	events = append(events, commentEvents...)

	// アクティビティイベントの変換
	if includeUpdates {
		actEvents, w := buildActivityEvents(issue, activities, opt)
		events = append(events, actEvents...)
		warnings = append(warnings, w...)
	}

	if events == nil {
		events = []TimelineEvent{}
	}

	// 時系列昇順ソート（同時刻は kind priority → id asc）
	sort.SliceStable(events, func(i, j int) bool {
		ti := events[i].Timestamp
		tj := events[j].Timestamp

		// nil timestamp は末尾
		if ti == nil && tj == nil {
			return events[i].ID < events[j].ID
		}
		if ti == nil {
			return false
		}
		if tj == nil {
			return true
		}

		if ti.Equal(*tj) {
			pi := timelineKindPriority[events[i].Kind]
			pj := timelineKindPriority[events[j].Kind]
			if pi != pj {
				return pi < pj
			}
			return events[i].ID < events[j].ID
		}
		return ti.Before(*tj)
	})

	return events, warnings
}

// buildCommentEvents はコメントスライスから TimelineEvent スライスを構築する。
func buildCommentEvents(comments []domain.Comment, opt CommentTimelineOptions) []TimelineEvent {
	if len(comments) == 0 {
		return nil
	}

	var result []TimelineEvent
	for _, c := range comments {
		// Since/Until フィルタ
		if c.Created != nil {
			if opt.Since != nil && c.Created.Before(*opt.Since) {
				continue
			}
			if opt.Until != nil && c.Created.After(*opt.Until) {
				continue
			}
		}

		event := TimelineEvent{
			ID:        c.ID,
			Kind:      TimelineEventKindComment,
			Timestamp: c.Created,
			Actor:     toUserRef(c.CreatedUser),
			Content:   c.Content,
		}
		result = append(result, event)
	}

	// MaxComments 制限（0 = 全件）
	if opt.MaxComments > 0 && len(result) > opt.MaxComments {
		result = result[:opt.MaxComments]
	}

	return result
}

// buildActivityEvents はアクティビティスライスから TimelineEvent スライスを構築する。
// 対象課題（issue.ID）に一致するもの、かつ type=1,2,14 のみを含む。
func buildActivityEvents(issue *domain.Issue, activities []domain.Activity, opt CommentTimelineOptions) ([]TimelineEvent, []domain.Warning) {
	var result []TimelineEvent
	var warnings []domain.Warning

	for _, a := range activities {
		// type チェック（1: created, 2: updated, 14: multi_updated のみ）
		if a.Type != 1 && a.Type != 2 && a.Type != 14 {
			continue
		}

		// content["id"] でフィルタ
		if !matchesIssueID(a.Content, issue.ID) {
			continue
		}

		// Since/Until フィルタ
		if a.Created != nil {
			if opt.Since != nil && a.Created.Before(*opt.Since) {
				continue
			}
			if opt.Until != nil && a.Created.After(*opt.Until) {
				continue
			}
		}

		kind := activityKind(a.Type)
		typeName := timelineActivityTypeName(a.Type)

		// changes の抽出
		changes, parseOK := extractChanges(a)
		if !parseOK {
			warnings = append(warnings, domain.Warning{
				Code:      "activity_changes_parse_failed",
				Message:   fmt.Sprintf("failed to parse changes for activity id=%d", a.ID),
				Component: "events",
				Retryable: false,
			})
		}

		event := TimelineEvent{
			ID:           a.ID,
			Kind:         kind,
			ActivityType: typeName,
			Timestamp:    a.Created,
			Actor:        toUserRef(a.CreatedUser),
			Changes:      changes,
		}
		result = append(result, event)
	}

	return result, warnings
}

// matchesIssueID は activity.Content["id"] が issueID に一致するかを判定する。
// JSON number 型の揺れ（float64, int, int64, json.Number）を吸収する。
func matchesIssueID(content map[string]interface{}, issueID int) bool {
	if content == nil {
		return false
	}
	raw, ok := content["id"]
	if !ok {
		return false
	}
	id := extractIntFromContent(raw)
	return id == int64(issueID)
}

// extractIntFromContent は interface{} から int64 を抽出する（型揺れ対応）。
func extractIntFromContent(v interface{}) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int:
		return int64(val)
	case int64:
		return val
	case int32:
		return int64(val)
	default:
		return -1
	}
}

// activityKind は activity type から TimelineEventKind を返す。
func activityKind(t int) TimelineEventKind {
	if t == 1 {
		return TimelineEventKindCreated
	}
	return TimelineEventKindUpdate
}

// timelineActivityTypeName は activity type 番号を文字列名に変換する。
func timelineActivityTypeName(t int) string {
	switch t {
	case 1:
		return "issue_created"
	case 2:
		return "issue_updated"
	case 14:
		return "issue_multi_updated"
	default:
		return "unknown"
	}
}

// extractChanges は activity.Content["changes"] から []TimelineChange を抽出する。
// 返値: (changes, parseOK)
// parseOK=false の場合は changes=nil で warning を出すこと。
func extractChanges(a domain.Activity) ([]TimelineChange, bool) {
	if a.Content == nil {
		return nil, true
	}
	raw, ok := a.Content["changes"]
	if !ok || raw == nil {
		return nil, true
	}

	// []interface{} にアサート
	slice, ok := raw.([]interface{})
	if !ok {
		return nil, false // malformed
	}

	var result []TimelineChange
	for _, item := range slice {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		change := TimelineChange{
			Field:    stringFromMap(m, "field"),
			OldValue: stringFromMap(m, "old_value"),
			NewValue: stringFromMap(m, "new_value"),
		}
		result = append(result, change)
	}
	return result, true
}

// stringFromMap は map から文字列値を取得する（キーが存在しない場合は空文字）。
func stringFromMap(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

// buildTimelineMeta は TimelineEvent スライスから TimelineMeta を構築する。
func buildTimelineMeta(events []TimelineEvent) TimelineMeta {
	meta := TimelineMeta{
		TotalEvents: len(events),
	}

	participantIDs := make(map[int]struct{})
	for _, e := range events {
		switch e.Kind {
		case TimelineEventKindComment:
			meta.CommentCount++
		case TimelineEventKindUpdate, TimelineEventKindCreated:
			meta.UpdateCount++
		}
		if e.Actor != nil {
			participantIDs[e.Actor.ID] = struct{}{}
		}
	}
	meta.ParticipantCount = len(participantIDs)
	return meta
}

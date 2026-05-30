package backlog

import (
	"context"
	"time"

	"github.com/youyo/logvalet/internal/domain"
)

// FetchUserActivities は Since/Until に基づいてページネーションしながらアクティビティを取得する。
// Backlog API はアクティビティの日付フィルタを持たないため、maxId によるカーソルページネーションと
// クライアントサイドの日付フィルタリングで対応する。
// opt には ActivityTypeIDs など追加のフィルタを指定できる。
func FetchUserActivities(ctx context.Context, client Client, userID string, since, until *time.Time, limit int, opt ...ListUserActivitiesOptions) ([]domain.Activity, error) {
	const batchSize = 100

	// オプションをマージ（追加フィールド: ActivityTypeIDs 等）
	baseOpt := ListUserActivitiesOptions{}
	if len(opt) > 0 {
		baseOpt = opt[0]
	}

	var result []domain.Activity
	var maxID int64 = 0

	for {
		// no-date-filter かつ limit < batchSize のときは1回で済むよう件数を絞る
		count := batchSize
		if since == nil && until == nil && limit > 0 && limit < batchSize {
			count = limit
		}

		fetchOpt := ListUserActivitiesOptions{
			ActivityTypeIDs: baseOpt.ActivityTypeIDs,
			Count:           count,
			MaxId:           int(maxID),
		}

		batch, err := client.ListUserActivities(ctx, userID, fetchOpt)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}

		done := false
		for _, a := range batch {
			if a.Created == nil {
				continue
			}
			// アクティビティは新しい順(降順)で返る。until より新しいものはスキップ。
			if until != nil && a.Created.After(*until) {
				continue
			}
			// since より古くなったら以降のアクティビティは全て古いので終了。
			if since != nil && a.Created.Before(*since) {
				done = true
				break
			}
			result = append(result, a)
			// 日付フィルタなし時は limit に達したら終了。
			if since == nil && until == nil && limit > 0 && len(result) >= limit {
				done = true
				break
			}
		}

		if done || len(batch) < batchSize {
			break
		}
		// 次ページカーソル: 最後のアクティビティより古いものを取得。
		maxID = batch[len(batch)-1].ID - 1
		if maxID <= 0 {
			break
		}
	}

	return result, nil
}

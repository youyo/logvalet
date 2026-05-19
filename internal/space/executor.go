package space

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/sync/semaphore"

	"github.com/youyo/logvalet/internal/backlog"
)

const defaultMaxConcurrency = 4

// ClientFactory は (ctx, SpaceRegistration) → backlog.Client を返す関数型。
// auth.SpaceAwareClientFactory と同一シグネチャなので値を直接代入できる。
// auth パッケージが space を import するため循環依存を避けてここで定義する。
type ClientFactory func(ctx context.Context, reg SpaceRegistration) (backlog.Client, error)

// Result は1スペースの実行結果。
// Result フィールドは *T なので失敗時（OK=false）は nil になる。
type Result[T any] struct {
	SpaceAlias string `json:"space"`
	Tenant     string `json:"tenant,omitempty"`
	BaseURL    string `json:"base_url,omitempty"`
	OK         bool   `json:"ok"`
	Result     *T     `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	ErrorCode  string `json:"error_code,omitempty"`
}

// Executor は fan-out 実行設定を保持する。
type Executor struct {
	Factory        ClientFactory
	MaxConcurrency int // <= 0 なら defaultMaxConcurrency にフォールバック
}

func (e *Executor) resolveMaxConcurrency() int {
	if e.MaxConcurrency <= 0 {
		return defaultMaxConcurrency
	}
	return e.MaxConcurrency
}

// ExecuteAcrossSpaces は spaces 一覧に対して fn を並列実行し、
// 入力順を保持した []Result[T] を返す。
// 1スペースの失敗は Result.OK=false に記録し、全体は続行する（partial failure）。
// context がキャンセルされた場合は semaphore 取得をスキップして early exit する。
func ExecuteAcrossSpaces[T any](
	ctx context.Context,
	executor *Executor,
	spaces []SpaceRegistration,
	fn func(context.Context, SpaceRegistration, backlog.Client) (T, error),
) []Result[T] {
	results := make([]Result[T], len(spaces))
	if len(spaces) == 0 {
		return results
	}

	maxConc := executor.resolveMaxConcurrency()
	sem := semaphore.NewWeighted(int64(maxConc))
	var wg sync.WaitGroup

	for i, reg := range spaces {
		i, reg := i, reg
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sem.Acquire(ctx, 1); err != nil {
				results[i] = Result[T]{
					SpaceAlias: reg.Alias,
					Tenant:     reg.Tenant,
					BaseURL:    reg.BaseURL,
					OK:         false,
					ErrorCode:  "context_cancelled",
					Error:      err.Error(),
				}
				return
			}
			defer sem.Release(1)
			results[i] = executeOne(ctx, executor.Factory, reg, fn)
		}()
	}

	wg.Wait()
	return results
}

func executeOne[T any](
	ctx context.Context,
	factory ClientFactory,
	reg SpaceRegistration,
	fn func(context.Context, SpaceRegistration, backlog.Client) (T, error),
) Result[T] {
	base := Result[T]{
		SpaceAlias: reg.Alias,
		Tenant:     reg.Tenant,
		BaseURL:    reg.BaseURL,
	}

	client, err := factory(ctx, reg)
	if err != nil {
		base.OK = false
		base.Error = err.Error()
		base.ErrorCode = mapErrorCode(err)
		return base
	}

	val, err := fn(ctx, reg, client)
	if err != nil {
		base.OK = false
		base.Error = err.Error()
		base.ErrorCode = mapErrorCode(err)
		return base
	}

	base.OK = true
	base.Result = &val
	return base
}

func mapErrorCode(err error) string {
	switch {
	case errors.Is(err, backlog.ErrUnauthorized):
		return "unauthorized"
	case errors.Is(err, backlog.ErrForbidden):
		return "forbidden"
	case errors.Is(err, backlog.ErrRateLimited):
		return "rate_limited"
	default:
		return "internal_error"
	}
}

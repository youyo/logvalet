package mcp

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// nonIdempotentTool / idempotentTool は toolCategories 上の実在ツール名を借用する。
// 実装詳細（どの create 系ツールが対象か）は tool_categories.go 側の一覧に従う。
const (
	nonIdempotentTool = "logvalet_issue_create"
	idempotentTool    = "logvalet_issue_update"
	unknownTool       = "logvalet_does_not_exist"
)

func TestIdempotencyCache_DuplicateExplicitArgKey(t *testing.T) {
	cache := NewIdempotencyCache(time.Minute)
	var calls int32

	fn := func() (any, error) {
		atomic.AddInt32(&calls, 1)
		return "result-1", nil
	}

	args := map[string]any{"idempotency_key": "key-a", "summary": "issue A"}

	r1, err1, dup1 := cache.Execute(nonIdempotentTool, args, fn)
	if err1 != nil || dup1 {
		t.Fatalf("初回呼び出しは非重複であるべき: err=%v dup=%v", err1, dup1)
	}

	r2, err2, dup2 := cache.Execute(nonIdempotentTool, args, fn)
	if err2 != nil {
		t.Fatalf("unexpected error: %v", err2)
	}
	if !dup2 {
		t.Fatalf("同一 idempotency key の2回目は重複と判定されるべき")
	}
	if r1 != r2 {
		t.Fatalf("2回目は初回結果を返すべき: r1=%v r2=%v", r1, r2)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("fn は1回だけ実行されるべき, got %d", got)
	}
}

func TestIdempotencyCache_DifferentKeysExecuteIndependently(t *testing.T) {
	cache := NewIdempotencyCache(time.Minute)
	var calls int32
	fn := func() (any, error) {
		n := atomic.AddInt32(&calls, 1)
		return n, nil
	}

	_, _, dup1 := cache.Execute(nonIdempotentTool, map[string]any{"idempotency_key": "key-a"}, fn)
	_, _, dup2 := cache.Execute(nonIdempotentTool, map[string]any{"idempotency_key": "key-b"}, fn)

	if dup1 || dup2 {
		t.Fatalf("異なる key は重複扱いされるべきではない: dup1=%v dup2=%v", dup1, dup2)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("異なる key はそれぞれ実行されるべき, got %d", got)
	}
}

func TestIdempotencyCache_HashFallbackWhenKeyMissing(t *testing.T) {
	cache := NewIdempotencyCache(time.Minute)
	var calls int32
	fn := func() (any, error) {
		atomic.AddInt32(&calls, 1)
		return "ok", nil
	}

	sameArgs := map[string]any{"summary": "同じ課題", "project_id": float64(1)}

	_, _, dup1 := cache.Execute(nonIdempotentTool, sameArgs, fn)
	_, _, dup2 := cache.Execute(nonIdempotentTool, sameArgs, fn)

	if dup1 {
		t.Fatalf("初回はハッシュ代替でも重複ではないはず")
	}
	if !dup2 {
		t.Fatalf("idempotency_key 未指定でも同一引数なら引数ハッシュで重複判定されるべき")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("fn は1回だけ実行されるべき, got %d", got)
	}
}

func TestIdempotencyCache_HashFallbackDifferentArgsExecuteIndependently(t *testing.T) {
	cache := NewIdempotencyCache(time.Minute)
	var calls int32
	fn := func() (any, error) {
		atomic.AddInt32(&calls, 1)
		return "ok", nil
	}

	_, _, dup1 := cache.Execute(nonIdempotentTool, map[string]any{"summary": "課題A"}, fn)
	_, _, dup2 := cache.Execute(nonIdempotentTool, map[string]any{"summary": "課題B"}, fn)

	if dup1 || dup2 {
		t.Fatalf("引数が異なれば重複扱いされるべきではない")
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("引数が異なる呼び出しはそれぞれ実行されるべき, got %d", got)
	}
}

func TestIdempotencyCache_MetaKeyTakesPrecedenceOverArgKey(t *testing.T) {
	cache := NewIdempotencyCache(time.Minute)
	var calls int32
	fn := func() (any, error) {
		atomic.AddInt32(&calls, 1)
		return "ok", nil
	}

	// _meta.idempotency_key が異なれば、引数直下の idempotency_key が同一でも
	// 別呼び出しとして扱われる（_meta 優先の検証）。
	args1 := map[string]any{
		"idempotency_key": "same-arg-key",
		"_meta":           map[string]any{"idempotency_key": "meta-key-1"},
	}
	args2 := map[string]any{
		"idempotency_key": "same-arg-key",
		"_meta":           map[string]any{"idempotency_key": "meta-key-2"},
	}

	_, _, dup1 := cache.Execute(nonIdempotentTool, args1, fn)
	_, _, dup2 := cache.Execute(nonIdempotentTool, args2, fn)

	if dup1 || dup2 {
		t.Fatalf("_meta の idempotency_key が異なる場合は重複扱いされるべきではない")
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("_meta が優先され別呼び出しとして実行されるべき, got %d", got)
	}

	// 同一の _meta.idempotency_key なら、引数直下の idempotency_key が異なっても重複扱い。
	args3 := map[string]any{
		"idempotency_key": "different-arg-key",
		"_meta":           map[string]any{"idempotency_key": "meta-key-1"},
	}
	_, _, dup3 := cache.Execute(nonIdempotentTool, args3, fn)
	if !dup3 {
		t.Fatalf("_meta の idempotency_key が一致する場合は重複扱いされるべき")
	}
}

func TestIdempotencyCache_PassthroughForNonWriteNonIdempotentTools(t *testing.T) {
	cache := NewIdempotencyCache(time.Minute)
	var calls int32
	fn := func() (any, error) {
		atomic.AddInt32(&calls, 1)
		return "ok", nil
	}

	args := map[string]any{"idempotency_key": "same-key"}

	for i := 0; i < 3; i++ {
		_, _, dup := cache.Execute(idempotentTool, args, fn)
		if dup {
			t.Fatalf("CategoryWriteNonIdempotent 以外のツールは重複判定されないべき")
		}
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("write-idempotent ツールはキャッシュを介さず毎回実行されるべき, got %d", got)
	}
}

func TestIdempotencyCache_PassthroughForUnknownTool(t *testing.T) {
	cache := NewIdempotencyCache(time.Minute)
	var calls int32
	fn := func() (any, error) {
		atomic.AddInt32(&calls, 1)
		return "ok", nil
	}

	args := map[string]any{"idempotency_key": "same-key"}

	cache.Execute(unknownTool, args, fn)
	cache.Execute(unknownTool, args, fn)

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("toolCategories 未登録のツールはキャッシュを介さず毎回実行されるべき, got %d", got)
	}
}

func TestIdempotencyCache_TTLExpiryAllowsReexecution(t *testing.T) {
	cache := NewIdempotencyCache(10 * time.Millisecond)
	var calls int32
	fn := func() (any, error) {
		atomic.AddInt32(&calls, 1)
		return "ok", nil
	}

	args := map[string]any{"idempotency_key": "key-ttl"}

	cache.Execute(nonIdempotentTool, args, fn)
	time.Sleep(30 * time.Millisecond)
	_, _, dup := cache.Execute(nonIdempotentTool, args, fn)

	if dup {
		t.Fatalf("TTL 切れ後は重複ではなく再実行されるべき")
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("TTL 切れ後は fn が再実行されるべき, got %d", got)
	}
}

func TestIdempotencyCache_ErrorResultIsAlsoCached(t *testing.T) {
	cache := NewIdempotencyCache(time.Minute)
	var calls int32
	wantErr := errTestSentinel{}
	fn := func() (any, error) {
		atomic.AddInt32(&calls, 1)
		return nil, wantErr
	}

	args := map[string]any{"idempotency_key": "key-err"}

	_, err1, _ := cache.Execute(nonIdempotentTool, args, fn)
	_, err2, dup2 := cache.Execute(nonIdempotentTool, args, fn)

	if err1 != wantErr || err2 != wantErr {
		t.Fatalf("エラー結果もキャッシュされ、2回目も同じエラーが返るべき: err1=%v err2=%v", err1, err2)
	}
	if !dup2 {
		t.Fatalf("エラーだった呼び出しも重複扱いの対象になるべき")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("fn は1回だけ実行されるべき, got %d", got)
	}
}

type errTestSentinel struct{}

func (errTestSentinel) Error() string { return "sentinel test error" }

// TestIdempotencyCache_ConcurrentSameKeyExecutesOnce は、同一 key での同時呼び出しが
// 発生しても fn が1回しか実行されず、後発の呼び出しが先行呼び出しの完了を待って
// 同じ結果を共有することを検証する（stream 再送によるレースを模擬）。
func TestIdempotencyCache_ConcurrentSameKeyExecutesOnce(t *testing.T) {
	cache := NewIdempotencyCache(time.Minute)
	var calls int32
	fn := func() (any, error) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(20 * time.Millisecond)
		return "concurrent-result", nil
	}

	args := map[string]any{"idempotency_key": "key-concurrent"}

	const n = 5
	var wg sync.WaitGroup
	dupFlags := make([]bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, dup := cache.Execute(nonIdempotentTool, args, fn)
			dupFlags[idx] = dup
		}(i)
	}
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("同時呼び出しでも fn は1回だけ実行されるべき, got %d", got)
	}

	dupCount := 0
	for _, d := range dupFlags {
		if d {
			dupCount++
		}
	}
	if dupCount != n-1 {
		t.Fatalf("先行1件以外は重複と判定されるべき: dupCount=%d, want %d", dupCount, n-1)
	}
}

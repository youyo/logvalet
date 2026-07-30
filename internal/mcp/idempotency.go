package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// DefaultIdempotencyTTL は idempotency_key が指定されなかった場合の引数ハッシュ
// 代替キーに適用する既定の短命 TTL。
const DefaultIdempotencyTTL = 5 * time.Minute

// IdempotencyCache は CategoryWriteNonIdempotent (create 系) ツールの重複実行を
// 防ぐための、プロセス内 in-memory かつ短命なキャッシュ。
//
// MCP 2026-07-28 で stream 再開が廃止されたことにより、クライアントが接続断からの
// 復旧のために同一の create 系ツール呼び出しを再送する可能性が上がった。本キャッシュは
// 「同一 idempotency key の呼び出しは1回しか実際には実行しない」ことを保証することで
// これに対策する。
//
// 制約（重要）: このキャッシュはプロセス内メモリのみで完結する。水平スケールした
// 複数の logvalet サーバープロセス（マルチインスタンス環境）では、あるインスタンスで
// 記録した idempotency key を他インスタンスから参照できないため、インスタンスを跨いだ
// 重複実行までは防げない。マルチインスタンス環境での永続化・共有担保が必要な場合は
// Gateway/インフラ側の責務とする（永続ストア方針は本キャッシュとは別軸であり、
// S23 のストア方針と矛盾しない）。
type IdempotencyCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]*idempotencyEntry
}

// idempotencyEntry は1つの idempotency key に対応する実行結果を保持する。
// done は「実行完了」を示すシグナルであり、同一キーで同時に呼び出された場合に
// 後続の呼び出しが先行呼び出しの完了を待つために使う（多重実行防止）。
type idempotencyEntry struct {
	done      chan struct{}
	result    any
	err       error
	expiresAt time.Time
}

// NewIdempotencyCache は ttl<=0 の場合 DefaultIdempotencyTTL を使う IdempotencyCache を生成する。
func NewIdempotencyCache(ttl time.Duration) *IdempotencyCache {
	if ttl <= 0 {
		ttl = DefaultIdempotencyTTL
	}
	return &IdempotencyCache{
		ttl:     ttl,
		entries: make(map[string]*idempotencyEntry),
	}
}

// Execute は toolName が toolCategories 上で CategoryWriteNonIdempotent の場合のみ
// 冪等性キャッシュを介して fn を実行する。それ以外のカテゴリ（read-only / write
// idempotent / destructive）や toolCategories 未登録のツールはキャッシュを介さず
// 常に fn をそのまま実行する（意味論を変えないため）。
//
// 戻り値の duplicate は「今回の呼び出しが過去の呼び出しの重複として扱われ、キャッシュ
// 済みの結果を返した」ことを示す明確なシグナルである。呼び出し側はこれを見て応答に
// 重複実行だった旨を付与できる。
//
// key の決定順序:
//  1. args["_meta"].(map[string]any)["idempotency_key"] （呼び出し側が params._meta で明示指定）
//  2. args["idempotency_key"] （呼び出し側が引数として明示指定）
//  3. 上記が無ければ、toolName + 引数全体の JSON ハッシュ（args ハッシュ代替）
func (c *IdempotencyCache) Execute(toolName string, args map[string]any, fn func() (any, error)) (result any, err error, duplicate bool) {
	spec, ok := toolCategories[toolName]
	if !ok || spec.Category != CategoryWriteNonIdempotent {
		result, err = fn()
		return result, err, false
	}

	key := cacheKeyFor(toolName, args)

	for {
		c.mu.Lock()
		entry, found := c.entries[key]
		if found {
			select {
			case <-entry.done:
				// 完了済みエントリ。TTL 内なら重複としてキャッシュ済み結果を返す。
				if time.Now().Before(entry.expiresAt) {
					c.mu.Unlock()
					return entry.result, entry.err, true
				}
				// TTL 切れ: 新規呼び出しとして扱うため下へフォールスルーする。
			default:
				// 実行中（同時呼び出し）。完了を待ってから再評価する。
				c.mu.Unlock()
				<-entry.done
				continue
			}
		}

		newEntry := &idempotencyEntry{done: make(chan struct{})}
		c.entries[key] = newEntry
		c.mu.Unlock()

		result, err = fn()

		c.mu.Lock()
		newEntry.result = result
		newEntry.err = err
		newEntry.expiresAt = time.Now().Add(c.ttl)
		close(newEntry.done)
		c.mu.Unlock()

		return result, err, false
	}
}

// cacheKeyFor は toolName と idempotencyKeyFromArgs の結果からキャッシュキーを組み立てる。
// explicit / hash を名前空間として混在させ、異なるツール間でキーが衝突しないよう
// toolName を必ず先頭に含める。
func cacheKeyFor(toolName string, args map[string]any) string {
	if key, ok := idempotencyKeyFromArgs(args); ok {
		return toolName + "\x00explicit\x00" + key
	}
	return toolName + "\x00hash\x00" + hashArgs(args)
}

// idempotencyKeyFromArgs は呼び出し側指定の idempotency key を取り出す。
// params._meta.idempotency_key を最優先し、次に引数直下の idempotency_key を見る。
// どちらも無ければ ("", false) を返す。
func idempotencyKeyFromArgs(args map[string]any) (string, bool) {
	if args == nil {
		return "", false
	}
	if meta, ok := args["_meta"].(map[string]any); ok {
		if key, ok := meta["idempotency_key"].(string); ok && key != "" {
			return key, true
		}
	}
	if key, ok := args["idempotency_key"].(string); ok && key != "" {
		return key, true
	}
	return "", false
}

// hashArgs は idempotency_key 未指定時の代替キーとして引数全体の SHA-256 ハッシュを
// 計算する。encoding/json は map[string]any のキーを常にアルファベット順で
// マーシャルするため、map の走査順に依存せず決定的なハッシュになる。
func hashArgs(args map[string]any) string {
	b, err := json.Marshal(args)
	if err != nil {
		// マーシャル不能な引数（滅多に発生しない）は %v へフォールバックする。
		b = []byte(fmt.Sprintf("%v", args))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

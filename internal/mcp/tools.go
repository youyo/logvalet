// Package mcp は logvalet MCP サーバーの実装を提供する。
// 公式 Go SDK (github.com/modelcontextprotocol/go-sdk) を使用して
// Streamable HTTP / stdio の MCP サーバーを起動する。
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
)

// ToolFunc は MCP tool ハンドラーの関数型。
// context.Context と backlog.Client、args map を受け取り、任意の結果またはエラーを返す。
type ToolFunc func(ctx context.Context, client backlog.Client, args map[string]any) (any, error)

// ToolRegistry は MCP サーバーへの tool 登録を管理する。
// server は ServerBackend インターフェースにのみ依存し、公式 Go SDK 等の
// 具体的な SDK 型には依存しない (backend.go / backend_official.go 参照)。
type ToolRegistry struct {
	server           ServerBackend
	client           backlog.Client
	factory          func(ctx context.Context) (backlog.Client, error)
	disableFilePaths bool // stdio モードでローカルファイルシステムへのアクセスを防止する
	idempotency      *IdempotencyCache // CategoryWriteNonIdempotent ツール向け。遅延初期化 (idempotencyCache 参照)
}

// idempotencyCache は r.idempotency を初回アクセス時に遅延初期化して返す。
// registerAllTools はサーバー起動時に単一 goroutine から逐次呼ばれるため、
// ここでの遅延初期化にロックは不要。
func (r *ToolRegistry) idempotencyCache() *IdempotencyCache {
	if r.idempotency == nil {
		r.idempotency = NewIdempotencyCache(DefaultIdempotencyTTL)
	}
	return r.idempotency
}

// wrapIdempotent は toolName が toolCategories 上で CategoryWriteNonIdempotent の
// 場合のみ、IdempotencyCache 経由で fn を実行するようラップする。それ以外の
// カテゴリ（read-only / write idempotent / destructive）や toolCategories 未登録の
// ツールは fn をそのまま返す（余計なオーバーヘッドや意味論変化を避けるため）。
//
// MCP 2026-07-28 で stream 再開が廃止され、クライアント再送による create 系
// ツールの重複実行リスクが上がったことへの対策 (IdempotencyCache 参照)。
func (r *ToolRegistry) wrapIdempotent(toolName string, fn ToolFunc) ToolFunc {
	spec, ok := toolCategories[toolName]
	if !ok || spec.Category != CategoryWriteNonIdempotent {
		return fn
	}
	cache := r.idempotencyCache()
	return func(ctx context.Context, client backlog.Client, args map[string]any) (any, error) {
		result, err, _ := cache.Execute(toolName, args, func() (any, error) {
			return fn(ctx, client, args)
		})
		return result, err
	}
}

// NewToolRegistryWithFactory は ClientFactory を使って per-user の backlog.Client を
// 動的に生成する ToolRegistry を返す。
// factory は MCP ツール呼び出し時に context.Context からユーザーを特定し、
// そのユーザー用の backlog.Client を返す。
func NewToolRegistryWithFactory(backend ServerBackend, factory func(ctx context.Context) (backlog.Client, error)) *ToolRegistry {
	return &ToolRegistry{server: backend, factory: factory}
}

// Register は tool を ServerBackend に登録する。
// ToolFunc が error を返した場合、自動的に IsError=true の ToolResult に変換する。
// factory が設定されている場合、リクエストの context から per-user クライアントを生成する。
func (r *ToolRegistry) Register(tool ToolDef, fn ToolFunc) {
	fn = r.wrapIdempotent(tool.Name, fn)
	r.server.RegisterTool(tool, func(ctx context.Context, args map[string]any) (ToolResult, error) {
		return r.callWithDefaultClient(ctx, fn, args)
	})
}

// NewToolDef は functional option を順に適用して ToolDef を組み立てる。
// SDK 非依存のビルダーパターンで、ToolDef 単体でツール定義を組み立てられる。
func NewToolDef(name string, opts ...func(*ToolDef)) ToolDef {
	t := ToolDef{Name: name}
	for _, opt := range opts {
		opt(&t)
	}
	return t
}

// WithDesc は ToolDef.Description を設定する。
func WithDesc(desc string) func(*ToolDef) {
	return func(t *ToolDef) { t.Description = desc }
}

// WithAnnotation は ToolDef.Annotation を設定する。
func WithAnnotation(a ToolAnnotation) func(*ToolDef) {
	return func(t *ToolDef) { t.Annotation = a }
}

// withParam は ParamSpec を ToolDef.Params に追加し、required なら Required にも追加する
// 共通ヘルパー。
func withParam(p ParamSpec, required bool) func(*ToolDef) {
	return func(t *ToolDef) {
		t.Params = append(t.Params, p)
		if required {
			t.Required = append(t.Required, p.Name)
		}
	}
}

// WithStringParam は string パラメータを追加する。
func WithStringParam(name string, required bool, desc string) func(*ToolDef) {
	return withParam(ParamSpec{Name: name, Type: ParamTypeString, Description: desc}, required)
}

// WithNumberParam は number パラメータを追加する。
func WithNumberParam(name string, required bool, desc string) func(*ToolDef) {
	return withParam(ParamSpec{Name: name, Type: ParamTypeNumber, Description: desc}, required)
}

// WithBooleanParam は boolean パラメータを追加する。
func WithBooleanParam(name string, required bool, desc string) func(*ToolDef) {
	return withParam(ParamSpec{Name: name, Type: ParamTypeBoolean, Description: desc}, required)
}

// callWithDefaultClient は factory または固定クライアントを使って fn を呼び出す。
// Register と共通の処理を切り出したヘルパー。
func (r *ToolRegistry) callWithDefaultClient(ctx context.Context, fn ToolFunc, args map[string]any) (ToolResult, error) {
	var c backlog.Client
	if r.factory != nil {
		var err error
		c, err = r.factory(ctx)
		if err != nil {
			return NewErrorToolResult(ToolError{Message: err.Error()}), nil
		}
	} else {
		c = r.client
	}
	result, err := fn(ctx, c, args)
	if err != nil {
		return NewErrorToolResult(ToolError{Message: err.Error()}), nil
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return NewErrorToolResult(ToolError{Message: "failed to marshal result: " + err.Error()}), nil
	}
	return NewTextToolResult(string(jsonBytes)), nil
}

// stringArg は args map から文字列引数を取り出すヘルパー。
// ok=false の場合は空文字列を返す。
func stringArg(args map[string]any, key string) (string, bool) {
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// boolArg は args map から bool 引数を取り出すヘルパー。
// ok=false の場合は false を返す。
func boolArg(args map[string]any, key string) (bool, bool) {
	v, ok := args[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// intArg は args map から数値引数を取り出すヘルパー。
// JSON number は float64 として渡されるため int に変換する。
func intArg(args map[string]any, key string) (int, bool) {
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}

// parseDateStr は "YYYY-MM-DD" 形式の文字列を time.Time に変換するヘルパー。
// MCP ツールの since/until 引数パース用。
func parseDateStr(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format (must be YYYY-MM-DD): %q", s)
	}
	return t, nil
}

// parseCSVIntList は "1,2,3" 形式の文字列を []int に変換する。
// 空文字列は nil を返す（未指定扱い）。
// 無効な整数が含まれる場合はエラー。
func parseCSVIntList(input, paramName string) ([]int, error) {
	if input == "" {
		return nil, nil
	}
	parts := strings.Split(input, ",")
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: must be comma-separated integers, got %q", paramName, input)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// parseCSVStringList は "a,b,c" 形式の文字列を []string に変換する。
// 空文字列は nil を返す（未指定扱い）。空要素はスキップする。
func parseCSVStringList(input string) []string {
	if input == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

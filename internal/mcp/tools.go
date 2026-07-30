// Package mcp は logvalet MCP サーバーの実装を提供する。
// mark3labs/mcp-go を使用して Streamable HTTP MCP サーバーを起動する。
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/space"
)

// ToolFunc は MCP tool ハンドラーの関数型。
// context.Context と backlog.Client、args map を受け取り、任意の結果またはエラーを返す。
type ToolFunc func(ctx context.Context, client backlog.Client, args map[string]any) (any, error)

// spaceRegCtxKey は context.Context に space.SpaceRegistration を格納するための非公開キー。
// unexported struct を使うことで他パッケージとの値衝突を防ぐ。
type spaceRegCtxKey struct{}

// contextWithSpace は ctx に SpaceRegistration を埋め込んだ派生 context を返す。
// multi-space fan-out や callWithSpaceClient で利用する。
func contextWithSpace(ctx context.Context, reg space.SpaceRegistration) context.Context {
	return context.WithValue(ctx, spaceRegCtxKey{}, reg)
}

// spaceInfoFromContext は ctx に埋め込まれた SpaceRegistration から
// (Alias, BaseURL) を取り出す。未設定または Alias/BaseURL のいずれかが空の場合は
// fallback の (fbSpace, fbBaseURL) を返す。
//
// multi-space モード（spaces=[...] / all_spaces=true）で呼ばれたツールが
// 実行対象 SpaceRegistration の Alias/BaseURL を AnalysisEnvelope メタデータに
// 反映できるようにするためのヘルパー。
func spaceInfoFromContext(ctx context.Context, fbSpace, fbBaseURL string) (string, string) {
	reg, ok := ctx.Value(spaceRegCtxKey{}).(space.SpaceRegistration)
	if !ok {
		return fbSpace, fbBaseURL
	}
	if reg.Alias == "" || reg.BaseURL == "" {
		slog.Warn("spaceInfoFromContext: reg が不完全なため fallback します",
			"alias", reg.Alias, "baseURL", reg.BaseURL,
			"fallbackSpace", fbSpace, "fallbackBaseURL", fbBaseURL)
		return fbSpace, fbBaseURL
	}
	return reg.Alias, reg.BaseURL
}

// ToolRegistry は MCP サーバーへの tool 登録を管理する。
// server は ServerBackend インターフェースにのみ依存し、mark3labs/mcp-go 等の
// 具体的な SDK 型には依存しない (backend.go / tooldef_mark3labs.go 参照)。
type ToolRegistry struct {
	server           ServerBackend
	client           backlog.Client
	factory          func(ctx context.Context) (backlog.Client, error)
	authorizationURL string
	disableFilePaths bool // stdio モードでローカルファイルシステムへのアクセスを防止する
	resolver         *space.Resolver
	spaceFactory     space.ClientFactory
}

// NewToolRegistry は新しい ToolRegistry を返す。
// authorizationURL は OAuth 未接続エラー時に _meta に付与する認可 URL。
// 空文字列の場合は従来挙動（Meta なし）。
func NewToolRegistry(s *mcpserver.MCPServer, client backlog.Client, authorizationURL string) *ToolRegistry {
	return NewToolRegistryWithBackend(NewMark3labsBackend(s), client, authorizationURL)
}

// NewToolRegistryWithMultiSpace は resolver と spaceFactory を持つ multi-space 対応の
// ToolRegistry を返す。resolver が nil の場合は RegisterWithSpaces/RegisterWithSpacesWrite
// は通常の Register と同等に動作する。
func NewToolRegistryWithMultiSpace(
	s *mcpserver.MCPServer,
	factory func(ctx context.Context) (backlog.Client, error),
	authorizationURL string,
	resolver *space.Resolver,
	spaceFactory space.ClientFactory,
) *ToolRegistry {
	return &ToolRegistry{
		server:           NewMark3labsBackend(s),
		factory:          factory,
		authorizationURL: authorizationURL,
		resolver:         resolver,
		spaceFactory:     spaceFactory,
	}
}

// NewToolRegistryWithFactory は ClientFactory を使って per-user の backlog.Client を
// 動的に生成する ToolRegistry を返す。
// factory は MCP ツール呼び出し時に context.Context からユーザーを特定し、
// そのユーザー用の backlog.Client を返す。
// authorizationURL は OAuth 未接続エラー時に _meta に付与する認可 URL。
// 空文字列の場合は従来挙動（Meta なし）。
func NewToolRegistryWithFactory(s *mcpserver.MCPServer, factory func(ctx context.Context) (backlog.Client, error), authorizationURL string) *ToolRegistry {
	return &ToolRegistry{server: NewMark3labsBackend(s), factory: factory, authorizationURL: authorizationURL}
}

// Register は tool を MCPServer に登録する。
// ToolFunc が error を返した場合、自動的に mcp.NewToolResultError に変換する。
// factory が設定されている場合、リクエストの context から per-user クライアントを生成する。
// factory が ErrProviderNotConnected / ErrTokenRefreshFailed / ErrTokenExpired を返し、
// authorizationURL が設定されている場合は _meta.authorization_url を付与する。
func (r *ToolRegistry) Register(tool ToolDef, fn ToolFunc) {
	r.server.RegisterTool(tool, func(ctx context.Context, args map[string]any) (ToolResult, error) {
		return r.callWithDefaultClient(ctx, fn, args)
	})
}

// injectSpaceParams は tool.Params に "spaces" と "all_spaces" を注入する。
// 既存の Params は保持される。
func injectSpaceParams(tool *ToolDef) {
	tool.Params = append(tool.Params,
		SpacesParamSpec("対象 Backlog スペースの alias 一覧（省略時はデフォルトスペースを使用）"),
		AllSpacesParamSpec("登録済みの全スペースを対象にする（spaces と同時指定不可）"),
	)
}

// injectSpaceParamWrite は write ツール用に "spaces" のみを注入する。
func injectSpaceParamWrite(tool *ToolDef) {
	tool.Params = append(tool.Params,
		SpacesWriteParamSpec("対象 Backlog スペースの alias（1件のみ指定可）"),
	)
}

// NewToolDef は functional option を順に適用して ToolDef を組み立てる。
// gomcp.NewTool(name, opts...) ビルダーパターンの logvalet 版で、既存の
// 呼び出しパターンを機械的に置き換えられるよう設計している。
func NewToolDef(name string, opts ...func(*ToolDef)) ToolDef {
	t := ToolDef{Name: name}
	for _, opt := range opts {
		opt(&t)
	}
	return t
}

// WithDesc は ToolDef.Description を設定する。gomcp.WithDescription 相当。
func WithDesc(desc string) func(*ToolDef) {
	return func(t *ToolDef) { t.Description = desc }
}

// WithAnnotation は ToolDef.Annotation を設定する。
// gomcp.WithToolAnnotation(readOnlyAnnotation(...)) 等の置き換えに使う。
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

// WithStringParam は string パラメータを追加する。gomcp.WithString(name, gomcp.Required(), gomcp.Description(desc)) 相当。
func WithStringParam(name string, required bool, desc string) func(*ToolDef) {
	return withParam(ParamSpec{Name: name, Type: ParamTypeString, Description: desc}, required)
}

// WithNumberParam は number パラメータを追加する。gomcp.WithNumber(...) 相当。
func WithNumberParam(name string, required bool, desc string) func(*ToolDef) {
	return withParam(ParamSpec{Name: name, Type: ParamTypeNumber, Description: desc}, required)
}

// WithBooleanParam は boolean パラメータを追加する。gomcp.WithBoolean(...) 相当。
func WithBooleanParam(name string, required bool, desc string) func(*ToolDef) {
	return withParam(ParamSpec{Name: name, Type: ParamTypeBoolean, Description: desc}, required)
}

// RegisterWithSpaces は read-only ツールを multi-space fan-out 対応で登録する。
// resolver が nil の場合は通常の Register と同等に動作する。
// args に "spaces" ([]string) または "all_spaces" (bool) を渡すと fan-out モードになる。
// "spaces" と "all_spaces" の同時指定はエラー。
// spaces/all_spaces 未指定時は DynamoDB UserPreference → 単一スペース fallback → default client の順で解決する。
func (r *ToolRegistry) RegisterWithSpaces(tool ToolDef, fn ToolFunc) {
	injectSpaceParams(&tool)
	r.server.RegisterTool(tool, func(ctx context.Context, args map[string]any) (ToolResult, error) {
		spaces := stringSliceArg(args, "spaces")
		allSpaces, _ := boolArg(args, "all_spaces")

		if r.resolver == nil {
			return r.callWithDefaultClient(ctx, fn, args)
		}

		if len(spaces) > 0 && allSpaces {
			return NewErrorToolResult(ToolError{Message: "spaces and all_spaces cannot be specified together"}), nil
		}

		userID, ok := auth.UserIDFromContext(ctx)
		if !ok {
			return r.callWithDefaultClient(ctx, fn, args)
		}

		scope := space.Scope{Aliases: spaces, AllSpaces: allSpaces}
		targets, err := r.resolver.Resolve(ctx, userID, scope)
		if err != nil {
			// spaces/all_spaces 未指定の場合は default client に fallback（既存動作を維持）
			if len(spaces) == 0 && !allSpaces {
				return r.callWithDefaultClient(ctx, fn, args)
			}
			return NewErrorToolResult(ToolError{Message: fmt.Sprintf("resolve spaces: %s", err.Error())}), nil
		}

		// spaces/all_spaces 未指定 → 単一スペースの通常レスポンス形式（配列ではない）
		if len(spaces) == 0 && !allSpaces {
			return r.callWithSpaceClient(ctx, fn, args, targets[0])
		}

		if r.spaceFactory == nil {
			return NewErrorToolResult(ToolError{Message: "spaceFactory が設定されていません"}), nil
		}
		executor := &space.Executor{Factory: r.spaceFactory}
		results := space.ExecuteAcrossSpaces[any](ctx, executor, targets,
			func(ctx context.Context, reg space.SpaceRegistration, client backlog.Client) (any, error) {
				ctx = contextWithSpace(ctx, reg)
				return fn(ctx, client, args)
			},
		)

		jsonBytes, err := json.Marshal(results)
		if err != nil {
			return NewErrorToolResult(ToolError{Message: "failed to marshal results: " + err.Error()}), nil
		}
		return NewTextToolResult(string(jsonBytes)), nil
	})
}

// RegisterWithSpacesWrite は write ツールを single-space 指定対応で登録する。
// spaces 未指定 → 既存動作（default client）。
// spaces=["foo"]（1件）→ foo スペースの client で fn を実行。
// spaces 複数 → エラー。
// all_spaces=true → エラー。
func (r *ToolRegistry) RegisterWithSpacesWrite(tool ToolDef, fn ToolFunc) {
	injectSpaceParamWrite(&tool)
	r.server.RegisterTool(tool, func(ctx context.Context, args map[string]any) (ToolResult, error) {
		spaces := stringSliceArg(args, "spaces")
		allSpaces, _ := boolArg(args, "all_spaces")

		if allSpaces {
			return NewErrorToolResult(ToolError{Message: "all_spaces is not supported for write operations"}), nil
		}
		if len(spaces) > 1 {
			return NewErrorToolResult(ToolError{Message: "multi-space write operations require exactly one space"}), nil
		}

		userID, ok := auth.UserIDFromContext(ctx)
		if !ok {
			return r.callWithDefaultClient(ctx, fn, args)
		}

		// spaces 未指定 → resolver で default スペースを解決（DynamoDB preference → 単一 enabled → fallback）
		if len(spaces) == 0 {
			if r.resolver == nil {
				return r.callWithDefaultClient(ctx, fn, args)
			}
			targets, err := r.resolver.Resolve(ctx, userID, space.Scope{})
			if err != nil {
				return r.callWithDefaultClient(ctx, fn, args)
			}
			return r.callWithSpaceClient(ctx, fn, args, targets[0])
		}

		// spaces=["foo"]（1件）→ resolver で解決して spaceFactory でクライアント生成
		if r.resolver == nil {
			return NewErrorToolResult(ToolError{Message: "resolver not configured for multi-space operations"}), nil
		}

		scope := space.Scope{Aliases: spaces}
		targets, err := r.resolver.Resolve(ctx, userID, scope)
		if err != nil {
			return NewErrorToolResult(ToolError{Message: fmt.Sprintf("resolve space: %s", err.Error())}), nil
		}
		if len(targets) == 0 {
			return NewErrorToolResult(ToolError{Message: fmt.Sprintf("space not found: %s", spaces[0])}), nil
		}

		return r.callWithSpaceClient(ctx, fn, args, targets[0])
	})
}

// callWithDefaultClient は factory または固定クライアントを使って fn を呼び出す。
// Register と共通の処理を切り出したヘルパー。
func (r *ToolRegistry) callWithDefaultClient(ctx context.Context, fn ToolFunc, args map[string]any) (ToolResult, error) {
	var c backlog.Client
	if r.factory != nil {
		var err error
		c, err = r.factory(ctx)
		if err != nil {
			if needsAuthorization(err) && r.authorizationURL != "" {
				return toolResultAuthRequired(err, r.authorizationURL), nil
			}
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

// callWithSpaceClient は指定 SpaceRegistration の spaceFactory でクライアントを生成し fn を呼び出す。
// needsAuthorization エラーの場合は authorizationURL を付与したレスポンスを返す。
func (r *ToolRegistry) callWithSpaceClient(ctx context.Context, fn ToolFunc, args map[string]any, reg space.SpaceRegistration) (ToolResult, error) {
	if r.spaceFactory == nil {
		return ToolResult{}, fmt.Errorf("spaceFactory が設定されていません")
	}
	client, err := r.spaceFactory(ctx, reg)
	if err != nil {
		if needsAuthorization(err) && r.authorizationURL != "" {
			return toolResultAuthRequired(err, r.authorizationURL), nil
		}
		return NewErrorToolResult(ToolError{Message: fmt.Sprintf("create client for space %s: %s", reg.Alias, err.Error())}), nil
	}
	ctx = contextWithSpace(ctx, reg)
	result, err := fn(ctx, client, args)
	if err != nil {
		return NewErrorToolResult(ToolError{Message: err.Error()}), nil
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return NewErrorToolResult(ToolError{Message: "failed to marshal result: " + err.Error()}), nil
	}
	return NewTextToolResult(string(jsonBytes)), nil
}

// needsAuthorization は GetValidToken / factory のエラーが OAuth 認可を要求するかを判定する。
// ホワイトリスト方式で判定する。
func needsAuthorization(err error) bool {
	return errors.Is(err, auth.ErrProviderNotConnected) ||
		errors.Is(err, auth.ErrTokenRefreshFailed) ||
		errors.Is(err, auth.ErrTokenExpired)
}

// toolResultAuthRequired は認可 URL 付きのツールエラー結果を返す。
// _meta.authorization_required と _meta.authorization_url を含む。
func toolResultAuthRequired(err error, url string) ToolResult {
	text := fmt.Sprintf(
		"Backlog authorization required. Open the following URL in your browser to connect:\n%s",
		url,
	)
	result := NewErrorToolResult(ToolError{Message: text})
	result.Meta = &ResultMeta{
		AuthorizationRequired: true,
		AuthorizationURL:      url,
	}
	return result
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

// stringSliceArg は args map から []string 引数を取り出すヘルパー。
// JSON デシリアライズにより []any として渡されるため、各要素を string に変換する。
// キーが存在しない、または型変換できない場合は nil を返す。
func stringSliceArg(args map[string]any, key string) []string {
	v, ok := args[key]
	if !ok || v == nil {
		return nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil
		}
		result = append(result, s)
	}
	return result
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

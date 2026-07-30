package mcp

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/youyo/logvalet/internal/app"
	"github.com/youyo/logvalet/internal/auth"
)

// passthrough_middleware.go は Backlog credential の Bearer passthrough モード
// (docs/specs/gateway-request-contract.md §4) 用の HTTP ミドルウェアを提供する。
//
// AgentCore Gateway は logvalet へのリクエストの Authorization ヘッダーに
// per-user の Backlog OAuth access token を注入する想定である。
// PassthroughAuthMiddleware はこのヘッダーを抽出し、リクエストスコープの
// context (auth.ContextWithPassthroughToken) へ運ぶ。抽出したトークンは
// リクエストの context にのみ保持され、logvalet プロセス内のグローバル状態にも
// tokenstore 等の永続ストアにも一切保存されない。
//
// HTTP トランスポートへの実際の配線 (internal/cli 配下) は別ステップの
// スコープであり、本ファイルは配線に必要な公開関数を提供するところまでを担う。

const bearerSchemePrefix = "Bearer "

// ExtractBearerToken は Authorization ヘッダーの生値から `Bearer <token>` の
// トークン部分を取り出す。スキームプレフィックスが無い、または前後の空白を除いた
// 結果が空文字列の場合は ok=false を返す。
func ExtractBearerToken(authorizationHeader string) (token string, ok bool) {
	if authorizationHeader == "" {
		return "", false
	}
	if !strings.HasPrefix(authorizationHeader, bearerSchemePrefix) {
		return "", false
	}
	token = strings.TrimSpace(strings.TrimPrefix(authorizationHeader, bearerSchemePrefix))
	if token == "" {
		return "", false
	}
	return token, true
}

// PassthroughAuthMiddleware は受信 HTTP リクエストの `Authorization: Bearer <token>`
// ヘッダーを抽出し、auth.ContextWithPassthroughToken で context に載せてから next を
// 呼び出す HTTP ミドルウェアを返す。
//
// ヘッダーが欠落している、またはスキームが `Bearer` でない場合は next を呼び出さず、
// 401 Unauthorized + spec §9 準拠の JSON エラーエンベロープを返す (契約 §4.3 の
// fail-fast 仕様: Backlog API 呼び出しを試みずに即座にエラーを返す)。
func PassthroughAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := ExtractBearerToken(r.Header.Get("Authorization"))
		if !ok {
			writePassthroughAuthErrorEnvelope(w)
			return
		}
		ctx := auth.ContextWithPassthroughToken(r.Context(), token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// writePassthroughAuthErrorEnvelope は Authorization: Bearer ヘッダー欠落時の
// 401 レスポンスを spec §9 のエラーエンベロープ形式で書き込む。
func writePassthroughAuthErrorEnvelope(w http.ResponseWriter) {
	envelope := app.NewErrorEnvelope(
		app.ErrorCodeAuthentication,
		"Missing or invalid Authorization: Bearer header for Backlog credential passthrough.",
		false,
	)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(envelope)
}

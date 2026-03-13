// Package credentials — OAuth localhost callback フロー実装。
// spec §5 OAuth flow 準拠。
package credentials

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// BuildAuthorizeURL は Backlog OAuth 認証URLを構築する。
// space は Backlog スペース名（例: "example-space"）。
// clientID は OAuth クライアントID。
// redirectURI は OAuth コールバック URI。
// state は CSRF 防止用ランダム文字列。
func BuildAuthorizeURL(space, clientID, redirectURI, state string) string {
	base := fmt.Sprintf("https://%s.backlog.com/OAuth2AccessRequest.action", space)
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("state", state)
	return base + "?" + params.Encode()
}

// TokenResponse は OAuth トークンエンドポイントのレスポンス。
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// ExchangeCode は authorization code を access_token + refresh_token に交換する。
// tokenURL は Backlog のトークンエンドポイント URL（テスト時は httptest.Server の URL）。
// clientID / clientSecret は OAuth クライアント認証情報。
// code は認可コード、redirectURI はコールバック URI。
func ExchangeCode(ctx context.Context, tokenURL, clientID, clientSecret, code, redirectURI string) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("oauth: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth: failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth: token endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("oauth: failed to parse token response: %w", err)
	}

	return &tokenResp, nil
}

// CallbackResult は OAuth コールバックサーバーが受信した認可コードと state。
type CallbackResult struct {
	Code  string
	State string
	Err   error
}

// StartCallbackServer はローカルコールバックサーバーを起動し、認可コードを待機する。
// コンテキストがキャンセルされるか、コールバックを受信すると結果チャンネルに送信する。
// redirectURI は "http://localhost:{port}/callback" 形式で返される。
// ポートは OS が自動割り当てする（ポート競合を避けるため）。
func StartCallbackServer(ctx context.Context) (<-chan CallbackResult, string, error) {
	// :0 でリッスンして OS にポートを割り当ててもらう
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", fmt.Errorf("oauth: failed to start callback server: %w", err)
	}

	port := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)

	resultCh := make(chan CallbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		errParam := r.URL.Query().Get("error")

		if errParam != "" {
			resultCh <- CallbackResult{Err: fmt.Errorf("oauth: authorization denied: %s", errParam)}
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body><h1>Authorization failed. You can close this window.</h1></body></html>"))
			return
		}

		if code == "" {
			resultCh <- CallbackResult{Err: fmt.Errorf("oauth: no code in callback")}
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		resultCh <- CallbackResult{Code: code, State: state}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body><h1>Authorization successful! You can close this window.</h1></body></html>"))
	})

	server := &http.Server{Handler: mux}

	go func() {
		defer ln.Close()
		// サーバー起動（コンテキストキャンセル時は停止）
		go func() {
			<-ctx.Done()
			// コンテキストがキャンセルされたらサーバーをシャットダウン
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			server.Shutdown(shutdownCtx)
		}()
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			// サーバーエラーをチャンネルに送信（まだ送信済みでない場合）
			select {
			case resultCh <- CallbackResult{Err: fmt.Errorf("oauth: callback server error: %w", err)}:
			default:
			}
		}
	}()

	// コンテキストキャンセル時のエラーを別 goroutine で処理
	go func() {
		<-ctx.Done()
		// チャンネルにまだ何も送信されていない場合にキャンセルエラーを送信
		select {
		case resultCh <- CallbackResult{Err: fmt.Errorf("oauth: context cancelled while waiting for callback: %w", ctx.Err())}:
		default:
		}
	}()

	return resultCh, redirectURI, nil
}

// GenerateState は CSRF 防止用のランダムな state 文字列を生成する。
// crypto/rand を使用して安全なランダム値を生成する。
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauth: failed to generate state: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// TokenExpiry は expires_in 秒後の有効期限を RFC3339 形式で返す。
func TokenExpiry(expiresIn int) string {
	return time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339)
}

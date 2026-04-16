package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/credentials"
)

const (
	// backlogProviderName はプロバイダー名。
	backlogProviderName = "backlog"

	// backlogTokenPath はトークンエンドポイントのパス。
	backlogTokenPath = "/api/v2/oauth2/token"

	// backlogUsersMyselfPath は /users/myself エンドポイントのパス。
	backlogUsersMyselfPath = "/api/v2/users/myself"

	// defaultHTTPTimeout は HTTP クライアントのデフォルトタイムアウト。
	defaultHTTPTimeout = 30 * time.Second
)

// BacklogOAuthProvider は Backlog OAuth 2.0 プロバイダーの実装。
// OAuthProvider interface を満たす。
type BacklogOAuthProvider struct {
	space        string       // Backlog スペース名（例: "example-space"）
	clientID     string       // OAuth クライアントID
	clientSecret string       // OAuth クライアントシークレット
	baseURL      string       // デフォルト: https://{space}.backlog.com（テスト時に httptest URL へ差し替え）
	httpClient   *http.Client // RefreshToken / GetCurrentUser で使用（ExchangeCode は credentials 内部で独自 Client を使用）
}

// NewBacklogOAuthProvider は BacklogOAuthProvider を構築する。
// space が空の場合は auth.ErrInvalidTenant を返す。
func NewBacklogOAuthProvider(space, clientID, clientSecret string) (*BacklogOAuthProvider, error) {
	if space == "" {
		return nil, auth.ErrInvalidTenant
	}
	return &BacklogOAuthProvider{
		space:        space,
		clientID:     clientID,
		clientSecret: clientSecret,
		baseURL:      "https://" + space + ".backlog.com",
		httpClient:   &http.Client{Timeout: defaultHTTPTimeout},
	}, nil
}

// Name はプロバイダー名 "backlog" を返す。
func (p *BacklogOAuthProvider) Name() string {
	return backlogProviderName
}

// BuildAuthorizationURL は Backlog OAuth 認可 URL を構築する。
// credentials.BuildAuthorizeURL をラップし、space と clientID は struct から取得する。
func (p *BacklogOAuthProvider) BuildAuthorizationURL(state, redirectURI string) (string, error) {
	u := credentials.BuildAuthorizeURL(p.space, p.clientID, redirectURI, state)
	return u, nil
}

// ExchangeCode は認可コードをトークンに交換する。
// credentials.ExchangeCode をラップし、TokenResponse → TokenRecord に変換する。
// 返り値の TokenRecord には UserID と ProviderUserID は設定されない。
func (p *BacklogOAuthProvider) ExchangeCode(ctx context.Context, code, redirectURI string) (*auth.TokenRecord, error) {
	tokenURL := p.baseURL + backlogTokenPath

	resp, err := credentials.ExchangeCode(ctx, tokenURL, p.clientID, p.clientSecret, code, redirectURI)
	if err != nil {
		return nil, fmt.Errorf("provider/backlog: exchange code failed: %w", err)
	}

	return p.toTokenRecord(resp), nil
}

// RefreshToken はリフレッシュトークンで新しいトークンを取得する。
// grant_type=refresh_token で Backlog token endpoint を呼び出す。
// 返り値の TokenRecord には UserID と ProviderUserID は設定されない。
func (p *BacklogOAuthProvider) RefreshToken(ctx context.Context, refreshToken string) (*auth.TokenRecord, error) {
	tokenURL := p.baseURL + backlogTokenPath

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", p.clientID)
	form.Set("client_secret", p.clientSecret)
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("provider/backlog: failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("provider/backlog: refresh token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("provider/backlog: failed to read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider/backlog: token endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp credentials.TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("provider/backlog: failed to parse refresh response: %w", err)
	}

	return p.toTokenRecord(&tokenResp), nil
}

// backlogUserResponse は Backlog /api/v2/users/myself のレスポンス構造体。
type backlogUserResponse struct {
	ID          int    `json:"id"`
	UserID      string `json:"userId"`
	Name        string `json:"name"`
	MailAddress string `json:"mailAddress"`
}

// GetCurrentUser はアクセストークンで Backlog の /api/v2/users/myself を呼び出し、
// ProviderUser にマッピングして返す。
func (p *BacklogOAuthProvider) GetCurrentUser(ctx context.Context, accessToken string) (*auth.ProviderUser, error) {
	userURL := p.baseURL + backlogUsersMyselfPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil)
	if err != nil {
		return nil, fmt.Errorf("provider/backlog: failed to create user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("provider/backlog: user request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("provider/backlog: failed to read user response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider/backlog: users/myself returned status %d: %s", resp.StatusCode, string(body))
	}

	var userResp backlogUserResponse
	if err := json.Unmarshal(body, &userResp); err != nil {
		return nil, fmt.Errorf("provider/backlog: failed to parse user response: %w", err)
	}

	return &auth.ProviderUser{
		ID:    strconv.Itoa(userResp.ID),
		Name:  userResp.Name,
		Email: userResp.MailAddress,
	}, nil
}

// toTokenRecord は credentials.TokenResponse を auth.TokenRecord に変換する。
// UserID と ProviderUserID は caller 側で設定すること。
func (p *BacklogOAuthProvider) toTokenRecord(resp *credentials.TokenResponse) *auth.TokenRecord {
	now := time.Now()
	return &auth.TokenRecord{
		Provider:     backlogProviderName,
		Tenant:       p.space,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		TokenType:    resp.TokenType,
		Expiry:       now.Add(time.Duration(resp.ExpiresIn) * time.Second),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

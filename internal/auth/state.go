package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// DefaultStateTTL は OAuth state トークンのデフォルト有効期間。
const DefaultStateTTL = 10 * time.Minute

// nonceBytes は nonce 生成に使用するバイト数。
const nonceBytes = 16

// StateClaims は OAuth state JWT のカスタムクレームを保持する。
// HMAC-SHA256 で署名され、CSRF 対策および callback 時のコンテキスト復元に使用する。
type StateClaims struct {
	UserID   string `json:"uid"`
	Tenant   string `json:"tenant"`
	Nonce    string `json:"nonce"`
	Continue string `json:"continue,omitempty"`
	jwt.RegisteredClaims
}

// generateNonce は crypto/rand で暗号学的に安全なランダム nonce を生成する。
// 返り値は hex エンコードされた文字列。
func generateNonce() (string, error) {
	b := make([]byte, nonceBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: failed to generate nonce: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateState は HMAC-SHA256 で署名された JWT state トークンを生成する。
//
// userID は idproxy で確定したユーザー識別子。空の場合 ErrUnauthenticated を返す。
// tenant は Backlog スペース識別子。空の場合 ErrInvalidTenant を返す。
// secret は HMAC 署名鍵。nil または空の場合 ErrStateInvalid を返す。
// ttl はトークン有効期間。0 以下の場合 ErrStateInvalid を返す。
func GenerateState(userID, tenant string, secret []byte, ttl time.Duration) (string, error) {
	return GenerateStateWithContinue(userID, tenant, "", secret, ttl)
}

// GenerateStateWithContinue は Continue フィールドを含む JWT state トークンを生成する。
//
// continueURL は Backlog OAuth 完了後に戻るパス。空文字は許可（継続先なし）。
// 非空の場合は ValidateContinueURL でバリデーションする。
// その他の引数は GenerateState と同様。
func GenerateStateWithContinue(userID, tenant, continueURL string, secret []byte, ttl time.Duration) (string, error) {
	if userID == "" {
		return "", ErrUnauthenticated
	}
	if tenant == "" {
		return "", ErrInvalidTenant
	}
	if len(secret) == 0 {
		return "", ErrStateInvalid
	}
	if ttl <= 0 {
		return "", ErrStateInvalid
	}

	nonce, err := generateNonce()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrStateInvalid, err)
	}

	now := time.Now()
	claims := &StateClaims{
		UserID:   userID,
		Tenant:   tenant,
		Nonce:    nonce,
		Continue: continueURL,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedString, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrStateInvalid, err)
	}

	return signedString, nil
}

// ValidateContinueURL は continue URL が同一オリジン相対パスかつ
// /authorize prefix に限定された安全な値であることを検証する。
//
// allowlist:
//   - "" (空文字): 許可（継続先なし）
//   - "/authorize" で始まる相対パス: 許可（クエリパラメータを含んでよい）
//
// 以下は拒否（ErrInvalidContinue を返す）:
//   - "//" で始まる（protocol-relative URL）
//   - "\" で始まる（backslash / Windows path 形式）
//   - Scheme が空でない（絶対 URL: "https://evil.example/..."）
//   - Host が空でない（"//" prefix 後にホスト名がある形式）
//   - Path が "/authorize" で始まらない
//
// クエリパラメータの値（"redirect_uri=http://..." 等）に "//" が含まれても
// url.Parse が Path と Query を分離するため正しく許可される。
func ValidateContinueURL(raw string) error {
	// 空文字は「継続先なし」として許容
	if raw == "" {
		return nil
	}
	// url.Parse 前に protocol-relative / backslash をブロック
	if strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "\\") {
		return ErrInvalidContinue
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidContinue, err)
	}
	// 絶対 URL（Scheme あり）または Host あり（"//" prefix 形式）を拒否
	if u.Scheme != "" || u.Host != "" {
		return ErrInvalidContinue
	}
	// /authorize prefix のみ許可
	if !strings.HasPrefix(u.Path, "/authorize") {
		return ErrInvalidContinue
	}
	return nil
}

// ValidateState は JWT state トークンを検証し、StateClaims を返す。
//
// 署名メソッドが HMAC でない場合（alg:none 攻撃等）は ErrStateInvalid を返す。
// 期限切れの場合は ErrStateExpired を返す。
// 署名不正・改竄・その他の不正は ErrStateInvalid を返す。
func ValidateState(stateJWT string, secret []byte) (*StateClaims, error) {
	claims := &StateClaims{}

	token, err := jwt.ParseWithClaims(stateJWT, claims, func(token *jwt.Token) (interface{}, error) {
		// 署名メソッドが HMAC であることを検証（アルゴリズム差し替え攻撃対策）
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		// jwt/v5 のエラーチェーンから期限切れを判別
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: %v", ErrStateExpired, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrStateInvalid, err)
	}

	if !token.Valid {
		return nil, ErrStateInvalid
	}

	// claims の整合性チェック
	if claims.UserID == "" {
		return nil, ErrStateInvalid
	}

	return claims, nil
}

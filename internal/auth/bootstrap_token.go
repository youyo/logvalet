package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/hkdf"
)

const (
	// DefaultBootstrapTokenTTL は bootstrap_token のデフォルト有効期間。
	DefaultBootstrapTokenTTL = 3 * time.Minute

	// BootstrapTokenAudience は bootstrap_token の想定受信者。
	BootstrapTokenAudience = "logvalet/multi-authorize"

	// BootstrapTokenIssuer は bootstrap_token の発行者。
	BootstrapTokenIssuer = "logvalet"

	// BootstrapTokenType は bootstrap_token の typ クレーム値。
	BootstrapTokenType = "user_bootstrap_v1"

	// hkdfInfo は HKDF の info 文字列。バージョン付きにして将来のローテーションを可能にする。
	hkdfInfo = "logvalet user bootstrap v1"
)

// BootstrapTokenClaims は bootstrap_token の JWT クレームを保持する。
type BootstrapTokenClaims struct {
	Typ         string `json:"typ"`
	BaseURLHash string `json:"base_url_hash"`
	AliasHash   string `json:"alias_hash"`
	JTI         string `json:"jti"`
	jwt.RegisteredClaims
}

// DeriveBootstrapKey は stateSecret（hex 文字列）から bootstrap token 専用 HS256 鍵を HKDF-SHA256 で導出する。
func DeriveBootstrapKey(stateSecretHex string) ([]byte, error) {
	secret, err := hex.DecodeString(stateSecretHex)
	if err != nil {
		// hex でなければ raw bytes としてそのまま使う
		secret = []byte(stateSecretHex)
	}
	if len(secret) == 0 {
		return nil, fmt.Errorf("%w: empty stateSecret", ErrBootstrapInvalid)
	}

	r := hkdf.New(sha256.New, secret, nil, []byte(hkdfInfo))
	out := make([]byte, 32)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, fmt.Errorf("auth: hkdf derive failed: %w", err)
	}
	return out, nil
}

// hashValue は入力を SHA-256 でハッシュして先頭 32 hex 文字（16 バイト）を返す。
func hashValue(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:32]
}

// normalizeBaseURLForHash は base_url をハッシュ計算前に正規化する。
// trailing slash を除去し、小文字化する。
func normalizeBaseURLForHash(baseURL string) string {
	u := strings.TrimRight(baseURL, "/")
	return strings.ToLower(u)
}

// GenerateBootstrapToken は bootstrap_token JWT を生成する。
// jti は呼び出し元で生成して渡す（NonceStore.Store と同じ値を使うため外部生成が必要）。
func GenerateBootstrapToken(userID, baseURL, alias string, key []byte, ttl time.Duration, jti string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("%w: userID is empty", ErrBootstrapInvalid)
	}
	if baseURL == "" {
		return "", fmt.Errorf("%w: baseURL is empty", ErrBootstrapInvalid)
	}
	if alias == "" {
		return "", fmt.Errorf("%w: alias is empty", ErrBootstrapInvalid)
	}
	if len(key) == 0 {
		return "", fmt.Errorf("%w: key is empty", ErrBootstrapInvalid)
	}
	if ttl <= 0 {
		return "", fmt.Errorf("%w: ttl must be positive", ErrBootstrapInvalid)
	}
	if jti == "" {
		return "", fmt.Errorf("%w: jti is empty", ErrBootstrapInvalid)
	}

	normalizedBaseURL := normalizeBaseURLForHash(baseURL)

	now := time.Now()
	claims := &BootstrapTokenClaims{
		Typ:         BootstrapTokenType,
		BaseURLHash: hashValue(normalizedBaseURL),
		AliasHash:   hashValue(alias),
		JTI:         jti,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Audience:  jwt.ClaimStrings{BootstrapTokenAudience},
			Issuer:    BootstrapTokenIssuer,
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrBootstrapInvalid, err)
	}
	return signed, nil
}

// ValidateBootstrapToken は bootstrap_token JWT を検証し、userID と jti を返す。
// alg=none・HS256 以外のアルゴリズム・クレーム不一致・期限切れを全て拒否する。
func ValidateBootstrapToken(tokenStr, baseURL, alias string, key []byte) (userID string, jti string, err error) {
	claims := &BootstrapTokenClaims{}

	token, parseErr := jwt.ParseWithClaims(
		tokenStr,
		claims,
		func(t *jwt.Token) (interface{}, error) {
			// HS256 以外（alg=none, RS256, HS384, HS512 等）を全て拒否
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			if t.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return key, nil
		},
		jwt.WithAudience(BootstrapTokenAudience),
		jwt.WithIssuer(BootstrapTokenIssuer),
		jwt.WithExpirationRequired(),
	)
	if parseErr != nil {
		if errors.Is(parseErr, jwt.ErrTokenExpired) {
			return "", "", fmt.Errorf("%w: %v", ErrBootstrapExpired, parseErr)
		}
		return "", "", fmt.Errorf("%w: %v", ErrBootstrapInvalid, parseErr)
	}

	if !token.Valid {
		return "", "", ErrBootstrapInvalid
	}

	// typ クレーム検証（トークン混同攻撃防止）
	if claims.Typ != BootstrapTokenType {
		return "", "", fmt.Errorf("%w: typ mismatch", ErrBootstrapInvalid)
	}

	// sub（userID）の空チェック
	if claims.Subject == "" {
		return "", "", fmt.Errorf("%w: empty sub", ErrBootstrapInvalid)
	}

	// jti 空チェック
	if claims.JTI == "" {
		return "", "", fmt.Errorf("%w: empty jti", ErrBootstrapInvalid)
	}

	// base_url_hash 検証
	normalizedBaseURL := normalizeBaseURLForHash(baseURL)
	expectedURLHash := hashValue(normalizedBaseURL)
	if claims.BaseURLHash != expectedURLHash {
		return "", "", fmt.Errorf("%w: base_url_hash mismatch", ErrBootstrapInvalid)
	}

	// alias_hash 検証
	expectedAliasHash := hashValue(alias)
	if claims.AliasHash != expectedAliasHash {
		return "", "", fmt.Errorf("%w: alias_hash mismatch", ErrBootstrapInvalid)
	}

	return claims.Subject, claims.JTI, nil
}

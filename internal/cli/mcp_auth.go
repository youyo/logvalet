package cli

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"

	idproxy "github.com/youyo/idproxy"
	"github.com/youyo/idproxy/store"
)

// parseSigningKey は PEM 形式の ECDSA P-256 秘密鍵をパースする。
// 空文字列の場合は新規ランダム鍵を生成する（single-instance 運用用途）。
func parseSigningKey(pemStr string) (*ecdsa.PrivateKey, error) {
	if pemStr == "" {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate signing key: %w", err)
		}
		return key, nil
	}

	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("signing-key: invalid PEM")
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("signing-key: %w", err)
	}
	if key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("signing-key: curve must be P-256, got %s", key.Curve.Params().Name)
	}
	return key, nil
}

// buildIDProxyStore は McpCmd の設定から idproxy.Store を構築する。
// 呼び出し元は利用後に Close() を呼ぶ責任がある。
func buildIDProxyStore(c *McpCmd) (idproxy.Store, error) {
	switch strings.ToLower(c.IDProxyStore) {
	case "", "memory":
		return store.NewMemoryStore(), nil
	case "dynamodb":
		s, err := store.NewDynamoDBStore(c.IDProxyStoreDynamoDBTable, c.IDProxyStoreDynamoDBRegion)
		if err != nil {
			return nil, fmt.Errorf("failed to create idproxy dynamodb store: %w", err)
		}
		return s, nil
	default:
		return nil, fmt.Errorf("invalid idproxy-store: %q", c.IDProxyStore)
	}
}

// BuildAuthConfig は McpCmd の認証フラグから idproxy.Config を構築する。
// 返される idproxy.Config.Store は利用後に Close() を呼ぶ必要がある。
func BuildAuthConfig(c *McpCmd) (idproxy.Config, error) {
	cookieSecret, err := hex.DecodeString(c.CookieSecret)
	if err != nil {
		return idproxy.Config{}, fmt.Errorf("cookie-secret: invalid hex: %w", err)
	}
	if len(cookieSecret) < 32 {
		return idproxy.Config{}, fmt.Errorf("cookie-secret: must be at least 32 bytes (64 hex chars), got %d bytes", len(cookieSecret))
	}

	signingKey, err := parseSigningKey(c.SigningKey)
	if err != nil {
		return idproxy.Config{}, err
	}

	idpStore, err := buildIDProxyStore(c)
	if err != nil {
		return idproxy.Config{}, err
	}

	var allowedDomains []string
	if c.AllowedDomains != "" {
		allowedDomains = strings.Split(c.AllowedDomains, ",")
		for i := range allowedDomains {
			allowedDomains[i] = strings.TrimSpace(allowedDomains[i])
		}
	}

	var allowedEmails []string
	if c.AllowedEmails != "" {
		allowedEmails = strings.Split(c.AllowedEmails, ",")
		for i := range allowedEmails {
			allowedEmails[i] = strings.TrimSpace(allowedEmails[i])
		}
	}

	return idproxy.Config{
		Providers: []idproxy.OIDCProvider{
			{
				Issuer:       c.OIDCIssuer,
				ClientID:     c.OIDCClientID,
				ClientSecret: c.OIDCClientSecret,
			},
		},
		AllowedDomains: allowedDomains,
		AllowedEmails:  allowedEmails,
		ExternalURL:    c.ExternalURL,
		CookieSecret:   cookieSecret,
		Store:          idpStore,
		OAuth: &idproxy.OAuthConfig{
			SigningKey: signingKey,
		},
	}, nil
}

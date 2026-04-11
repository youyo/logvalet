package cli

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	idproxy "github.com/youyo/idproxy"
	"github.com/youyo/idproxy/store"
)

// BuildAuthConfig は McpCmd の認証フラグから idproxy.Config を構築する。
func BuildAuthConfig(c *McpCmd) (idproxy.Config, error) {
	cookieSecret, err := hex.DecodeString(c.CookieSecret)
	if err != nil {
		return idproxy.Config{}, fmt.Errorf("cookie-secret: invalid hex: %w", err)
	}
	if len(cookieSecret) < 32 {
		return idproxy.Config{}, fmt.Errorf("cookie-secret: must be at least 32 bytes (64 hex chars), got %d bytes", len(cookieSecret))
	}

	signingKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return idproxy.Config{}, fmt.Errorf("failed to generate signing key: %w", err)
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
		Store:          store.NewMemoryStore(),
		OAuth: &idproxy.OAuthConfig{
			SigningKey: signingKey,
		},
	}, nil
}

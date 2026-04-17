package cli_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/cli"
)

func mustGenerateECDSAPEM(t *testing.T) (string, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	return string(pemBytes), key
}

func TestBuildAuthConfig_ValidInput(t *testing.T) {
	cmd := &cli.McpCmd{
		ExternalURL:  "https://example.com",
		OIDCIssuer:   "https://accounts.google.com",
		OIDCClientID: "client-id",
		CookieSecret: strings.Repeat("ab", 32),
	}
	cfg, err := cli.BuildAuthConfig(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ExternalURL != "https://example.com" {
		t.Errorf("ExternalURL = %q, want %q", cfg.ExternalURL, "https://example.com")
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0].Issuer != "https://accounts.google.com" {
		t.Errorf("unexpected Providers: %v", cfg.Providers)
	}
	if cfg.OAuth == nil || cfg.OAuth.SigningKey == nil {
		t.Fatal("OAuth.SigningKey is nil")
	}
}

func TestBuildAuthConfig_InvalidHex(t *testing.T) {
	cmd := &cli.McpCmd{CookieSecret: "ZZZZ"}
	_, err := cli.BuildAuthConfig(cmd)
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestBuildAuthConfig_ShortSecret(t *testing.T) {
	cmd := &cli.McpCmd{CookieSecret: strings.Repeat("ab", 16)}
	_, err := cli.BuildAuthConfig(cmd)
	if err == nil {
		t.Fatal("expected error for short secret")
	}
}

func TestBuildAuthConfig_AllowedDomainsParsing(t *testing.T) {
	cmd := &cli.McpCmd{
		CookieSecret:   strings.Repeat("ab", 32),
		OIDCIssuer:     "https://example.com",
		OIDCClientID:   "id",
		AllowedDomains: "a.com, b.com",
	}
	cfg, err := cli.BuildAuthConfig(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.AllowedDomains) != 2 || cfg.AllowedDomains[0] != "a.com" || cfg.AllowedDomains[1] != "b.com" {
		t.Errorf("AllowedDomains = %v, want [a.com b.com]", cfg.AllowedDomains)
	}
}

func TestBuildAuthConfig_EmptyOptionalFields(t *testing.T) {
	cmd := &cli.McpCmd{
		CookieSecret: strings.Repeat("ab", 32),
		OIDCIssuer:   "https://example.com",
		OIDCClientID: "id",
	}
	cfg, err := cli.BuildAuthConfig(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AllowedDomains != nil {
		t.Errorf("AllowedDomains = %v, want nil", cfg.AllowedDomains)
	}
	if cfg.AllowedEmails != nil {
		t.Errorf("AllowedEmails = %v, want nil", cfg.AllowedEmails)
	}
}

func TestBuildAuthConfig_SigningKeyIsECDSA_P256(t *testing.T) {
	cmd := &cli.McpCmd{
		CookieSecret: strings.Repeat("ab", 32),
		OIDCIssuer:   "https://example.com",
		OIDCClientID: "id",
	}
	cfg, err := cli.BuildAuthConfig(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	key, ok := cfg.OAuth.SigningKey.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatal("SigningKey is not *ecdsa.PrivateKey")
	}
	if key.Curve.Params().Name != "P-256" {
		t.Errorf("curve = %s, want P-256", key.Curve.Params().Name)
	}
}

func TestBuildAuthConfig_SigningKeyFromPEM(t *testing.T) {
	pemStr, want := mustGenerateECDSAPEM(t)
	cmd := &cli.McpCmd{
		CookieSecret: strings.Repeat("ab", 32),
		OIDCIssuer:   "https://example.com",
		OIDCClientID: "id",
		SigningKey:   pemStr,
	}
	cfg, err := cli.BuildAuthConfig(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := cfg.OAuth.SigningKey.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatal("SigningKey is not *ecdsa.PrivateKey")
	}
	if !got.Equal(want) {
		t.Error("SigningKey does not match PEM-provided key")
	}
}

func TestBuildAuthConfig_SigningKeyInvalidPEM(t *testing.T) {
	cmd := &cli.McpCmd{
		CookieSecret: strings.Repeat("ab", 32),
		OIDCIssuer:   "https://example.com",
		OIDCClientID: "id",
		SigningKey:   "not-a-pem",
	}
	_, err := cli.BuildAuthConfig(cmd)
	if err == nil || !strings.Contains(err.Error(), "signing-key") {
		t.Fatalf("expected signing-key error, got: %v", err)
	}
}

func TestBuildAuthConfig_IDProxyStore_Memory(t *testing.T) {
	cmd := &cli.McpCmd{
		CookieSecret: strings.Repeat("ab", 32),
		OIDCIssuer:   "https://example.com",
		OIDCClientID: "id",
		IDProxyStore: "memory",
	}
	cfg, err := cli.BuildAuthConfig(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Store == nil {
		t.Fatal("Store is nil")
	}
	defer cfg.Store.Close()
}

func TestBuildAuthConfig_IDProxyStore_Invalid(t *testing.T) {
	cmd := &cli.McpCmd{
		CookieSecret: strings.Repeat("ab", 32),
		OIDCIssuer:   "https://example.com",
		OIDCClientID: "id",
		IDProxyStore: "postgres",
	}
	_, err := cli.BuildAuthConfig(cmd)
	if err == nil || !strings.Contains(err.Error(), "invalid idproxy-store") {
		t.Fatalf("expected invalid-store error, got: %v", err)
	}
}

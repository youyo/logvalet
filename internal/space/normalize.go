package space

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const maxAliasLength = 64

// NormalizeBaseURL は入力 URL を "https://host" 形式に正規化する。
// scheme なし → https を付与、末尾スラッシュ除去、path/query/fragment は拒否。
func NormalizeBaseURL(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("space: base URL must not be empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("space: invalid base URL %q: %w", raw, err)
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("space: base URL must use https scheme, got %q", u.Scheme)
	}
	if u.Host == "" {
		return "", errors.New("space: base URL must have a host")
	}
	if u.Path != "" && u.Path != "/" && strings.TrimRight(u.Path, "/") != "" {
		return "", fmt.Errorf("space: base URL must not have a path: %q", raw)
	}
	if u.RawQuery != "" {
		return "", fmt.Errorf("space: base URL must not have query parameters: %q", raw)
	}
	if u.Fragment != "" {
		return "", fmt.Errorf("space: base URL must not have a fragment: %q", raw)
	}
	return u.Scheme + "://" + u.Host, nil
}

// DeriveAliasFromBaseURL は BaseURL からデフォルト alias を導出する。
// *.backlog.com / *.backlog.jp のサブドメイン第一ラベルを返す。
// カスタムドメインの場合は "" を返す（呼び出し側がユーザーに入力を求めること）。
func DeriveAliasFromBaseURL(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	host := strings.ToLower(u.Hostname())
	if strings.HasSuffix(host, ".backlog.com") || strings.HasSuffix(host, ".backlog.jp") {
		parts := strings.SplitN(host, ".", 2)
		return parts[0], nil
	}
	return "", nil
}

// DeriveInitialTenant は BaseURL から暫定 tenant を導出する。
// *.backlog.com / *.backlog.jp → サブドメイン第一ラベル（小文字）。
// カスタムドメイン → "" を返す（GetSpace() で spaceKey を取得してから設定すること）。
func DeriveInitialTenant(baseURL string) (string, error) {
	return DeriveAliasFromBaseURL(baseURL)
}

// ValidateAlias は alias が許可文字・長さ制限を満たすかを検証する。
func ValidateAlias(alias string) error {
	if alias == "" {
		return fmt.Errorf("space: alias must not be empty")
	}
	if len(alias) > maxAliasLength {
		return fmt.Errorf("space: alias must not exceed %d characters", maxAliasLength)
	}
	if strings.HasPrefix(alias, "-") {
		return fmt.Errorf("space: alias must not start with a hyphen")
	}
	for _, r := range alias {
		if !isAliasChar(r) {
			return fmt.Errorf("space: alias contains invalid character %q", r)
		}
	}
	return nil
}

func isAliasChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.'
}

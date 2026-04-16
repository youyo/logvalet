package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/auth"
)

// TestTokenRecord_AllFields は TokenRecord の全フィールドが正しく保持されることを確認する。
func TestTokenRecord_AllFields(t *testing.T) {
	now := time.Now()
	r := auth.TokenRecord{
		UserID:         "user1",
		Provider:       "backlog",
		Tenant:         "example.backlog.com",
		AccessToken:    "access-token-value",
		RefreshToken:   "refresh-token-value",
		TokenType:      "Bearer",
		Scope:          "read write",
		Expiry:         now,
		ProviderUserID: "provider-user-1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if r.UserID != "user1" {
		t.Errorf("UserID = %q, want %q", r.UserID, "user1")
	}
	if r.Provider != "backlog" {
		t.Errorf("Provider = %q, want %q", r.Provider, "backlog")
	}
	if r.Tenant != "example.backlog.com" {
		t.Errorf("Tenant = %q, want %q", r.Tenant, "example.backlog.com")
	}
	if r.AccessToken != "access-token-value" {
		t.Errorf("AccessToken = %q, want %q", r.AccessToken, "access-token-value")
	}
	if r.RefreshToken != "refresh-token-value" {
		t.Errorf("RefreshToken = %q, want %q", r.RefreshToken, "refresh-token-value")
	}
	if r.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want %q", r.TokenType, "Bearer")
	}
	if r.Scope != "read write" {
		t.Errorf("Scope = %q, want %q", r.Scope, "read write")
	}
	if !r.Expiry.Equal(now) {
		t.Errorf("Expiry = %v, want %v", r.Expiry, now)
	}
	if r.ProviderUserID != "provider-user-1" {
		t.Errorf("ProviderUserID = %q, want %q", r.ProviderUserID, "provider-user-1")
	}
	if !r.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", r.CreatedAt, now)
	}
	if !r.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", r.UpdatedAt, now)
	}
}

// TestTokenRecord_String_MaskAccessToken は String() がアクセストークンをマスクすることを確認する。
func TestTokenRecord_String_MaskAccessToken(t *testing.T) {
	r := auth.TokenRecord{
		AccessToken:  "abcdef123456",
		RefreshToken: "refreshtoken1",
	}
	s := r.String()
	// アクセストークン "abcdef123456" (12文字) は "ab...56" にマスクされる
	if !strings.Contains(s, "ab...56") {
		t.Errorf("String() = %q, want to contain masked access token %q", s, "ab...56")
	}
	// 生のアクセストークンが含まれてはいけない
	if strings.Contains(s, "abcdef123456") {
		t.Errorf("String() = %q, must not contain raw access token", s)
	}
}

// TestTokenRecord_String_MaskRefreshToken は String() がリフレッシュトークンをマスクすることを確認する。
func TestTokenRecord_String_MaskRefreshToken(t *testing.T) {
	r := auth.TokenRecord{
		AccessToken:  "accesstoken12",
		RefreshToken: "xyz789abcde",
	}
	s := r.String()
	// リフレッシュトークン "xyz789abcde" (11文字) は "xy...de" にマスクされる
	if !strings.Contains(s, "xy...de") {
		t.Errorf("String() = %q, want to contain masked refresh token %q", s, "xy...de")
	}
	// 生のリフレッシュトークンが含まれてはいけない
	if strings.Contains(s, "xyz789abcde") {
		t.Errorf("String() = %q, must not contain raw refresh token", s)
	}
}

// TestTokenRecord_String_ShortToken は短いトークン（7文字以下）が "***" にマスクされることを確認する。
func TestTokenRecord_String_ShortToken(t *testing.T) {
	r := auth.TokenRecord{
		AccessToken: "ab",
	}
	s := r.String()
	if !strings.Contains(s, "***") {
		t.Errorf("String() = %q, want to contain %q for short token", s, "***")
	}
}

// TestTokenRecord_String_EmptyToken は空トークンが "***" にマスクされることを確認する。
func TestTokenRecord_String_EmptyToken(t *testing.T) {
	r := auth.TokenRecord{
		AccessToken: "",
	}
	s := r.String()
	if !strings.Contains(s, "***") {
		t.Errorf("String() = %q, want to contain %q for empty token", s, "***")
	}
}

// TestMaskToken_Boundary は maskToken のしきい値境界（7文字→"***"、8文字→マスク形式）を確認する。
// maskToken は内部関数のため TokenRecord.String() を通じて間接的にテストする。
func TestMaskToken_Boundary(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		wantMasked  bool   // true: "***" expected, false: "ab...XX" expected
		wantContain string // expected substring in String() output
	}{
		{
			name:        "7文字は***",
			token:       "1234567",
			wantMasked:  true,
			wantContain: "***",
		},
		{
			name:        "8文字はマスク形式",
			token:       "12345678",
			wantMasked:  false,
			wantContain: "12...78",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := auth.TokenRecord{AccessToken: tt.token}
			s := r.String()
			if !strings.Contains(s, tt.wantContain) {
				t.Errorf("String() = %q, want to contain %q", s, tt.wantContain)
			}
			if tt.wantMasked && strings.Contains(s, tt.token) {
				t.Errorf("String() = %q, must not contain raw token %q", s, tt.token)
			}
		})
	}
}

// TestTokenRecord_IsExpired_Future は未来の expiry で IsExpired() が false を返すことを確認する。
func TestTokenRecord_IsExpired_Future(t *testing.T) {
	r := auth.TokenRecord{
		Expiry: time.Now().Add(1 * time.Hour),
	}
	if r.IsExpired() {
		t.Error("IsExpired() = true for future expiry, want false")
	}
}

// TestTokenRecord_IsExpired_Past は過去の expiry で IsExpired() が true を返すことを確認する。
func TestTokenRecord_IsExpired_Past(t *testing.T) {
	r := auth.TokenRecord{
		Expiry: time.Now().Add(-1 * time.Hour),
	}
	if !r.IsExpired() {
		t.Error("IsExpired() = false for past expiry, want true")
	}
}

// TestTokenRecord_IsExpired_Zero はゼロ値の expiry で IsExpired() が true を返すことを確認する。
func TestTokenRecord_IsExpired_Zero(t *testing.T) {
	r := auth.TokenRecord{} // Expiry is zero value
	if !r.IsExpired() {
		t.Error("IsExpired() = false for zero expiry, want true (zero value treated as expired)")
	}
}

// TestTokenRecord_NeedsRefresh_NotDue は expiry まで十分な時間がある場合に NeedsRefresh() が false を返すことを確認する。
func TestTokenRecord_NeedsRefresh_NotDue(t *testing.T) {
	r := auth.TokenRecord{
		Expiry: time.Now().Add(10 * time.Minute),
	}
	if r.NeedsRefresh(5 * time.Minute) {
		t.Error("NeedsRefresh(5min) = true when expiry is 10min away, want false")
	}
}

// TestTokenRecord_NeedsRefresh_Due は margin 内に expiry が迫っている場合に NeedsRefresh() が true を返すことを確認する。
func TestTokenRecord_NeedsRefresh_Due(t *testing.T) {
	r := auth.TokenRecord{
		Expiry: time.Now().Add(3 * time.Minute),
	}
	if !r.NeedsRefresh(5 * time.Minute) {
		t.Error("NeedsRefresh(5min) = false when expiry is 3min away, want true")
	}
}

// TestTokenRecord_NeedsRefresh_Expired は既に期限切れの場合に NeedsRefresh() が true を返すことを確認する。
func TestTokenRecord_NeedsRefresh_Expired(t *testing.T) {
	r := auth.TokenRecord{
		Expiry: time.Now().Add(-1 * time.Minute),
	}
	if !r.NeedsRefresh(5 * time.Minute) {
		t.Error("NeedsRefresh(5min) = false when already expired, want true")
	}
}

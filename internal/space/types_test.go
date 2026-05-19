package space_test

import (
	"testing"

	"github.com/youyo/logvalet/internal/space"
)

// T1: AuthType 定数値の検証
func TestAuthType_Constants(t *testing.T) {
	if space.AuthTypeOAuth != "oauth" {
		t.Errorf("AuthTypeOAuth = %q, want %q", space.AuthTypeOAuth, "oauth")
	}
	// credentials.go の AuthTypeAPIKey = "api_key" と一致すること
	if space.AuthTypeAPIKey != "api_key" {
		t.Errorf("AuthTypeAPIKey = %q, want %q", space.AuthTypeAPIKey, "api_key")
	}
}

// T2: SpaceStatus 定数値の検証
func TestSpaceStatus_Constants(t *testing.T) {
	tests := []struct {
		name string
		got  space.SpaceStatus
		want string
	}{
		{"Unknown", space.SpaceStatusUnknown, "unknown"},
		{"OK", space.SpaceStatusOK, "ok"},
		{"Unauthorized", space.SpaceStatusUnauthorized, "unauthorized"},
		{"NotConnected", space.SpaceStatusNotConnected, "not_connected"},
		{"Disabled", space.SpaceStatusDisabled, "disabled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.got) != tt.want {
				t.Errorf("SpaceStatus%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

// T3: SpaceRegistration のゼロ値検証
func TestSpaceRegistration_ZeroValue(t *testing.T) {
	var r space.SpaceRegistration
	if r.UserID != "" {
		t.Errorf("UserID should be empty, got %q", r.UserID)
	}
	if r.Alias != "" {
		t.Errorf("Alias should be empty, got %q", r.Alias)
	}
	if r.Disabled != false {
		t.Errorf("Disabled should be false by default")
	}
	if !r.LastVerifiedAt.IsZero() {
		t.Errorf("LastVerifiedAt should be zero value")
	}
	if !r.CreatedAt.IsZero() {
		t.Errorf("CreatedAt should be zero value")
	}
}

// T4: UserPreference のゼロ値検証
func TestUserPreference_ZeroValue(t *testing.T) {
	var p space.UserPreference
	if p.DefaultSpaceAlias != "" {
		t.Errorf("DefaultSpaceAlias should be empty string, got %q", p.DefaultSpaceAlias)
	}
	if p.UserID != "" {
		t.Errorf("UserID should be empty, got %q", p.UserID)
	}
}

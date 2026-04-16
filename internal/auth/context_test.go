package auth_test

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/auth"
)

func TestContextWithUserID_RoundTrip(t *testing.T) {
	ctx := auth.ContextWithUserID(context.Background(), "user-42")
	got, ok := auth.UserIDFromContext(ctx)
	if !ok {
		t.Fatal("UserIDFromContext returned ok=false, want true")
	}
	if got != "user-42" {
		t.Errorf("UserIDFromContext = %q, want %q", got, "user-42")
	}
}

func TestUserIDFromContext_EmptyContext(t *testing.T) {
	got, ok := auth.UserIDFromContext(context.Background())
	if ok {
		t.Error("UserIDFromContext on empty context returned ok=true, want false")
	}
	if got != "" {
		t.Errorf("UserIDFromContext = %q, want empty string", got)
	}
}

func TestUserIDFromContext_EmptyString(t *testing.T) {
	ctx := auth.ContextWithUserID(context.Background(), "")
	got, ok := auth.UserIDFromContext(ctx)
	if ok {
		t.Error("UserIDFromContext with empty string should return ok=false")
	}
	if got != "" {
		t.Errorf("UserIDFromContext = %q, want empty string", got)
	}
}

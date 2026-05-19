package space

import (
	"context"
	"errors"
	"testing"

	"github.com/youyo/logvalet/internal/config"
)

func TestResolver_AllSpaces_ReturnsOnlyEnabled(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo", Status: SpaceStatusOK})
	_ = store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "bar", Status: SpaceStatusOK})
	_ = store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "baz", Status: SpaceStatusDisabled})

	r := NewResolver(store)
	got, err := r.Resolve(ctx, "u1", Scope{AllSpaces: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 enabled spaces, got %d", len(got))
	}
	for _, s := range got {
		if s.Status == SpaceStatusDisabled {
			t.Errorf("disabled space %q should be excluded", s.Alias)
		}
	}
}

func TestResolver_AllSpaces_DisabledFlag(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Disabled=true の space のみ → enabled が 0件 → ErrNoSpacesRegistered
	_ = store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo", Disabled: true})

	r := NewResolver(store)
	_, err := r.Resolve(ctx, "u1", Scope{AllSpaces: true})
	if !errors.Is(err, ErrNoSpacesRegistered) {
		t.Fatalf("expected ErrNoSpacesRegistered for all-disabled store, got %v", err)
	}
}

func TestResolver_AllSpaces_Empty(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "baz", Status: SpaceStatusDisabled})

	r := NewResolver(store)
	_, err := r.Resolve(ctx, "u1", Scope{AllSpaces: true})
	if !errors.Is(err, ErrNoSpacesRegistered) {
		t.Fatalf("expected ErrNoSpacesRegistered, got %v", err)
	}
}

func TestResolver_Aliases_SingleAlias(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})

	r := NewResolver(store)
	got, err := r.Resolve(ctx, "u1", Scope{Aliases: []string{"foo"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Alias != "foo" {
		t.Fatalf("expected [{foo}], got %v", got)
	}
}

func TestResolver_Aliases_MultipleAliases_OrderPreserved(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})
	_ = store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "bar"})

	r := NewResolver(store)
	got, err := r.Resolve(ctx, "u1", Scope{Aliases: []string{"bar", "foo"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 spaces, got %d", len(got))
	}
	if got[0].Alias != "bar" || got[1].Alias != "foo" {
		t.Fatalf("order not preserved: got [%s, %s]", got[0].Alias, got[1].Alias)
	}
}

func TestResolver_Aliases_UnknownAlias(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	r := NewResolver(store)
	_, err := r.Resolve(ctx, "u1", Scope{Aliases: []string{"unknown"}})
	if !errors.Is(err, ErrSpaceNotFound) {
		t.Fatalf("expected ErrSpaceNotFound, got %v", err)
	}
}

func TestResolver_Aliases_PartiallyUnknown(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})

	r := NewResolver(store)
	_, err := r.Resolve(ctx, "u1", Scope{Aliases: []string{"foo", "missing"}})
	if !errors.Is(err, ErrSpaceNotFound) {
		t.Fatalf("expected ErrSpaceNotFound, got %v", err)
	}
}

func TestResolver_BothSpacesAndAllSpaces_Error(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	r := NewResolver(store)
	_, err := r.Resolve(ctx, "u1", Scope{Aliases: []string{"foo"}, AllSpaces: true})
	if !errors.Is(err, ErrInvalidSpaceScope) {
		t.Fatalf("expected ErrInvalidSpaceScope, got %v", err)
	}
}

func TestResolver_DefaultSpace_FromPreference(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})
	_ = store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "bar"})
	_ = store.PutPreference(ctx, &UserPreference{UserID: "u1", DefaultSpaceAlias: "foo"})

	r := NewResolver(store)
	got, err := r.Resolve(ctx, "u1", Scope{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Alias != "foo" {
		t.Fatalf("expected [{foo}], got %v", got)
	}
}

func TestResolver_DefaultSpace_FallbackToSingleSpace(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})

	r := NewResolver(store)
	got, err := r.Resolve(ctx, "u1", Scope{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Alias != "foo" {
		t.Fatalf("expected [{foo}], got %v", got)
	}
}

func TestResolver_DefaultSpace_MultipleSpacesNoDefault(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})
	_ = store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "bar"})

	r := NewResolver(store)
	_, err := r.Resolve(ctx, "u1", Scope{})
	if !errors.Is(err, ErrNoDefaultSpace) {
		t.Fatalf("expected ErrNoDefaultSpace, got %v", err)
	}
}

func TestResolver_LegacyProfileFallback(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	cfg := &config.ResolvedConfig{
		Space:   "myspace",
		BaseURL: "https://myspace.backlog.com",
		AuthRef: "default",
	}

	r := NewResolver(store, WithLegacyProfileFallback(cfg))
	got, err := r.Resolve(ctx, "u1", Scope{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 space from legacy fallback, got %d", len(got))
	}
	if got[0].BaseURL != "https://myspace.backlog.com" {
		t.Fatalf("unexpected BaseURL: %s", got[0].BaseURL)
	}
}

func TestResolver_LegacyProfileFallback_BuildBaseURL(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// BaseURLなし・Spaceのみの場合に BaseURL を構築する
	cfg := &config.ResolvedConfig{
		Space:   "myspace",
		AuthRef: "default",
	}

	r := NewResolver(store, WithLegacyProfileFallback(cfg))
	got, err := r.Resolve(ctx, "u1", Scope{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 space, got %d", len(got))
	}
	if got[0].BaseURL != "https://myspace.backlog.com" {
		t.Fatalf("expected constructed BaseURL, got: %s", got[0].BaseURL)
	}
}

func TestResolver_LegacyProfileFallback_NoConfig(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	r := NewResolver(store)
	_, err := r.Resolve(ctx, "u1", Scope{})
	if !errors.Is(err, ErrNoDefaultSpace) {
		t.Fatalf("expected ErrNoDefaultSpace, got %v", err)
	}
}

func TestResolver_LegacyProfileFallback_StoreHasSpaces_FallbackSkipped(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})

	cfg := &config.ResolvedConfig{
		Space:   "legacy",
		BaseURL: "https://legacy.backlog.com",
	}

	r := NewResolver(store, WithLegacyProfileFallback(cfg))
	got, err := r.Resolve(ctx, "u1", Scope{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Alias != "foo" {
		t.Fatalf("expected store space foo, got %v", got)
	}
}

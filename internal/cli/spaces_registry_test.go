package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/cli"
	"github.com/youyo/logvalet/internal/space"
)

// ---- ヘルパー ----

func newMemStore() *space.MemoryStore {
	return space.NewMemoryStore()
}

func mustUpsert(t *testing.T, s space.Store, reg *space.SpaceRegistration) {
	t.Helper()
	ctx := context.Background()
	if err := s.Upsert(ctx, reg); err != nil {
		t.Fatalf("Upsert(%s/%s): %v", reg.UserID, reg.Alias, err)
	}
}

func mustSetDefault(t *testing.T, s space.Store, userID, alias string) {
	t.Helper()
	ctx := context.Background()
	if err := s.PutPreference(ctx, &space.UserPreference{
		UserID:            userID,
		DefaultSpaceAlias: alias,
	}); err != nil {
		t.Fatalf("PutPreference: %v", err)
	}
}

// ---- T1: lv spaces list（スペースなし）----

func TestSpacesListCmd_NoSpaces(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	var stdout bytes.Buffer

	cmd := cli.SpacesListCmd{}
	err := cmd.RunWithStore(&stdout, store, "local")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, stdout.String())
	}
	spaces, ok := out["spaces"].([]interface{})
	if !ok {
		t.Fatalf("spaces field missing or not array: %v", out)
	}
	if len(spaces) != 0 {
		t.Errorf("spaces len = %d, want 0", len(spaces))
	}
}

// ---- T2: lv spaces list（スペースあり）----

func TestSpacesListCmd_WithSpaces(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	mustUpsert(t, store, &space.SpaceRegistration{
		UserID:   "local",
		Alias:    "foo",
		Tenant:   "foo",
		BaseURL:  "https://foo.backlog.com",
		AuthType: space.AuthTypeOAuth,
		Status:   space.SpaceStatusOK,
	})
	mustUpsert(t, store, &space.SpaceRegistration{
		UserID:   "local",
		Alias:    "bar",
		Tenant:   "bar",
		BaseURL:  "https://bar.backlog.com",
		AuthType: space.AuthTypeAPIKey,
		Status:   space.SpaceStatusUnknown,
	})
	mustSetDefault(t, store, "local", "foo")

	var stdout bytes.Buffer
	cmd := cli.SpacesListCmd{}
	if err := cmd.RunWithStore(&stdout, store, "local"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, stdout.String())
	}

	spaces, ok := out["spaces"].([]interface{})
	if !ok || len(spaces) != 2 {
		t.Fatalf("want 2 spaces, got %v", out["spaces"])
	}

	defaultCount := 0
	for _, v := range spaces {
		m := v.(map[string]interface{})
		if _, hasAlias := m["alias"]; !hasAlias {
			t.Error("missing alias field")
		}
		if _, hasAuthType := m["auth_type"]; !hasAuthType {
			t.Error("missing auth_type field")
		}
		if _, hasStatus := m["status"]; !hasStatus {
			t.Error("missing status field")
		}
		if def, _ := m["default"].(bool); def {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Errorf("default count = %d, want 1", defaultCount)
	}
}

// ---- T3: lv spaces remove（基本成功）----

func TestSpacesRemoveCmd_Success(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	mustUpsert(t, store, &space.SpaceRegistration{
		UserID: "local", Alias: "foo", Tenant: "foo",
		BaseURL: "https://foo.backlog.com", AuthType: space.AuthTypeOAuth,
	})
	mustSetDefault(t, store, "local", "foo")

	var stdout bytes.Buffer
	cmd := cli.SpacesRemoveCmd{Alias: "foo"}
	if err := cmd.RunWithStore(&stdout, store, "local"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	got, err := store.Get(ctx, "local", "foo")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got != nil {
		t.Error("expected space to be deleted, but it still exists")
	}
}

// ---- T4: lv spaces remove（残スペースあり → default 自動更新）----

func TestSpacesRemoveCmd_DefaultSpaceUpdated_WhenRemainingSpaces(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	mustUpsert(t, store, &space.SpaceRegistration{
		UserID: "local", Alias: "foo", Tenant: "foo",
		BaseURL: "https://foo.backlog.com", AuthType: space.AuthTypeOAuth,
		CreatedAt: time.Now(),
	})
	mustUpsert(t, store, &space.SpaceRegistration{
		UserID: "local", Alias: "bar", Tenant: "bar",
		BaseURL: "https://bar.backlog.com", AuthType: space.AuthTypeOAuth,
		CreatedAt: time.Now().Add(time.Second),
	})
	mustSetDefault(t, store, "local", "foo")

	var stdout bytes.Buffer
	cmd := cli.SpacesRemoveCmd{Alias: "foo"}
	if err := cmd.RunWithStore(&stdout, store, "local"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	pref, err := store.GetPreference(ctx, "local")
	if err != nil {
		t.Fatalf("GetPreference error: %v", err)
	}
	if pref == nil || pref.DefaultSpaceAlias == "" {
		t.Error("expected default space to be updated, got empty")
	}
	if pref.DefaultSpaceAlias == "foo" {
		t.Error("default space should not be 'foo' after removal")
	}
}

// ---- T5: lv spaces remove（残スペースなし → default クリア）----

func TestSpacesRemoveCmd_DefaultSpaceCleared_WhenNoRemaining(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	mustUpsert(t, store, &space.SpaceRegistration{
		UserID: "local", Alias: "foo", Tenant: "foo",
		BaseURL: "https://foo.backlog.com", AuthType: space.AuthTypeOAuth,
	})
	mustSetDefault(t, store, "local", "foo")

	var stdout bytes.Buffer
	cmd := cli.SpacesRemoveCmd{Alias: "foo"}
	if err := cmd.RunWithStore(&stdout, store, "local"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	pref, _ := store.GetPreference(ctx, "local")
	if pref != nil && pref.DefaultSpaceAlias != "" {
		t.Errorf("expected default to be cleared, got %q", pref.DefaultSpaceAlias)
	}

	out := stdout.String()
	if out == "" {
		t.Error("expected output message")
	}
}

// ---- T6: lv spaces remove（存在しないエイリアス）----

func TestSpacesRemoveCmd_NotExist(t *testing.T) {
	t.Parallel()
	store := newMemStore()

	var stdout bytes.Buffer
	cmd := cli.SpacesRemoveCmd{Alias: "nonexistent"}
	err := cmd.RunWithStore(&stdout, store, "local")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, space.ErrSpaceNotFound) {
		t.Errorf("error = %v, want ErrSpaceNotFound", err)
	}
}

// ---- T7: lv spaces use（成功）----

func TestSpacesUseCmd_Success(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	mustUpsert(t, store, &space.SpaceRegistration{
		UserID: "local", Alias: "foo", Tenant: "foo",
		BaseURL: "https://foo.backlog.com", AuthType: space.AuthTypeOAuth,
	})

	var stdout bytes.Buffer
	cmd := cli.SpacesUseCmd{Alias: "foo"}
	if err := cmd.RunWithStore(&stdout, store, "local"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	pref, err := store.GetPreference(ctx, "local")
	if err != nil {
		t.Fatalf("GetPreference error: %v", err)
	}
	if pref == nil || pref.DefaultSpaceAlias != "foo" {
		t.Errorf("DefaultSpaceAlias = %v, want foo", pref)
	}
}

// ---- T8: lv spaces use（未登録エイリアス）----

func TestSpacesUseCmd_NotRegistered(t *testing.T) {
	t.Parallel()
	store := newMemStore()

	var stdout bytes.Buffer
	cmd := cli.SpacesUseCmd{Alias: "nonexistent"}
	err := cmd.RunWithStore(&stdout, store, "local")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, space.ErrSpaceNotFound) {
		t.Errorf("error = %v, want ErrSpaceNotFound", err)
	}
}

// ---- T9: lv spaces verify（接続 OK）----

func TestSpacesVerifyCmd_Connected(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	mustUpsert(t, store, &space.SpaceRegistration{
		UserID:   "local",
		Alias:    "foo",
		Tenant:   "foo",
		BaseURL:  "https://foo.backlog.com",
		AuthType: space.AuthTypeAPIKey,
		Status:   space.SpaceStatusUnknown,
	})

	// APIKey スペースは token 不要 → verifyFn が ok を返す
	var stdout bytes.Buffer
	cmd := cli.SpacesVerifyCmd{}
	verifyFn := func(_ context.Context, reg space.SpaceRegistration) cli.VerifyResult {
		return cli.VerifyResult{Alias: reg.Alias, OK: true, Status: "ok"}
	}
	if err := cmd.RunWithStore(&stdout, store, "local", verifyFn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %s", stdout.String())
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	r := results[0].(map[string]interface{})
	if r["status"] != "ok" {
		t.Errorf("status = %v, want ok", r["status"])
	}
}

// ---- T10: lv spaces verify（token_missing）----

func TestSpacesVerifyCmd_TokenMissing(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	mustUpsert(t, store, &space.SpaceRegistration{
		UserID:   "local",
		Alias:    "foo",
		Tenant:   "foo",
		BaseURL:  "https://foo.backlog.com",
		AuthType: space.AuthTypeOAuth,
		Status:   space.SpaceStatusNotConnected,
	})

	var stdout bytes.Buffer
	cmd := cli.SpacesVerifyCmd{}
	verifyFn := func(_ context.Context, reg space.SpaceRegistration) cli.VerifyResult {
		return cli.VerifyResult{
			Alias:     reg.Alias,
			OK:        false,
			Status:    "error",
			ErrorCode: "token_missing",
			Message:   "run 'lv spaces connect --alias foo' to reconnect",
		}
	}
	if err := cmd.RunWithStore(&stdout, store, "local", verifyFn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %s", stdout.String())
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	r := results[0].(map[string]interface{})
	if r["error_code"] != "token_missing" {
		t.Errorf("error_code = %v, want token_missing", r["error_code"])
	}
	msg, _ := r["message"].(string)
	if msg == "" {
		t.Error("expected non-empty message for token_missing")
	}
}

// ---- T11: lv spaces verify（unauthorized）----

func TestSpacesVerifyCmd_Unauthorized(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	mustUpsert(t, store, &space.SpaceRegistration{
		UserID:   "local",
		Alias:    "foo",
		Tenant:   "foo",
		BaseURL:  "https://foo.backlog.com",
		AuthType: space.AuthTypeOAuth,
		Status:   space.SpaceStatusUnauthorized,
	})

	var stdout bytes.Buffer
	cmd := cli.SpacesVerifyCmd{}
	verifyFn := func(_ context.Context, reg space.SpaceRegistration) cli.VerifyResult {
		return cli.VerifyResult{
			Alias:     reg.Alias,
			OK:        false,
			Status:    "error",
			ErrorCode: "unauthorized",
		}
	}
	if err := cmd.RunWithStore(&stdout, store, "local", verifyFn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %s", stdout.String())
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	r := results[0].(map[string]interface{})
	if r["error_code"] != "unauthorized" {
		t.Errorf("error_code = %v, want unauthorized", r["error_code"])
	}
}

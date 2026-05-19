package space

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMemoryStore_UpsertAndGet(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	reg := &SpaceRegistration{
		UserID:  "u1",
		Alias:   "foo",
		Tenant:  "example",
		BaseURL: "https://example.backlog.com",
	}
	if err := s.Upsert(ctx, reg); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := s.Get(ctx, "u1", "foo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.UserID != "u1" || got.Alias != "foo" || got.Tenant != "example" {
		t.Errorf("unexpected value: %+v", got)
	}
}

func TestMemoryStore_UpsertOverwrite(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo", Tenant: "old"})
	s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo", Tenant: "new"})

	got, err := s.Get(ctx, "u1", "foo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Tenant != "new" {
		t.Errorf("expected Tenant=new, got %s", got.Tenant)
	}
}

func TestMemoryStore_List_UserIsolation(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})
	s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "bar"})
	s.Upsert(ctx, &SpaceRegistration{UserID: "u2", Alias: "foo"})

	u1List, err := s.List(ctx, "u1")
	if err != nil {
		t.Fatalf("List u1: %v", err)
	}
	if len(u1List) != 2 {
		t.Errorf("expected 2 for u1, got %d", len(u1List))
	}
	for _, r := range u1List {
		if r.UserID != "u1" {
			t.Errorf("u2 data leaked into u1 List: %+v", r)
		}
	}

	u2List, err := s.List(ctx, "u2")
	if err != nil {
		t.Fatalf("List u2: %v", err)
	}
	if len(u2List) != 1 {
		t.Errorf("expected 1 for u2, got %d", len(u2List))
	}
}

func TestMemoryStore_List_Empty(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	result, err := s.List(ctx, "nobody")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty, got %d items", len(result))
	}
}

func TestMemoryStore_Delete_TargetOnly(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})
	s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "bar"})

	if err := s.Delete(ctx, "u1", "foo"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := s.Get(ctx, "u1", "foo")
	if err != nil {
		t.Fatalf("Get foo: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete, got value")
	}

	got, err = s.Get(ctx, "u1", "bar")
	if err != nil {
		t.Fatalf("Get bar: %v", err)
	}
	if got == nil {
		t.Error("bar should still exist")
	}
}

func TestMemoryStore_Delete_DifferentUserSameAlias(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: "foo"})
	s.Upsert(ctx, &SpaceRegistration{UserID: "u2", Alias: "foo"})

	if err := s.Delete(ctx, "u1", "foo"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := s.Get(ctx, "u2", "foo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Error("u2/foo should still exist after deleting u1/foo")
	}
}

func TestMemoryStore_Delete_NotExist(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	if err := s.Delete(ctx, "u1", "nonexistent"); err != nil {
		t.Errorf("Delete of nonexistent should not error, got: %v", err)
	}
}

func TestMemoryStore_Preference_GetPut(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	pref := &UserPreference{UserID: "u1", DefaultSpaceAlias: "foo"}
	if err := s.PutPreference(ctx, pref); err != nil {
		t.Fatalf("PutPreference: %v", err)
	}

	got, err := s.GetPreference(ctx, "u1")
	if err != nil {
		t.Fatalf("GetPreference: %v", err)
	}
	if got == nil {
		t.Fatal("GetPreference returned nil")
	}
	if got.DefaultSpaceAlias != "foo" {
		t.Errorf("expected foo, got %s", got.DefaultSpaceAlias)
	}
}

func TestMemoryStore_Preference_GetNotExist(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	got, err := s.GetPreference(ctx, "nobody")
	if err != nil {
		t.Fatalf("GetPreference should not error for unset user, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for unset user, got %+v", got)
	}
}

func TestMemoryStore_Preference_UserIsolation(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.PutPreference(ctx, &UserPreference{UserID: "u1", DefaultSpaceAlias: "foo"})
	s.PutPreference(ctx, &UserPreference{UserID: "u2", DefaultSpaceAlias: "bar"})

	p1, _ := s.GetPreference(ctx, "u1")
	p2, _ := s.GetPreference(ctx, "u2")

	if p1.DefaultSpaceAlias != "foo" {
		t.Errorf("u1: expected foo, got %s", p1.DefaultSpaceAlias)
	}
	if p2.DefaultSpaceAlias != "bar" {
		t.Errorf("u2: expected bar, got %s", p2.DefaultSpaceAlias)
	}
}

func TestMemoryStore_Close_Idempotent(t *testing.T) {
	s := NewMemoryStore()

	if err := s.Close(); err != nil {
		t.Fatalf("Close 1st: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close 2nd: %v", err)
	}
}

func TestMemoryStore_Concurrent_Upsert(t *testing.T) {
	t.Parallel()
	s := NewMemoryStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		alias := string(rune('a' + i))
		wg.Add(1)
		go func(a string) {
			defer wg.Done()
			s.Upsert(ctx, &SpaceRegistration{UserID: "u1", Alias: a})
		}(alias)
	}
	wg.Wait()

	list, err := s.List(ctx, "u1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 10 {
		t.Errorf("expected 10, got %d", len(list))
	}
}

func TestMemoryStore_NonceStore_StoreAndConsume(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	if err := s.Store(ctx, "u1", "nonce1", 1*time.Minute); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := s.Consume(ctx, "u1", "nonce1"); err != nil {
		t.Fatalf("Consume 1st: %v", err)
	}

	if err := s.Consume(ctx, "u1", "nonce1"); err != ErrNonceAlreadyUsed {
		t.Errorf("Consume 2nd: expected ErrNonceAlreadyUsed, got %v", err)
	}
}

func TestMemoryStore_NonceStore_ConsumeNotExist(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	if err := s.Consume(ctx, "u1", "ghost"); err != ErrNonceAlreadyUsed {
		t.Errorf("expected ErrNonceAlreadyUsed for nonexistent nonce, got %v", err)
	}
}

func TestMemoryStore_NonceStore_UserIsolation(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	if err := s.Store(ctx, "u1", "nonce1", 1*time.Minute); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := s.Consume(ctx, "u2", "nonce1"); err != ErrNonceAlreadyUsed {
		t.Errorf("expected ErrNonceAlreadyUsed for different user, got %v", err)
	}
}

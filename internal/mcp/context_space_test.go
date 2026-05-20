package mcp

import (
	"context"
	"testing"

	"github.com/youyo/logvalet/internal/space"
)

// T1-a: context にスペースがない場合は fallback 値を返す。
func TestSpaceInfoFromContext_NoSpaceInContext_UsesFallback(t *testing.T) {
	ctx := context.Background()
	alias, baseURL := spaceInfoFromContext(ctx, "fallback-space", "https://fallback.example.com")
	if alias != "fallback-space" {
		t.Errorf("expected alias=fallback-space, got %q", alias)
	}
	if baseURL != "https://fallback.example.com" {
		t.Errorf("expected baseURL=https://fallback.example.com, got %q", baseURL)
	}
}

// T1-b: context に SpaceRegistration がある場合は reg.Alias / reg.BaseURL を返す。
func TestSpaceInfoFromContext_WithSpaceInContext_ReturnsRegistrationValues(t *testing.T) {
	reg := space.SpaceRegistration{Alias: "megumilog", BaseURL: "https://megumilog.backlog.jp"}
	ctx := contextWithSpace(context.Background(), reg)
	alias, baseURL := spaceInfoFromContext(ctx, "ignored", "https://ignored")
	if alias != "megumilog" {
		t.Errorf("expected alias=megumilog, got %q", alias)
	}
	if baseURL != "https://megumilog.backlog.jp" {
		t.Errorf("expected baseURL=https://megumilog.backlog.jp, got %q", baseURL)
	}
}

// T1-c: Alias または BaseURL のいずれかが空なら fallback を使う（R4）。
func TestSpaceInfoFromContext_EmptyRegistration_UsesFallback(t *testing.T) {
	cases := []space.SpaceRegistration{
		{Alias: "", BaseURL: "https://x.example"},
		{Alias: "x", BaseURL: ""},
	}
	for _, r := range cases {
		ctx := contextWithSpace(context.Background(), r)
		a, b := spaceInfoFromContext(ctx, "fb-s", "https://fb")
		if a != "fb-s" || b != "https://fb" {
			t.Errorf("expected fallback for %+v, got alias=%q baseURL=%q", r, a, b)
		}
	}
}

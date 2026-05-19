package e2e_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/credentials"
	"github.com/youyo/logvalet/internal/space"
)

// fakeBacklogServer は最小限の Backlog API モックサーバーを構築する。
// expectedToken が空でない場合、Bearer トークンを検証して不一致なら 401 を返す。
func fakeBacklogServer(t *testing.T, expectedToken string, responseBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if expectedToken != "" {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer "+expectedToken {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"errors":[{"message":"unauthorized","code":11}]}`))
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseBody))
	}))
}

// TC1: issue list --all-spaces → foo + bar 両方の結果が返る
func TestE2E_FanOut_MultipleBacklogServers(t *testing.T) {
	fooIssues := `[{"id":1,"issueKey":"FOO-1","summary":"foo issue","description":"","status":{"id":1,"name":"未対応"}}]`
	barIssues := `[{"id":2,"issueKey":"BAR-1","summary":"bar issue","description":"","status":{"id":1,"name":"未対応"}}]`

	fooServer := fakeBacklogServer(t, "token-foo", fooIssues)
	defer fooServer.Close()

	barServer := fakeBacklogServer(t, "token-bar", barIssues)
	defer barServer.Close()

	fooReg := space.SpaceRegistration{
		UserID:  "user1",
		Alias:   "foo",
		Tenant:  "foo",
		BaseURL: fooServer.URL,
	}
	barReg := space.SpaceRegistration{
		UserID:  "user1",
		Alias:   "bar",
		Tenant:  "bar",
		BaseURL: barServer.URL,
	}

	factory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		var token string
		if reg.Alias == "foo" {
			token = "token-foo"
		} else {
			token = "token-bar"
		}
		cred := &credentials.ResolvedCredential{
			AuthType:    credentials.AuthTypeOAuth,
			AccessToken: token,
		}
		return backlog.NewHTTPClient(backlog.ClientConfig{
			BaseURL:    reg.BaseURL,
			Credential: cred,
		}), nil
	}

	ex := &space.Executor{
		Factory:        factory,
		MaxConcurrency: 2,
	}

	type issueResult struct {
		IssueKey string `json:"issueKey"`
		Summary  string `json:"summary"`
	}

	results := space.ExecuteAcrossSpaces(
		context.Background(),
		ex,
		[]space.SpaceRegistration{fooReg, barReg},
		func(ctx context.Context, reg space.SpaceRegistration, c backlog.Client) (json.RawMessage, error) {
			issues, err := c.ListIssues(ctx, backlog.ListIssuesOptions{})
			if err != nil {
				return nil, err
			}
			b, err := json.Marshal(issues)
			if err != nil {
				return nil, err
			}
			return json.RawMessage(b), nil
		},
	)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for i, r := range results {
		if !r.OK {
			t.Errorf("results[%d] (space=%s) OK=false, error=%s", i, r.SpaceAlias, r.Error)
		}
	}

	if results[0].SpaceAlias != "foo" {
		t.Errorf("results[0].SpaceAlias = %q, want foo", results[0].SpaceAlias)
	}
	if results[1].SpaceAlias != "bar" {
		t.Errorf("results[1].SpaceAlias = %q, want bar", results[1].SpaceAlias)
	}
}

// TC2: 片方が 401 → partial failure（ok:true 1件 + ok:false 1件）
func TestE2E_FanOut_PartialFailure(t *testing.T) {
	fooIssues := `[{"id":1,"issueKey":"FOO-1","summary":"foo issue","description":"","status":{"id":1,"name":"未対応"}}]`

	fooServer := fakeBacklogServer(t, "token-foo", fooIssues)
	defer fooServer.Close()

	// bar サーバーは常に 401 を返す
	barServer := fakeBacklogServer(t, "wrong-token", "")
	defer barServer.Close()

	fooReg := space.SpaceRegistration{
		UserID:  "user1",
		Alias:   "foo",
		Tenant:  "foo",
		BaseURL: fooServer.URL,
	}
	barReg := space.SpaceRegistration{
		UserID:  "user1",
		Alias:   "bar",
		Tenant:  "bar",
		BaseURL: barServer.URL,
	}

	factory := func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		// bar は意図的に間違ったトークンで接続
		var token string
		if reg.Alias == "foo" {
			token = "token-foo"
		} else {
			token = "token-bar" // bar サーバーは wrong-token を期待するので 401 になる
		}
		cred := &credentials.ResolvedCredential{
			AuthType:    credentials.AuthTypeOAuth,
			AccessToken: token,
		}
		return backlog.NewHTTPClient(backlog.ClientConfig{
			BaseURL:    reg.BaseURL,
			Credential: cred,
		}), nil
	}

	ex := &space.Executor{
		Factory:        factory,
		MaxConcurrency: 2,
	}

	results := space.ExecuteAcrossSpaces(
		context.Background(),
		ex,
		[]space.SpaceRegistration{fooReg, barReg},
		func(ctx context.Context, reg space.SpaceRegistration, c backlog.Client) (string, error) {
			_, err := c.ListIssues(ctx, backlog.ListIssuesOptions{})
			if err != nil {
				return "", err
			}
			return "ok", nil
		},
	)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// foo は成功
	if !results[0].OK {
		t.Errorf("results[0] (foo) should be OK=true, got OK=false error=%s", results[0].Error)
	}

	// bar は失敗
	if results[1].OK {
		t.Errorf("results[1] (bar) should be OK=false")
	}
	if results[1].ErrorCode != "unauthorized" {
		t.Errorf("results[1].ErrorCode = %q, want unauthorized", results[1].ErrorCode)
	}
}

// TC3: user A の all_spaces が user B のスペースを含まない（user isolation）
func TestE2E_UserIsolation_AllSpaces(t *testing.T) {
	fooIssues := `[{"id":1,"issueKey":"FOO-1","summary":"foo issue","description":"","status":{"id":1,"name":"未対応"}}]`
	bazIssues := `[{"id":3,"issueKey":"BAZ-1","summary":"baz issue","description":"","status":{"id":1,"name":"未対応"}}]`

	fooServer := fakeBacklogServer(t, "", fooIssues)
	defer fooServer.Close()
	bazServer := fakeBacklogServer(t, "", bazIssues)
	defer bazServer.Close()

	store := space.NewMemoryStore()
	ctx := context.Background()

	// userA: spaces [foo, bar の代わりに fooServer]
	store.Upsert(ctx, &space.SpaceRegistration{
		UserID:  "userA",
		Alias:   "foo",
		Tenant:  "foo",
		BaseURL: fooServer.URL,
	})

	// userB: spaces [baz]
	store.Upsert(ctx, &space.SpaceRegistration{
		UserID:  "userB",
		Alias:   "baz",
		Tenant:  "baz",
		BaseURL: bazServer.URL,
	})

	resolver := space.NewResolver(store)

	// userA の all_spaces を取得
	userASpaces, err := resolver.Resolve(ctx, "userA", space.Scope{AllSpaces: true})
	if err != nil {
		t.Fatalf("Resolve userA: %v", err)
	}

	// userB の all_spaces を取得
	userBSpaces, err := resolver.Resolve(ctx, "userB", space.Scope{AllSpaces: true})
	if err != nil {
		t.Fatalf("Resolve userB: %v", err)
	}

	// userA のスペースに baz が含まれないことを確認
	for _, s := range userASpaces {
		if s.Alias == "baz" {
			t.Errorf("userA's spaces should not contain baz (userB's space)")
		}
		if s.UserID != "userA" {
			t.Errorf("userA's space has wrong UserID: %q", s.UserID)
		}
	}

	// userB のスペースに foo が含まれないことを確認
	for _, s := range userBSpaces {
		if s.Alias == "foo" {
			t.Errorf("userB's spaces should not contain foo (userA's space)")
		}
		if s.UserID != "userB" {
			t.Errorf("userB's space has wrong UserID: %q", s.UserID)
		}
	}

	if len(userASpaces) != 1 || userASpaces[0].Alias != "foo" {
		t.Errorf("userA should have exactly [foo], got %v", userASpaces)
	}
	if len(userBSpaces) != 1 || userBSpaces[0].Alias != "baz" {
		t.Errorf("userB should have exactly [baz], got %v", userBSpaces)
	}
}

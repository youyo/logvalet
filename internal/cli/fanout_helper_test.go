package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/credentials"
	"github.com/youyo/logvalet/internal/domain"
	"github.com/youyo/logvalet/internal/space"
)

// ---- ヘルパー ----

// newTestSpaceReg は alias と baseURL を持つ SpaceRegistration を作成する。
func newTestSpaceReg(alias, baseURL string) space.SpaceRegistration {
	return space.SpaceRegistration{
		UserID:   "local",
		Alias:    alias,
		Tenant:   alias,
		BaseURL:  baseURL,
		AuthType: space.AuthTypeAPIKey,
	}
}

// staticIssueServer は /api/v2/issues を固定の Issues 配列で応答するテストサーバー。
func staticIssueServer(t *testing.T, issues []domain.Issue) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(issues)
	}))
}

// errorServer は常に HTTP 401 を返すテストサーバー。
func errorServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"errorCode": "AuthenticationException"})
	}))
}

// buildTestFactory は SpaceRegistration の BaseURL に対して backlog.NewHTTPClient を
// API キーなしで生成するシンプルなテスト用 ClientFactory。
func buildTestFactory() space.ClientFactory {
	return func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		cred := &credentials.ResolvedCredential{
			AuthType:    credentials.AuthTypeAPIKey,
			APIKey:      "test-api-key",
			Source:      "test",
		}
		return backlog.NewHTTPClient(backlog.ClientConfig{
			BaseURL:    reg.BaseURL,
			Credential: cred,
		}), nil
	}
}

// ---- T1: --spaces なし → runAcrossSpaces が buildRunContext パスを使う ----
// この統合テストは実際の buildRunContext が credentials を必要とするため、
// 単体での検証は fanout_helper のロジックに集中する。
// buildRunContext は runner_test.go でカバーされているため、ここでは
// multi-space モードのロジックのみを直接テストする。

// T2: buildMultiSpaceContext - 2スペースへの fan-out で全成功
func TestRunAcrossSpaces_TwoSpaces_AllSuccess(t *testing.T) {
	fooIssues := []domain.Issue{{ID: 1, Summary: "foo-issue"}}
	barIssues := []domain.Issue{{ID: 2, Summary: "bar-issue"}}

	fooSrv := staticIssueServer(t, fooIssues)
	defer fooSrv.Close()
	barSrv := staticIssueServer(t, barIssues)
	defer barSrv.Close()

	regs := []space.SpaceRegistration{
		newTestSpaceReg("foo", fooSrv.URL),
		newTestSpaceReg("bar", barSrv.URL),
	}
	factory := buildTestFactory()

	var buf bytes.Buffer
	exitCode, err := runAcrossSpacesWithFactory(
		context.Background(),
		factory,
		regs,
		&buf,
		func(ctx context.Context, reg space.SpaceRegistration, client backlog.Client) ([]domain.Issue, error) {
			return client.ListIssues(ctx, backlog.ListIssuesOptions{Limit: 100})
		},
	)

	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exit code は 0 期待, 実際 %d", exitCode)
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("JSON パースエラー: %v, 出力: %s", err, buf.String())
	}
	if len(results) != 2 {
		t.Fatalf("結果数: 期待 2, 実際 %d", len(results))
	}
	// 両方 ok=true
	for _, r := range results {
		if ok, _ := r["ok"].(bool); !ok {
			t.Errorf("スペース %v: ok=true 期待, 実際 false", r["space"])
		}
	}
}

// T3: partial failure - fooが成功、barが401 → exit code 8、partialFailureError を返す
func TestRunAcrossSpaces_PartialFailure(t *testing.T) {
	fooIssues := []domain.Issue{{ID: 1, Summary: "foo-issue"}}

	fooSrv := staticIssueServer(t, fooIssues)
	defer fooSrv.Close()
	barSrv := errorServer(t)
	defer barSrv.Close()

	regs := []space.SpaceRegistration{
		newTestSpaceReg("foo", fooSrv.URL),
		newTestSpaceReg("bar", barSrv.URL),
	}
	factory := buildTestFactory()

	var buf bytes.Buffer
	exitCode, err := runAcrossSpacesWithFactory(
		context.Background(),
		factory,
		regs,
		&buf,
		func(ctx context.Context, reg space.SpaceRegistration, client backlog.Client) ([]domain.Issue, error) {
			return client.ListIssues(ctx, backlog.ListIssuesOptions{Limit: 100})
		},
	)

	// partial failure → exit code 8 かつ partialFailureError
	if exitCode != 8 {
		t.Fatalf("exit code は 8 期待, 実際 %d", exitCode)
	}
	if err == nil {
		t.Fatal("partialFailureError が期待されたが nil")
	}
	var pfe *partialFailureError
	if !errors.As(err, &pfe) {
		t.Fatalf("*partialFailureError が期待されたが %T", err)
	}

	var results []map[string]interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &results); jsonErr != nil {
		t.Fatalf("JSON パースエラー: %v, 出力: %s", jsonErr, buf.String())
	}
	if len(results) != 2 {
		t.Fatalf("結果数: 期待 2, 実際 %d", len(results))
	}

	// 入力順が保持されること
	if results[0]["space"] != "foo" {
		t.Errorf("results[0].space: 期待 foo, 実際 %v", results[0]["space"])
	}
	if results[1]["space"] != "bar" {
		t.Errorf("results[1].space: 期待 bar, 実際 %v", results[1]["space"])
	}
	// foo は ok=true
	if ok, _ := results[0]["ok"].(bool); !ok {
		t.Errorf("foo: ok=true 期待")
	}
	// bar は ok=false
	if ok, _ := results[1]["ok"].(bool); ok {
		t.Errorf("bar: ok=false 期待")
	}
}

// T4: 全スペース失敗 → exit code 1、allFailureError を返す
func TestRunAcrossSpaces_AllFailure(t *testing.T) {
	barSrv := errorServer(t)
	defer barSrv.Close()

	regs := []space.SpaceRegistration{
		newTestSpaceReg("bar", barSrv.URL),
	}
	factory := buildTestFactory()

	var buf bytes.Buffer
	exitCode, err := runAcrossSpacesWithFactory(
		context.Background(),
		factory,
		regs,
		&buf,
		func(ctx context.Context, reg space.SpaceRegistration, client backlog.Client) ([]domain.Issue, error) {
			return client.ListIssues(ctx, backlog.ListIssuesOptions{Limit: 100})
		},
	)

	// 全失敗 → exit code 1、allFailureError
	if exitCode != 1 {
		t.Fatalf("exit code は 1 期待, 実際 %d", exitCode)
	}
	if err == nil {
		t.Fatal("allFailureError が期待されたが nil")
	}
	var afe *allFailureError
	if !errors.As(err, &afe) {
		t.Fatalf("*allFailureError が期待されたが %T", err)
	}
}

// T5: ファクトリエラー → ok=false、allFailureError を返す
func TestRunAcrossSpaces_FactoryError(t *testing.T) {
	regs := []space.SpaceRegistration{
		newTestSpaceReg("foo", "http://localhost:9"),
	}
	errFactory := space.ClientFactory(func(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
		return nil, errors.New("factory error")
	})

	var buf bytes.Buffer
	exitCode, err := runAcrossSpacesWithFactory(
		context.Background(),
		errFactory,
		regs,
		&buf,
		func(ctx context.Context, reg space.SpaceRegistration, client backlog.Client) ([]domain.Issue, error) {
			return client.ListIssues(ctx, backlog.ListIssuesOptions{Limit: 100})
		},
	)

	if exitCode != 1 {
		t.Fatalf("exit code は 1 期待, 実際 %d", exitCode)
	}
	var afe *allFailureError
	if !errors.As(err, &afe) {
		t.Fatalf("*allFailureError が期待されたが %T", err)
	}
}

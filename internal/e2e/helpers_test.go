//go:build e2e

package e2e_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/credentials"
)

// e2eEnv は E2E テストに必要な環境変数をまとめた構造体。
type e2eEnv struct {
	APIKey     string
	Space      string
	ProjectKey string
	IssueKey   string
}

// loadE2EEnv は環境変数から E2E テスト設定を読み込む。
// 必須変数が未設定の場合は t.Skip() でテストをスキップする。
func loadE2EEnv(t *testing.T) *e2eEnv {
	t.Helper()

	apiKey := os.Getenv("LOGVALET_E2E_API_KEY")
	if apiKey == "" {
		t.Skip("LOGVALET_E2E_API_KEY が未設定のため E2E テストをスキップします")
	}

	space := os.Getenv("LOGVALET_E2E_SPACE")
	if space == "" {
		t.Skip("LOGVALET_E2E_SPACE が未設定のため E2E テストをスキップします")
	}

	projectKey := os.Getenv("LOGVALET_E2E_PROJECT_KEY")
	if projectKey == "" {
		t.Skip("LOGVALET_E2E_PROJECT_KEY が未設定のため E2E テストをスキップします")
	}

	issueKey := os.Getenv("LOGVALET_E2E_ISSUE_KEY")
	if issueKey == "" {
		// issue_key はオプション扱い。空の場合はプロジェクトキー + "-1" を使用。
		issueKey = fmt.Sprintf("%s-1", projectKey)
	}

	return &e2eEnv{
		APIKey:     apiKey,
		Space:      space,
		ProjectKey: projectKey,
		IssueKey:   issueKey,
	}
}

// newE2EClient は E2E テスト用の Backlog クライアントを生成する。
func newE2EClient(env *e2eEnv) backlog.Client {
	baseURL := fmt.Sprintf("https://%s.backlog.com", env.Space)
	cred := &credentials.ResolvedCredential{
		AuthType: credentials.AuthTypeAPIKey,
		APIKey:   env.APIKey,
		Source:   "env",
	}
	return backlog.NewHTTPClient(backlog.ClientConfig{
		BaseURL:    baseURL,
		Credential: cred,
	})
}

// e2eBaseURL は E2E テスト用の BaseURL を返す。
func e2eBaseURL(env *e2eEnv) string {
	return fmt.Sprintf("https://%s.backlog.com", env.Space)
}

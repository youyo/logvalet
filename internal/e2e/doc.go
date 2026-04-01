// Package e2e は E2E テストを含む。
// E2E テストは //go:build e2e ビルドタグで分離されている。
// 実行するには: go test -tags e2e ./internal/e2e/
// 環境変数が必要:
//   - LOGVALET_E2E_API_KEY: Backlog API キー
//   - LOGVALET_E2E_SPACE: Backlog スペース名
//   - LOGVALET_E2E_PROJECT_KEY: テスト用プロジェクトキー
//   - LOGVALET_E2E_ISSUE_KEY: テスト用課題キー（省略時は PROJECT_KEY-1 を使用）
package e2e

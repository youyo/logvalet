---
title: logvalet MCP 認証 E2E テスト
project: logvalet
author: planning-agent
created: 2026-04-11
status: Ready for Review
complexity: M
---

# logvalet MCP 認証 E2E テスト

## コンテキスト

先ほど `internal/cli/mcp.go` に idproxy 認証ミドルウェアを追加した（commit 05a6655）。
現在のユニットテストは Validate / BuildAuthConfig / healthHandler をカバーしているが、
**認証付き HTTP サーバーの実際の動作**（401 拒否、Bearer 通過、OAuth フロー）は未検証。

idproxy の `testutil` パッケージ（MockIdP / MockMCP）を再利用し、
logvalet 固有の Handler Topology を E2E で検証する。

## スコープ

### 実装範囲
- `internal/cli/mcp_integration_test.go` を新規作成（`//go:build integration`）
- idproxy testutil.MockIdP で OIDC を完全モック
- 認証付き MCP サーバーの HTTP レベルテスト

### スコープ外
- MCP ツール実行の検証（Backlog API モック必要、別タスク）
- 実 OIDC プロバイダーとの結合テスト

## テスト設計書

### テストヘルパー

`setupTestAuthServer(t *testing.T)` — テスト用の認証付き HTTP サーバーを構築:
1. `testutil.NewMockIdP(t)` で Fake OIDC Provider 起動
2. ECDSA P-256 署名鍵生成
3. `idproxy.Config` 構築（MockIdP を Provider に設定）
4. `idproxy.New()` で Auth 初期化
5. logvalet と同じ Handler Topology を組み立て:
   - topMux: `/healthz` → healthHandler, `/` → `auth.Wrap(mcpMux)`
   - mcpMux: `/mcp` → 簡易エコーハンドラー（200 + JSON 応答）
6. `httptest.NewServer(topMux)` を返す

`newNoRedirectClient()` — リダイレクトを追跡しない HTTP クライアント（302 キャプチャ用）

### テストケース

| ID | テスト名 | シナリオ | 期待結果 |
|----|----------|----------|----------|
| E1 | `TestIntegration_HealthzBypassesAuth` | GET /healthz（トークンなし） | 200 + `{"status":"ok"}` |
| E2 | `TestIntegration_UnauthenticatedMCPReturns401` | POST /mcp（トークンなし、Accept: application/json） | 401 Unauthorized |
| E3 | `TestIntegration_ValidBearerTokenAccessesMCP` | POST /mcp + `Authorization: Bearer <valid_jwt>` | 200 |
| E4 | `TestIntegration_InvalidBearerTokenReturns401` | POST /mcp + `Authorization: Bearer invalid-token` | 401 |
| E5 | `TestIntegration_ExpiredBearerTokenReturns401` | POST /mcp + 期限切れ JWT | 401 |
| E6 | `TestIntegration_OAuthMetadataDiscovery` | GET /.well-known/oauth-authorization-server | 200 + issuer, endpoints |
| E7 | `TestIntegration_DynamicClientRegistration` | POST /register + redirect_uris | 201 + client_id |
| E8 | `TestIntegration_BrowserLoginRedirectsToIdP` | GET /login（no-redirect client） | 302 → MockIdP /authorize |

### Bearer トークン生成

E3: `mockIdP.IssueAccessToken()` で有効な JWT を生成（audience=srv.URL, 有効期限=1時間後）
E5: `mockIdP.IssueAccessToken()` で期限切れ JWT を生成（expiresAt=過去）

## 実装手順

### Step 1: テストファイル作成

**新規ファイル:** `internal/cli/mcp_integration_test.go`

```go
//go:build integration

package cli_test

import (
    "context"
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    idproxy "github.com/youyo/idproxy"
    "github.com/youyo/idproxy/store"
    "github.com/youyo/idproxy/testutil"
    "github.com/youyo/logvalet/internal/cli"
)
```

ヘルパー + テスト8件を1ファイルに収める。

### Step 2: テスト実行

```bash
go test -tags=integration ./internal/cli/... -run TestIntegration -v
```

## リスク評価

| リスク | 重大度 | 対策 |
|--------|--------|------|
| MockIdP の OIDC discovery が idproxy.New() に時間がかかる | 低 | httptest.Server はローカル接続、数ms |
| Bearer トークンの audience 不一致で検証失敗 | 中 | setupTestAuthServer で srv.URL を audience に設定 |
| auth.Wrap() の内部パス判定がテスト環境で異なる | 低 | idproxy 自身の integration_test.go で同パターンが検証済み |

## 変更ファイル一覧

| ファイル | 操作 | 概要 |
|----------|------|------|
| `internal/cli/mcp_integration_test.go` | 新規 | E2E 認証テスト 8件 |

## 検証方法

```bash
# integration テスト実行
go test -tags=integration ./internal/cli/... -run TestIntegration -v

# 通常テスト（integration テストは除外される）
go test ./internal/cli/...
```

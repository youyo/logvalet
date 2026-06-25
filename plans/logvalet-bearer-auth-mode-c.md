# logvalet Bearer認証 (Mode C) 実装計画

## 背景・目的

Claude Tag（2026-06-23リリース）は Slack 上で動作するAIエージェント。
Claude Tag から logvalet-mcp（Lambda Function URL）を Remote MCP として接続するために、
Claude Tag が対応する認証方式（Bearer）に合わせた Mode C を追加する。

現在の認証モード:
| Mode | MCP認証 | Backlog認証 |
|------|---------|------------|
| A | なし（開発/信頼環境） | API Key |
| B (--auth) | OIDC (idproxy) | API Key |
| B + OAuth | OIDC (idproxy) | Backlog OAuth |

追加するモード:
| Mode | MCP認証 | Backlog認証 | 用途 |
|------|---------|------------|------|
| **C (auth-mode=bearer)** | 静的Bearer Token | 専用Backlogユーザー API Key | Claude Tag専用 |

## セキュリティ要件

- Bearer トークンと Backlog API Key は **必ず別々の値** を使う（漏洩リスクの分離）
- タイミング攻撃対策: `sha256.Sum256` + `crypto/subtle.ConstantTimeCompare` で長さリークを排除
- 最小トークン長: 32文字以上（推奨: `openssl rand -hex 32` 生成）
- HTTPS はLambda Function URLが担保（TLS終端）
- `/healthz` エンドポイントはBearer認証不要（AWS内部ヘルスチェック）
- **fail-closed 設計**: `LOGVALET_MCP_AUTH_MODE=bearer` かつ `LOGVALET_MCP_BEARER_TOKEN` 空 → 起動時にエラー終了
- Mode B（OIDC）と Mode C（Bearer）は排他。`auth-mode=bearer` のまま `--auth` 設定は `Validate()` でエラー
- **専用BacklogユーザーはMinimal privilege**: Claude Tag が必要とする操作（課題参照・コメント等）に権限を絞る。静的トークン漏洩時の被害最小化が最後の防波堤

## アーキテクチャ: 別スタック必須

既存 `function.json` には `LOGVALET_MCP_AUTH`（OIDC設定済み）が含まれる。
`Validate()` の排他制約上、**同一スタックに OIDC と Bearer の設定は共存できない**。

→ **Claude Tag専用 Lambda 関数を別スタックとして立てる（本スコープ内）**

| | 既存スタック | Claude Tag専用スタック（新規） |
|--|------------|---------------------------|
| Lambda関数名 | `logvalet-mcp` | `logvalet-mcp-tag` |
| SSMプレフィックス | `/logvalet-mcp/` | `/logvalet-mcp-tag/` |
| `LOGVALET_MCP_AUTH_MODE` | `oidc` | `bearer` |
| `LOGVALET_MCP_BEARER_TOKEN` | 未設定 | 専用シークレット |
| `LOGVALET_API_KEY` | 既存ユーザー | 専用Backlogユーザー |

## 実装スコープ

### リポジトリ1: `github.com/youyo/logvalet`

#### Step 1: テスト先行 (Red)

**新規ファイル** `internal/cli/mcp_bearer_test.go`

```
テストケース:
- TestBearerAuthMiddleware_ValidToken: 正しいトークンで200
- TestBearerAuthMiddleware_MissingHeader: Authorizationヘッダーなしで401
- TestBearerAuthMiddleware_WrongScheme: Basicスキームで401
- TestBearerAuthMiddleware_WrongToken: 誤ったトークンで401
- TestBearerAuthMiddleware_HealthzBypasses: /healthzはトークンなしで200（別パスで登録されるため対象外）
- TestMcpCmdValidate_BearerModeRequiresToken: auth-mode=bearer かつ token 空でエラー
- TestMcpCmdValidate_BearerAndOIDCMutuallyExclusive: auth-mode=bearer かつ --auth=true でエラー
- TestMcpCmdValidate_BearerTokenTooShort: 32文字未満でエラー
- TestMcpCmdValidate_BearerTokenValid: 32文字以上でパス
```

#### Step 2: 実装 (Green)

**新規ファイル** `internal/cli/mcp_bearer.go`

```go
// bearerAuthMiddleware は静的Bearerトークンで認証するHTTPミドルウェア。
// sha256ハッシュ化 + subtle.ConstantTimeCompare で長さ・内容ともにタイミング安全な比較を行う。
// RFC 7235 に従いスキーム名は case-insensitive で比較する（"bearer" も "Bearer" も受け付ける）。
func bearerAuthMiddleware(token string) func(http.Handler) http.Handler {
    hashed := sha256.Sum256([]byte(token))
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            auth := r.Header.Get("Authorization")
            // スキーム名は case-insensitive (RFC 7235)
            lower := strings.ToLower(auth)
            const prefix = "bearer "
            if !strings.HasPrefix(lower, prefix) {
                http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
                return
            }
            provided := sha256.Sum256([]byte(auth[len(prefix):]))
            if subtle.ConstantTimeCompare(provided[:], hashed[:]) != 1 {
                http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

**変更ファイル** `internal/cli/mcp.go`

```
McpCmd に追加するフィールド:
  AuthMode    string `name:"auth-mode" help:"auth mode: oidc|bearer|none (overrides --auth flag)" group:"auth" env:"LOGVALET_MCP_AUTH_MODE"`
  BearerToken string `name:"bearer-token" help:"static bearer token for mode=bearer (min 32 chars)" group:"auth" env:"LOGVALET_MCP_BEARER_TOKEN"`

Validate() 変更:
  - auth-mode の正規化: ""→"none", "oidc"は既存--authと同等, "bearer"→Mode C
  - auth-mode=bearer かつ Auth=true → エラー「--auth-mode=bearer and --auth are mutually exclusive」
  - auth-mode=bearer かつ BearerToken=="" → エラー「--bearer-token is required when --auth-mode=bearer」（fail-closed）
  - auth-mode=bearer かつ len(BearerToken)<32 → エラー「--bearer-token: must be at least 32 characters」

後方互換 / resolvedAuthMode() の真理値表（全組み合わせをテストで担保する）:
  | Auth(--auth) | AuthMode | 期待resolved |
  |---|---|---|
  | true  | ""       | oidc  ← 既存本番スタックの生命線。ここが壊れると無認証化 |
  | false | ""       | none  |
  | any   | "oidc"   | oidc  |
  | false | "bearer" | bearer |
  | true  | "bearer" | Validate error（排他） |
  | true  | "none"   | Validate error（--auth と "none" は矛盾, 明示エラー） |
```

**Run()の最終的な分岐構造**:
```go
authMode := c.resolvedAuthMode() // "oidc" | "bearer" | "none"

if authMode == "oidc" {
    // 既存: OIDC (idproxy) モード（変更なし）
    ...
} else if authMode == "bearer" {
    // 新規: Mode C (Bearer認証) — fail-closed（Validate()で保証済み）
    topMux := http.NewServeMux()
    topMux.HandleFunc("/healthz", healthHandler)  // 認証不要
    topMux.Handle("/", bearerAuthMiddleware(c.BearerToken)(innerMux))
    handler = topMux
    fmt.Fprintf(os.Stderr, "logvalet MCP server (bearer auth) listening on %s/mcp\n", addr)
} else {
    // 既存: 認証なし
    innerMux.HandleFunc("/healthz", healthHandler)
    handler = innerMux
    fmt.Fprintf(os.Stderr, "logvalet MCP server listening on %s/mcp\n", addr)
}
```

#### Step 3: Refactor

- `resolvedAuthMode()` を `McpCmd` のメソッドとして抽出（`--auth` と `--auth-mode` の統合ロジック）
- 401 レスポンスの Content-Type: `application/json` を明示

### リポジトリ2: `github.com/heptagon-inc/logvalet-mcp`

#### 新規ファイル: `function-tag.json`

既存 `function.json` をベースに Claude Tag専用スタックを定義:
```json
{
  "FunctionName": "logvalet-mcp-tag",
  "Description": "logvalet MCP server for Claude Tag (Bearer auth)",
  ...
  "Environment": {
    "Variables": {
      "LOGVALET_BASE_URL":             "{{ ssm `/logvalet-mcp-tag/LOGVALET_BASE_URL` }}",
      "LOGVALET_SPACE":                "{{ ssm `/logvalet-mcp-tag/LOGVALET_SPACE` }}",
      "LOGVALET_API_KEY":              "{{ ssm `/logvalet-mcp-tag/LOGVALET_API_KEY` }}",
      "LOGVALET_MCP_AUTH_MODE":        "bearer",
      "LOGVALET_MCP_BEARER_TOKEN":     "{{ ssm `/logvalet-mcp-tag/LOGVALET_MCP_BEARER_TOKEN` }}",
      "AWS_LWA_INVOKE_MODE":           "response_stream"
    }
  }
}
```

※ OIDC関連・DynamoDB関連は全て省略（Claude Tag専用スタックには不要）
※ `LOGVALET_SPACE_STORE_TYPE` は省略 → `buildSpaceStore()` がエラー → Warn ログのみで SpaceStore disabled になる経路に依存（space 管理ツールは Claude Tag 専用スタックでは不要なため許容）
※ `LOGVALET_SPACE_STORE_TYPE=none` は非正規値（memory/sqlite/dynamodbのみ有効）なので使用しない

#### 変更ファイル: 既存 `function.json`

後方互換 fail-open を排除するため `LOGVALET_MCP_AUTH_MODE: "oidc"` を明示追加:
```json
"LOGVALET_MCP_AUTH_MODE": "oidc"
```
これにより既存 OIDC スタックが `resolvedAuthMode()` の "" 推論パスに依存しなくなる。

**デプロイ順序**: 既存スタックに新バイナリをデプロイする際、`AUTH_MODE:"oidc"` が同時に入らなくても後方互換（`--auth=true + auth-mode=""→oidc`）のためサービスは継続する。よって deploy 順序は安全。ただし明示化を優先するため同一プッシュで入れることを推奨。

#### 変更ファイル: `mise.toml`

```toml
[tasks."deploy-tag"]
description = "Deploy Claude Tag dedicated MCP (Bearer auth) to Lambda"
depends = ["package"]
run = "lambroll deploy --function function-tag.json --function-url function_url.json"
```

※ `function_url.json` は既存を共用（AuthType: NONE, InvokeMode: RESPONSE_STREAM は同じ）

#### 変更ファイル: `README.md`

Mode C セクションを追加:
- 概要表に Mode C 行を追加
- 「5. Mode C: Bearer Token（Claude Tag専用）」セクション
  - SSMパラメータ登録コマンド（`/logvalet-mcp-tag/` プレフィックス）
  - Claude Tag への設定方法（認証タイプ: Bearer, URLとトークン）
  - **セキュリティ注意事項**: BearerトークンとAPI Keyを共通にしないこと
- 環境変数リファレンス表に `LOGVALET_MCP_AUTH_MODE` / `LOGVALET_MCP_BEARER_TOKEN` を追記

## デプロイフロー

1. logvalet本体にBearer認証を実装・テスト → `go test ./...` パス確認
2. logvalet 新バージョンをリリース（タグ打ち）
3. logvalet-mcp の `mise.toml` の `LOGVALET_VERSION` を更新
4. SSMパラメータを `/logvalet-mcp-tag/` プレフィックスで登録:
   ```bash
   aws ssm put-parameter --name /logvalet-mcp-tag/LOGVALET_MCP_BEARER_TOKEN \
     --value "$(openssl rand -hex 32)" --type SecureString
   aws ssm put-parameter --name /logvalet-mcp-tag/LOGVALET_API_KEY \
     --value "<専用BacklogユーザーのAPIキー>" --type SecureString
   aws ssm put-parameter --name /logvalet-mcp-tag/LOGVALET_BASE_URL \
     --value "<スペースURL>" --type String
   aws ssm put-parameter --name /logvalet-mcp-tag/LOGVALET_SPACE \
     --value "<スペース名>" --type String
   ```
5. `mise run deploy-tag` で Claude Tag専用Lambdaをデプロイ
6. Lambda Function URLを取得して Claude Tag に登録（Bearer認証タイプ）

## 完了条件

- [ ] `go test ./internal/cli/...` が全パス（`resolvedAuthMode()` 真理値表の全6パターンを含む）
- [ ] `go vet ./...` が警告なし
- [ ] `gofmt -l` が差分なし
- [ ] `auth-mode=bearer` + `BearerToken` 設定時に Bearer認証が有効になること
- [ ] `auth-mode=bearer` + `BearerToken` 空 → 起動エラー（fail-closed確認）
- [ ] `auth-mode=bearer` + `--auth=true` → バリデーションエラー
- [ ] `--auth=true` + `auth-mode=""` → OIDC（既存動作の回帰確認）
- [ ] `--auth=true` + `auth-mode="none"` → バリデーションエラー
- [ ] `/healthz` が Bearer認証なしで200を返すこと
- [ ] 既存の OIDC モード（`logvalet-mcp`）が壊れていないこと（回帰確認）
- [ ] `mise run deploy-tag` で `logvalet-mcp-tag` がデプロイできること
- [ ] Claude Tag から Bearer認証で `/mcp` エンドポイントに接続できること

## 未スコープ（今回対象外）

- Claude Tag からの初回接続時に OAuth discovery が発生しないかの事前確認
  → 初回疎通時にログを見て `WWW-Authenticate` 要求がないことを確認すれば十分
- レートリミット / リクエストログ / トークンローテーション
- Bearer認証での Function URL を IAM SigV4 + AWS_SigV4 認証に変更することの検討
  → ユーザーが明示的に Bearer（最簡・Claude Tag専用）を選択済み。設計判断は確定

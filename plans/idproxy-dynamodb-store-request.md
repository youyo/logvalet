# idproxy への修正依頼: DynamoDB Store 実装 (v0.2.0)

## 依頼元・背景

- 依頼元: `github.com/youyo/logvalet` (本リポジトリ)
- 背景: logvalet MCP サーバー (`logvalet mcp`) を AWS Lambda Function URL にマルチインスタンス構成でデプロイするにあたり、`idproxy.Store` インターフェース実装が **現状 `MemoryStore` のみ** のため、コンテナ間で以下の状態が共有されない:
  - DCR (Dynamic Client Registration) クライアント
  - OAuth 認可コード
  - セッション
- 結果として Claude.ai コネクタから並行リクエストすると `redirect_uri is not allowed` / `invalid_client` / 401 が発生する。
- 回避策として `reserved-concurrent-executions=1` で 1 コンテナ固定運用中。根本対応として DynamoDB Store を導入したい。

logvalet 側の改修計画は `plans/serene-whistling-wirth.md` にまとめてあり、そちらは idproxy v0.2.0 リリースを前提としている。

## 依頼内容

### 1. DynamoDB Store 実装を追加

**新規ファイル:** `store/dynamodb.go`

- `idproxy.Store` インターフェース (`store.go` で定義) を満たす `DynamoDBStore` 型を実装する。
- 既存 `MemoryStore` と同じ API シグネチャを DynamoDB 上の操作で置き換える:
  - `GetClient` / `PutClient` (DCR クライアント)
  - 認可コード保存・取得・削除
  - セッション保存・取得・削除
  - その他 `Store` interface の全メソッド
- テーブル構造 (提案):
  - パーティションキー: `pk` (String) — `client:<client_id>`, `code:<code>`, `session:<sid>` の形式
  - TTL 属性: `ttl` (Number) — 認可コード・セッションは短期失効
  - 値: `data` (Bytes or Map) — JSON シリアライズ
- コンストラクタ:
  ```go
  func NewDynamoDBStore(tableName, region string) (*DynamoDBStore, error)
  ```
- テスト容易性のため、mock client 注入版も用意してほしい:
  ```go
  func NewDynamoDBStoreWithClient(client DynamoDBClient, tableName string) *DynamoDBStore
  ```
  (logvalet 側 `internal/auth/tokenstore/dynamodb.go` と同一パターン)

### 2. ユニットテスト追加

**新規ファイル:** `store/dynamodb_test.go`

- 既存 `store/memory_test.go` と同じシナリオを DynamoDB 版で実装。
- DynamoDB Local または mock client で動作すること。
- カバレッジ対象: CRUD 全メソッド、TTL による自動失効、並行アクセス。

### 3. リリース

- SemVer: v0.2.0 (API 追加のため minor bump)。
- GitHub Release + タグ `v0.2.0` を打つ。
- CHANGELOG に「DynamoDB Store 実装追加」を記載。

## logvalet 側の利用イメージ

```go
// logvalet: internal/cli/mcp_auth.go
import "github.com/youyo/idproxy/store"

switch strings.ToLower(c.IDProxyStore) {
case "", "memory":
    idpStore = store.NewMemoryStore()
case "dynamodb":
    s, err := store.NewDynamoDBStore(c.IDProxyStoreDynamoDBTable, c.IDProxyStoreDynamoDBRegion)
    if err != nil {
        return idproxy.Config{}, err
    }
    idpStore = s
}
```

## 非目標 (Out of Scope)

- Redis / Postgres Store 対応 (将来の別タスク)
- signing key ローテーション機構 (logvalet 側で PEM を環境変数から注入する設計で対応するため idproxy 側の変更不要)
- 既存 MemoryStore の後方非互換な変更 (追加のみ)

## 参考リンク

- logvalet-mcp 側 Plan: `github.com/heptagon-inc/logvalet-mcp` の `plans/logvalet-multi-instance-support.md` に詳細な運用背景と検証手順あり。
- 現状の問題を再現した CloudWatch Logs: logvalet-mcp 運用者に確認可能。

## 完了条件

- [ ] `store/dynamodb.go` 実装
- [ ] `store/dynamodb_test.go` テスト全緑
- [ ] `go test ./...` が既存含め全緑
- [ ] v0.2.0 タグ付け + GitHub Release 公開
- [ ] README に DynamoDB Store 使用例を追記

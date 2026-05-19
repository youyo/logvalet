# MS06: DynamoDB SpaceStore + NonceStore DynamoDB

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS01, MS06a

## 目的

remote MCP 向け DynamoDB 実装を提供する。
NonceStore の DynamoDB 実装（C3: OAuth state replay 防止）も同一ファイルで実装。
NonceStore interface は MS01/MS06a で定義済みとする。

## 完了条件

- [ ] `internal/space/dynamodbstore.go` — DynamoDBStore（Store + NonceStore 実装）
- [ ] `internal/space/dynamodbstore_test.go` — localstack を使った全テスト pass
- [ ] Nonce の二重消費が ErrNonceAlreadyUsed を返す
- [ ] `docs/development.md` に localstack セットアップ手順追記（RL2 対応）
- [ ] `go test ./internal/space/...` パス（localstack 環境で）

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

### dynamodbstore_test.go

```
// build tag で localstack テストを切り替える
//go:build integration

T1-T12: MemoryStore と同一テストを DynamoDBStore で実行（Store interface 共通テスト）
T13-T15: NonceStore テスト（MemoryStore と同一）

T16: TestDynamoDBStore_Nonce_ConsumeWithConditionExpression
    - Store nonce → DynamoDB に PK=USER#u1, SK=NONCE#nonce1 が保存される
    - 1回目 Consume → 成功（ConditionExpression: attribute_exists(pk) で Delete）
    - 2回目 Consume → ErrNonceAlreadyUsed（Delete 失敗 = 既に削除済み）

T17: TestDynamoDBStore_Nonce_TTL
    - Store nonce with TTL → DynamoDB の TTL 属性が設定される
    （実際の TTL 期限切れは localstack では確認困難のため属性確認のみ）

T18: TestDynamoDBStore_TenantDuplicateCheck
    - Upsert({u1/foo, tenant:"myorg"}), Upsert({u1/bar, tenant:"myorg"})
    - GSI query で同一 userID + tenant が2件ある状態を確認
    （重複検出は application layer の責務: Upsert 前に caller が確認する）

T19: TestDynamoDBStore_Close_Idempotent
    - Close() 2回 → どちらもエラーなし
```

---

## 2. ファイル一覧

### 新規作成

| ファイル | 内容 |
|---------|------|
| `internal/space/dynamodbstore.go` | DynamoDBStore（Store + NonceStore 実装） |
| `internal/space/dynamodbstore_test.go` | T1-T19（build tag: integration） |

### 更新

| ファイル | 内容 |
|---------|------|
| `docs/development.md` | localstack セットアップ手順追加 |

---

## 3. DynamoDB キー設計

```text
テーブル名: LOGVALET_SPACE_DYNAMODB_TABLE（デフォルト: logvalet-spaces）
（既存 TokenStore テーブル: LOGVALET_AUTH_DYNAMODB_TABLE は別テーブル）

PK         SK                    用途
USER#<uid> SPACE#<alias>         SpaceRegistration
USER#<uid> PREF                  UserPreference
USER#<uid> NONCE#<nonce>         NonceRecord（TTL 属性付き）

GSI: gsi-user-tenant
  PK: user_id（文字列）
  SK: tenant（文字列）
  用途: 同一 user/tenant の重複チェック（eventually consistent, Software Check）
```

---

## 4. 実装

### DynamoDBStore のシグネチャ

```go
package space

import (
    "context"
    "sync"
    "time"

    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type DynamoDBSpaceAPI interface {
    GetItem(ctx context.Context, params *dynamodb.GetItemInput, ...) (*dynamodb.GetItemOutput, error)
    PutItem(ctx context.Context, params *dynamodb.PutItemInput, ...) (*dynamodb.PutItemOutput, error)
    DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, ...) (*dynamodb.DeleteItemOutput, error)
    Query(ctx context.Context, params *dynamodb.QueryInput, ...) (*dynamodb.QueryOutput, error)
    TransactWriteItems(ctx context.Context, params *dynamodb.TransactWriteItemsInput, ...) (*dynamodb.TransactWriteItemsOutput, error)
}

type DynamoDBStore struct {
    client    DynamoDBSpaceAPI
    tableName string
    mu        sync.RWMutex
    closed    bool
}

func NewDynamoDBStore(tableName, region string) (*DynamoDBStore, error)
func NewDynamoDBStoreWithClient(tableName string, client DynamoDBSpaceAPI) *DynamoDBStore

// Store interface
func (s *DynamoDBStore) List(ctx context.Context, userID string) ([]SpaceRegistration, error)
func (s *DynamoDBStore) Get(ctx context.Context, userID, alias string) (*SpaceRegistration, error)
func (s *DynamoDBStore) Upsert(ctx context.Context, reg *SpaceRegistration) error
func (s *DynamoDBStore) Delete(ctx context.Context, userID, alias string) error
func (s *DynamoDBStore) GetPreference(ctx context.Context, userID string) (*UserPreference, error)
func (s *DynamoDBStore) PutPreference(ctx context.Context, pref *UserPreference) error
func (s *DynamoDBStore) Close() error

// NonceStore 実装（C3: consume-once で replay attack 防止）
func (s *DynamoDBStore) Store(ctx context.Context, userID, nonce string, ttl time.Duration) error
func (s *DynamoDBStore) Consume(ctx context.Context, userID, nonce string) error
```

---

## 5. NonceStore DynamoDB の実装詳細（C3 対応）

### Nonce.Store

```go
// PK=USER#<uid>, SK=NONCE#<nonce>, TTL 属性付き
func (s *DynamoDBStore) Store(ctx context.Context, userID, nonce string, ttl time.Duration) error {
    pk := "USER#" + userID
    sk := "NONCE#" + nonce
    expiresAt := time.Now().Add(ttl).Unix() // TTL は Unix 秒
    _, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
        TableName: &s.tableName,
        Item: map[string]types.AttributeValue{
            "pk":         &types.AttributeValueMemberS{Value: pk},
            "sk":         &types.AttributeValueMemberS{Value: sk},
            "expires_at": &types.AttributeValueMemberN{Value: strconv.FormatInt(expiresAt, 10)},
        },
    })
    return err
}
```

### Nonce.Consume（ConditionExpression で consume-once）

```go
// ConditionExpression で "存在する場合のみ Delete" → 失敗 = 使用済み or 存在しない
func (s *DynamoDBStore) Consume(ctx context.Context, userID, nonce string) error {
    pk := "USER#" + userID
    sk := "NONCE#" + nonce
    _, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
        TableName:           &s.tableName,
        Key: map[string]types.AttributeValue{
            "pk": &types.AttributeValueMemberS{Value: pk},
            "sk": &types.AttributeValueMemberS{Value: sk},
        },
        ConditionExpression: aws.String("attribute_exists(pk)"),
    })
    if err != nil {
        var ccf *types.ConditionalCheckFailedException
        if errors.As(err, &ccf) {
            return ErrNonceAlreadyUsed
        }
        return err
    }
    return nil
}
```

### Delete + PREF 更新（TransactWriteItems）

```go
// lv spaces remove での SPACE + PREF 同時更新
func (s *DynamoDBStore) DeleteWithPreference(ctx context.Context, userID, alias, newDefaultAlias string) error {
    // TransactWriteItems: Delete SPACE#alias + Update PREF
    // ...
}
```

---

## 6. localstack セットアップ（RL2 対応）

`docs/development.md` に以下を追記:

```markdown
## DynamoDB テスト（localstack）

### 前提
- Docker がインストール済みであること

### 起動
```bash
docker run -d -p 4566:4566 localstack/localstack
```

### テーブル作成
```bash
aws --endpoint-url=http://localhost:4566 dynamodb create-table \
  --table-name logvalet-spaces \
  --attribute-definitions \
    AttributeName=pk,AttributeType=S \
    AttributeName=sk,AttributeType=S \
  --key-schema \
    AttributeName=pk,KeyType=HASH \
    AttributeName=sk,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST
```

### テスト実行
```bash
DYNAMODB_ENDPOINT=http://localhost:4566 go test -tags integration ./internal/space/...
```
```

---

## 7. 実装手順（TDD サイクル）

### Step 1: Red

1. `internal/space/dynamodbstore_test.go` を作成（T1-T19、build tag: integration）
2. localstack を起動してテーブル作成
3. `go test -tags integration ./internal/space/...` → コンパイルエラー

### Step 2: Green

1. `internal/space/dynamodbstore.go` を実装
2. `go test -tags integration ./internal/space/...` → 全テストパス

### Step 3: Refactor

- 既存 `internal/auth/tokenstore/dynamodb.go` のパターンを踏襲
- エラーハンドリングの統一

---

## 8. 検証コマンド

```bash
# localstack 起動後
DYNAMODB_ENDPOINT=http://localhost:4566 go test -tags integration ./internal/space/... -v
go build ./...
go vet ./...
```

---

## 9. 次のマイルストーン

MS05 + MS06 完了後 → MS07（SpaceStore 設定 validation）が着手可能。
MS06 + MS06a + MS08 完了後 → MS10（MultiSpaceOAuthHandler）が着手可能。

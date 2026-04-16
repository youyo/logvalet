# M09: DynamoDB TokenStore 詳細計画

## 概要

DynamoDBStore が auth.TokenStore interface を実装する。
Lambda 本命の永続ストアとして、VPC 不要でマルチインスタンス耐性を持つ。

## 設計決定

| # | 決定 | 理由 |
|---|------|------|
| 1 | 単一 PK（`USER#userID#provider#tenant`）、SK なし | ポイントルックアップのみ。リスト操作は不要 |
| 2 | DynamoDBAPI interface でモック注入可能 | ユニットテストに DynamoDB Local 不要 |
| 3 | NewDynamoDBStore + NewDynamoDBStoreWithClient の2コンストラクタ | 本番は前者、テストは後者 |
| 4 | Close 後の操作はエラー返却 | SQLiteStore と同じ契約 |
| 5 | Close() 自体は no-op（DynamoDB は接続管理不要） | ただし closed フラグで状態追跡 |

## テーブル設計

```
テーブル名: 環境変数 LOGVALET_TOKEN_STORE_DYNAMODB_TABLE
PK: pk (String) = "USER#<userID>#<provider>#<tenant>"
SK: なし

属性:
- access_token (S)
- refresh_token (S)
- token_type (S)
- scope (S)
- expiry (S) — RFC3339 UTC
- provider_user_id (S)
- created_at (S) — RFC3339 UTC
- updated_at (S) — RFC3339 UTC
```

## 対象ファイル

| ファイル | 操作 |
|---------|------|
| `internal/auth/tokenstore/dynamodb.go` | 新規 |
| `internal/auth/tokenstore/dynamodb_test.go` | 新規 |
| `internal/auth/tokenstore/factory.go` | 修正 |
| `internal/auth/tokenstore/factory_test.go` | 修正 |

## 追加依存

- `github.com/aws/aws-sdk-go-v2`
- `github.com/aws/aws-sdk-go-v2/config`
- `github.com/aws/aws-sdk-go-v2/service/dynamodb`
- `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue`

## TDD テストケース

### Red（失敗テスト先行）

1. **Put → Get ラウンドトリップ**: Put した TokenRecord が Get で全フィールド一致で返る
2. **Get 未存在**: 存在しないキーで nil, nil が返る（エラーではない）
3. **Put 上書き**: 同一キーに2回 Put → 最新が返る
4. **Delete → Get**: Delete 後の Get が nil, nil
5. **Delete 未存在**: 存在しないキーの Delete はエラーにならない
6. **Close() 冪等**: 複数回呼んでもエラーなし
7. **Close 後の操作**: Get/Put/Delete がエラー返却
8. **ユーザー隔離**: Put(userA) → Get(userB) が nil, nil
9. **ファクトリー統合**: NewTokenStore(dynamodb, cfg) が DynamoDBStore を返す

### Green（最小実装）

- DynamoDBAPI interface（GetItem, PutItem, DeleteItem）
- mockDynamoDBClient（テスト用モック）
- DynamoDBStore struct + CRUD メソッド
- ファクトリー更新

### Refactor

- エラーメッセージ統一
- PK 生成を共通ヘルパーに

## 実装ステップ

1. `go get` で aws-sdk-go-v2 依存追加
2. dynamodb_test.go にテストケース記述（Red）
3. dynamodb.go に DynamoDBAPI interface + DynamoDBStore 実装（Green）
4. factory.go の StoreTypeDynamoDB → NewDynamoDBStore に差し替え
5. factory_test.go を更新
6. `go test ./internal/auth/tokenstore/...` 全パス確認
7. `go vet ./...` パス確認

## リスク評価

| リスク | 影響 | 対策 |
|-------|------|------|
| aws-sdk-go-v2 の依存サイズ | バイナリサイズ増加 | DynamoDB 関連のみ import |
| DynamoDB Local なしの CI | 統合テスト不可 | mock ベースユニットテストで常時カバー |
| PK 衝突 | データ上書き | userID+provider+tenant の3要素で一意性保証 |

## 完了基準

- [x] DynamoDBStore が auth.TokenStore interface を実装
- [x] mock ベースユニットテスト全パス
- [x] ファクトリーから DynamoDBStore 生成可能
- [x] `go test ./...` 全パス
- [x] `go vet ./...` パス

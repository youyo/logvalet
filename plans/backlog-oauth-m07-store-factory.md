# M07: TokenStore ファクトリー

## 概要

`NewTokenStore(cfg *auth.OAuthEnvConfig) (auth.TokenStore, error)` を提供する。
`cfg.TokenStoreType` に基づいて適切な TokenStore 実装を返す。

## 対象ファイル

- `internal/auth/errors.go` — ErrNotImplemented 追加
- `internal/auth/tokenstore/factory.go` — ファクトリー関数
- `internal/auth/tokenstore/factory_test.go` — テスト

## TDD テストケース

| # | 入力 | 期待結果 |
|---|------|---------|
| 1 | `StoreTypeMemory` | MemoryStore を返す（Put/Get ラウンドトリップ成功） |
| 2 | `StoreTypeSQLite` | `ErrNotImplemented` を返す（M08 で解除） |
| 3 | `StoreTypeDynamoDB` | `ErrNotImplemented` を返す（M09 で解除） |
| 4 | 不正値（"unknown"） | `ErrInvalidStoreType` を返す |
| 5 | 空文字列（デフォルト） | MemoryStore を返す |

## 設計

- `cfg *auth.OAuthEnvConfig` を受け取る（M08/M09 で SQLitePath, DynamoDB 設定が必要になるため）
- `ErrNotImplemented` は `auth.errors.go` に追加（他センチネルエラーと同じ場所）
- `ErrInvalidStoreType` は `auth.config.go` に既存のものを再利用

## リスク

- なし。30 行程度のシンプルな switch 文。

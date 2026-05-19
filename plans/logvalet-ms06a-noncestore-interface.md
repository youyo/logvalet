# MS06a: NonceStore interface 定義（確認マイルストーン）

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS01

## 目的

MS01 で定義した `NonceStore` interface が `internal/space/store.go` に存在し、
`go build` が通ることを確認する軽量マイルストーン。

MS10（MultiSpaceOAuthHandler）が NonceStore interface に依存するため、
MS06（DynamoDB 実装）の完了を待たずに MS10 の実装を開始できるようにする（RC2 対応）。

## 完了条件

- [ ] `internal/space.NonceStore` interface が利用可能（MS01 で定義済みを確認）
- [ ] `internal/space.ErrNonceAlreadyUsed` が利用可能
- [ ] `go build ./...` パス

---

## 作業内容

MS01 が完了していれば本 MS の実装作業はほぼ不要。
以下を確認して完了とする:

```bash
# NonceStore interface が定義されていることを確認
grep -n "NonceStore" internal/space/store.go

# ErrNonceAlreadyUsed が定義されていることを確認
grep -n "ErrNonceAlreadyUsed" internal/space/errors.go

# ビルドが通ることを確認
go build ./...
```

---

## 次のマイルストーン

MS06a 完了（= MS01 確認）後、以下が着手可能:
- MS06（DynamoDB SpaceStore + NonceStore DynamoDB 実装）
- MS10（StateClaims 拡張 + MultiSpaceOAuthHandler: MS06a + MS08 が揃えば着手可能）

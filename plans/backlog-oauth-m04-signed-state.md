# M04: Signed State (JWT ベース) — 詳細計画

## 概要

OAuth 2.0 フローの CSRF 対策として、HMAC-SHA256 で署名された JWT を state パラメータとして使用する。
外部ストア不要で Lambda との相性が良い signed state payload 方式を採用する。

## スペック参照

- `docs/specs/logvalet_backlog_oauth_coding_agent_prompt.md` §2 state 管理
- `plans/backlog-oauth-roadmap.md` M04 セクション

## 前提条件

| 項目 | 状態 |
|------|------|
| M01: TokenRecord 型・エラー定義 | 完了 |
| M03: OAuthEnvConfig (OAuthStateSecret) | 完了 |
| golang-jwt/jwt/v5 v5.3.1 | go.mod に存在（indirect） |

## 成果物

| ファイル | 内容 |
|---------|------|
| `internal/auth/state.go` | StateClaims, GenerateState(), ValidateState() |
| `internal/auth/state_test.go` | TDD テストケース |
| `internal/auth/errors.go` | ErrStateExpired, ErrStateInvalid 追加 |

## 設計

### StateClaims struct

```go
type StateClaims struct {
    UserID string `json:"uid"`
    Tenant string `json:"tenant"`
    Nonce  string `json:"nonce"`
    jwt.RegisteredClaims
}
```

- `UserID`: idproxy で確定したユーザー識別子。callback 時にユーザーコンテキストと照合する
- `Tenant`: Backlog スペース識別子（例: `example.backlog.com`）。callback 時にどのスペースの認可かを特定する
- `Nonce`: crypto/rand で生成する 16 バイトのランダム値（hex エンコード）。リプレイ攻撃対策
- `jwt.RegisteredClaims`: ExpiresAt (exp) で TTL を制御

### GenerateState 関数

```go
func GenerateState(userID, tenant string, secret []byte, ttl time.Duration) (string, error)
```

処理フロー:
1. userID が空文字列なら ErrUnauthenticated を返す
2. tenant が空文字列なら ErrInvalidTenant を返す
3. secret が nil または空なら ErrStateInvalid を返す
4. ttl <= 0 なら ErrStateInvalid を返す
5. crypto/rand で 16 バイトの nonce を生成
6. StateClaims を構築（UserID, Tenant, Nonce, ExpiresAt = now + ttl, IssuedAt = now）
7. jwt.NewWithClaims(jwt.SigningMethodHS256, claims) で JWT 生成
8. secret で署名して JWT 文字列を返す

### ValidateState 関数

```go
func ValidateState(stateJWT string, secret []byte) (*StateClaims, error)
```

処理フロー:
1. jwt.ParseWithClaims で JWT をパース
2. 署名メソッドが HMAC であることを検証（アルゴリズム差し替え攻撃対策）
3. エラー判別: `errors.Is(err, jwt.ErrTokenExpired)` → `fmt.Errorf("%w: %v", ErrStateExpired, err)` を返す（元エラー詳細を保持）
4. その他の JWT パースエラー（署名不正・改竄含む） → `fmt.Errorf("%w: %v", ErrStateInvalid, err)` を返す
5. claims.UserID が空なら ErrStateInvalid を返す
6. 成功時 *StateClaims を返す

**備考**: callback 時の「state 内 UserID と現在セッション UserID の照合」は M14 (callback handler) の責務とする

### デフォルト TTL

```go
const DefaultStateTTL = 10 * time.Minute
```

### エラー追加 (errors.go)

```go
var (
    ErrStateExpired = errors.New("auth: state token expired")
    ErrStateInvalid = errors.New("auth: state token invalid")
)
```

## TDD 計画

### Phase 1: Red（失敗するテストを先に書く）

#### テストケース一覧

| # | テスト名 | 期待結果 |
|---|---------|---------|
| 1 | `TestGenerateState_ValidInput` | エラーなしで非空の JWT 文字列を返す |
| 2 | `TestGenerateState_EmptyUserID` | ErrUnauthenticated を返す |
| 3 | `TestGenerateState_EmptyTenant` | ErrInvalidTenant を返す |
| 4 | `TestGenerateState_NilSecret` | ErrStateInvalid を返す |
| 5 | `TestGenerateState_EmptySecret` | ErrStateInvalid を返す |
| 6 | `TestGenerateState_ZeroTTL` | TTL=0 で ErrStateInvalid を返す |
| 7 | `TestGenerateState_NegativeTTL` | TTL<0 で ErrStateInvalid を返す |
| 8 | `TestValidateState_RoundTrip` | GenerateState → ValidateState で claims (UserID, Tenant, Nonce) 復元 |
| 9 | `TestValidateState_WrongSecret` | 異なる secret で ErrStateInvalid (errors.Is) |
| 10 | `TestValidateState_ExpiredState` | 過去の ExpiresAt を持つ JWT を手動構築し ErrStateExpired (errors.Is) |
| 11 | `TestValidateState_TamperedPayload` | 改竄 JWT で ErrStateInvalid |
| 12 | `TestValidateState_EmptyString` | 空文字列で ErrStateInvalid |
| 13 | `TestGenerateState_NonceUniqueness` | 2回の呼び出しで異なる nonce |
| 14 | `TestValidateState_AlgorithmNone` | alg:none 攻撃で ErrStateInvalid |

#### ExpiredState テストの実装方法

`time.Sleep` は使用しない。直接過去の ExpiresAt を設定した JWT を手動構築する:
```go
claims := &StateClaims{
    UserID: "user1",
    Tenant: "example.backlog.com",
    Nonce:  "test-nonce",
    RegisteredClaims: jwt.RegisteredClaims{
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
        IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
    },
}
token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
stateJWT, _ := token.SignedString(secret)
// → ValidateState(stateJWT, secret) で ErrStateExpired
```

#### エラーアサーションの方針

テストでは `errors.Is(err, ErrStateExpired)` / `errors.Is(err, ErrStateInvalid)` を使用する。
エラーは `fmt.Errorf("%w: %v", sentinel, originalErr)` で wrap されるため、`errors.Is` で判定可能。

### Phase 2: Green（テストを通す最小限の実装）

1. `internal/auth/errors.go` に ErrStateExpired, ErrStateInvalid を追加
2. `internal/auth/state.go` に StateClaims, DefaultStateTTL, GenerateState, ValidateState を実装
3. `go test ./internal/auth/...` で全テスト green 確認

### Phase 3: Refactor（テストが通る状態でコード整理）

- nonce 生成をヘルパー関数に抽出（テスタビリティ向上）
- エラーメッセージの統一性確認
- godoc コメントの充実

## 実装ステップ

1. `go get github.com/golang-jwt/jwt/v5` で直接依存に昇格
2. `internal/auth/errors.go` に ErrStateExpired, ErrStateInvalid を追加
3. `internal/auth/state_test.go` を作成（全 14 テストケース）
4. `go test` で全テスト FAIL を確認（Red）
5. `internal/auth/state.go` を実装
6. `go test` で全テスト PASS を確認（Green）
7. リファクタリング実施
8. `go test` で全テスト PASS を再確認（Refactor）
9. `go vet ./internal/auth/...` で lint チェック
10. git commit

## リスク評価

| リスク | 影響度 | 対策 |
|--------|-------|------|
| JWT ライブラリの期限切れ判定精度 | 低 | テストで明示的に過去の ExpiresAt を設定して検証 |
| alg:none 攻撃 | 高 | ParseWithClaims で SigningMethodHMAC を明示チェック |
| nonce の衝突 | 極低 | 16 bytes の crypto/rand は実用上衝突しない |
| secret の鍵長不足 | 中 | M03 で最小 16 bytes を Validate() で強制済み |
| タイムゾーン問題 | 低 | jwt/v5 は UTC ベースで統一 |
| go.mod で indirect の golang-jwt | 低 | go.mod に直接 require を追加して明示化 |

## セキュリティ考慮

1. **署名アルゴリズム固定**: SigningMethodHS256 のみ許可。alg:none / RS256 差し替え攻撃を防止
2. **nonce によるリプレイ対策**: 毎回ユニークな nonce で同一 state の再利用を防止
3. **短い TTL**: 10分のデフォルト TTL で窃取リスクを最小化
4. **secret のバリデーション**: 空/nil の secret は即座にエラー
5. **claims の整合性チェック**: UserID 空チェックで不正な state を拒否

## 依存関係

- `github.com/golang-jwt/jwt/v5` — JWT 生成・検証
- `crypto/rand` — nonce 生成（標準ライブラリ）
- `encoding/hex` — nonce の hex エンコード（標準ライブラリ）

## コミット

```
feat(auth): Signed State (JWT HMAC-SHA256) を実装

Plan: plans/backlog-oauth-m04-signed-state.md
```

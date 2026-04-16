# M05: OAuthProvider interface と Backlog 実装

## 概要

OAuthProvider interface を定義し、BacklogOAuthProvider を実装する。
既存の `credentials.BuildAuthorizeURL` / `credentials.ExchangeCode` をラップしつつ、
`RefreshToken` / `GetCurrentUser` を新規実装する。

## 前提（前マイルストーンからのハンドオフ）

| マイルストーン | 提供物 |
|-------------|--------|
| M01 | `TokenRecord` struct, `ProviderUser` struct, センチネルエラー群 |
| M03 | `OAuthEnvConfig` に `BacklogClientID`, `BacklogClientSecret`, `BacklogRedirectURL` |
| M04 | `StateClaims` に `Tenant` フィールド追加済み |

## 対象ファイル

| ファイル | 内容 |
|---------|------|
| `internal/auth/provider/provider.go` | OAuthProvider interface 定義 |
| `internal/auth/provider/backlog.go` | BacklogOAuthProvider 実装 |
| `internal/auth/provider/backlog_test.go` | テスト（httptest ベース） |

## 設計

### OAuthProvider interface

```go
// OAuthProvider は外部 OAuth プロバイダーとのやりとりを抽象化する。
// Backlog, GitHub, Google 等の provider を統一的に扱うための interface。
type OAuthProvider interface {
    // Name はプロバイダー名を返す（例: "backlog"）。
    Name() string

    // BuildAuthorizationURL は OAuth 認可 URL を構築する。
    BuildAuthorizationURL(state, redirectURI string) (string, error)

    // ExchangeCode は認可コードをトークンに交換する。
    ExchangeCode(ctx context.Context, code, redirectURI string) (*auth.TokenRecord, error)

    // RefreshToken はリフレッシュトークンで新しいトークンを取得する。
    RefreshToken(ctx context.Context, refreshToken string) (*auth.TokenRecord, error)

    // GetCurrentUser はアクセストークンで現在のユーザー情報を取得する。
    GetCurrentUser(ctx context.Context, accessToken string) (*auth.ProviderUser, error)
}
```

### BacklogOAuthProvider struct

```go
type BacklogOAuthProvider struct {
    space        string       // Backlog スペース名（例: "example-space"）
    clientID     string
    clientSecret string
    baseURL      string       // デフォルト: https://{space}.backlog.com（テスト時に httptest URL へ差し替え）
    httpClient   *http.Client // RefreshToken / GetCurrentUser で使用。テスト時に差し替え可能
}
```

**注意**: `credentials.ExchangeCode` は内部で独自の `http.Client` を生成するため、
struct の `httpClient` は `RefreshToken` と `GetCurrentUser` のみで使用される。
これは `ExchangeCode` が tokenURL をパラメータとして受け取るため httptest で問題なく動作する。

### コンストラクタ

```go
func NewBacklogOAuthProvider(space, clientID, clientSecret string) (*BacklogOAuthProvider, error)
```

- `space` が空の場合は `auth.ErrInvalidTenant` を返す
- `baseURL` はデフォルトで `"https://" + space + ".backlog.com"`
- `httpClient` はデフォルトで `&http.Client{Timeout: 30 * time.Second}`

### テスト用ヘルパー

テスト時に `baseURL` を httptest サーバーの URL に差し替えるため、
テストファイル内で直接フィールドを設定する（unexported フィールドは同一パッケージ内からアクセス可能）。

```go
// backlog_test.go 内
p, _ := NewBacklogOAuthProvider("test-space", "client-id", "client-secret")
p.baseURL = ts.URL  // httptest サーバーの URL に差し替え
```

### 既存コード再利用

| メソッド | 再利用 | 詳細 |
|---------|--------|------|
| `BuildAuthorizationURL` | `credentials.BuildAuthorizeURL` をラップ | space, clientID は struct から取得 |
| `ExchangeCode` | `credentials.ExchangeCode` をラップ | `p.baseURL + "/api/v2/oauth2/token"` を tokenURL として渡す、TokenResponse → TokenRecord 変換 |
| `RefreshToken` | **新規実装** | `p.baseURL + "/api/v2/oauth2/token"` に grant_type=refresh_token で POST |
| `GetCurrentUser` | **新規実装** | `p.baseURL + "/api/v2/users/myself"` を GET で呼び出し |

### TokenResponse → TokenRecord 変換

```go
func (p *BacklogOAuthProvider) toTokenRecord(resp *credentials.TokenResponse) *auth.TokenRecord {
    now := time.Now()
    return &auth.TokenRecord{
        Provider:     "backlog",
        Tenant:       p.space,
        AccessToken:  resp.AccessToken,
        RefreshToken: resp.RefreshToken,
        TokenType:    resp.TokenType,
        Expiry:       now.Add(time.Duration(resp.ExpiresIn) * time.Second),
        CreatedAt:    now,
        UpdatedAt:    now,
    }
}
```

注: `UserID` と `ProviderUserID` は caller（TokenManager / Handler）側で設定する。

### RefreshToken 実装

- token endpoint: `https://{space}.backlog.com/api/v2/oauth2/token`
- パラメータ: `grant_type=refresh_token`, `client_id`, `client_secret`, `refresh_token`
- Content-Type: `application/x-www-form-urlencoded`
- レスポンスは `credentials.TokenResponse` と同じ JSON 形式

### GetCurrentUser 実装

- endpoint: `https://{space}.backlog.com/api/v2/users/myself`
- Authorization: `Bearer {accessToken}`
- レスポンス例:
  ```json
  {
    "id": 12345,
    "userId": "example-user",
    "name": "Example User",
    "mailAddress": "user@example.com"
  }
  ```
- マッピング: `id` → `ProviderUser.ID`（string 変換）, `name` → `Name`, `mailAddress` → `Email`

## TDD テストケース

### Red フェーズ（先に書く失敗テスト）

1. **TestBacklogProvider_Name**: `Name()` が `"backlog"` を返す
2. **TestBacklogProvider_NewWithEmptySpace**: space 空で `ErrInvalidTenant`
3. **TestBacklogProvider_BuildAuthorizationURL**: 正しい Backlog OAuth URL が返る
   - response_type=code, client_id, redirect_uri, state パラメータの存在確認
   - URL ホストが `{space}.backlog.com` であること
4. **TestBacklogProvider_ExchangeCode**: httptest サーバーで正しいパラメータ送信を検証
   - grant_type=authorization_code の送信確認
   - TokenRecord の各フィールドが正しく変換されること
   - Expiry が現在時刻 + ExpiresIn 秒の近辺であること
5. **TestBacklogProvider_ExchangeCode_Error**: token endpoint がエラーを返した場合
6. **TestBacklogProvider_RefreshToken**: grant_type=refresh_token で新しい TokenRecord 返却
   - refresh_token パラメータの送信確認
   - client_id, client_secret の送信確認
7. **TestBacklogProvider_RefreshToken_Error**: refresh 失敗時のエラー
8. **TestBacklogProvider_GetCurrentUser**: `/api/v2/users/myself` のレスポンスを ProviderUser にマッピング
   - Authorization: Bearer ヘッダの確認
   - ID (int→string), Name, Email の正しい変換
9. **TestBacklogProvider_GetCurrentUser_Error**: API エラー時の処理
10. **TestBacklogProvider_GetCurrentUser_Unauthorized**: 401 レスポンス時のエラー

### Green フェーズ

上記テストを通す最小限の実装。

### Refactor フェーズ

- tokenURL 構築の共通化
- エラーメッセージの統一
- httpClient の注入パターン確認

## 実装ステップ

1. `internal/auth/provider/` ディレクトリ作成
2. `provider.go`: OAuthProvider interface 定義
3. `backlog_test.go`: 全テストケースを Red で記述
4. `backlog.go`: テストを Green にする実装
5. Refactor: コード整理
6. `go test ./internal/auth/provider/...` で全テスト通過確認
7. `go vet ./internal/auth/provider/...` でlint通過確認

## リスク評価

| リスク | 影響 | 対策 |
|-------|------|------|
| credentials.ExchangeCode が http.Client をハードコード | struct の httpClient が ExchangeCode で使われない | ExchangeCode は tokenURL を引数で受け取るため baseURL 経由で httptest.Server の URL を渡せば問題なし。httpClient は RefreshToken/GetCurrentUser のみで使用（struct コメントで明記） |
| Backlog API のレスポンス形式変更 | GetCurrentUser のパース失敗 | JSON パースのテストで型の一致を検証 |
| RefreshToken のレスポンスが ExchangeCode と異なる可能性 | 変換ロジック不一致 | 同じ TokenResponse 型でデコード、httptest で形式を固定 |

## 完了条件

- [x] `go test ./internal/auth/provider/...` が全パス
- [x] `go vet ./internal/auth/provider/...` がエラーなし
- [x] OAuthProvider interface が将来の provider 追加に対応する汎用設計
- [x] 既存コード（credentials パッケージ）に変更なし
- [x] BacklogOAuthProvider が全5メソッドを実装

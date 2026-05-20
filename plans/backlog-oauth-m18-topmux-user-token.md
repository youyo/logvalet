# M18: Multi-Space OAuth authorize を topMux に直接登録し signed user_token で userID を運ぶ

- Status: Plan
- Owner: architect (Agent Teams logvalet-oauth-fix)
- Date: 2026-05-20
- Related: M14 (single-space callback), M16 (MCP integration), MS10 (multi-space OAuth handler), MS14 (MCP spaces tools)
- Branch: `fix-multi-space-oauth-topmux-user-token`

---

## 1. 目的・完了条件

### 目的
MCP の `logvalet_space_connect_url` で生成した URL からブラウザで OAuth フローを開始したとき、`idproxy` の `continue` リダイレクトを経由せずに `base_url` パラメータが二重エンコードされずに `MultiSpaceOAuthHandler.HandleAuthorize` まで到達できるようにする。

### 完了条件
1. `logvalet_space_connect_url` で取得した URL をブラウザで開いて OAuth 認可を完走できる
2. 完走後 `logvalet_space_list` で `megumilog.backlog.jp` が `status=ok` として表示される
3. 既存 single-space フロー（`/oauth/backlog/{authorize,callback,status,disconnect}` 経由）が引き続き動作する
4. 既存ユニットテスト・E2E テストが全て green を維持する
5. user_token と state JWT のトークン混同攻撃が成立しないこと（`typ` claim で検証）

### 根本原因（再確認）
- `internal/cli/mcp.go:254-324` で `innerMux` に MCP + OAuth ルートを登録した後、`bridge` → `authMW.Wrap` → `topMux` の順でラップしている
- そのため multi-space `/oauth/backlog/authorize` も idproxy 認証必須となり、未ログインのブラウザは idproxy の `/login` → 認証完了後に `continue=<元URL>` に戻される
- idproxy が `continue` を組み立てる際に `base_url=https%3A%2F%2Fmegumilog.backlog.jp` の `%2F` を `/` に置換するため、クエリ値が `https://` で打ち切られて `invalid URL escape "%3A"` になる

---

## 2. 現状アーキテクチャ図

```
ブラウザ
  │ GET https://example.com/oauth/backlog/authorize?base_url=https%3A%2F%2Fmegumilog.backlog.jp&alias=megumilog
  ▼
topMux ── "/" ──▶ BacklogAuthorizeGate(任意)
                    └─▶ authMW.Wrap (idproxy)
                         │ ▼ 未認証なら /login → 認証後に "continue=<元URL>" にリダイレクト
                         │   このとき URL を一度 url.Values 化するため %2F が / に decode される
                         │   (= 根本原因)
                         ▼
                       bridge (idproxy.User.Subject → auth.ContextWithUserID)
                         ▼
                       innerMux
                         ├─ /mcp ──────────────────────▶ MCP server
                         └─ /oauth/backlog/authorize ── MultiSpaceOAuthHandler.HandleAuthorize
                            └ auth.UserIDFromContext(ctx) で idproxy 注入 userID を取り出す
```

問題点:
- multi-space authorize が「idproxy 認証必須」と「クリーン URL での起動」という相反する制約を同時に持たされている

---

## 3. 修正後アーキテクチャ図 (Option A + 案B callback 単一化)

```
ブラウザ
  │ GET https://example.com/oauth/backlog/multi/authorize?
  │      base_url=https%3A%2F%2Fmegumilog.backlog.jp&alias=megumilog&bootstrap_token=<JWT>
  ▼
topMux
  ├─ "GET /oauth/backlog/multi/authorize" ──▶ MultiSpaceOAuthHandler.HandleAuthorize  (★ idproxy 外)
  │     │ 1. bootstrap_token 検証 (HKDF 派生鍵 + typ/aud/iss/base_url_hash/jti)
  │     │ 2. jti を NonceStore.Consume (one-time)
  │     │ 3. auth.ContextWithUserID(ctx, claims.UserID) で uid 注入
  │     │ 4. state JWT 発行 (flow="multi", base_url, alias)
  │     │ 5. Referrer-Policy: no-referrer を付与
  │     │ 6. 302 → Backlog 認可画面 (redirect_uri = "/oauth/backlog/callback")
  │     ▼
  │   Backlog OAuth → redirect_uri = "/oauth/backlog/callback" (= single と共有)
  │
  ├─ "/healthz" ──▶ healthHandler
  └─ "/"        ──▶ BacklogAuthorizeGate → authMW.Wrap (idproxy)
                       └─ bridge → innerMux
                             ├─ /mcp   ──▶ MCP server
                             ├─ /oauth/backlog/authorize         (single-space, idproxy 認証あり)
                             ├─ /oauth/backlog/callback ──┐
                             ├─ /oauth/backlog/status            (single-space)
                             └─ /oauth/backlog/disconnect        (single-space)
                                                          │
                                                          ▼
                                              OAuthHandler.HandleCallback
                                                  │ state JWT pre-parse
                                                  │   ├─ flow=="multi" ─▶ MultiSpaceOAuthHandler.HandleCallback
                                                  │   │                      (state 検証 → token 保存 → SpaceStore.Upsert)
                                                  │   └─ それ以外       ─▶ 既存 single-space ロジック
```

### 案 B (callback 単一化) を採用した理由
- Backlog OAuth app の redirect_uri が単一しか登録できない仕様（team-lead 確認済み）
- callback 経路の認証境界は state JWT (HMAC 署名) に集約されているため、idproxy 内側でも外側でも署名検証で安全性は同等
- callback パスを分けないことで Backlog OAuth app 設定変更が不要 → 既存運用に対する影響ゼロ

### Go net/http.ServeMux のルーティング動作（案 B 採用後）
- Go 1.22+ の `net/http.ServeMux` の Method+Path pattern を採用し、明示的に method を限定する:
  ```go
  topMux.HandleFunc("GET /oauth/backlog/multi/authorize", msh.HandleAuthorize) // multi authorize のみ idproxy 外
  topMux.Handle("/", topHandler)  // catch-all → bridge → authMW.Wrap → innerMux
  // callback は innerMux 側 (idproxy 内) で登録され、dispatcher で multi/single 分岐
  ```
- ServeMux は longest-prefix match + method 一致を採用するため `/oauth/backlog/multi/authorize` が優先される。順序依存はない。
- これにより multi-space authorize は idproxy ラップを完全に迂回し、かつ `/oauth/backlog/multi/authorize/extra` のような prefix 攻撃が成立しないことを保証する。
- callback (`/oauth/backlog/callback`) は引き続き innerMux 側（idproxy ラップ内）に登録し、`OAuthHandler.HandleCallback` の dispatcher で state.flow に応じて multi/single を分岐。

### 別パス (`/multi/...`) に分離する理由
- single-space の `/oauth/backlog/authorize` は `BacklogAuthorizeGate` から `continue=<idproxy /authorize URL>` 付きで呼ばれる前提があり、`idproxy` 認証 + `auth.UserIDFromContext` に依存している（既存契約）
- 同一パスで multi/single を分岐させると、`InstallOAuthRoutes` 側に「どちらに振るか」を判断するロジックが入り脆い
- パスを分けることで「idproxy ラップ内側 vs 外側」の二箇所登録を構造的に保証できる
- 既存テスト（`multi_space_oauth_handler_test.go` 等）は `httptest.NewRequest` でハンドラを直接呼んでいるためルーティングに影響しないが、これは正当化材料ではない。**真の理由は上記の routing 分離要求**である。

---

## 4. signed user_token (= bootstrap_token) の設計

### 4.1 目的
MCP 経路で確定済みの userID を、idproxy 認証無しで `MultiSpaceOAuthHandler.HandleAuthorize` に渡す。

### 4.2 設計判断（GPT-5.5 レビュー反映）
- 命名: 計画 §4 以降では **`bootstrap_token`** という呼称を採用（外部レビューで bearer credential であることを示唆する命名が推奨されたため）。URL パラメータ名も `bootstrap_token` とする。
- 既存 `internal/auth/state.go` の `StateClaims` (HS256 JWT) パターンを踏襲しつつ、**用途別 key derivation で鍵を分離**する:
  - `stateSecret` は raw key として保持
  - `bootstrapKey = HKDF-SHA256(stateSecret, salt=nil, info="logvalet user bootstrap v1", length=32)`
  - `oauthStateKey` は引き続き raw `stateSecret` を使用（既存トークンとの後方互換のため、HKDF 移行は別 milestone で扱う）
- `typ` claim も保険として追加（key 分離だけでなく claim レベルでも分離）

### 4.3 Claims 定義 (`internal/auth/user_token.go`)

```go
type BootstrapTokenClaims struct {
    UserID       string `json:"sub"`
    Typ          string `json:"typ"`             // 固定: "user_bootstrap_v1"
    BaseURLHash  string `json:"base_url_hash"`   // sha256(normalized base_url) の先頭 16 hex (L3: 衝突耐性 64bit、URL長 2KB 制約内に収めるための切り詰め。HMAC 署名で改竄不可なので 16hex で十分)
    AliasHash    string `json:"alias_hash"`      // sha256(alias)        の先頭 16 hex (L3: 同上)
    JTI          string `json:"jti"`             // one-time use 識別子 (32hex = 128bit; NonceStore キーの衝突を実質ゼロにする)
    jwt.RegisteredClaims                          // aud="logvalet/multi-authorize", iss="logvalet",
                                                  // exp, iat
}

const (
    DefaultBootstrapTokenTTL = 3 * time.Minute   // 計画より短縮 (10→3)
    BootstrapTokenAudience   = "logvalet/multi-authorize"
    BootstrapTokenIssuer     = "logvalet"
    BootstrapTokenType       = "user_bootstrap_v1"
)
```

### 4.4 セキュリティ評価

| 観点 | 対策 |
|---|---|
| リプレイ攻撃 | `jti` を one-time use にする。**既存 `space.NonceStore` を流用**し、authorize 成功時に `Consume` する（既存 state nonce と同じインターフェイス）。`jti` キーは衝突回避のため `bs:` プレフィックスを付与: `bs:<jti>` |
| トークン混同 (state↔bootstrap) | `typ="user_bootstrap_v1"` を必須検証 + 用途別 HKDF 派生鍵で署名 → state JWT の鍵では署名検証に通らない（二重防御） |
| トークン横展開（盗まれた token で別 space に登録） | `base_url_hash` / `alias_hash` を claim に束縛。authorize 受信時に `request.URL.Query()["base_url"]` / `["alias"]` の hash と一致検証 → 不一致は 401 |
| 用途分離 (key separation) | HKDF で state JWT 用と bootstrap_token 用を info string で分離。state JWT の検証実装ミスが bootstrap 検証に波及しない。**stateSecret が漏洩した場合は両方危殆化するため、運用手順として「(1) `LOGVALET_MCP_OAUTH_STATE_SECRET` を新 secret にローテーション → (2) NonceStore (DynamoDB / SQLite) をフラッシュして既存 jti / nonce を無効化 → (3) 全 MCP インスタンスを再起動」をセットで実施する**（M2 反映） |
| ログ漏洩 (access log) | (a) `bootstrap_token` 生値はサーバーログ・アプリログに出さない。(b) authorize ハンドラ応答に `Referrer-Policy: no-referrer` を設定する。(c) 後段 Backlog 認可画面への 302 location には `bootstrap_token` を含めない（state JWT は Backlog に渡るが、bootstrap_token は authorize ハンドラ内で消費して終わる）。(d) ドキュメントとして access_log で query を redact する設定を推奨 |
| 短命化 | TTL を 3 分に短縮（10 → 3 分） |
| MCP クライアント側ログ | `logvalet_space_connect_url` の戻り値 `authorization_url` に bootstrap_token が含まれる → MCP クライアント (Claude 等) の会話ログに残る点は受容リスクとして docs に明記。one-time + 短 TTL + base_url 束縛で実質的な悪用は不可 |

### 4.5 既存 ValidateState 関数との関係
- `ValidateState` は touch しない（後方互換のため既存挙動を完全維持）
- 新規 `ValidateBootstrapToken(tokenStr, expectedBaseURL, expectedAlias, key) (userID, jti string, err error)` を別関数として追加
  - 純粋関数: 暗号検証 + claim 束縛検証のみ。`NonceStore` には依存しない
  - jti の one-time consume は handler 側の責務（§7 検証順序 Step 6 参照）
  - 引数の `expectedBaseURL` は handler 側で `space.NormalizeBaseURL` を通した値を渡す前提
- 将来 state JWT も HKDF + `typ` 化する場合は別 milestone (M19+) で扱う

### 4.6 鍵導出ヘルパー (`internal/auth/keys.go` 新設)
```go
// DeriveBootstrapKey は stateSecret から bootstrap token 専用 HS256 鍵を導出する。
// info string をバージョン付きにすることで将来のローテーションを可能にする。
func DeriveBootstrapKey(stateSecret []byte) []byte {
    h := hkdf.New(sha256.New, stateSecret, nil, []byte("logvalet user bootstrap v1"))
    out := make([]byte, 32)
    _, _ = io.ReadFull(h, out)
    return out
}
```

---

## 5. 修正対象ファイル一覧

| ファイル | 変更内容 |
|---|---|
| `internal/auth/user_token.go` | **新規**: `BootstrapTokenClaims`, `GenerateBootstrapToken`, `ValidateBootstrapToken`, `DefaultBootstrapTokenTTL`, 定数 (Audience/Issuer/Type) |
| `internal/auth/user_token_test.go` | **新規**: round-trip / typ mismatch / aud mismatch / base_url mismatch / alias mismatch / expired / tampered / alg=none 拒否 / alg 別 / key confusion / jti replay |
| `internal/auth/keys.go` | **新規**: `DeriveBootstrapKey(stateSecret []byte) []byte` (HKDF-SHA256, info="logvalet user bootstrap v1") |
| `internal/auth/keys_test.go` | **新規**: stable / different info / length |
| `internal/auth/errors.go` | bootstrap 関連エラー (`ErrBootstrapInvalid`, `ErrBootstrapExpired`, `ErrBootstrapReplayed`) を追加 |
| `internal/mcp/tools_space_registry.go` | `spaceConnectURL` シグネチャ拡張 (bootstrapKey, TTL, nonceStore 受領)、bootstrap_token 生成 + jti store + URL 付与 |
| `internal/mcp/tools_space_registry_test.go` | 既存テストを新シグネチャに追従、bootstrap_token 付与・jti store・key 不在 fail-safe の検証ケース追加 |
| `internal/mcp/server.go` | `ServerConfig` に `BootstrapKey []byte`, `BootstrapTokenTTL time.Duration`, `MultiSpaceAuthorizeURL string`, `NonceStore space.NonceStore` を追加し、`RegisterSpaceRegistryTools` に橋渡し |
| `internal/transport/http/multi_space_oauth_handler.go` | (a) `bootstrapKey` フィールド追加。(b) `HandleAuthorize` 冒頭で `bootstrap_token` 検証 + jti consume + `ContextWithUserID` で uid 注入。(c) `HandleCallback` も state.UserID から ctx に uid 注入してから既存ロジックを動かす。(d) `Referrer-Policy: no-referrer` ヘッダ付与 |
| `internal/transport/http/multi_space_oauth_handler_test.go` | 新フロー用テスト追加（§9 Step2 / Step3 参照） |
| `internal/cli/mcp.go` | topMux に `GET /oauth/backlog/multi/authorize` を **idproxy ラップ外**で直接登録（Method+Path pattern）。callback は **既存 `/oauth/backlog/callback` を維持**し、ラップ内のまま使う（後述「案B: callback 単一化」）。`InstallOAuthRoutes` から multi-space 引数を撤去 |
| `internal/cli/mcp_oauth.go` | (a) `InstallOAuthRoutes` シグネチャから `msh` を削除。(b) `OAuthDeps` に `MultiSpaceAuthorizeURL`, `BootstrapKey` を追加（**`MultiSpaceRedirectURL` は廃止**: redirect_uri は single と共有）。(c) `BuildOAuthDeps` で `MultiSpaceOAuthHandler` 構築時に `bootstrapKey = auth.DeriveBootstrapKey(secret)` を渡す。(d) `MultiSpaceOAuthHandler` の `redirectURI` は `cfg.BacklogRedirectURL` (= single-space と同一) をそのまま受ける |
| `internal/cli/mcp_oauth_test.go` | `InstallOAuthRoutes` 新シグネチャに追従。multi-space 登録は別テストへ。**`mcp_oauth_e2e_test.go:268` を含む全 caller を grep して更新する** |
| `internal/cli/mcp_oauth_e2e_test.go` | multi-space フロー E2E を追加: ブラウザ → `/oauth/backlog/multi/authorize?...&bootstrap_token=...` → 302 → Backlog → `/oauth/backlog/callback?code=...&state=...` (state.flow="multi") → 200 |
| `internal/auth/state.go` | **H1 反映**: `StateClaims` に `Flow string` + `Typ string` フィールド追加（共に `omitempty` で後方互換）。定数 `OAuthStateAudience` / `OAuthStateIssuer` / `OAuthStateTypeV1` を追加。`GenerateStateWithSpaceInfo` は `flow="multi"` + `typ="oauth_state_v1"` + `aud` + `iss` を必須セット。`GenerateStateWithContinue` / `GenerateState` は `flow="single"` + `typ="oauth_state_v1"` + `aud` + `iss` を必須セット。`ValidateState` に typ 空受理 (backward compat) + typ 非空時は aud/iss を厳密検証 のロジック追加 |
| `internal/auth/state_test.go` | H1 反映: typ/aud/iss round-trip テスト、backward compat (`typ=""` 受理) テスト、unknown typ 拒否テスト追加（Step 5-A 参照） |
| `internal/transport/http/oauth_handler.go` (callback dispatcher) | `OAuthHandler.HandleCallback` 冒頭で state JWT を pre-parse し、`claims.Flow == "multi"` なら **`MultiSpaceOAuthHandler.HandleCallback` に処理を委譲**する。`flow=""`（既存 single-space）または `flow="single"` は従来通り |
| `go.mod` / `go.sum` | `golang.org/x/crypto/hkdf` を依存追加 |

### 案 B: callback 単一化（Backlog 側の redirect_uri 単一制約への対応）

**確定事項** (team-lead → 2026-05-20):
- Backlog OAuth app は **redirect_uri を 1 つしか登録できない** ことが確認された
- 結論: callback パスは **`/oauth/backlog/callback` で固定**し、ハンドラ層で multi/single を分岐する

#### ルーティング/フロー全体図

```
ブラウザ ─── /oauth/backlog/multi/authorize?base_url=...&bootstrap_token=...
                │
                ▼  (topMux, idproxy ラップ外)
        MultiSpaceOAuthHandler.HandleAuthorize
                │ bootstrap_token 検証 → state JWT 発行 (flow="multi", base_url, alias) → 302
                ▼
        Backlog 認可画面 ─── ユーザー承認 ───▶ redirect_uri = "/oauth/backlog/callback"
                                                       │
                                                       ▼  (innerMux, idproxy ラップ内)
                                                OAuthHandler.HandleCallback
                                                       │ state JWT pre-parse
                                                       │   ├─ flow == "multi" → MultiSpaceOAuthHandler.HandleCallback へ委譲
                                                       │   └─ それ以外           → 既存 single-space ロジック
```

#### state JWT への強化（H1: devils-advocate 反映、M18 スコープに取込）

multi callback の認証境界が state JWT 単発検証のみになるため、**新規発行トークンを必ず強化する**。検証側は backward compat を保つ。

```go
// internal/auth/state.go - StateClaims に追加
type StateClaims struct {
    UserID   string `json:"uid"`
    Tenant   string `json:"tenant"`
    Nonce    string `json:"nonce"`
    Continue string `json:"continue,omitempty"`
    BaseURL  string `json:"base_url,omitempty"`
    Alias    string `json:"alias,omitempty"`
    Flow     string `json:"flow,omitempty"`  // "multi" | "single" (新規必須) | "" (旧 token、検証時 "single" 扱い)
    Typ      string `json:"typ,omitempty"`   // "oauth_state_v1" (新規必須) | "" (旧 token、検証時 typ チェック skip)
    jwt.RegisteredClaims                     // Audience, Issuer, ExpiresAt, IssuedAt
}

// 定数
const (
    OAuthStateAudience = "logvalet/oauth-callback"
    OAuthStateIssuer   = "logvalet"
    OAuthStateTypeV1   = "oauth_state_v1"
)
```

**生成側（必ず追加）**:
- `GenerateStateWithSpaceInfo`: `Flow="multi"` + `Typ="oauth_state_v1"` + `aud="logvalet/oauth-callback"` + `iss="logvalet"` を必須セット
- `GenerateStateWithContinue` / `GenerateState`: `Flow="single"` + `Typ="oauth_state_v1"` + `aud` + `iss` を必須セット（**既存呼出は値を増やすだけで挙動は変わらない**）

**検証側（backward compat）**:
- `ValidateState`:
  - `claims.Typ == ""` の旧 token は受理（既存セッションを壊さない）。デプロイ後 max 10 分（state TTL）で自然消滅
  - `claims.Typ == "oauth_state_v1"` なら `aud="logvalet/oauth-callback"` / `iss="logvalet"` を必須検証
  - `claims.Typ` が上記以外（未知の値）は ErrStateInvalid で拒否
  - `claims.Flow == ""` は `single` 扱い（旧 token はすべて single だった）
  - `claims.Flow == "multi"` でも `claims.Tenant`/`claims.BaseURL`/`claims.Alias` の空チェックは既存通り

**dispatcher での flow 判定**:
- §8.1 で `claims.Flow == "multi"` をチェックする際、`""` も `"single"` も dispatch しない（single 扱い）
- 結果: 旧 single token は壊れず、新 single token も dispatch されず、新 multi token のみ multi handler へ委譲

**rollout 戦略**:
- 本 milestone デプロイ後、`Typ=oauth_state_v1` 検証は **typ が "" 以外なら厳密** に行う
- 旧 `Typ=""` token は最大 `state TTL = 10 分` 後に自動失効するため、デプロイ後 10 分以上経過してから typ skip 経路は実質的に死に経路となる（M19 で完全撤去可能）

#### callback dispatcher の責務

`OAuthHandler.HandleCallback` 冒頭で:
1. state JWT を **pre-parse** する（既存の検証ロジックと同じ secret/方式）
2. `claims.Flow == "multi"` なら `h.multiSpaceHandler.HandleCallback(w, r)` を呼び return
3. それ以外は既存 single-space ロジックをそのまま実行

**重要な設計判断**:
- pre-parse の失敗（state 改竄/期限切れ等）は既存と同じエラーレスポンスで処理 → 攻撃者に「multi/single どちらが存在するか」を漏らさない
- multi 委譲後は `MultiSpaceOAuthHandler.HandleCallback` 側で nonce 消費 + token 保存を行う（既存実装をそのまま流用）
- idproxy ラップ内に居続けるため、callback は idproxy セッションが必要 → **これは仕様**（ユーザーは authorize 開始時に idproxy 認証済みのはず）

#### multi callback で idproxy セッションが必要な理由（受容リスク）

- authorize は bootstrap_token で idproxy 不要だったが、callback では Backlog が idproxy 認証セッションのある状態で戻すため、ユーザーは事実上 idproxy ログイン済み状態
- 仮にユーザーが authorize 完走後に idproxy セッションを切れた状態で callback を踏むケースは想定外（実運用では即時 302 が連続するため発生しない）
- もし発生した場合は idproxy ログイン画面に飛ばされる → 再ログイン後 `continue` で `/oauth/backlog/callback?code=...&state=...` に戻る → state JWT が valid なら成功
- callback 経路で `auth.UserIDFromContext` が取れない場合は `claims.UserID` をそのまま使う（multi 側ハンドラ既存ロジック）→ idproxy セッション無くても完走可能

### `MultiSpaceOAuthHandler` の構築依存

- `redirectURI`: `cfg.BacklogRedirectURL` (= `<externalURL>/oauth/backlog/callback`、single と同一)
- `bootstrapKey`: `auth.DeriveBootstrapKey(stateSecret)`
- 他は既存と同じ

### `BuildOAuthDeps` 内の構築順序（**重要 / 案 B 必須**）

案 B では `OAuthHandler` が `*MultiSpaceOAuthHandler` を依存として持つため、現状の構築順序（line 75-102: provider → store → tm → factory → OAuthHandler）の後に MultiSpaceHandler を構築する流れは **循環/順序依存** になる。

採用: **Option A (構築順序を入れ替える)**

```
provider → store → tm → factory
    ↓
1. MultiSpaceOAuthHandler を先に構築 (spaceStore が nil なら nil のまま)
    ↓
2. OAuthHandler を後で構築し、引数で *MultiSpaceOAuthHandler を渡す（nil 可）
```

理由: setter 方式 (`handler.AttachMultiSpaceHandler`) はミューテーションを許す → コンストラクタ完了後の不変性が崩れて並行安全性を考慮する必要が生じる。コンストラクタ引数で一発渡しが Go 慣行的にもクリーン。

### `NewOAuthHandler` シグネチャ変更の影響範囲

- `internal/transport/http/oauth_handler_test.go`: T1-T15 等の全テストで `NewOAuthHandler(...)` を呼出 → **末尾引数に nil を追加して更新**
- `internal/transport/http/oauth_handler_continue_test.go`: 同様に更新
- `internal/cli/mcp_oauth.go:97-102`: 上記「構築順序入れ替え」と同時に末尾引数を追加
- 確認コマンド: `grep -rn "NewOAuthHandler(" internal/` で全 caller を網羅
- Step 3 Red 段階で大量のシグネチャ不一致エラーが出るのは想定内（既存テストは nil を渡せば挙動変わらず通る）

### `InstallOAuthRoutes` の明示的取り扱い
- **撤去**: 第 3 引数 `msh *MultiSpaceOAuthHandler` を削除する
- multi-space ルートは topMux に直接登録（`mcp.go` 側で実施）
- 撤去理由: 同一関数内で「idproxy ラップ外」と「ラップ内」両方を登録するのは構造上不可能（`InstallOAuthRoutes` は 1 つの mux しか受け取らないため）
- これで dead code / 二重登録のリスクを排除
- caller の全更新が必要: `grep -rn "InstallOAuthRoutes" internal/ cmd/` で網羅

### `EnsureBacklogConnected` / `BacklogAuthorizeGate` への影響
- `EnsureBacklogConnected` (`mcp_auto_redirect.go:54`) は `/oauth/backlog/` prefix を skip するため `/oauth/backlog/multi/...` も自動的に skip 対象 → **変更不要**
- `BacklogAuthorizeGate` (`backlog_authorize_gate.go`) は idproxy の `/authorize` だけを対象 → multi-space ルートを撃たない → **変更不要**
- 上記 2 点は plan に明示記載することで reviewer の追加質問を抑える

### `multi_space_oauth_handler.go:140-143` の `QueryUnescape` 防御
- topMux 直接登録後は二重エンコードのシナリオが消えるが、**コード自体は残す**
- 理由: 将来 reverse-proxy 等の中間機が同様の挙動を見せても破綻しない defense-in-depth として安価
- コメントを更新して「topMux 直登録により通常時は不要だが、中間プロキシ対策として残置」と明記

---

## 6. spaceConnectURL の動作変更（詳細）

### Before
```go
func spaceConnectURL(ctx, args, authBaseURL) {
    q := url.Values{}
    q.Set("base_url", rawBaseURL)
    if alias != "" { q.Set("alias", alias) }
    return authBaseURL + "?" + q.Encode()
}
```

### After
```go
// シグネチャ拡張: bootstrapKey []byte, ttl time.Duration を ServerConfig 経由で受領
func spaceConnectURL(ctx, args, multiAuthURL, bootstrapKey, ttl) {
    userID, ok := auth.UserIDFromContext(ctx)
    if !ok { return error "authentication required" }

    // base_url 正規化 + alias 導出 (authorize と同じロジックで先行検証)
    baseURL, err := space.NormalizeBaseURL(rawBaseURL)
    if err != nil { return error }
    alias := stringArg(args, "alias")
    if alias == "" {
        alias, _ = space.DeriveAliasFromBaseURL(baseURL)
    }

    jti := randomHex(16)

    // jti を NonceStore.Store して one-time use の前提を整える。
    // authorize 側で Consume されないまま放置されても TTL で expire するので運用負荷は小さい。
    if err := nonceStore.Store(ctx, userID, "bs:"+jti, ttl); err != nil {
        return error("failed to register one-time bootstrap nonce")
    }

    token, err := auth.GenerateBootstrapToken(auth.BootstrapTokenInput{
        UserID:    userID,
        BaseURL:   baseURL,
        Alias:     alias,
        Key:       bootstrapKey,
        TTL:       ttl,
        JTI:       jti,
    })
    if err != nil { return error }

    q := url.Values{}
    q.Set("base_url", rawBaseURL)
    if alias != "" { q.Set("alias", alias) }
    q.Set("bootstrap_token", token)
    return multiAuthURL + "?" + q.Encode()
}
```

### 既存呼出契約
- `RegisterSpaceRegistryTools` 経由でしか呼ばれない → bootstrapKey/TTL/`nonceStore` は `ToolRegistry` 構築時の `ServerConfig` に持たせる
- bootstrapKey 未設定の場合は token を付与せず、authorize ハンドラ側が 401 を返す（fail-safe）
- nonceStore 未設定の場合も同じ fail-safe（multi-space モード未有効と同等扱い）
- base_url / alias を先行検証することで、不正な base_url を bootstrap_token に焼き付けないようにする

### one-time use の運用契約（重要）
- **Store フェーズ**: `spaceConnectURL` で `nonceStore.Store(ctx, userID, "bs:"+jti, ttl)` を実施 → token 発行
- **Consume フェーズ**: `HandleAuthorize` で `nonceStore.Consume(ctx, claims.UserID, "bs:"+claims.JTI)` を実施 → 成功すると以降同じ token は使えない
- **M4 反映 — 部分失敗の取り扱い**: `nonceStore.Store` 成功直後に `GenerateBootstrapToken` が失敗した場合は token を発行せずエラーを返す。Store 済みの jti は **TTL で自然消失するのを許容する**（明示的な rollback は不要）。理由: jti は未公開のためリプレイ攻撃面はなく、TTL 経過後の自動削除に依存しても security 上の影響なし
- `ErrNonceAlreadyUsed` が返れば 401 を返す（後述 §7）
- 既存 state nonce (`MultiSpaceOAuthHandler.HandleAuthorize:207` の `nonceStore.Store`) と key prefix が衝突しないよう `bs:` を採用

---

## 7. MultiSpaceOAuthHandler.HandleAuthorize の差分

```go
// Referer 漏洩抑止 (常時)
w.Header().Set("Referrer-Policy", "no-referrer")

// 追加（query 取得部分の前）
bootstrapToken := r.URL.Query().Get("bootstrap_token")
if bootstrapToken == "" {
    writeJSONError(w, 401, errCodeUnauthenticated, "bootstrap_token is required")
    return
}

// query から base_url/alias を取り出し、claim と比較するために正規化する
rawBaseURL := r.URL.Query().Get("base_url")
queryAlias := r.URL.Query().Get("alias")
normalizedBaseURL, err := space.NormalizeBaseURL(rawBaseURL) // 失敗は既存 400 と同じ扱い
// ... (alias 未指定なら DeriveAliasFromBaseURL)

// ValidateBootstrapToken は純粋関数（暗号 + claim 束縛検証のみ）
// NonceStore I/O は handler 側の責務として分離する
userID, jti, err := auth.ValidateBootstrapToken(bootstrapToken, normalizedBaseURL, resolvedAlias, h.bootstrapKey)
if err != nil {
    h.logger.WarnContext(ctx, "multi-space authorize rejected",
        slog.String("reason", classifyBootstrapErr(err))) // typ_mismatch/aud_mismatch/url_mismatch/expired/invalid
    writeJSONError(w, 401, errCodeUnauthenticated, "bootstrap_token invalid")
    return
}

// 既存 UserIDFromContext(ctx) パスが動くように uid を注入
ctx = auth.ContextWithUserID(ctx, userID)
r = r.WithContext(ctx)

// jti を NonceStore で one-time consume (handler 側で実施 — トランザクション境界の制御を集約)
if err := h.nonceStore.Consume(ctx, userID, "bs:"+jti); err != nil {
    if errors.Is(err, space.ErrNonceAlreadyUsed) {
        h.logger.WarnContext(ctx, "multi-space authorize rejected",
            slog.String("reason", "bootstrap_jti_replayed"),
            slog.String("user_id", userID))
        writeJSONError(w, 401, errCodeUnauthenticated, "bootstrap_token replayed")
        return
    }
    h.logger.ErrorContext(ctx, "multi-space authorize failed",
        slog.String("reason", "nonce_consume_failed"))
    writeJSONError(w, 500, errCodeInternalError, errMsgInternalError)
    return
}
// ↓ 以降の既存ロジック (state JWT 生成 → Backlog 302) は無変更
```

### 設計判断: なぜ ctx に注入するか
- 既存 `auth.UserIDFromContext(ctx)` (line 175) を残せば本体ロジックの blast radius が極小
- 別経路で userID を渡すと「ctx 経由 / 引数経由」の二重実装になり保守性が落ちる

### 検証順序（fail fast）
1. method == GET (既存通り)
2. `Referrer-Policy: no-referrer` ヘッダ設定
3. base_url 取得 + 正規化（既存通り）
4. alias 取得 + 導出（既存通り）
5. bootstrap_token 取得 + 検証（**新規**）
6. jti を NonceStore.Consume（**新規 / one-time use**）
7. ctx に userID 注入（**新規**）
8. `auth.GenerateStateWithSpaceInfo` で state JWT 発行 → 内部で `flow="multi"` がセットされる（**案 B 連携**）
9. `provider.BuildAuthorizationURL(state, h.redirectURI)` で Backlog 認可 URL を組み立て → 302
   - **重要**: `h.redirectURI` は single-space と共有の `cfg.BacklogRedirectURL` (= `/oauth/backlog/callback`)
   - Backlog 側は redirect_uri 完全一致で検証するため、ここで値がズレると認可が失敗する

---

## 8. HandleCallback の差分 (案 B: callback 単一化)

### 8.1 OAuthHandler.HandleCallback の dispatcher 化

callback は `/oauth/backlog/callback` 単一に統合（既存 single-space と共有）。`OAuthHandler.HandleCallback` の **`ValidateState` 成功直後・`claims.Tenant != h.tenant` チェックより前** に dispatcher を挿入する。

**挿入位置（既存 line 番号基準）**:
- 既存 `oauth_handler.go:292` の `claims, err := auth.ValidateState(state, h.stateSecret)` 成功後
- 既存 `oauth_handler.go:310` の `if claims.Tenant != h.tenant` チェック **より前**

これは必須: `flow="multi"` の場合 `claims.Tenant` は新規スペースの tenant であり single-space の `h.tenant` と一致しない。dispatcher を tenant チェックの後に置くと multi flow が `400 invalid_tenant` で弾かれる。

```go
// internal/transport/http/oauth_handler.go
// OAuthHandler 構築時に MultiSpaceOAuthHandler を optional 依存として受け取る
type OAuthHandler struct {
    // ... existing fields
    multiSpaceHandler *MultiSpaceOAuthHandler  // nil 可（multi 無効時）
}

func (h *OAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
    // 既存: method check, error クエリ, code/state 空チェック (line 256-289) はそのまま
    // ...

    // 既存: state JWT 検証 (line 292)
    claims, err := auth.ValidateState(state, h.stateSecret)
    if err != nil {
        // 既存のエラー処理（state_expired / state_invalid）にフォールスルー
        // → multi/single どちらを意図したか攻撃者に漏らさない
        return // 既存エラーパスへ
    }

    // ★ NEW (line 292 と 310 の間に挿入): multi フローへの委譲
    //   fail closed: multi handler 未設定なら single にフォールバックしない
    if claims.Flow == "multi" {
        if h.multiSpaceHandler == nil {
            writeJSONError(w, 500, errCodeInternalError, "multi-space handler not configured")
            return
        }
        h.multiSpaceHandler.HandleCallback(w, r)
        return
    }

    // 既存: claims.Tenant != h.tenant チェック (line 310) 以降は変更なし
}
```

**意図的な二重 ValidateState**: dispatcher で一度 parse → `MultiSpaceOAuthHandler.HandleCallback` でもう一度 parse。HMAC 検証は cheap、defense in depth として許容。

### 8.2 MultiSpaceOAuthHandler.HandleCallback の差分

委譲先で `claims.Flow == "multi"` の二重確認を行う（防御的）。idproxy ctx に uid が居なくても `claims.UserID` を信頼して動かす:

```go
func (h *MultiSpaceOAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
    // 1. method check (既存)
    // 2. error/code/state 取得 (既存)

    claims, err := auth.ValidateState(stateJWT, h.stateSecret)
    // ... 既存エラー処理

    // 防御的: flow="multi" を確認（dispatcher が誤呼び出しした場合の保険）
    if claims.Flow != "multi" {
        writeJSONError(w, 400, errCodeStateInvalid, errMsgStateInvalid)
        return
    }

    // M5 反映: idproxy ctx 由来の userID と state.UserID を比較する dead check は撤去。
    // 多発ケース（authorize 後に idproxy セッション切れ）で意味のあるガードにならず、
    // ctx 注入後は両者が常に等しくなる "もはや認証ではない" 状態だったため。
    //
    // 新しい認証境界:
    //   1. state JWT 署名検証 (HS256 + secret)
    //   2. claims.Typ == "oauth_state_v1"
    //   3. claims.Aud == "logvalet/oauth-callback"
    //   4. claims.Iss == "logvalet"
    //   5. claims.Flow == "multi" (dispatcher 二重チェック)
    //   6. state nonce one-time consume (Tenant + Nonce で NonceStore.Consume)
    // を通った時点で `claims.UserID` を信頼する。
    ctx = auth.ContextWithUserID(ctx, claims.UserID)
    r = r.WithContext(ctx)

    // 以降は既存ロジック (nonce consume → exchange → SaveToken → Upsert)
}
```

### 8.3 設計判断
- **fail closed**: `multiSpaceHandler` 未設定で `flow="multi"` を受けたら 500（single にフォールバックしない）
- **state pre-parse の失敗は既存エラーで返す**: dispatcher 失敗時に「multi/single どちらが期待されたか」を攻撃者に漏らさない
- **flow フラグ自体は signed JWT 内**: 改竄不可
- **二重チェック**: dispatcher 側 (`claims.Flow == "multi"`) と委譲先 (`claims.Flow != "multi"` で reject) の両方で確認 → どちらか実装ミスしても安全側に倒れる
- **M5 反映: dead check を削除**: `ctxUserID != claims.UserID` 比較は ctx 注入後に恒等的に成立するため削除。新しい認証境界は state JWT 強化 (typ/aud/iss/flow) + state nonce one-time consume で構成する

---

## 9. TDD 実装ステップ

各ステップで Red → Green → Refactor を 1 サイクルとする。

### Step 0: Backlog OAuth app の redirect_uri 設定確認（**H3 反映: 着手前 gating**）

実装着手前に以下を確認し、**確認完了するまで Step 1 以降に進まない**:

- [x] Backlog OAuth app の redirect_uri 登録枠数を確認 → **単一登録のみ対応** (2026-05-20 team-lead 確認済)
- [x] 採用方針確定 → **案 B (callback 単一化 + state.flow 分岐)** で確定
- [x] `LOGVALET_MCP_BACKLOG_REDIRECT_URL` の現値変更不要を確認
- [x] Backlog OAuth app の設定変更不要を確認
- [ ] `docs/specs/multi_space_oauth.md` に案 B 採用経緯を追記（Step 7 で実施）

**確認結果が異なっていた場合の代替フロー**:
- 案 A (redirect_uri 別 path): Step 5/6 の差分が大幅変更（topMux に callback も追加、state.flow 撤去）
- 案 C (新規 OAuth client 作成): `BacklogClientID` を multi 用に分離、`OAuthEnvConfig` 拡張

本 milestone は **案 B 確定済**のため Step 1 以降をそのまま進められる。

### Step 1: `auth.GenerateBootstrapToken` / `ValidateBootstrapToken` + `DeriveBootstrapKey`
- **Red**: `internal/auth/user_token_test.go` に以下を書く
  - `TestGenerateBootstrapToken_RoundTrip`: 生成 → 検証で sub/base_url_hash/alias_hash/aud/iss/typ が一致
  - `TestValidateBootstrapToken_TypMismatch`: typ="oauth_state" 等 → ErrBootstrapInvalid
  - `TestValidateBootstrapToken_AudienceMismatch`: aud 不正 → ErrBootstrapInvalid
  - `TestValidateBootstrapToken_BaseURLMismatch`: 別 base_url を Verify 時に渡す → ErrBootstrapInvalid
  - `TestValidateBootstrapToken_AliasMismatch`: 別 alias を Verify 時に渡す → ErrBootstrapInvalid
  - `TestValidateBootstrapToken_Expired`: TTL=1ns で sleep 後 ErrBootstrapExpired
  - `TestValidateBootstrapToken_Tampered`: 末尾 1 文字書換で ErrBootstrapInvalid
  - `TestValidateBootstrapToken_EmptyUID`: claims.sub が空なら ErrBootstrapInvalid
  - `TestValidateBootstrapToken_AlgNone`: alg=none を強制したトークンを拒否
  - `TestValidateBootstrapToken_AlgRS256_Rejected`: 別 alg を拒否
  - ~~`TestValidateBootstrapToken_JTIReplayed`~~: 削除（jti consume は handler 責務に分離したため、Step 2 の `TestHandleAuthorize_BootstrapTokenJTIReplay` でカバー）
  - `TestGenerateBootstrapToken_BaseURLWithPath_Rejected` (**Q1 反映**): path/query 付き base_url は `space.NormalizeBaseURL` のエラーで弾かれる
  - `TestValidateBootstrapToken_BaseURLNormalizedRoundTrip` (**Q1 反映**): 発行時 `foo.backlog.com` (scheme なし) → 検証時 `https://foo.backlog.com/` (trailing slash) で一致
  - `TestValidateBootstrapToken_KeyConfusion_StateSecret`: raw stateSecret で署名したトークンが bootstrapKey で検証失敗
  - `TestDeriveBootstrapKey_Stable`: 同じ stateSecret から 2 回 derive して同一結果
  - `TestDeriveBootstrapKey_DifferentInfoStrings`: info 文字列を変えると別鍵
  - `TestGenerateBootstrapToken_BaseURLNormalized` (**M3 反映**): trailing slash 違い (`https://foo.backlog.com` vs `https://foo.backlog.com/`) でも同じ hash になる
  - `TestGenerateBootstrapToken_BaseURLCaseInsensitive` (**M3 反映**): ホスト名は case-insensitive（`Foo.backlog.com` と `foo.backlog.com` が同一）。scheme 大文字 (`HTTPS://...`) も小文字に正規化
  - `TestGenerateBootstrapToken_BaseURLIDN` (**M3 反映**): IDN ホスト名（例: `日本語.backlog.com` → Punycode `xn--wgv71a119e.backlog.com`）が正規化後に一意な hash になる
  - `TestGenerateBootstrapToken_AliasNormalized` (**M3 反映**): alias の前後空白除去 + 小文字化を `space.ValidateAlias` と整合させる
- **Green**: `internal/auth/user_token.go` に最小実装 + `internal/auth/keys.go` に `DeriveBootstrapKey`
  - `base_url_hash` の入力は `space.NormalizeBaseURL` を必ず通す（M3 対応）。`space.NormalizeAlias` は存在しないため、`alias_hash` の入力は caller 側で `strings.ToLower(strings.TrimSpace(alias))` 相当の正規化 → `space.ValidateAlias` で検証する責務とする
  - `space.NormalizeBaseURL` は strict validator（path/query 付き URL を弾く）。これは仕様として受け入れる — token 発行時に壊れた URL を排除できる利点が大きい
  - 既存 ad-hoc 正規化（`TrimRight + ToLower` 等）があれば削除し、`space.NormalizeBaseURL` に統一する
- **Refactor**: エラー定数を `internal/auth/errors.go` に移動

### Step 2: `MultiSpaceOAuthHandler.HandleAuthorize` の bootstrap_token 経路
- **Red**: `multi_space_oauth_handler_test.go` に
  - `TestHandleAuthorize_BootstrapTokenMissing` → 401
  - `TestHandleAuthorize_BootstrapTokenInvalid` → 401
  - `TestHandleAuthorize_BootstrapTokenBaseURLMismatch`: token は valid だが URL query の base_url が異なる → 401
  - `TestHandleAuthorize_BootstrapTokenSuccess`: idproxy ctx 注入なしで 302 し、Backlog 認可 URL を発行
  - `TestHandleAuthorize_BootstrapTokenJTIReplay`: 同じ token を 2 回送ると 2 回目が 401
  - `TestHandleAuthorize_BootstrapTokenJTIReplay_AcrossHandlers` (**H2 反映**): 2 つの `MultiSpaceOAuthHandler` instance が同一 `NonceStore` (SQLite shared DB) を共有している場合、片方で consume すると他方からも replay が拒否される
  - `TestHandleAuthorize_HEAD_405` (**M1 反映**): HEAD request では jti が consume されず 405 を返す（HEAD で焼き切られる問題の回避）
  - `TestHandleAuthorize_BaseURLNormalization_EdgeCases` (**M3 反映**): trailing slash / 大文字小文字 / 全角コロン / IDN punycode 変換 のバリアントが正規化後に同じ token と一致する
  - `TestHandleAuthorize_DuplicateBaseURLQuery_400` (**M3 反映**): `?base_url=A&base_url=B` のように同じキーが複数回来た場合は `400 invalid_request` を返す（最初の値を採用する Go の標準挙動だと攻撃者が任意の base_url を後付けで上書きできる可能性があるため、明示的に拒否）
  - `TestHandleAuthorize_DuplicateBootstrapTokenQuery_400` (**M3 反映**): `?bootstrap_token=A&bootstrap_token=B` も同様に 400
  - `TestHandleAuthorize_ReferrerPolicy`: 成功時にも失敗時にも `Referrer-Policy: no-referrer` ヘッダが付与される
  - `TestHandleAuthorize_PathPattern`: `/oauth/backlog/multi/authorize/extra` のような prefix 攻撃が成立しないこと（ルーティング統合テストで担保）
- **Green**: handler 改修 + `NewMultiSpaceOAuthHandler` に `bootstrapKey []byte` 引数追加
  - **HEAD/OPTIONS は jti consume 前に 405 を返す** (M1 対応): method check を query parse より前に置く（既存の line 126 を維持しつつ）
- **Refactor**: helper 関数 `extractUserIDFromBootstrap(r, key, ns)` で切り出し

### Step 3: state JWT に `Flow` フィールド追加 + callback dispatcher 化（**案 B の核**）
- **Red**:
  - `internal/auth/state_test.go`
    - `TestStateClaims_FlowRoundTrip`: `GenerateStateWithSpaceInfo` で `Flow="multi"` がセットされ、`ValidateState` でラウンドトリップ
    - `TestStateClaims_FlowEmpty_BackwardCompat`: 既存 `GenerateState` / `GenerateStateWithContinue` は `Flow=""`
  - `internal/transport/http/oauth_handler_test.go` (dispatcher テスト)
    - `TestHandleCallback_Dispatch_MultiFlow`: `flow="multi"` の state を持つ callback が `MultiSpaceOAuthHandler.HandleCallback` に委譲される
    - `TestHandleCallback_Dispatch_SingleFlow`: `flow=""` は既存 single ロジックを実行
    - `TestHandleCallback_Dispatch_MultiHandlerMissing_500`: `multiSpaceHandler==nil` で `flow="multi"` は 500 (fail closed)
    - `TestHandleCallback_Dispatch_StateInvalid_ExistingError`: state 検証失敗時は dispatcher 判定前に既存エラーで返す (multi/single 漏洩防止)
  - `internal/transport/http/multi_space_oauth_handler_test.go`
    - `TestHandleCallback_DefensiveFlowCheck`: `flow != "multi"` の state を直接渡すと 400 state_invalid (防御的)
    - `TestHandleCallback_NoIdproxyContext_StillSucceeds`: idproxy ctx に uid 無くても state.UserID で完走
    - 既存テスト群を `flow="multi"` 込みの state JWT に追従させる
- **Green**:
  - `StateClaims.Flow` フィールド追加 (omitempty)
  - `GenerateStateWithSpaceInfo` 内で `Flow="multi"` を必ずセット
  - `NewOAuthHandler` シグネチャに `multiSpaceHandler *MultiSpaceOAuthHandler` (optional) を追加
  - `OAuthHandler.HandleCallback` 冒頭に dispatcher ロジックを実装
  - `MultiSpaceOAuthHandler.HandleCallback` に防御的 flow チェックと idproxy セッション切れ時のフォールバック注入を追加
- **Refactor**: dispatcher を `dispatchCallback(claims)` ヘルパーに切り出し

### Step 4: `tools_space_registry.spaceConnectURL` の bootstrap_token 付与
- **Red**: `tools_space_registry_test.go` に
  - `TestSpaceConnectURL_IncludesBootstrapToken` → URL に `bootstrap_token=...` が含まれ、parse すると base_url_hash / alias_hash / aud / iss / typ が claim に焼かれている
  - `TestSpaceConnectURL_NoContextUser_Errors` → ctx に uid が無い場合は明示エラー
  - `TestSpaceConnectURL_KeyMissing_Errors`: bootstrapKey 未設定の fail-safe（明示エラーかつ token 無付与で URL を返さない）
  - `TestSpaceConnectURL_BaseURLInvalid_Errors`: 不正な base_url を渡すと token 生成前にエラー
- **Green**: `spaceConnectURL` 改修 + `RegisterSpaceRegistryTools` シグネチャ拡張

### Step 5: state JWT 強化（`Typ` + `aud` + `iss` 必須化、backward compat） & `InstallOAuthRoutes` シグネチャ整理

state JWT 強化は H1 対応の核。`InstallOAuthRoutes` のシグネチャ整理は同じファイル群を触るためまとめて 1 Step に統合する。

**5-A: state JWT 強化（H1）**
- **Red**: `internal/auth/state_test.go` に以下を追加
  - `TestStateClaims_TypAudIss_RoundTrip`: 新規発行 token で `Typ="oauth_state_v1"` / `Audience` / `Issuer` がラウンドトリップ
  - `TestValidateState_NewToken_RejectsWrongAudience`: `Typ="oauth_state_v1"` で aud 不正 → ErrStateInvalid
  - `TestValidateState_NewToken_RejectsWrongIssuer`: `Typ="oauth_state_v1"` で iss 不正 → ErrStateInvalid
  - `TestValidateState_NewToken_RejectsUnknownTyp`: `Typ="something_else"` → ErrStateInvalid
  - `TestValidateState_OldToken_AcceptedWithoutTypAud`: `Typ=""` の旧 token は受理（backward compat）
  - `TestValidateState_FlowDefaultsToSingle`: `Flow=""` は single 扱い
  - `TestValidateState_FlowMulti_RequiresBaseURLAlias`: multi なのに BaseURL/Alias 空は ErrStateInvalid
  - `TestGenerateStateWithSpaceInfo_SetsMultiFlow`: 内部で `Flow="multi"` + `Typ="oauth_state_v1"` をセット
  - `TestGenerateStateWithContinue_SetsSingleFlow`: 内部で `Flow="single"` + `Typ="oauth_state_v1"` をセット
  - `TestGenerateState_SetsSingleFlow`: 同上
- **Green**: `internal/auth/state.go` の `StateClaims` に `Flow`/`Typ` フィールド追加、定数 `OAuthStateAudience` / `OAuthStateIssuer` / `OAuthStateTypeV1` を追加、`ValidateState` に backward compat ロジック追加
- **Refactor**: validation helper を関数化（`validateNewTokenClaims`）

**5-B: `InstallOAuthRoutes` シグネチャ整理**
- **Red**: 既存 `mcp_oauth_test.go` / `mcp_oauth_e2e_test.go` の `InstallOAuthRoutes(mux, deps.Handler, ...)` 呼出を `InstallOAuthRoutes(mux, deps.Handler)` に書き換え + multi-space ルートは登録されないことを確認するテスト追加
- **Green**: `mcp_oauth.go` の関数シグネチャ修正、内部の `if msh != nil { ... }` 削除
- **Refactor**: コメント整理

**5-C: integration tag テストの整備 (L2 反映)**
- 本 Step 末尾で `go test -tags=integration ./... -race -count=1` をローカル実行し、unit と integration の両方が green になるよう CI 設定も更新
- 新規追加した integration tag テスト一覧:
  - `internal/transport/http/multi_space_oauth_handler_integration_test.go`（Step 2 で導入、`//go:build integration`）
  - `internal/cli/mcp_oauth_integration_test.go`（Step 6 で導入、`//go:build integration`）
- 既存の lint/vet も `make ci` 等のラッパー経由で実行

### Step 6: `internal/cli/mcp.go` の routing 改修（**案 B: authorize のみ topMux 直登録**）

**L1 反映**: 修正対象範囲は `internal/cli/mcp.go:254-324`（既存記述の `:255-313` を訂正）。

- **Red**: E2E テストを `mcp_oauth_e2e_test.go` に追加
  - `TestE2E_MultiSpaceAuthorize_NoIdproxy`: idproxy セッション無しでも `/oauth/backlog/multi/authorize?base_url=...&bootstrap_token=...` が 302 する
  - `TestE2E_MultiSpaceAuthorize_MethodPost_405`: POST だと 405 (Method+Path pattern により)
  - `TestE2E_MultiSpaceAuthorize_HEAD_405` (**M1 反映**): HEAD request だと 405 を返し、jti を consume しないことを確認。理由: **Go の `GET /...` Method+Path pattern は HEAD も自動マッチする**ため、Slack unfurl / Twitter card / ブラウザ prefetch などが authorize URL を HEAD で叩くと jti が prefetch で焼き切られる可能性がある。Handler 冒頭で `r.Method == http.MethodGet` のみを許可し、HEAD/OPTIONS は jti consume 前に即時 405 を返す
  - `TestE2E_MultiSpaceAuthorize_PrefixExtra_404`: `/oauth/backlog/multi/authorize/extra` は idproxy 側の catch-all に到達（401/302/404 いずれにせよ multi handler は呼ばれない）
  - `TestE2E_SingleSpaceAuthorize_StillGatedByIdproxy`: 既存 `/oauth/backlog/authorize` は引き続き idproxy 認証必須（401 or redirect）
  - `TestE2E_BaseURLEncoding`: `base_url=https%3A%2F%2Ffoo.backlog.com` が正しく decode される
  - `TestE2E_CallbackDispatch_MultiFlow`: idproxy 認証済みブラウザで `/oauth/backlog/callback?code=...&state=<flow=multi,typ=oauth_state_v1>` を踏むと multi 側に委譲され `SpaceStore.Upsert` が走る
  - `TestE2E_CallbackDispatch_SingleFlow`: 既存 `flow=""` (旧 token) と `flow="single"` (新 token) の single callback が両方とも壊れていない
  - `TestE2E_NonceStore_SharedAcrossInstances` (**H2 反映**): 2 つの handler instance が同一 SQLite ファイルを共有しているとき、片方で authorize 完了 → もう片方に同じ bootstrap_token を投げると replay 拒否される
- **Green**: `mcp.go:254-324` を改修
  - `topMux.HandleFunc("GET /oauth/backlog/multi/authorize", oauthDeps.MultiSpaceHandler.HandleAuthorize)` を idproxy ラップ前に登録（**authorize のみ topMux**）
  - **callback は idproxy 内側のまま** (`InstallOAuthRoutes` 経由で innerMux に登録)
  - `InstallOAuthRoutes(innerMux, oauthDeps.Handler)` (multi 引数撤去、callback は dispatcher で multi に委譲)
  - `NewOAuthHandler` 呼出時に `oauthDeps.MultiSpaceHandler` を渡す（dispatcher の委譲先として）
  - **NonceStore deploy 制約警告（H2 反映）**: `--auth` モード起動時に `LOGVALET_SPACE_STORE_TYPE=memory` だった場合、`slog.Warn` で「単一プロセス前提です。複数インスタンスデプロイでは sqlite/dynamodb を使ってください」を出力する
- **Refactor**: 起動ログのルート一覧を更新（authorize のみ multi 専用 path、callback は共有 + dispatcher で振り分け）

**L2 反映 (Step 7 / Step 5 と共通)**: integration tag テストの実行を `Step 7` に明記
- `go test ./... -race -count=1`（unit）
- `go test -tags=integration ./... -race`（integration: H2 の cross-instance replay 拒否、SQLite 共有テスト等）

### Step 7: 統合テスト & 既存テスト全体 green 維持 (**L2 反映**)
- `go test ./... -race -count=1` 実行（unit）
- `go test -tags=integration ./... -race -count=1` 実行（integration tag、H2 の SQLite 共有 / cross-instance replay 拒否テストを含む）
- `go vet ./...` パス
- `golangci-lint run` パス（CI と同じ条件）
- 失敗があれば修正
- 既存テストがすべて green であることを確認（特に `oauth_handler_test.go` / `oauth_handler_continue_test.go` の旧 token backward compat 系）

---

## 10. リスク評価

| リスク | 影響 | 緩和策 |
|---|---|---|
| トークン混同攻撃 (state↔bootstrap) | bootstrap_token に state JWT が通る/逆 | (a) HKDF で用途別鍵を導出 → 署名段階で拒否、(b) `typ` claim 検証必須化（二重防御） |
| bootstrap_token リプレイ | 漏れたトークンを使い回し | `jti` を `NonceStore` で one-time consume + TTL=3分 |
| bootstrap_token 漏洩での横展開 | 別 base_url/alias への接続開始 | `base_url_hash` / `alias_hash` を claim に束縛し authorize で一致検証 |
| URL query 経由のトークン漏洩 | access log / Referer / 履歴 | (a) authorize レスポンスに `Referrer-Policy: no-referrer` を付与、(b) Backlog への 302 location には bootstrap_token を含めない（authorize ハンドラ内で消費して終わる）、(c) サーバー側 access_log で `bootstrap_token` を redact する設定例を docs に追加 |
| MCP クライアント側ログに残る | Claude 等の会話履歴 | one-time + 短 TTL + base_url 束縛 で実質悪用不可。docs で受容リスクとして明記 |
| alg=none / 別 alg | 署名スキップ攻撃 | jwt/v5 標準パターン: 検証コールバック内で `*SigningMethodHMAC` を厳密チェック（既存 `ValidateState` と同じパターン） |
| Go ServeMux 仕様への依存 | Go バージョン差 | Method+Path pattern は Go 1.22+ で stable。go.mod で 1.22+ を要求していることを確認、または 1.21 以下なら明示的 mux ライブラリで代替 |
| **dispatcher 経由でも flow 偽装される** (案 B 固有) | `flow="multi"` 偽装で multi handler を呼ぶ攻撃 | state JWT は HS256 署名付き → flow claim は改竄不可。MultiSpaceOAuthHandler 側で防御的に `claims.Flow != "multi"` を再チェック → 二重防御 |
| **multi callback で idproxy ログイン要求** (案 B 固有) | UX 悪化（再ログイン） | 通常運用では authorize 完走時点で idproxy セッション有効。万一切れていても `claims.UserID` で完走可能（§8.2 のフォールバック） |
| **dispatcher 実装漏れで single にフォールバック** (案 B 固有) | multi の token が single ロジックで処理されて壊れる | fail closed: `multiSpaceHandler==nil` で `flow="multi"` を受けたら 500 (§8.3) |
| `oauth_handler_test.go` の既存テストが state.Flow="" の前提で多数 | regression | Step 3 で全テストを `flow="multi"` / `flow=""` 両方で網羅 |
| **HEAD/preflight で jti が焼き切られる** (M1: devils-advocate 反映) | プリロード機構やセキュリティスキャンが authorize URL を HEAD で叩くと jti が consume されて以降使えなくなる | method check を query parse より前に置き HEAD/OPTIONS は 405 で即時返す（Step 2/Step 6 のテストで担保） |
| **base_url/alias normalization の漏れで bootstrap_token が一致しない** (M3) | 正常 URL でも 401 になる UX 障害 | `space.NormalizeBaseURL` を生成/検証両方で必ず通す。alias は caller 側で `strings.ToLower(strings.TrimSpace(...))` → `space.ValidateAlias` で検証（NormalizeAlias は存在しないため）。Step 1/2 で edge case テスト追加 |
| **`MemoryStore` deploy 制約** (H2 必須) | `MemoryStore` は in-process 専用。**multi-instance deploy（Lambda Multi-AZ / 複数 Pod / ALB+EC2）では `DynamoDBStore` 必須**。MemoryStore のままだと jti / state nonce のリプレイが成立する。Store ノードと Consume ノードがすれ違えば token は常に拒否され UX 障害にもなる | (a) docs/specs/multi_space_oauth.md に deploy マトリクス明記 (Memory=単一プロセス専用, SQLite=単一ノード永続化, DynamoDB=multi-instance 必須)、(b) `--auth` 起動時に `LOGVALET_SPACE_STORE_TYPE=memory` なら `slog.Warn` 出力、(c) Step 6 E2E に `TestE2E_NonceStore_SharedAcrossInstances`（2 instance 共有 store でリプレイ拒否）、(d) Step 2 ユニットに `TestHandleAuthorize_BootstrapTokenJTIReplay_AcrossHandlers`、(e) Step 7 で `go test -tags=integration ./...` 実行を必須化 |
| **state JWT 単発検証のみで callback 認証境界が薄い** (H1) | aud/iss/typ が無いと別用途 JWT が誤認識される可能性 | M18 スコープに state JWT 強化を取込（§5 「state JWT への強化」、Step 5-A で TDD） |
| 既存テストの routing 想定崩壊 | テスト失敗 | ステップ毎にテスト実行、E2E で旧フロー確認 |
| `InstallOAuthRoutes` API 変更が外部ユーザに影響 | 後方互換 | logvalet は単一バイナリで `cmd/logvalet` からしか呼ばれないため影響なし。internal package 扱い |
| bootstrapKey 不在で連携失敗 | fail-safe | `spaceConnectURL` で key 未設定なら明示エラー JSON を返す |
| OAuth redirect_uri の不一致 | Backlog 側エラー | §10.x で詳細確認手順を定義 |

### redirect_uri 設定（**確定済み — 2026-05-20**）
- team-lead 確認結果: **Backlog OAuth app は redirect_uri を単一しか登録できない**
- 採用: **案 B (callback 単一化)** → callback path は `/oauth/backlog/callback` のまま維持
- 振り分け: state JWT の `flow` フラグで callback dispatcher が multi/single に分岐
- 詳細は §5「案 B: callback 単一化」を参照
- 結果:
  - `LOGVALET_MCP_BACKLOG_REDIRECT_URL` は変更不要（既存値をそのまま使用）
  - Backlog OAuth app 側の設定変更不要
  - 既存 single-space フローの挙動は完全維持（`flow=""` で既存パスへ）

### NonceStore deploy 制約（H2: devils-advocate 反映、**運用ドキュメント必須**）

bootstrap_token の `jti` one-time use は `space.NonceStore` の `Store`/`Consume` を流用するため、デプロイ形態によって以下の制約が発生する。

| デプロイ形態 | 採用可能な NonceStore | 制約 |
|---|---|---|
| 単一プロセス (logvalet mcp ローカル / 単一 EC2) | `MemoryNonceStore`（プロセス内 map）| 再起動で nonce 全消失 → 起動直後のリプレイは TTL で潰せるが、別プロセスへフェイルオーバーは不可 |
| 複数プロセス共有 (ALB + 複数 EC2 / Lambda マルチコンテナ) | `DynamoDBNonceStore` (= `DynamoDBStore` の `NonceStore` interface 実装) **必須** | Conditional Put で atomic な consume を保証 |
| 単一ノード永続化 (EC2 単独 + 再起動跨ぎ) | `SQLiteNonceStore` (= `SQLiteStore`) **必須** | 共有ボリュームではないため複数ノード不可 |

**運用ドキュメント追記**:
- `docs/specs/multi_space_oauth.md`（新規）に上記マトリクスを記載
- 環境変数 `LOGVALET_SPACE_STORE_TYPE` (`memory|sqlite|dynamodb`) の選択指針を明記
- マルチインスタンスデプロイで `MemoryNonceStore` を選ぶと「ノード A で Store → ノード B で Consume」がすれ違って bootstrap_token がブロックされるリスクがあることを警告

**起動時バリデーション**:
- `internal/cli/mcp.go` の `McpCmd.Validate` で C1 制約 (`ValidateSpaceStoreConfig`) は既に存在
- 本 milestone では multi-instance 想定で `--auth` モード時に `LOGVALET_SPACE_STORE_TYPE != "memory"` を **strict check** することを Step 6 のサブステップに含める（後方互換のため warning から開始し、M19 で error 化を検討）

### 既存 state JWT への防御強化（**M18 スコープに取込済 — H1 対応**）
- 当初は後続 milestone 送りとしていたが、devils-advocate H1 指摘で M18 必須化
- 本 milestone で実施する内容（§5 / Step 5-A 参照）:
  - `aud="logvalet/oauth-callback"`
  - `iss="logvalet"`
  - `typ="oauth_state_v1"`
  - `flow="single"` / `flow="multi"`
- 既存 token (`typ=""`) は state TTL=10 分以内に自然消滅するため、検証側は typ 空を一時的に受理する backward compat を実装
- 残課題（M19 候補）:
  - 検証側の typ 空受理パスを撤去（デプロイ完全反映後）
  - state JWT 専用鍵への HKDF 分離（現状は raw stateSecret を共用）

---

## 11. 既存挙動の保証 (regression guard)

| シナリオ | 期待挙動 | テスト |
|---|---|---|
| single-space `/oauth/backlog/authorize` | idproxy 認証あり → state JWT 発行 → Backlog 302 | `oauth_handler_test.go` (既存) |
| single-space `/oauth/backlog/callback` (flow="") | idproxy 認証あり → dispatcher が single ロジックへ → token 保存 → JSON / continue redirect | `oauth_handler_continue_test.go` (既存) + Step3 dispatcher テスト |
| single-space `/oauth/backlog/status` | idproxy 認証あり → JSON | 既存 |
| MCP `/mcp` エンドポイント | idproxy 認証あり → MCP JSON-RPC | 既存 |
| `BacklogAuthorizeGate` 経由の `/authorize` redirect | 未接続なら `/oauth/backlog/authorize?continue=...` に 302 | `mcp_oauth_e2e_test.go` (既存) |
| `EnsureBacklogConnected` ブラウザ自動 redirect | 未接続なら authorize へ 302 | `mcp_auto_redirect_test.go` (既存) |
| **新規** multi-space `/oauth/backlog/multi/authorize?bootstrap_token=...` | idproxy 不要、bootstrap_token 検証 → state JWT (flow="multi") 発行 → 302 | **新規 E2E テスト** |
| **新規** multi-space `/oauth/backlog/callback` (flow="multi") | idproxy 認証経由で到達 → dispatcher が multi に委譲 → token 保存 + SpaceStore.Upsert | **新規 E2E テスト `TestE2E_CallbackDispatch_MultiFlow`** |
| **挙動緩和 (案 B 固有)** multi-space callback で idproxy セッション無し | 200 成功（state.UserID で完走） | `TestHandleCallback_NoIdproxyContext_StillSucceeds`。**既存 single-space callback の挙動 (401) は変更なし** — multi flow のみ緩和。state JWT 署名検証で安全性担保 |

---

## 12. 外部レビュー結果との照合

### Copilot GPT (gpt-5.5) レビュー結果 — 2026-05-20 取得済み

| # | GPT 指摘 | 採否 | 反映先 |
|---|---|---|---|
| G1 | TTL=10分 → 1〜3 分に短縮 | ✅ 採用 | §4.3 `DefaultBootstrapTokenTTL = 3 * time.Minute` |
| G2 | `jti` で one-time use | ✅ 採用 | §4.4 リプレイ対策、既存 NonceStore 流用 |
| G3 | `aud` / `iss` を必須化 | ✅ 採用 | §4.3 claim 定義、`BootstrapTokenAudience` / `BootstrapTokenIssuer` |
| G4 | `base_url_hash` / `alias_hash` を claim に束縛 | ✅ 採用 | §4.3 / §7 検証ロジックで一致確認 |
| G5 | stateSecret 共用は弱い → HKDF 用途別鍵導出 | ✅ 採用 | §4.2, §4.6 `DeriveBootstrapKey` |
| G6 | URL query 載せは漏洩面が広い → Referrer-Policy / 命名変更 | ✅ 採用 | §4.2 `bootstrap_token` 命名 / §7 ヘッダ付与 |
| G7 | Go 1.22+ Method+Path pattern を使う | ✅ 採用 | §3 `GET /oauth/backlog/multi/authorize` |
| G8 | callback 認証境界が state JWT 単体 → state にも aud/iss/typ 必要 | △ 後続 milestone | §10 末尾「後続 milestone」/ §14 follow-up |
| G9 | alg=none / 別 alg / 異常入力テストを増やす | ✅ 採用 | §9 Step1 に追加テストケース |
| G10 | redirect_uri は single/multi 両方サポートを Backlog 側で確認すべき | ✅ 確認済 (2026-05-20) | Backlog は redirect_uri 単一登録のみ対応 → **案 B (callback 単一化 + state.flow 分岐)** を採用。§5 / §8 / §10 を案 B 用に書き換え |
| G11 | `user_token` 命名を `bootstrap_token` 等 bearer credential らしい名前へ | ✅ 採用 | §4.2 で命名変更 |
| G12 | server-side code 方式（短命 code → store 引き）への変更案 | ❌ 不採用 | 理由: 本 milestone のスコープ拡大が大きい。bootstrap_token 強化策（jti one-time + base_url 束縛 + 短 TTL + HKDF + Referrer-Policy）で実質的に同等のセキュリティ姿勢を達成済み。code 方式は将来 M20 で検討 |
| G13 | trailing slash / prefix 攻撃テスト | ✅ 採用 | §9 Step6 `TestE2E_MultiSpaceAuthorize_PrefixExtra_404` |

### advisor (Claude) からの指摘 (取り込み済み)
1. ✅ 別パス分離の真の理由を「単一パスでの idproxy ラップ内外混在は不可能」と明記 → §3 / §5 に記載
2. ✅ bootstrap_token に `typ` claim を導入してトークン混同を防止 → §4.2 / §4.4 に記載
3. ✅ bootstrap_token 検証後は `ContextWithUserID` で既存ハンドラを最小変更 → §7 / §8 に記載
4. ✅ `InstallOAuthRoutes` から `msh` 引数を削除する旨を明記 → §5 / §9-Step5 に記載
5. ✅ ServeMux の longest-prefix 動作を明記 → §3 に記載（Method+Path pattern にアップグレード）
6. ✅ `QueryUnescape` 防御は defense-in-depth として残す → §5 に記載
7. ✅ `EnsureBacklogConnected` / `BacklogAuthorizeGate` が影響を受けない理由を明記 → §5 に記載

### advisor (Claude) 最終レビューで追加された 3 つのブロッカー指摘 (取り込み済み)
- **B1**: `jti` の `Store` フェーズが未定義 → §6 「one-time use の運用契約」と擬似コードで明示
- **B2**: `BuildOAuthDeps` で multi-space redirect_uri を分離する必要 → §5 「redirect_uri 分離設計」に追記
- **B3**: `golang.org/x/crypto/hkdf` 依存追加 → §5 ファイル一覧に追加

### devils-advocate レビュー結果 — 2026-05-20 取得済み (取り込み済み)

| ID | 指摘 | 重要度 | 採否 | 反映先 |
|---|---|---|---|---|
| H1 | state JWT 強化 (`typ`/`aud`/`iss`/`flow`) を M18 スコープに取込 | High | ✅ 採用 | §5 「state JWT への強化」 / Step 5-A |
| H2 | NonceStore deploy 制約 + cross-instance replay 拒否 E2E | High | ✅ 採用 | §10 リスク表「`MemoryStore` deploy 制約」 / §10 「NonceStore deploy 制約」セクション / Step 2 / Step 6 |
| H3 | redirect_uri は案 B (callback 単一化) で確定 | High | ✅ 採用 (確定済) | §5 / §10 redirect_uri 設定 |
| M1 | HEAD request で jti が焼き切られる | Medium | ✅ 採用 | Step 2 `TestHandleAuthorize_HEAD_405` / Step 6 `TestE2E_MultiSpaceAuthorize_HEAD_405` |
| M3 | base_url/alias normalization edge case | Medium | ✅ 採用 | Step 1 / Step 2 で normalization + duplicate query 拒否 + IDN テスト |
| L1 | mcp.go 行番号修正 (254-324) | Low | ✅ 採用 | §1 line 24 / §5 / §9-Step6 で訂正 |
| L2 | `go test -tags=integration` 明記 | Low | ✅ 採用 | §9-Step5 5-C / §9-Step7 / §13 で明記 |

### devils-advocate 最終ゲート再批評で追加された修正 (2026-05-20 取込) ✅

| ID | 指摘 | 反映先 |
|---|---|---|
| H2 final | §10 リスク表本体に NonceStore deploy 制約行を入れること（前回はサブセクション止まりだった） | §10 リスク表に「`MemoryStore` deploy 制約」行を新設、文言は指示通り |
| M1 final | `TestE2E_MultiSpaceAuthorize_HEAD_405` を正式名称化、Go `GET /...` pattern が HEAD を受理する事実を理由として明記 | §9 Step 6 |
| M2 | §4.4 の「鍵漏洩耐性」→「用途分離 (key separation)」、stateSecret 漏洩時の rotation + nonce flush 運用手順を追記 | §4.4 |
| M3 final | duplicate query / IDN のテストを追加 | Step 1 `TestGenerateBootstrapToken_BaseURLIDN` / Step 2 `TestHandleAuthorize_DuplicateBaseURLQuery_400` ほか |
| M4 | §6 に「Store 後の Generate 失敗は TTL で自然消失を許容」と明記 | §6 「one-time use の運用契約」 |
| L1 final | §1 line 24 の `cmd/logvalet/mcp.go:255-312` を `internal/cli/mcp.go:254-324` に修正、Step 6 タイトルも訂正 | §1 / §9 Step 6 タイトル |
| L2 final | Step 5-C を新設し integration tag テストの実行を明記 | §9 Step 5-C |
| L3 | §4.3 の hash 16 hex 切り詰めに「URL 長 2KB 制約内 + HMAC 署名で改竄不可なので 64bit 衝突耐性で十分」と理由付け追記 | §4.3 |
| H3 final | Step 0 として「Backlog OAuth app の redirect_uri 設定確認」gating を明示。案 B 確定済みのチェックボックス付き | §9 Step 0 |
| M5 | §8.2 の `ctxUserID != claims.UserID` dead check を削除。新しい認証境界（state JWT 強化 + nonce one-time）を明示 | §8.2 / §8.3 |

---

## 13. 完了確認手順

1. `go test ./... -race -count=1` が全 green
2. `go test -tags=integration ./... -race -count=1` が全 green（**L2 反映**）
3. `go vet ./...` パス、`golangci-lint run` パス
4. ローカル起動:
   ```
   LOGVALET_MCP_OAUTH_STATE_SECRET=<64hex> \
   LOGVALET_MCP_BACKLOG_CLIENT_ID=... \
   LOGVALET_SPACE_STORE_TYPE=sqlite \    # H2 反映: multi-instance を意識した値
   ... \
   logvalet mcp --auth --external-url https://...
   ```
5. Claude 等の MCP クライアントから `logvalet_space_connect_url` 呼出 → URL 取得（`bootstrap_token=...&base_url=...` を含む）
6. ブラウザで URL を開く → 直接 Backlog 認可画面に遷移すること (idproxy ログイン画面を経由しない)
7. 認可完了 → `/oauth/backlog/callback` (共有パス) に到達 → dispatcher が `flow="multi"` を検出 → `MultiSpaceOAuthHandler.HandleCallback` で処理 → `{"status":"connected"}` JSON
8. `logvalet_space_list` で `megumilog.backlog.jp` が `status=ok` で見えること
9. 既存 single-space フロー（`logvalet_my_week` 等）も引き続き動作すること
10. 同じ bootstrap_token URL を 2 回開くと 2 回目が 401 (jti replay 拒否) になること

### 前提環境
- Go 1.26.1（go.mod で確認済み） → Method+Path pattern (`GET /...`) を問題なく使用可能

---

## 14. Follow-up（後続 milestone への送り）

本 M18 完了後に検討する課題:

1. **M19 候補: state JWT 強化の最終化**
   - 本 M18 で `Typ="oauth_state_v1"` を導入したが backward compat のため typ 空を一時受理している
   - M19 では typ 空受理経路を撤去し、必須化を完全反映する（デプロイ後 24h 経過してから安全に実施可能）
   - state JWT 専用鍵への HKDF 分離（現状は raw stateSecret を共用）
   - GPT 指摘 G8 / devils-advocate H1 の最終クリーンアップ

2. **M20 候補: bootstrap token を server-side code 方式へ移行**
   - 短命 code → server-side store (DynamoDB) で userID/base_url/alias を引く方式
   - URL query 漏洩リスクをさらに減らす
   - GPT 指摘 G12 への対応

3. **アクセスログ redact のドキュメント化**
   - ALB / CloudFront / nginx などのフロント設定で `bootstrap_token` / `state` / `code` を redact する設定例
   - docs/specs/multi_space_oauth.md に追記

4. **stateSecret の HKDF 完全移行**
   - state JWT 側も HKDF 派生鍵に移行（M19 と組み合わせ）

5. **callback パス分離の再検討（Backlog OAuth app 仕様変更時）**
   - 現状 Backlog は redirect_uri 単一登録のみ → 案 B (callback 単一化 + state.flow 分岐) で対応
   - 将来 Backlog が複数 redirect_uri をサポートした場合、`/oauth/backlog/multi/callback` に分離して dispatcher を撤去できる
   - 移行コスト極小（state.Flow フィールドは残しても無害）

6. **docs/specs/multi_space_oauth.md の整備**
   - 案 B 採用の経緯と state.flow 分岐の挙動を運用ドキュメントとして残す
   - access_log redact 設定と組み合わせて公開

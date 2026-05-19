# MS03: BaseURL / Alias / Tenant 正規化

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS01

## 目的

スペース登録時の入力（BaseURL・Alias・Tenant）を安全に正規化する関数を実装する。
特にカスタムドメインの tenant 導出問題（OAuth 鶏卵問題: GPT-5.4 指摘）に対処する。

## 完了条件

- [ ] `internal/space/normalize.go` — 正規化関数群
- [ ] `internal/space/normalize_test.go` — 全テストケース pass
- [ ] `go test ./internal/space/...` パス

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

### normalize_test.go

#### NormalizeBaseURL

```
T1: TestNormalizeBaseURL_NoScheme
    - "foo.backlog.com" → "https://foo.backlog.com"

T2: TestNormalizeBaseURL_TrailingSlash
    - "https://foo.backlog.com/" → "https://foo.backlog.com"
    - "https://foo.backlog.com///" → "https://foo.backlog.com"

T3: TestNormalizeBaseURL_AlreadyNormalized
    - "https://foo.backlog.com" → "https://foo.backlog.com"（変化なし）

T4: TestNormalizeBaseURL_BacklogJP
    - "https://foo.backlog.jp" → "https://foo.backlog.jp"
    - "foo.backlog.jp" → "https://foo.backlog.jp"

T5: TestNormalizeBaseURL_PathRejected
    - "https://foo.backlog.com/api/v2" → error（path 付きは拒否）

T6: TestNormalizeBaseURL_QueryRejected
    - "https://foo.backlog.com?x=1" → error（query 付きは拒否）

T7: TestNormalizeBaseURL_FragmentRejected
    - "https://foo.backlog.com#section" → error（fragment 付きは拒否）

T8: TestNormalizeBaseURL_EmptyRejected
    - "" → error

T9: TestNormalizeBaseURL_CustomDomain
    - "https://backlog.example.com" → "https://backlog.example.com"（カスタムドメインも通る）
```

#### DeriveAliasFromBaseURL

```
T10: TestDeriveAliasFromBaseURL_BacklogCom
    - "https://foo.backlog.com" → "foo"

T11: TestDeriveAliasFromBaseURL_BacklogJP
    - "https://foo.backlog.jp" → "foo"

T12: TestDeriveAliasFromBaseURL_Uppercase
    - "https://FOO.backlog.com" → "foo"（小文字化）

T13: TestDeriveAliasFromBaseURL_CustomDomain
    - "https://backlog.example.com" → ""（カスタムドメインは alias 自動生成不可）
    - 返り値が "" のとき呼び出し側はユーザーに alias 入力を求める

T14: TestDeriveAliasFromBaseURL_HyphenInName
    - "https://foo-bar.backlog.com" → "foo-bar"
```

#### DeriveInitialTenant

```
T15: TestDeriveInitialTenant_BacklogCom
    - "https://foo.backlog.com" → "foo", nil

T16: TestDeriveInitialTenant_BacklogJP
    - "https://foo.backlog.jp" → "foo", nil

T17: TestDeriveInitialTenant_CustomDomain
    - "https://backlog.example.com" → "", nil
    - （空文字は「GetSpace() で確認が必要」の意味。エラーではない）

T18: TestDeriveInitialTenant_CaseSensitivity
    - "https://FOO.backlog.com" → "foo"（小文字化）
```

#### ValidateAlias

```
T19: TestValidateAlias_Valid
    - "foo" → nil
    - "foo-bar" → nil
    - "foo_bar" → nil
    - "foo.bar" → nil
    - "foo123" → nil

T20: TestValidateAlias_Empty
    - "" → error

T21: TestValidateAlias_InvalidChars
    - "foo bar" → error（スペース）
    - "foo/bar" → error（スラッシュ）
    - "foo@bar" → error（@ 記号）

T22: TestValidateAlias_TooLong
    - 64文字超の alias → error

T23: TestValidateAlias_StartsWithHyphen
    - "-foo" → error（org ブランチ命名規則と合わせてハイフン先頭は禁止）
```

---

## 2. ファイル一覧

### 新規作成

| ファイル | 内容 |
|---------|------|
| `internal/space/normalize.go` | NormalizeBaseURL, DeriveAliasFromBaseURL, DeriveInitialTenant, ValidateAlias |
| `internal/space/normalize_test.go` | T1-T23 |

---

## 3. 実装

### normalize.go

```go
package space

import (
    "fmt"
    "net/url"
    "strings"
)

const maxAliasLength = 64

// NormalizeBaseURL は入力 URL を "https://host" 形式に正規化する。
// scheme なし → https を付与、末尾スラッシュ除去、path/query/fragment は拒否。
func NormalizeBaseURL(raw string) (string, error) {
    if raw == "" {
        return "", fmt.Errorf("space: base URL must not be empty")
    }
    // scheme なしなら https を補完
    if !strings.Contains(raw, "://") {
        raw = "https://" + raw
    }
    u, err := url.Parse(raw)
    if err != nil {
        return "", fmt.Errorf("space: invalid base URL %q: %w", raw, err)
    }
    if u.Path != "" && u.Path != "/" {
        return "", fmt.Errorf("space: base URL must not have a path: %q", raw)
    }
    if u.RawQuery != "" {
        return "", fmt.Errorf("space: base URL must not have query parameters: %q", raw)
    }
    if u.Fragment != "" {
        return "", fmt.Errorf("space: base URL must not have a fragment: %q", raw)
    }
    return strings.TrimRight(u.Scheme+"://"+u.Host, "/"), nil
}

// DeriveAliasFromBaseURL は BaseURL からデフォルト alias を導出する。
// *.backlog.com / *.backlog.jp のサブドメイン第一ラベルを返す。
// カスタムドメインの場合は "" を返す（呼び出し側がユーザーに入力を求めること）。
func DeriveAliasFromBaseURL(baseURL string) (string, error) {
    u, err := url.Parse(baseURL)
    if err != nil {
        return "", err
    }
    host := strings.ToLower(u.Hostname())
    if strings.HasSuffix(host, ".backlog.com") || strings.HasSuffix(host, ".backlog.jp") {
        parts := strings.SplitN(host, ".", 2)
        return parts[0], nil
    }
    // カスタムドメイン: 自動生成不可
    return "", nil
}

// DeriveInitialTenant は BaseURL から暫定 tenant を導出する。
// *.backlog.com / *.backlog.jp → サブドメイン第一ラベル（小文字）。
// カスタムドメイン → "" を返す（GetSpace() で spaceKey を取得してから設定すること）。
func DeriveInitialTenant(baseURL string) (string, error) {
    return DeriveAliasFromBaseURL(baseURL) // 同一ロジック
}

// ValidateAlias は alias が許可文字・長さ制限を満たすかを検証する。
func ValidateAlias(alias string) error {
    if alias == "" {
        return fmt.Errorf("space: alias must not be empty")
    }
    if len(alias) > maxAliasLength {
        return fmt.Errorf("space: alias must not exceed %d characters", maxAliasLength)
    }
    if strings.HasPrefix(alias, "-") {
        return fmt.Errorf("space: alias must not start with a hyphen")
    }
    for _, r := range alias {
        if !isAliasChar(r) {
            return fmt.Errorf("space: alias contains invalid character %q", r)
        }
    }
    return nil
}

func isAliasChar(r rune) bool {
    return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
        (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.'
}
```

---

## 4. 実装手順（TDD サイクル）

### Step 1: Red

1. `internal/space/normalize_test.go` を作成（T1-T23）
2. `go test ./internal/space/...` → コンパイルエラー

### Step 2: Green

1. `internal/space/normalize.go` を実装
2. `go test ./internal/space/...` → 全テストパス

### Step 3: Refactor

- `DeriveAliasFromBaseURL` と `DeriveInitialTenant` が同一ロジックであることを文書化
- カスタムドメインの鶏卵問題の説明コメントを追加

---

## 5. 実装の要点

### カスタムドメインの取り扱い（GPT-5.4 指摘対応）

```text
カスタムドメインで DeriveInitialTenant が "" を返す場合の登録フロー:
  lv spaces add / connect → tenant が "" なら
  → ユーザーに "Your Backlog space key (e.g. 'my-org'):" を求める（CLI 側の責務）
  → または API key mode で GetSpace を呼んで spaceKey を自動取得

alias が "" を返す場合:
  → ユーザーに alias を入力させる（必須入力）
```

---

## 6. 検証コマンド

```bash
go test ./internal/space/... -v -run TestNormalize
go test ./internal/space/... -v -run TestDeriveAlias
go test ./internal/space/... -v -run TestValidateAlias
go build ./...
go vet ./...
```

---

## 7. 次のマイルストーン

MS02 + MS03 完了後 → MS04（Space Resolver）が着手可能。

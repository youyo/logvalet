# MS11: CLI --spaces / --all-spaces グローバルフラグ

Roadmap: plans/logvalet-multi-space-roadmap.md
依存: MS04

## 目的

全コマンドで使える `--spaces` / `--all-spaces` フラグを追加する。
既存の `--space`（単一スペース指定）は後方互換で残す（literal space name のまま、alias ベースに変換しない）。

## 完了条件

- [ ] `internal/cli/global_flags.go` に Spaces/AllSpaces フィールド追加
- [ ] `internal/cli/global_flags_test.go` に新テスト追加
- [ ] `parseSpacesFlag` 関数実装（H2 対応: comma-separated パース）
- [ ] `--spaces + --all-spaces` 同時指定で ErrInvalidSpaceScope
- [ ] `go test ./internal/cli/...` パス

---

## 1. TDD テストケース一覧（Red フェーズで先に書く）

### global_flags_test.go への追加

```
T1: TestParseSpacesFlag_SingleAlias
    - parseSpacesFlag("foo") → ["foo"], nil

T2: TestParseSpacesFlag_MultipleAliases
    - parseSpacesFlag("foo,bar,baz") → ["foo","bar","baz"], nil

T3: TestParseSpacesFlag_EmptyString
    - parseSpacesFlag("") → nil, nil（指定なし扱い）

T4: TestParseSpacesFlag_EmptyElement
    - parseSpacesFlag("foo,,bar") → error（空要素は拒否）
    - parseSpacesFlag(",") → error
    - parseSpacesFlag("foo,") → error

T5: TestParseSpacesFlag_Whitespace_Trimmed
    - parseSpacesFlag("foo, bar") → ["foo","bar"]（前後の空白は trim）

T6: TestParseSpacesFlag_Deduplication
    - parseSpacesFlag("foo,foo,bar") → ["foo","bar"]（重複は静かにスキップ）

T7: TestGlobalFlags_Validate_SpacesAndAllSpaces_Conflict
    - GlobalFlags{Spaces: "foo", AllSpaces: true}.Validate() → error
    - エラーメッセージに "--spaces and --all-spaces" が含まれる

T8: TestGlobalFlags_Validate_SpacesOnly_OK
    - GlobalFlags{Spaces: "foo"}.Validate() → nil

T9: TestGlobalFlags_Validate_AllSpacesOnly_OK
    - GlobalFlags{AllSpaces: true}.Validate() → nil

T10: TestGlobalFlags_Validate_SpaceAndSpaces_Independent
    - GlobalFlags{Space: "foo", Spaces: "bar"}.Validate() → nil
    （--space と --spaces の同時指定は許可。意味: --spaces "bar" が優先される）

T11: TestGlobalFlags_Kong_DoubleSpaces_Warning
    （note: Kong の string 型フラグは2回渡すと後勝ちになる動作を
      help テキストで明記する仕様テスト。実際の動作確認は CLI E2E で行う）
```

---

## 2. ファイル一覧

### 更新

| ファイル | 内容 |
|---------|------|
| `internal/cli/global_flags.go` | Spaces/AllSpaces フィールド追加、parseSpacesFlag 実装 |
| `internal/cli/global_flags_test.go` | T1-T11 追加 |

---

## 3. 実装

### global_flags.go への追加

```go
// GlobalFlags への追加フィールド
type GlobalFlags struct {
    // ... 既存フィールド ...
    
    // Space は既存の単一スペース指定（literal space name）。後方互換で残す。
    // --spaces / --all-spaces との組み合わせ: --spaces が指定された場合は Spaces が優先。
    Space string `short:"s" help:"specify Backlog space name directly (env: LOGVALET_SPACE)" env:"LOGVALET_SPACE"`

    // Spaces は comma-separated なスペース alias 指定（新規）。
    // 複数指定: --spaces foo,bar
    // 注意: --spaces foo --spaces bar のように2回渡すと後者で上書きされます。
    //       複数スペースは --spaces foo,bar のように comma-separated で指定してください。
    Spaces    string `help:"comma-separated space aliases for multi-space operations (env: LOGVALET_SPACES)" env:"LOGVALET_SPACES"`
    
    // AllSpaces はユーザーの登録済み全スペースを対象にする。
    AllSpaces bool   `help:"run against all spaces registered for the current user (env: LOGVALET_ALL_SPACES)" env:"LOGVALET_ALL_SPACES"`
}

// Validate は GlobalFlags のバリデーション。
func (g *GlobalFlags) Validate() error {
    if g.APIKey != "" && g.AccessToken != "" {
        return fmt.Errorf("--api-key and --access-token are mutually exclusive")
    }
    if g.Spaces != "" && g.AllSpaces {
        return fmt.Errorf("--spaces and --all-spaces are mutually exclusive")
    }
    return nil
}

// parseSpacesFlag は "--spaces foo,bar" を []string{"foo","bar"} に変換する（H2 対応）。
// 空文字列は nil を返す（指定なし扱い）。
// 空要素（"foo,,bar" 等）は error。重複は静かにスキップ。
func parseSpacesFlag(s string) ([]string, error) {
    if s == "" {
        return nil, nil
    }
    parts := strings.Split(s, ",")
    seen := make(map[string]bool, len(parts))
    result := make([]string, 0, len(parts))
    for _, p := range parts {
        p = strings.TrimSpace(p)
        if p == "" {
            return nil, fmt.Errorf("--spaces: empty alias in %q (use comma-separated list like 'foo,bar')", s)
        }
        if seen[p] {
            continue
        }
        seen[p] = true
        result = append(result, p)
    }
    return result, nil
}

// buildSpaceScope は GlobalFlags から space.Scope を構築する。
// --spaces / --all-spaces が指定された場合のみ multi-space モードになる。
func buildSpaceScope(g *GlobalFlags) (*space.Scope, error) {
    if g.Spaces == "" && !g.AllSpaces {
        return nil, nil // multi-space モードでない
    }
    aliases, err := parseSpacesFlag(g.Spaces)
    if err != nil {
        return nil, err
    }
    return &space.Scope{
        Aliases:   aliases,
        AllSpaces: g.AllSpaces,
    }, nil
}
```

---

## 4. 後方互換性

```text
--space foo（既存フラグ）:
  → buildRunContext の既存パスを完全保持
  → https://<space>.backlog.com として literal 解釈
  → alias ベースの SpaceResolver は使わない

--spaces foo（新規フラグ）:
  → SpaceResolver 経由（registry から alias "foo" を解決）
  → buildRunContext を拡張して space.Scope を渡す

両方指定時（--space foo --spaces bar）:
  → --spaces が優先される（buildSpaceScope が non-nil を返す）
  → --space は無視
```

---

## 5. 実装手順（TDD サイクル）

### Step 1: Red

1. `internal/cli/global_flags_test.go` に T1-T11 を追加
2. `go test ./internal/cli/...` → コンパイルエラー（parseSpacesFlag 未定義）

### Step 2: Green

1. `internal/cli/global_flags.go` を更新
2. `go test ./internal/cli/...` → 全テストパス

### Step 3: Refactor

- `buildSpaceScope` のコメントで `--space` との優先順位を明記

---

## 6. 検証コマンド

```bash
go test ./internal/cli/... -v -run TestParseSpacesFlag
go test ./internal/cli/... -v -run TestGlobalFlags
go build ./...
go vet ./...
```

---

## 7. 次のマイルストーン

MS11 完了後:
- MS08 + MS10 + MS11 完了後 → MS12（lv spaces 管理コマンド）が着手可能

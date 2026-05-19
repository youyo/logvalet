# logvalet multi-space spec 批評レポート

devils-advocate による脆弱性・矛盾・リスク分析

---

## サマリー

| 深刻度    | 件数 |
|-----------|------|
| CRITICAL  | 4    |
| HIGH      | 6    |
| MEDIUM    | 5    |
| LOW       | 3    |
| **合計**  | **18** |

---

## CRITICAL（実装前に必ず解決すべき）

### C1: SQLite local mode の userID="local" 固定によるデータ漏洩リスク

**問題:**  
§4.5 でローカル CLI の user_id を `"local"` 固定にしている。remote MCP の DynamoDB Store をポイントしたまま誰かが誤って SQLite Store を使った場合、全ユーザーのスペース登録が `user_id="local"` に格納される。これは remote MCP の multi-tenant userID 分離（§9.4）を完全に破壊する。

**発生シナリオ:**  
- デプロイ設定ミス（`LOGVALET_SPACE_STORE_TYPE=sqlite` を remote MCP に渡してしまう）
- テスト環境のデフォルト設定が local になっている状態で誤って本番相当環境に流用

**影響:**  
ユーザー A の space が `user_id="local"` で保存されていると、ユーザー B の all_spaces が A の spaces を返す。認証情報の横断漏洩に発展する可能性がある。

**推奨対処:**  
remote MCP モードでは SQLite Store の使用を起動時に validation で拒否する。`LOGVALET_SPACE_STORE_TYPE=sqlite` + `LOGVALET_MCP_MODE=remote` の組み合わせはエラーにする。または SQLite Store が `user_id="local"` 以外を拒否するか、remote MCP でのみ userID 検証を強化する。

---

### C2: DynamoDB と TokenStore の atomic 性欠如（Space 登録と Token 保存が分離）

**問題:**  
§4.4 で SpaceStore（logvalet-spaces テーブル）と TokenStore（logvalet-auth テーブル）は別テーブルで管理する。OAuth callback 時に `SpaceRegistry.Upsert` と `TokenStore.Put` を順番に実行するが（§10.2）、この2操作に atomic 保証がない。

**発生シナリオ:**  
1. `TokenStore.Put` 成功 → `SpaceRegistry.Upsert` 失敗: Token は存在するが Space 未登録。`all_spaces` で対象スペースが現れないが、Token はローカルに残り続ける。
2. `SpaceRegistry.Upsert` 成功 → `TokenStore.Put` 失敗: Space は登録されているが Token なし。次回 fan-out 時に `not_connected` エラーが発生し、ユーザーに原因が分からない。

**影響:**  
不整合状態が永続化し、`lv spaces verify` でも復旧不能（verify は token を使って接続するが token がない状態）。ユーザーは `lv spaces remove && lv spaces connect` で手動再登録が必要になる。

**推奨対処:**  
OAuth callback での2ステップ書き込みを「先に Token を確実に保存し、Space Upsert は失敗してもリトライ可能」な設計にする。またはべき等な再試行で最終整合性を達成する仕組みを明記する。DynamoDB TransactWriteItems は別テーブル間では使えないため、べき等リトライ + 障害検知の運用フローを spec に追記する。

---

### C3: OAuth state の one-time-use が MVP 省略可能とされている（replay attack リスク）

**問題:**  
§10.3 で「MVP では state の one-time-use 実装（nonce 消費）は省略可」と明示されている。しかし multi-space 登録フローでは state JWT に `base_url` や `alias` が含まれており、期限内の replay は以下を可能にする:

1. OAuth callback を2回送ると、SpaceRegistry.Upsert は idempotent なので見かけ上エラーにならない。
2. default space の条件付き write (`if DefaultSpaceAlias == ""`) が1回目で成功していれば2回目は no-op だが、1回目が失敗して2回目が成功すると意図しない default space が設定される。
3. 攻撃者が正規の OAuth callback URL を傍受して再送した場合（MITM・ログ漏洩・SSRF 経由）、ユーザーのスペース登録が上書きされる。

**影響:**  
スペース登録の hijack。認証トークンが攻撃者の request に紐づいた形で保存される可能性（code exchange は正規ユーザーが行っているので限定的だが、default space の書き換えによる誘導攻撃は成立する）。

**推奨対処:**  
nonce 消費（Redis/DynamoDB で nonce を TTL 付きで1回のみ使用済みマーク）は MVP から必須にする。特に multi-space 登録フロー（`MultiSpaceOAuthHandler`）にのみ限定的に実装することで影響範囲を最小化できる。

---

### C4: `lv spaces rename` のコマンド定義はあるが仕様が完全に空

**問題:**  
§8.2 の SpacesCmd struct で `lv spaces rename <old> <new>` がコマンドとして列挙されているが、実装仕様が spec 全体を通じて一切記載されていない。§12.3 では「alias rename しても tenant は変えない（SpaceRegistration.Tenant は immutable）」と記載されているが、rename の際の以下が未定義:

1. DynamoDB では SK を変更できないため `SK=SPACE#<old>` を `SK=SPACE#<new>` に変えるには **Delete + Put** が必要。これは atomic でない（C2 と同様の問題）。
2. UserPreference.DefaultSpaceAlias が old alias を指している場合の自動更新有無。
3. MCP tool が過去にキャッシュした alias 参照への影響。
4. rename 途中で失敗した場合のロールバック手順。

**影響:**  
rename を実装したとき DynamoDB の Delete + Put が partial failure すると alias が消える（space 消失）か重複する。DefaultSpaceAlias が古い alias のまま残ると `no_default_space` エラーが発生する。

**推奨対処:**  
MVP では `lv spaces rename` を除外するか、DynamoDB での safe rename（TransactWriteItems で同一 PK の Delete + Put を同一 user 下で実行）の実装仕様を明記する。MVP から除外する場合は SpacesCmd からも削除する。

---

## HIGH

### H1: Executor.MaxConcurrency = 0 のとき deadlock が発生する

**問題:**  
§7.2 で `semaphore.NewWeighted(int64(maxConcurrency))` を使うと記載されているが、`maxConcurrency = 0` の場合 `semaphore.NewWeighted(0)` は重みが0のセマフォを生成する。`Acquire(ctx, 1)` は永遠にブロックし、goroutine がすべて停止する。

`LOGVALET_SPACE_FANOUT_CONCURRENCY=0` を設定したユーザーや、設定値のパースに問題があったときに再現する。

**推奨対処:**  
`maxConcurrency <= 0` の場合はデフォルト値（4）にフォールバックするか、起動時に validation error として拒否する。

---

### H2: `--spaces` の Kong パース方法が spec に未定義（comma-separated vs 複数フラグ渡し）

**問題:**  
§8.1 で `--spaces foo,bar,baz` という comma-separated 形式を採用しているが、Kong では同一フラグを複数回渡す方式（`--spaces foo --spaces bar`）も一般的。Kong の `[]string` 型フラグは複数回渡しをサポートするが、`string` 型に comma-separated を自分でパースする場合は Kong のデフォルト動作と齟齬が生じる。

spec では `GlobalFlags.Spaces string` と定義しているが（§24.8）:
- `--spaces foo --spaces bar` と指定した場合、2回目の値が1回目を上書きするか、comma-joined されるかが Kong の型によって変わる。
- `string` 型なら2回目が上書き → `bar` のみ対象になりユーザーが気づかない。
- `[]string` 型なら重複値がそのまま入り、comma-split 処理と衝突する（`["foo,bar", "baz"]` のような混在）。

**推奨対処:**  
`GlobalFlags.Spaces` を `[]string` にして Kong の複数渡しに対応し、各要素を comma-split + flatten する。または `string` 型を維持して `--spaces foo --spaces bar` を明示的にエラーにする validation を追加する。

---

### H3: default space が削除されたときのフォールバックが未定義

**問題:**  
`lv spaces remove <alias>` で current default space を削除した場合、§5.3 の resolve 優先順位5（legacy profile fallback）に落ちる前に `ErrNoDefaultSpace` を返す可能性がある。§8.2.5 では removal 時に PREF 更新を TransactWriteItems で行うと記載されているが、**削除後に PREF.DefaultSpaceAlias をどの値に設定するか（空文字/残り1件のエイリアス/null）は未定義**。

特に: 登録スペースが1件だけのときにそれを削除した後、Resolver の fallback 4（「1件だけなら使う」）は0件になっているため機能せず、fallback 5（legacy profile）もなければ `ErrNoDefaultSpace` で静かに CLI が死ぬ。ユーザーへのメッセージが `ErrNoDefaultSpace` のみでは原因が分かりにくい。

**推奨対処:**  
削除時に「削除後の default space 解決」を明示仕様化する。候補: (1) 残りスペースが1件以上あれば最初の enabled space を自動で default に設定する、(2) 削除後に `lv spaces use <alias>` を促す警告を出す。

---

### H4: 65 MCP ツールへの `spaces` 引数追加で既存 LLM プロンプトが破壊される可能性

**問題:**  
§9.1 で全 MCP tool に `spaces` / `all_spaces` 引数を追加する。これはツールの JSON Schema 変更を意味し、**既存の LLM プロンプト・エージェント定義・Claude Code スキル定義（skills/ 配下）が新しい引数スキーマを知らない状態で動き続ける**。

既存プロンプトに明示的にツール引数を指定している場合（例: スキルファイルに JSON schema を埋め込んでいる）、引数スキーマの変更によって validation error または unexpected behavior が発生する。

特に `logvalet_issue_list` のような高頻度ツールでは影響が大きい。

**推奨対処:**  
- `spaces` / `all_spaces` の両引数は `optional` で default=null にする（現在の spec でも optional だが、LLM が `spaces: []` を明示的に渡したとき current/default として扱うか、empty array として別扱いにするかを明記する）。
- スキルファイルの更新要否を migration checklist に追加する。
- Phase 2 ツール対応後に skills/ のスキーマを同期するタスクをマイルストーンに含める。

---

### H5: token refresh の concurrent call protection が TokenStore 変更時にも必要

**問題:**  
既存 `auth/manager.go` では `singleflight.Group` で同一 `(userID, provider, tenant)` のキーに対する refresh を1回だけ実行する（実装確認済み）。

しかし multi-space 対応後、`ExecuteAcrossSpaces` が並列に `SpaceAwareClientFactory` を呼び、複数スペースが**同じ tenant を共有**する（alias が異なるが tenant が同一）ケースで:
- `SpaceRegistration.Alias = "prod-ro"` と `SpaceRegistration.Alias = "prod-rw"` が同一 tenant `"myorg"` を指している場合
- 両方のスペースが同時に token refresh を必要とするとき
- singleflight のキーは `(userID, provider, tenant)` なので1回しか実行されないが、これは正しい動作

このケースは **正しく動作するが spec に記載がない**。実装者が tenant 共有ケースを考慮せずに別のキー設計をすると race condition になる。

**推奨対処:**  
spec に「同一 tenant を複数 alias が参照するケースの token refresh は singleflight で自動的に dedup される」と明記する。SpaceAwareClientFactory の実装注意事項として記載する。

---

### H6: `partial_failure` の exit code が既存定義と衝突

**問題:**  
§28 で `partial_failure` の CLI exit code を `2` と定義している。しかし既存 CLAUDE.md の exit code 定義では:
```
2: argument / validation error
```

`partial failure` は argument error ではなく実行時エラーであり、同じ exit code 2 を使うことで `--spaces invalid_alias`（argument error）と `--spaces foo --spaces bar` で bar が unauthorized（partial failure）の区別がスクリプトからできなくなる。

**推奨対処:**  
`partial_failure` 用の新しい exit code を追加する（例: exit code 5 は現在 `resource_not_found` だが、その番号は既存で使用中のため7等の空きを使う）か、既存の exit code 2 は argument/validation のみとし、partial failure は exit code 6（API error）に割り当てる。

---

## MEDIUM

### M1: カスタムドメインの tenant 導出で GetSpace が失敗したときの登録フロー未定義

**問題:**  
§12.3 で「カスタムドメインでは OAuth authorize 前に token がないため GetSpace が呼べない」問題の解決策として「ユーザーへの手動 tenant 入力」を採用している。しかし `lv spaces add`（API key mode）のカスタムドメインケースで:
- ユーザーが tenant 名（spaceKey）を間違えて入力した場合
- GetSpace で `spaceKey != ユーザー入力 tenant` を検出したときの動作（reject/warn/overwrite）が未定義

また spec 内で `DeriveInitialTenant` の戻り値が空文字のとき「GetSpace を呼ぶ」とあるが、API key mode では GetSpace 呼び出しに使う credential（API key）が正しいか確認するタイミングが前後する。

**推奨対処:**  
カスタムドメイン登録フローを独立したステップとして spec に追記:「GetSpace で spaceKey を取得し、ユーザー入力 tenant と一致しない場合は警告して上書き確認を求める」。

---

### M2: `lv spaces connect` と `lv space connect`（コマンド名の不統一）

**問題:**  
§8.2 では `lv spaces connect`（SpacesCmd の下）と記載しているが、§8.2.2 のタイトルは「lv space connect」（単数形）になっている。また §9.3 MCP tool 名は `logvalet_space_connect_url`（単数形）。

さらに §22 MVP スコープでは `lv spaces list/add/connect/use/verify`（複数形）と明記しているが、一部のサブセクション見出しとコマンド例が単数形と複数形で混在している。

**影響:**  
実装者が単数形コマンド（`lv space connect`）と複数形コマンド（`lv spaces connect`）を別々に作ってしまうリスクがある。spec の不統一が実装の混乱を招く。

**推奨対処:**  
§8.2 の全サブセクション見出しを `lv spaces connect`（複数形）に統一する。

---

### M3: `SpaceStatus = "disabled"` のとき `lv spaces verify` が何を返すか未定義

**問題:**  
§4.1 で `SpaceStatusDisabled` が定義されているが、`lv spaces verify --all-spaces` 実行時に disabled なスペースへの検証動作が未定義。

選択肢:
1. disabled space はスキップして返さない
2. disabled space は `{"ok": false, "error_code": "disabled"}` として返す
3. disabled space は verify から除外し、`lv spaces list` だけに表示する

また Resolver の §5.3 では「enabled spaces のみ返す」と記載しているが、disabled を設定する CLI コマンドが spec に存在しない（remove はあるが disable はない）。

**推奨対処:**  
disabled の設定・解除コマンドを MVP スコープから除外するなら `SpaceStatusDisabled` の使用シナリオを削除する。使用するなら verify と Resolver の挙動を明記する。

---

### M4: DynamoDB GSI による重複 tenant チェックが eventually consistent

**問題:**  
§4.4 で「Option A: GSI で (user_id, tenant) の複合インデックスを作り、Upsert 前に重複 tenant チェック」を MVP 採用としている。しかし DynamoDB GSI は **eventually consistent** であり、高速な連続登録（例: OAuth callback が連続して届く場合）で重複が検出されないことがある。

spec 内でも「race はまれであり、Upsert の idempotent 化で実害を最小化」と記載されているが、`all_spaces` 実行時に同一 tenant への重複 fan-out は並行 API call を引き起こし Backlog API の rate limit に影響する。

**推奨対処:**  
「eventually consistent だが許容する」判断は明示的に doc に残す。加えて `ExecuteAcrossSpaces` の前処理で tenant 重複を dedup する（登録ステップではなく実行ステップで防ぐ）アプローチを追記する。

---

### M5: rate limiting fan-out 時のリトライ・バックオフが未定義

**問題:**  
§25.2 でデフォルト4並列を推奨しているが、Backlog API の rate limit は「300 req/min」とある。ユーザーが10スペース登録して `--all-spaces` で重いオペレーション（issue list + activity list）を同時実行すると、1回のリクエストで 20+ API call が発生する。複数ユーザーが同時実行した場合のサーバー全体でのスロットリングは考慮されていない。

また `rate_limited` エラーコードはエラー一覧（§28）に存在するが、`ExecuteAcrossSpaces` 内での 429 受信時の挙動（即失敗/リトライ/バックオフ）が未定義。

**推奨対処:**  
`ExecuteAcrossSpaces` での 429 処理方針を明記: 「MVP では即 `rate_limited` エラーとして partial failure 扱い」「将来的に指数バックオフを追加」。rate limit をユーザーに伝えるエラーメッセージに現在のオペレーション数を含めることを推奨する。

---

## LOW

### L1: migration path の remote MCP deploy 順序が「doc に明記」されているが doc が存在しない

**問題:**  
§30.2 で「remote MCP deploy 順序を doc に明記」とあるが、参照先ドキュメントが spec 内に存在しない。実際に DynamoDB テーブル作成前に新コードをデプロイすると `TableNotFoundException` が発生しサービス断になる。

**推奨対処:**  
deploy 手順を spec または ops runbook として別途作成し、§30.2 からリンクする。

---

### L2: SQLite テーブル名が既存 TokenStore と衝突しない根拠の明示が不十分

**問題:**  
§24.9 で「SpaceStore の SQLite テーブル名は `spaces`, `user_preferences` として衝突しない」と記載されているが、SQLite の場合 TokenStore と SpaceStore が同一ファイル（同一パス）を使う可能性がある。§15.1 で `LOGVALET_SPACE_SQLITE_PATH` が独立しているため衝突はしないが、既存 TokenStore の SQLite ファイルパスが明示されていない。

**推奨対処:**  
既存 SQLite TokenStore のデフォルトパスを §24.9 または §4.5 に明記し、SpaceStore との分離を保証する。

---

### L3: `lv spaces list` の `lv space list`（コマンド名）との混乱

**問題:**  
§8.2.1 のコマンド例が `lv space list`（単数形）になっているが、コマンド定義は `lv spaces list`（複数形・SpacesCmd）のはず。既存の `lv space info` / `lv space disk-usage` / `lv space digest` と混同しやすい。

**推奨対処:**  
§8.2.1 のコマンド例を `lv spaces list` に修正する。

---

## GPT vs 自分の差分サマリー

| 指摘カテゴリ | GPT | 自分 |
|---|---|---|
| SQLite "local" 漏洩リスク | 言及なし | **C1** |
| SpaceStore/TokenStore atomic 性 | 抽象的に指摘 | **C2（具体的シナリオ付き）** |
| OAuth replay attack | 記載ありとして懸念表明 | **C3（MVP 省略可となっているリスクを具体化）** |
| rename 仕様の空白 | 言及なし | **C4** |
| MaxConcurrency=0 deadlock | 言及なし | **H1** |
| Kong --spaces 複数渡し | 言及なし | **H2** |
| default space 削除後 | 言及なし | **H3** |
| 65ツール schema 変更の LLM 破壊 | 言及なし | **H4** |
| token refresh concurrent (singleflight) | 指摘あり（ただし既存実装で対応済みを知らない） | **H5（実装確認済み + spec 未記載の指摘）** |
| partial_failure exit code 衝突 | 言及なし | **H6** |
| DynamoDB GSI 整合性 | 抽象的に指摘 | **M4（eventually consistent を具体化）** |
| rate limit fan-out | 指摘あり | **M5（既存指摘と一致）** |
| --space/--spaces 混同リスク | 指摘あり | - |
| write 操作の安全策 | 抽象的に指摘 | spec に記載あり（§13.2）として処理 |
| generics 型安全性 | 一般論 | spec §7.2 で Go 制約は対処済みとして処理 |

GPT は一般的な anti-pattern を列挙したが、実コード確認（singleflight の存在、global_flags.go の構造）や spec の行番号に基づいた具体的な指摘は自分のみ。

---

## 総評

spec の完成度は高い。多くの既知リスク（replay attack、tenant 重複、DynamoDB 別テーブル化）が既に architect 段階で対処されている。

しかし以下の4点が実装に入る前に解決を要する:

1. **SQLite/DynamoDB の store type validation（C1）**: remote MCP 環境で SQLite store が使えないよう起動時に強制する。
2. **OAuth callback の 2-phase write に対する最終整合性の明文化（C2）**: 「失敗したら再登録してください」だけでなく、Space 未登録+Token 残留状態をユーザーが自己診断できる `lv spaces verify` のエラーメッセージを具体化する。
3. **state nonce 消費の MVP 必須化（C3）**: multi-space 登録フロー限定でよいが、「省略可」は撤回する。
4. **rename コマンドの MVP 除外または仕様完備（C4）**: 半端な状態で実装に入るとデータ損失リスクがある。

---

*Reviewed by: devils-advocate — devflow:team Phase 4 on 2026-05-20*

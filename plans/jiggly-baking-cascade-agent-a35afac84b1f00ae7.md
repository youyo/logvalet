# 批評レポート: logvalet v3 ロードマップ実装計画

## 概要

計画: jiggly-baking-cascade.md（M17-M19）
評価対象: 3マイルストーン、API拡張 + MCP サーバー + スキルリネーム

## 問題点リスト（優先度順）

### 1. [致命的] MCP SDK の Streamable HTTP 実装状況が未検証

**根拠:**
- 計画は mark3labs/mcp-go が「Streamable HTTP 対応」していると仮定
- リスク評価でも「Streamable HTTP 対応不完全」が「重大度: 高」「確率: 中」
- PoC 実施が M18 開始「前」と明記されているが、実際のコード依存検証がない
- mark3labs/mcp-go の実装品質・成熟度・ライセンス（Apache 2.0）はまだ未確認（計画時点）

**影響:**
- M18 実装開始後に Streamable HTTP が未対応判明 → 代替 SSE fallback への急遽切り替え
- 設計変更に伴う全 18 MCP tool の再実装可能性
- Go 1.26.1 との互換性確認なし

**結論:**
M18 開始判定が甘い。PoC は M17 完了後「M18 開始前のゲート」ではなく、「今すぐ実施」すべき。

---

### 2. [致命的] ダウンロード系の IO モデルが曖昧

**根拠:**
- `DownloadSharedFile(ctx, projectKey, fileID) (io.ReadCloser, string, error)` と定義
- 第2戻り値 `string` がファイル名なのか Content-Disposition なのか明記なし
- CLI で `--output PATH` を指定した場合：
  - io.ReadCloser をファイルに書き込む処理が誰の責任か不明
  - Client メソッドか CLI コマンド（shared_file.go）か明示なし

**影響:**
- ファイルダウンロード実装時に「Client は ReadCloser のみ返す」vs「Client が --output を処理」の判断で開発者が迷走
- テストケース SF-3, IA-3 で「io.ReadCloser + filename」と表記だけで実装仕様が曖昧
- 大容量ファイル時のメモリ効率性が実装依存に

**結論:**
「Client は ReadCloser と filename（Content-Disposition から抽出）を返す。CLI が --output 指定時にファイル保存」と明確に設計仕様に記述すべき。

---

### 3. [致命的] AddStar の排他性バリデーションが CLI 層か API 層か不明

**根拠:**
- CLI コマンド: `logvalet star add (--issue-id ID | --comment-id ID | ...)`
- テスト ST-E2: 「複数対象指定」→「validation error (排他)」
- 実装手順では「Run() 内で指定数カウント。1 でなければエラー」と記述
- しかし AddStarRequest の定義がない（types.go に追加予定だが仕様不明）

**影響:**
- 複数フラグ指定を「Kong パーサー層」で排他制御するのか「Run() メソッド」で検証するのか未定
- Kong の `mutually_exclusive` フラグ機能があるのに、Run() で重複チェックするコード臭
- テスト実装が先行（Red）するが、実装仕様が不透明

**結論:**
AddStarRequest の完全定義（必須フィールド、排他パターン）を計画に追加。Kong のバリデーション戦略を明記。

---

### 4. [重大] MCP tool の 18 個という数字が恣意的

**根拠:**
- tool 一覧（表 line 237-256）に 18 個列挙
- 実装計画では「18個の MCP tool を CLI コマンド単位で公開」と明記
- しかし対応表を見ると：
  - `project_list` が CLI に無い（CLI に `logvalet project list` 無し？）
  - `activity_list` の対応 CLI が `activity list` だが、実装されているのか未確認
  - Download/Delete 系ツール（shared-file download, issue attachment delete）が MCP tool 一覧にない

**影響:**
- 実装漏れリスク：tool 一覧と実際の CLI コマンドの乖離
- テスト設計書では tool 名が記載されているが、CLI コマンドとの対応マップがない
- MCP server.go で tool 登録時に「登録漏れ」発生の可能性

**結論:**
tool 一覧を「CLI コマンド ↔ MCP tool 名」のマッピング表として再整理。
漏れ/ズレを確認した上で最終化。

---

### 5. [重大] スキルリネーム後の既存ユーザー移行戦略が不在

**根拠:**
- M19: `skills/backlog/*` → `skills/logvalet-*/` への移動＋ name 変更
- 既存ユーザーが `npx skills add backlog:report` で導入している可能性が高い
- リスク評価に「スキルリネームで既存環境破壊」が記載（重大度: 中, 確率: 中）
- 対策: 「README に移行手順記載」とあるが、具体的な手順がない

**影響:**
- リネーム後、旧スキル削除のタイミングが不明（すぐか、1バージョン後か？）
- 旧スキル存在時の競合処理なし（両方導入されている場合）
- ユーザーが旧 backlog:* スキルを手動削除後、npx skills add logvalet:* する作業が漏れる可能性
- Homebrew tap リリース時の v3.0.0 でいきなり前提が変わる

**結論:**
- M19 を「Deprecation period」に変更：
  - v3.0.0: logvalet-* スキルを追加（backlog:* は並存）
  - v4.0.0: backlog:* スキルを削除
  - README に移行ガイド・Deprecation warning を記載

---

### 6. [重大] MCP server の認証が既存 API Key を使うが、マルチプロジェクト運用時の設計不明

**根拠:**
- 計画 line 221: 「認証: 既存の API Key を使用」
- Client は 1 つの Backlog space（認証済み）を想定
- MCP server が複数のクライアント（異なる権限）から呼ばれる場合：
  - logvalet mcp は認証済みユーザー固定
  - MCP tool が複数ユーザーのコンテキストを切り替える手段がない

**影響:**
- MCP server を複数プロジェクト・複数ユーザーで共有する構成がサポートされない
- 「logvalet mcp --api-key KEY」の拡張が必要になる可能性
- spec §3.1（認証）との整合性が不明

**結論:**
- 認証モデルを明記：「logvalet mcp は 1 API Key = 1 space」
- または、MCP tool レベルで API Key override を設計

---

### 7. [重大] download 系メソッドの戻り値型が stdlib 非推奨パターン

**根拠:**
```go
DownloadSharedFile(ctx context.Context, projectKey string, fileID int64) (io.ReadCloser, string, error)
```
- Go 標準的パターンは `([]byte, error)` または `(io.ReadCloser, error)`
- 複数戻り値（ReadCloser + filename + error）は unusual

**影響:**
- 呼び出し側が「io.ReadCloser を意図的に close する」責任を明記する必要がある
- テストでリーク検出器（lintertools）でリソースリーク検出困難
- mock_client の実装複雑化

**結論:**
代替案を検討：
1. `(*DownloadResponse, error)` 構造体化
2. Filename を Header から抽出（HTTP層で処理）
3. Client は ReadCloser のみ、CLI が filename パース

---

### 8. [中大] テスト設計書が Client 層のみで CLI 層のテストなし

**根拠:**
- テスト設計書（M17: SF-1 〜 ST-2）は全て Client メソッドのテスト
- CLI コマンド層のテストが記載されていない
- 例：`logvalet shared-file download --project KEY FILE-ID --output /tmp/test.txt`
  - --output ファイル保存の成功/失敗テスト
  - パストラバーサル対策（SF-EC3）は設計にはあるが、テスト設計にない

**影響:**
- CLI 層実装時に「テストどうしよう」と迷走
- golden test フレームワークで JSON/YAML 出力は テストされるが、ファイル操作は手動検証
- 既存の issue_list_test.go 等との テストパターン不一致

**結論:**
CLI テスト設計書を追加：
- CLI integration test（internal/cli/shared_file_test.go）
- ファイル保存のエッジケース
- 権限エラー時の exit code 検証

---

### 9. [中大] mark3labs/mcp-go の Tool InputSchema 設計がプレースホルダー

**根拠:**
- ToolRegistry: `schema mcp.ToolInputSchema`
- MCP tool 一覧では tool 名だけで、JSON Schema が記載されていない
- 例：`issue_get {issue_key:"TEST-1"}` の schema はどう定義？

**影響:**
- MCP server が tool を advertise する際に InputSchema を送信
- schema が不正確だと MCP client が誤ったパラメータを送信
- 実装時に mark3labs/mcp-go の ToolInputSchema API を理解する必要あり

**結論:**
m1個の tool ごとに InputSchema 定義を計画に追加。
例：
```go
schema: {
  "type": "object",
  "properties": {
    "issue_key": {"type": "string", "description": "Issue key (e.g., TEST-1)"}
  },
  "required": ["issue_key"]
}
```

---

### 10. [中] M18 の tools_team.go が何をするのか不明

**根拠:**
- ファイル一覧に `internal/mcp/tools_team.go` が存在
- tool 一覧に Team 関連 tool がない
  - `user_list`, `team_*` はない
- CLI にも `logvalet team` コマンドなし（ls internal/cli/ で確認）

**影響:**
- tools_team.go が「単なるプレースホルダー」なのか「実装遅延」なのか不明
- ファイル一覧の不正確さ → 実装時の混乱

**結論:**
tools_team.go を削除するか、実装する Team tool を明示。

---

### 11. [中] M17 の IO ReadCloser streaming が本当に実装可能か未検証

**根拠:**
- リスク評価: 「大容量ファイルダウンロードでメモリ枯渇」→ 「io.ReadCloser streaming。io.Copy で直接ファイルへ」
- Backlog API の download エンドポイント（GET /shared-files/{fileID}/download）が存在するはずだが、Backlog API ドキュメント未参照

**影響:**
- http_client.go 実装時に「Backlog API がストリーミング対応か」が不明
- API がバイナリ全体をメモリに載せてから返す場合、io.ReadCloser 設計が無意味

**結論:**
M17 開始前に Backlog API ドキュメント確認。
API が streaming 対応でない場合、設計見直し。

---

### 12. [中] AddStarRequest の構造体定義が types.go だけで、どの形式か不明

**根拠:**
- 計画 line 126: `internal/backlog/types.go に AddStarRequest 追加`
- 実際の定義仕様がない
- CLI コマンドから見ると：
  ```
  logvalet star add (--issue-id ID | --comment-id ID | --wiki-id ID | --pr-id ID | --pr-comment-id ID)
  ```

**影響:**
- AddStarRequest が `{IssueID: *int64, CommentID: *int64, ...}` なのか
- `{Target: string, ID: int64}` なのか定義不明
- Backlog API のエンドポイント定義に依存（複数エンドポイント? 単一エンドポイント?）

**結論:**
Backlog API 仕様確認 → AddStarRequest struct + Client メソッド実装仕様を明記。

---

## 前提の脆弱性

### 前提1: 「mark3labs/mcp-go は Streamable HTTP に完全対応している」
**崩れる条件:**
- mark3labs/mcp-go が SSE のみ対応
- HTTP サーバーが Unix socket 要求（logvalet mcp の --port オプション と矛盾）
- 証拠: リスク評価で「Streamable HTTP 対応不完全」が既に「高確率」と指摘されている

**結果:**
- M18 がブロック
- 代替 SDK 検討（他の Go MCP SDK？）必要

---

### 前提2: 「既存 CLI コマンドと MCP tool は 1:1 対応」
**崩れる条件:**
- project_list, activity_list が実は CLI に実装されていない
- Download/Delete が CLI に実装されているが MCP tool 一覧から漏れ
- 新 API (M17) が tool 一覧に漏れ

**結果:**
- 実装時に「tool 作ってみたら CLI コマンドが無かった」
- 逆に「CLI コマンドを作ったが tool が定義されていない」

---

### 前提3: 「スキルリネームはユーザー向け deprecation warning で OK」
**崩れる条件:**
- README に書いても、既存ユーザーが見ないケースが大多数
- npx skills add backlog:report で既に導入済み環境が古いスキルをキャッシュしている
- GitHub Actions/CI に backlog:* スキルが pin されている場合の互換性破壊

**結果:**
- v3.0.0 リリース後、ユーザーから「backlog:report が動かない」という報告増加
- Deprecation period なし → 互換性破壊の苦情

---

### 前提4: 「既存 Client interface に 7 メソッド追加するだけで充分」
**崩れる条件:**
- Download 系が io.ReadCloser の扱いで新しい抽象化が必要
- 複数プロジェクト・複数認証が必要になった場合、Client interface を拡張必要
- HTTP Client の内部実装が複雑化（download URI が異なるなど）

**結果:**
- interface を後付け変更 → 既存モック実装が破壊
- mock_client.go の変更が M17, M18, M19 全マイルストーン に波及

---

### 前提5: 「MCP tool の JSON 結果は既存 render エンジンと整合」
**崩れる条件:**
- MCP tool が返す JSON スキーマが、既存の domain model と異なる
- 例：issue_get tool が異なるフィールド名を返す
- render.JSON が domain.Issue を想定しているが、MCP はカスタムスキーマ

**結果:**
- 計画では「tool 結果は JSON テキスト」と簡潔に記述
- 実装時に render 層との整合が問題化

---

## 見落とされたリスク

### R1: Backlog API の rate limiting
**リスク:**
- logvalet mcp が複数のクライアントから同時呼び出し → rate limit 到達
- 計画に rate limit 対策（backoff, queue）がない

**発生条件:**
- MCP server を複数 Claude instance で使用
- デモンストレーション時に短時間で複数 tool call

**影響:**
- 429 Too Many Requests を MCP client に返す
- Tool call failure の不明確なエラーメッセージ

---

### R2: Context cancellation の処理が incomplete
**リスク:**
- DownloadSharedFile(ctx, ...) が ctx.Done() を受けたとき、HTTP stream はどうなる？
- ReadCloser を返した後に ctx cancel → ReadCloser は close すべきか否か？

**発生条件:**
- MCP client が tool call を timeout
- ユーザーが CLI に Ctrl-C

**影響:**
- HTTP connection leak
- テスト設計に ctx cancellation テストなし

---

### R3: Backlog API の仕様変更への追従
**リスク:**
- 計画作成から実装 3-4ヶ月間で Backlog API が変更される可能性
- SharedFile API, IssueAttachment API のフィールドが増える場合、domain model を変更

**発生条件:**
- Backlog が API を拡張（e.g., metadata フィールド追加）

**影響:**
- Golden test が fail （既存の golden file と出力が異なる）
- User から「新フィールドが出ていない」という報告

---

### R4: MCP tool の error handling が undefined
**リスク:**
- Backlog API エラーを MCP CallToolResult にどう変換するか不明
- Client メソッドが ErrNotFound, ErrUnauthorized を返すが、MCP tool はどう処理？

**発生条件:**
- MCP client が不存在リソースを要求（issue_get with invalid key）
- API key expired

**影響:**
- ToolHandler が `IsError: true` で返すが、detail message がない
- MCP client が原因を特定できない

---

### R5: download 系の filename sanitization が甘い
**リスク:**
- パストラバーサル対策は `filepath.Base()` と記載（SF-EC3）
- しかし HTTP response の Content-Disposition が不正な値を返す場合を考慮していない

**発生条件:**
- Backlog API が Content-Disposition に `/../../../etc/passwd` を返す（想定外）

**影響:**
- `filepath.Base("/../../../etc/passwd")` = `"passwd"`
- ただし、その他の filename encoding (UTF-8, Latin-1) が混在する場合の処理不明

---

### R6: tools_meta.go がなぜ必要なのか不明
**リスク:**
- ファイル一覧に `tools_meta.go` がある
- 「Meta 系 tool」が何を指すのか未定義
- Backlog API の「meta」はしばしば project metadata（status, category etc）を指す

**発生条件:**
- tools_meta.go を実装する際に「何を実装すべきか」で判断が分かれる

**影響:**
- 実装漏れ or 冗長な実装

---

### R7: Go module 依存が整理されていない
**リスク:**
- go.mod に mark3labs/mcp-go を追加（v は未指定）
- indirect dependency の増加によるbuild time 増加

**発生条件:**
- GitHub Actions の CI が slow になる

**影響:**
- リリースパイプラインが遅延

---

## 見落とされたアーキテクチャ矛盾

### A1: Client interface の IO戻り値がインターフェース污染
- `DownloadSharedFile` が io.ReadCloser を返すが、これは HTTP 層の詳細実装を expose
- domain package の設計哲学（domain models only）と矛盾

### A2: MCP ToolRegistry が internal/mcp に独立しているが、CLI cmd との責任境界が曖昧
- shared-file list コマンドと shared_file_list tool で処理が重複
- 共通化戦略なし

### A3: AddStar の「排他性」が Kong + Run() で分散
- バリデーション責任が不明確

---

## ⚠️ 最も危険な1点

**MCP SDK (mark3labs/mcp-go) の Streamable HTTP 対応が未検証のまま M18 開始を想定**

### 理由

1. **計画は mark3labs/mcp-go に完全依存**
   - 他の Go MCP SDK の検討なし
   - SSE fallback は言及されているが、実装仕様がない

2. **リスク評価でも「高確率・高重大度」として挙げているのに、ゲート条件が弱い**
   - 「M18 開始前に PoC」と書いているが、PoC 失敗時の代替案がない
   - いつ PoC をするのか？（M17 完了時点？今？）

3. **実装開始後の発見が致命的**
   - mark3labs/mcp-go が Streamable HTTP 未対応 → 全 18 tool の実装方針変更
   - Go 1.26.1 との互換性問題 → Go version 変更検討
   - 3-4週間の実装遅延

4. **Go 社のエコシステムでは「MCP SDK の成熟度」が長期的な懸念**
   - anthropic-ai/sdk-go が正式な Go SDK なのに、mark3labs/mcp-go？
   - ライセンス Apache 2.0 ✓ だが、maintenance status 不明

### 結論

**即座に実施すべき:**
```
1. mark3labs/mcp-go の github repo を確認
   - Streamable HTTP サポート status
   - Go 1.26.1 compatibility
   - Latest version, maintenance status
   
2. PoC を「今」実装
   - mcp server を起動
   - HTTP POST で tool call
   - Response format 確認
   
3. PoC 失敗時の代替案を決定
   - Anthropic の official SDK か？
   - SSE only で妥協か？
```

**このゲートをクリアするまで M18 計画は「Draft」のままにすべき。**

---

## その他の指摘（軽微）

- **L1**: リスク評価表のリスク対策が曖昧（「SSE fallback」の実装仕様が無い）
- **L2**: ドキュメント更新計画に logvalet_SKILL.md が含まれているが、新スキル追加時の手順がない
- **L3**: Download エンドポイントが Backlog API に実在するかの確認資料がない


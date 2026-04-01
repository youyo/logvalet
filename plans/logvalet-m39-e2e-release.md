# M39: Phase 2 E2E テスト + リリース

## 目的

Phase 2（triage-materials / periodic-digest）の完了検証として、実 Backlog API に対する E2E テストを追加する。

## 依存

- M38（Phase 2 ドキュメント整備）完了済み

## タスク

- [x] `internal/e2e/triage_materials_e2e_test.go` — TriageMaterialsBuilder の E2E テスト
- [x] `internal/e2e/periodic_digest_e2e_test.go` — PeriodicDigestBuilder の E2E テスト（weekly / daily）

## テスト設計

### ビルドタグ

`//go:build e2e` で分離。通常の `go test ./...` では実行されない。

### 必要な環境変数

| 変数 | 説明 |
|------|------|
| `LOGVALET_E2E_API_KEY` | Backlog API キー（必須） |
| `LOGVALET_E2E_SPACE` | Backlog スペース名（例: `heptagon`）（必須） |
| `LOGVALET_E2E_PROJECT_KEY` | テスト対象プロジェクトキー（必須） |
| `LOGVALET_E2E_ISSUE_KEY` | テスト対象課題キー（省略時は `{PROJECT_KEY}-1`） |

### 実行方法

```bash
LOGVALET_E2E_API_KEY=xxx \
LOGVALET_E2E_SPACE=yourspace \
LOGVALET_E2E_PROJECT_KEY=YOURPROJECT \
go test -tags e2e ./internal/e2e/...
```

### テスト内容

#### triage_materials_e2e_test.go

- `TestE2E_TriageMaterials` — デフォルトオプションで triage-materials を生成し、Envelope 構造と TriageMaterials フィールドを検証
- `TestE2E_TriageMaterials_CustomClosedStatus` — カスタム完了ステータスを指定して生成

#### periodic_digest_e2e_test.go

- `TestE2E_PeriodicDigest_Weekly` — weekly モードで digest を生成し、期間・集計カウントを検証
- `TestE2E_PeriodicDigest_Daily` — daily モードで digest を生成し、期間差（約 24h）を検証

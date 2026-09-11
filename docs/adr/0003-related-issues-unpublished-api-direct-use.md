# 0003. 関連課題（Related Issues）機能を未公開 API 直接利用で実装する

## ステータス

承認済み（issue #63、2026-07-31）

## コンテキスト

Backlog の「関連課題（Related Issues）」機能は公開 API ドキュメント
（developer.nulab.com）に対応エンドポイントが掲載されておらず、logvalet では
これまで対応不能と判断していた。issue #63 の調査により、Nulab 公式 OSS クライアント
`nulab/backlog-js` の PR #181（2026-07-28 マージ）で `GET|POST /api/v2/issues/:key/relatedIssues`
と `DELETE .../relatedIssues/:relatedIssueId` が実装済みであること、Nulab メンバーが
レビュー・マージしていること、Nulab 公式 MCP サーバー `nulab/backlog-mcp-server` にも
同機能がラップ済みであることが一次情報で確認できた。logvalet 自身の実機検証（読み取り
専用）でも `GET /api/v2/issues/:key/relatedIssues` が HTTP 200 を返すことを確認済み。

関連課題は課題の文脈理解に直結し、LLM-first digest というプロダクト設計思想と合致する
一方、公式ドキュメント未掲載のエンドポイントである以上、Nulab が将来仕様変更・非推奨化
する可能性は排除できない。

## 決定

未公開 API `/api/v2/issues/:key/relatedIssues` を直接利用して実装する
（issue #63 の実装アプローチ候補1・推奨案を採用）。

- `internal/backlog/` の Client interface に `ListRelatedIssues` / `AddRelatedIssue` /
  `DeleteRelatedIssue` を追加し、既存の issue attachment 系と同じ縦串（interface →
  HTTPClient → MockClient → CLI → MCP → docs）で実装する。
- レスポンス形状は一次情報（backlog-js PR #181 の型定義）で裏取りし、根拠 URL をコードの
  doc コメントに記録する。防御的パースとして `type` フィールドは enum 化せず string の
  まま保持し、未知値・欠落のいずれでもパース成功する。
- 未公開 API である旨を spec doc・README・コード内コメントに明記し、Nulab の正式
  ドキュメント公開後に正式サポートへ格上げする方針とする。
- POST/DELETE の実機 E2E 検証は行わない（テスト方針がモックのみのため）。未検証事項は
  防御的パース・空 body 許容・エラーマッピングのテストでカバーする。

## 検討した代替案

### 代替案: 本文パースによる疑似関連課題

コメント・説明欄から課題キーパターン（`[A-Z0-9_]+-\d+`）を正規表現で抽出し、「言及課題」
として digest に出す、公開 API のみで完結する保守的なアプローチ（issue #63 の実装
アプローチ候補2）。

却下理由: 公式の関連課題機能（UI からの明示的な追加・双方向反映）とは別物であり、
ユーザーが実際に登録した関連課題を正確に再現できない（本文中の課題キー言及と、UI 上の
「関連課題として追加」は別概念）。未公開 API のリスク（仕様変更・非推奨化の可能性）を
避けられる利点はあるが、Nulab 公式 OSS（backlog-js・backlog-mcp-server）が同エンドポイント
を採用し実機検証済みと報告していることから実在性・安定性の確度は高いと判断し、機能の
正確性を優先して未公開 API の直接利用を選んだ。

## 影響

- `internal/domain/issue_related.go` に `RelatedIssue` 型が追加された。
- `internal/backlog/issue_related.go` に3メソッドの HTTPClient/MockClient 実装が追加された。
  `DeleteRelatedIssue` は空 body（204/200 empty）でもエラーにしない専用経路を持つ。
- `internal/cli/issue_related.go` に `lv issue related list|add|remove` が追加された。
- `internal/mcp/tools_issue.go` に `logvalet_issue_related_list/_add/_delete` の3ツールが
  追加され、MCP ツール総数が 72 から 75 になった。
- README.md / README.ja.md / spec doc / SKILL.md に未公開 API である旨の注記とともに
  ドキュメント同期が行われた。
- digest / issue context への関連課題組み込みはスコープ外とし、後続チケットへ委ねた
  （envelope 拡張と golden test 更新が連動するため）。
- 将来 Nulab がこのエンドポイントの仕様を変更・非推奨化した場合、防御的パースにより
  即座には壊れないが、機能自体の見直しが必要になるリスクを許容している。

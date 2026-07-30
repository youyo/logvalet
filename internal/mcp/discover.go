package mcp

// discover.go は server/discover (SEP-2575 JSON-RPC メソッド) のレスポンス内容を
// logvalet 向けに確定させるための構成点を提供する (S12, issue #52)。
//
// S03 スパイク (docs/specs/spike-go-sdk-2026-07-28.md (b)) の実測により、公式 Go SDK
// (github.com/modelcontextprotocol/go-sdk) は server/discover を完全に自動処理する:
//
//   - supportedVersions: SDK 組み込みの供給バージョン一覧 (トランスポートがサポート
//     する範囲に自動でフィルタされる)。
//   - capabilities: *officialmcp.Server に登録された機能から自動導出される
//     (logvalet は tool のみ登録するため logging + tools capability になる)。
//   - serverInfo (_meta["io.modelcontextprotocol/serverInfo"]): newOfficialMCPServer
//     (server.go) が渡す *officialmcp.Implementation{Name, Version} から自動付与される。
//
// logvalet 側で上書きが必要な項目は無い: serverInfo.name/version は
// newOfficialMCPServer が既に Implementation{Name: "logvalet", Version: ver} を渡して
// おり正しい値になっているため、discover 用の追加実装は不要 (二重実装しない)。
//
// そのため本ファイルにプロダクションコードは無く、discover_test.go が公式 SDK 経由の
// 実際の server/discover レスポンスを golden として固定する。

package mcp

// export_test.go: 同パッケージ内部の unexported ヘルパーを
// package mcp_test 側のテストから利用するための test-only エクスポート。

// SpaceInfoFromContextForTest は spaceInfoFromContext を外部テストへ公開する。
var SpaceInfoFromContextForTest = spaceInfoFromContext

// ContextWithSpaceForTest は contextWithSpace を外部テストへ公開する。
var ContextWithSpaceForTest = contextWithSpace

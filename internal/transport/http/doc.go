// Package http は logvalet の OAuth HTTP ハンドラー群を提供する。
//
// パッケージ名が標準ライブラリの net/http と衝突するため、
// caller 側では次のような alias を推奨する:
//
//	import httptransport "github.com/youyo/logvalet/internal/transport/http"
//
// M13 段階ではハンドラー定義のみを提供する。
// MCP サーバーへのルーティング統合は M16 で行う。
package http

package mcp

import gomcp "github.com/mark3labs/mcp-go/mcp"

// readOnlyAnnotation は list/get/stats/digest/health などの参照系ツールに適用する。
// 環境を変更せず、retry しても副作用なし。
func readOnlyAnnotation(title string) gomcp.ToolOption {
	return gomcp.WithToolAnnotation(gomcp.ToolAnnotation{
		Title:          title,
		ReadOnlyHint:   gomcp.ToBoolPtr(true),
		IdempotentHint: gomcp.ToBoolPtr(true),
		OpenWorldHint:  gomcp.ToBoolPtr(true),
	})
}

// writeAnnotation は非破壊の書き込み系ツールに適用する。
// idempotent は create=false / update=true 等で切り替える。
func writeAnnotation(title string, idempotent bool) gomcp.ToolOption {
	return gomcp.WithToolAnnotation(gomcp.ToolAnnotation{
		Title:           title,
		ReadOnlyHint:    gomcp.ToBoolPtr(false),
		DestructiveHint: gomcp.ToBoolPtr(false),
		IdempotentHint:  gomcp.ToBoolPtr(idempotent),
		OpenWorldHint:   gomcp.ToBoolPtr(true),
	})
}

// destructiveAnnotation は delete 系など破壊的更新ツールに適用する。
func destructiveAnnotation(title string) gomcp.ToolOption {
	return gomcp.WithToolAnnotation(gomcp.ToolAnnotation{
		Title:           title,
		ReadOnlyHint:    gomcp.ToBoolPtr(false),
		DestructiveHint: gomcp.ToBoolPtr(true),
		IdempotentHint:  gomcp.ToBoolPtr(true),
		OpenWorldHint:   gomcp.ToBoolPtr(true),
	})
}

package mcp

// annotationBoolPtr は bool リテラルへのポインタを返す小さなヘルパー。
// ToolAnnotation の *bool フィールドは「未設定」と「明示的に false」を
// 区別するために値ではなくポインタで保持する。
func annotationBoolPtr(b bool) *bool { return &b }

// readOnlyAnnotation は list/get/stats/digest/health などの参照系ツールに適用する。
// 環境を変更せず、retry しても副作用なし。
func readOnlyAnnotation(title string) ToolAnnotation {
	return ToolAnnotation{
		Title:          title,
		ReadOnlyHint:   annotationBoolPtr(true),
		IdempotentHint: annotationBoolPtr(true),
		OpenWorldHint:  annotationBoolPtr(true),
	}
}

// writeAnnotation は非破壊の書き込み系ツールに適用する。
// idempotent は create=false / update=true 等で切り替える。
func writeAnnotation(title string, idempotent bool) ToolAnnotation {
	return ToolAnnotation{
		Title:           title,
		ReadOnlyHint:    annotationBoolPtr(false),
		DestructiveHint: annotationBoolPtr(false),
		IdempotentHint:  annotationBoolPtr(idempotent),
		OpenWorldHint:   annotationBoolPtr(true),
	}
}

// destructiveAnnotation は delete 系など破壊的更新ツールに適用する。
func destructiveAnnotation(title string) ToolAnnotation {
	return ToolAnnotation{
		Title:           title,
		ReadOnlyHint:    annotationBoolPtr(false),
		DestructiveHint: annotationBoolPtr(true),
		IdempotentHint:  annotationBoolPtr(true),
		OpenWorldHint:   annotationBoolPtr(true),
	}
}

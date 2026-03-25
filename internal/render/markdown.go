package render

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

type MarkdownRenderer struct{}

func NewMarkdownRenderer() *MarkdownRenderer {
	return &MarkdownRenderer{}
}

func (r *MarkdownRenderer) Render(w io.Writer, data any) error {
	// JSON 経由で any に変換
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	switch val := v.(type) {
	case []any:
		return renderTable(w, val)
	case map[string]any:
		return renderKeyValueList(w, val)
	default:
		_, err := fmt.Fprintf(w, "%v\n", data)
		return err
	}
}

func renderTable(w io.Writer, rows []any) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "(データなし)")
		return err
	}

	// 最初の行からキーを取得
	firstRow, ok := rows[0].(map[string]any)
	if !ok {
		// map でない場合は JSON フォールバック
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}

	// キーの順序を安定させる（ソート）
	keys := make([]string, 0, len(firstRow))
	for k := range firstRow {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// ヘッダー行
	fmt.Fprintf(w, "| %s |\n", strings.Join(keys, " | "))
	// セパレーター行
	seps := make([]string, len(keys))
	for i := range seps {
		seps[i] = "----"
	}
	fmt.Fprintf(w, "| %s |\n", strings.Join(seps, " | "))

	// データ行
	for _, row := range rows {
		rowMap, ok := row.(map[string]any)
		if !ok {
			continue
		}
		vals := make([]string, len(keys))
		for i, k := range keys {
			vals[i] = formatCellValue(rowMap[k])
		}
		fmt.Fprintf(w, "| %s |\n", strings.Join(vals, " | "))
	}
	return nil
}

func renderKeyValueList(w io.Writer, obj map[string]any) error {
	// キーの順序を安定させる
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		val := formatCellValue(obj[k])
		fmt.Fprintf(w, "- **%s**: %s\n", k, val)
	}
	return nil
}

// formatCellValue はセルの値をフォーマットする。
// map で "name" キーがあればその値を表示、なければ JSON 文字列。
func formatCellValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case map[string]any:
		if name, ok := val["name"]; ok {
			return fmt.Sprintf("%v", name)
		}
		b, _ := json.Marshal(val)
		return string(b)
	case []any:
		if len(val) == 0 {
			return ""
		}
		// 配列要素が name を持つ map なら name のカンマ区切り
		names := make([]string, 0, len(val))
		for _, item := range val {
			if m, ok := item.(map[string]any); ok {
				if name, ok := m["name"]; ok {
					names = append(names, fmt.Sprintf("%v", name))
					continue
				}
			}
			b, _ := json.Marshal(item)
			names = append(names, string(b))
		}
		return strings.Join(names, ", ")
	case float64:
		// 整数なら小数点なしで表示
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case bool:
		return fmt.Sprintf("%v", val)
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

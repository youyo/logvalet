package render

import (
	"encoding/json"
	"io"
)

// GanttTableRenderer は Issue 専用のガントテーブル形式で出力するレンダラー。
type GanttTableRenderer struct {
	space string
}

// NewGanttTableRenderer は GanttTableRenderer を生成する。
func NewGanttTableRenderer(space string) *GanttTableRenderer {
	return &GanttTableRenderer{space: space}
}

// Render は data をガントテーブル形式で w に書き出す（スタブ）。
func (r *GanttTableRenderer) Render(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

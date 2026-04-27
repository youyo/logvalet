package mcp

import (
	"encoding/json"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
)

// TestToolCategories_CoversAllRegisteredTools は toolCategories マップと
// s.ListTools() が完全一致することを検証する（件数 42 を含む）。
func TestToolCategories_CoversAllRegisteredTools(t *testing.T) {
	s := NewServer(backlog.NewMockClient(), "test", ServerConfig{})
	tools := s.ListTools()

	// toolCategories に存在しないツールが登録されていないか
	for name := range tools {
		if _, ok := toolCategories[name]; !ok {
			t.Errorf("tool %q は登録されているが toolCategories にない", name)
		}
	}

	// toolCategories に存在するツールが登録されているか
	for name := range toolCategories {
		if _, ok := tools[name]; !ok {
			t.Errorf("toolCategories に %q があるが ListTools に存在しない", name)
		}
	}

	// 件数一致
	const expectedCount = 65
	if len(tools) != expectedCount {
		t.Errorf("ツール数: expected %d, got %d", expectedCount, len(tools))
	}
	if len(toolCategories) != expectedCount {
		t.Errorf("toolCategories 件数: expected %d, got %d", expectedCount, len(toolCategories))
	}
}

// TestToolAnnotations_MatchCategorySpec は各ツールの Annotations フィールドが
// toolCategories のカテゴリ仕様と一致することを検証する。
func TestToolAnnotations_MatchCategorySpec(t *testing.T) {
	s := NewServer(backlog.NewMockClient(), "test", ServerConfig{})
	tools := s.ListTools()

	for name, st := range tools {
		spec, ok := toolCategories[name]
		if !ok {
			// TestToolCategories_CoversAllRegisteredTools で検出されるので skip
			continue
		}
		ann := st.Tool.Annotations
		title := ann.Title

		if title == "" {
			t.Errorf("tool %q: Title が空", name)
		}
		if title != spec.Title {
			t.Errorf("tool %q: Title = %q, want %q", name, title, spec.Title)
		}

		assertBoolPtr := func(field string, got *bool, want bool) {
			t.Helper()
			if got == nil {
				t.Errorf("tool %q: %s が nil（%v を期待）", name, field, want)
				return
			}
			if *got != want {
				t.Errorf("tool %q: %s = %v, want %v", name, field, *got, want)
			}
		}

		switch spec.Category {
		case CategoryReadOnly:
			assertBoolPtr("ReadOnlyHint", ann.ReadOnlyHint, true)
			assertBoolPtr("IdempotentHint", ann.IdempotentHint, true)
			assertBoolPtr("OpenWorldHint", ann.OpenWorldHint, true)
		case CategoryWriteNonIdempotent:
			assertBoolPtr("ReadOnlyHint", ann.ReadOnlyHint, false)
			assertBoolPtr("DestructiveHint", ann.DestructiveHint, false)
			assertBoolPtr("IdempotentHint", ann.IdempotentHint, false)
			assertBoolPtr("OpenWorldHint", ann.OpenWorldHint, true)
		case CategoryWriteIdempotent:
			assertBoolPtr("ReadOnlyHint", ann.ReadOnlyHint, false)
			assertBoolPtr("DestructiveHint", ann.DestructiveHint, false)
			assertBoolPtr("IdempotentHint", ann.IdempotentHint, true)
			assertBoolPtr("OpenWorldHint", ann.OpenWorldHint, true)
		case CategoryDestructive:
			assertBoolPtr("ReadOnlyHint", ann.ReadOnlyHint, false)
			assertBoolPtr("DestructiveHint", ann.DestructiveHint, true)
			assertBoolPtr("IdempotentHint", ann.IdempotentHint, true)
			assertBoolPtr("OpenWorldHint", ann.OpenWorldHint, true)
		}
	}
}

// TestToolAnnotations_JSONSerialization は json.Marshal で annotations キーが
// 正しく出力されることを検証する（MCP クライアントが読む JSON 構造の回帰テスト）。
func TestToolAnnotations_JSONSerialization(t *testing.T) {
	s := NewServer(backlog.NewMockClient(), "test", ServerConfig{})
	tools := s.ListTools()

	for name, st := range tools {
		spec, ok := toolCategories[name]
		if !ok {
			continue
		}

		jsonBytes, err := json.Marshal(st.Tool)
		if err != nil {
			t.Fatalf("tool %q: json.Marshal error: %v", name, err)
		}

		var m map[string]any
		if err := json.Unmarshal(jsonBytes, &m); err != nil {
			t.Fatalf("tool %q: json.Unmarshal error: %v", name, err)
		}

		ann, ok := m["annotations"]
		if !ok {
			t.Errorf("tool %q: JSON に annotations キーがない", name)
			continue
		}

		annMap, ok := ann.(map[string]any)
		if !ok {
			t.Errorf("tool %q: annotations が map でない: %T", name, ann)
			continue
		}

		// Title が存在し非空
		titleVal, ok := annMap["title"]
		if !ok || titleVal == "" {
			t.Errorf("tool %q: annotations.title が空または存在しない", name)
		}

		// カテゴリ別に readOnlyHint を確認
		switch spec.Category {
		case CategoryReadOnly:
			if v, ok := annMap["readOnlyHint"]; !ok || v != true {
				t.Errorf("tool %q (ReadOnly): annotations.readOnlyHint = %v, want true", name, v)
			}
		case CategoryDestructive:
			if v, ok := annMap["destructiveHint"]; !ok || v != true {
				t.Errorf("tool %q (Destructive): annotations.destructiveHint = %v, want true", name, v)
			}
		}
	}
}

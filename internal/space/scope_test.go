package space_test

import (
	"testing"

	"github.com/youyo/logvalet/internal/space"
)

// T7: Scope のゼロ値検証
func TestScope_ZeroValue(t *testing.T) {
	var s space.Scope
	if s.AllSpaces != false {
		t.Errorf("AllSpaces should be false by default")
	}
	if s.Aliases != nil {
		t.Errorf("Aliases should be nil by default, got %v", s.Aliases)
	}
}

// T8: Scope は AllSpaces と Aliases を同時に保持できることを確認
// バリデーション（AllSpaces=true かつ len(Aliases)>0 はエラー）は Resolver 側で行う。
// Scope 型自体は両フィールドを保持できる（struct の設計確認）。
func TestScope_AllSpaces_And_Aliases_MutualExclusion(t *testing.T) {
	s := space.Scope{
		AllSpaces: true,
		Aliases:   []string{"foo"},
	}
	// Scope 型は両方のフィールドを保持できる（バリデーションは Resolver で行う）
	if !s.AllSpaces {
		t.Errorf("AllSpaces should be true")
	}
	if len(s.Aliases) != 1 || s.Aliases[0] != "foo" {
		t.Errorf("Aliases should be [\"foo\"], got %v", s.Aliases)
	}
}

package main

import (
	"slices"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

func newTestParser(t *testing.T) *kong.Kong {
	t.Helper()
	var root cli.CLI
	parser, err := kong.New(&root,
		kong.Name("logvalet"),
		kong.Exit(func(code int) {}),
	)
	if err != nil {
		t.Fatalf("kong.New failed: %v", err)
	}
	return parser
}

func sliceContains(s []string, v string) bool {
	return slices.Contains(s, v)
}

func TestCollectCompletions(t *testing.T) {
	t.Run("C1: リーフノードでフラグが出力される", func(t *testing.T) {
		parser := newTestParser(t)
		got, ok := collectCompletions(parser, []string{"--completion-bash", "user", "list"})
		if !ok {
			t.Fatal("true が返されるべき")
		}
		if !sliceContains(got, "--offset") {
			t.Errorf("--offset が含まれていない: %v", got)
		}
		if !sliceContains(got, "--count") {
			t.Errorf("--count が含まれていない: %v", got)
		}
	})

	t.Run("C2: 親フラグ（GlobalFlags）も含まれる", func(t *testing.T) {
		parser := newTestParser(t)
		got, ok := collectCompletions(parser, []string{"--completion-bash", "user", "list"})
		if !ok {
			t.Fatal("true が返されるべき")
		}
		if !sliceContains(got, "--format") {
			t.Errorf("--format が含まれていない: %v", got)
		}
		if !sliceContains(got, "--profile") {
			t.Errorf("--profile が含まれていない: %v", got)
		}
	})

	t.Run("C3: サブコマンドとフラグが両方出力される", func(t *testing.T) {
		parser := newTestParser(t)
		got, ok := collectCompletions(parser, []string{"--completion-bash", "issue", ""})
		if !ok {
			t.Fatal("true が返されるべき")
		}
		// サブコマンド候補
		if !sliceContains(got, "list") {
			t.Errorf("list が含まれていない: %v", got)
		}
		// グローバルフラグ
		if !sliceContains(got, "--format") {
			t.Errorf("--format が含まれていない: %v", got)
		}
	})

	t.Run("C4: トップレベルのサブコマンド（既存動作維持）", func(t *testing.T) {
		parser := newTestParser(t)
		got, ok := collectCompletions(parser, []string{"--completion-bash", ""})
		if !ok {
			t.Fatal("true が返されるべき")
		}
		if !sliceContains(got, "auth") {
			t.Errorf("auth が含まれていない: %v", got)
		}
		if !sliceContains(got, "issue") {
			t.Errorf("issue が含まれていない: %v", got)
		}
		if !sliceContains(got, "user") {
			t.Errorf("user が含まれていない: %v", got)
		}
	})

	t.Run("E1: --completion-bash なし", func(t *testing.T) {
		parser := newTestParser(t)
		got, ok := collectCompletions(parser, []string{"auth", "login"})
		if ok {
			t.Error("false が返されるべき")
		}
		if got != nil {
			t.Errorf("nil が返されるべき: %v", got)
		}
	})

	t.Run("E2: -- プレフィクスでフラグのみ（サブコマンドなし）", func(t *testing.T) {
		parser := newTestParser(t)
		got, ok := collectCompletions(parser, []string{"--completion-bash", "user", "list", "--"})
		if !ok {
			t.Fatal("true が返されるべき")
		}
		for _, c := range got {
			if len(c) < 2 || c[:2] != "--" {
				t.Errorf("フラグ以外の候補が含まれている: %q in %v", c, got)
			}
		}
	})

	t.Run("E3: プレフィクスマッチ --c で始まるフラグのみ", func(t *testing.T) {
		parser := newTestParser(t)
		got, ok := collectCompletions(parser, []string{"--completion-bash", "user", "list", "--c"})
		if !ok {
			t.Fatal("true が返されるべき")
		}
		if !sliceContains(got, "--count") {
			t.Errorf("--count が含まれていない: %v", got)
		}
		// --format は --c で始まらないので含まれないはず
		if sliceContains(got, "--format") {
			t.Errorf("--format は --c プレフィクスにマッチしないはず: %v", got)
		}
		// 全て --c で始まること
		for _, c := range got {
			if len(c) < 3 || c[:3] != "--c" {
				t.Errorf("--c で始まらない候補が含まれている: %q in %v", c, got)
			}
		}
	})
}

func TestHandleCompletionBash(t *testing.T) {
	t.Run("--completion-bash がない場合は false を返す", func(t *testing.T) {
		parser := newTestParser(t)
		if handleCompletionBash(parser, []string{"auth", "login"}) {
			t.Error("--completion-bash がないのに true が返された")
		}
	})

	t.Run("--completion-bash でトップレベルコマンドを返す", func(t *testing.T) {
		parser := newTestParser(t)
		got := handleCompletionBash(parser, []string{"--completion-bash", ""})
		if !got {
			t.Fatal("true が返されるべき")
		}
		// stdout に出力されるので、関数が true を返すことだけ確認
	})

	t.Run("--completion-bash auth でサブコマンドを返す", func(t *testing.T) {
		parser := newTestParser(t)
		got := handleCompletionBash(parser, []string{"--completion-bash", "auth"})
		if !got {
			t.Fatal("true が返されるべき")
		}
	})
}

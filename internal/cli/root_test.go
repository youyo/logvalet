package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/youyo/logvalet/internal/cli"
)

func TestRootCLI_Parse(t *testing.T) {
	t.Run("--help が解析できる", func(t *testing.T) {
		var root cli.CLI
		p, err := kong.New(&root,
			kong.Name("logvalet"),
			kong.Description("Backlog 向け LLM-first CLI"),
			kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
			kong.Exit(func(int) {}), // テスト内でのos.Exit防止
		)
		if err != nil {
			t.Fatalf("kong.New() エラー: %v", err)
		}
		// --help は Exit を呼ぶので HelpExit を使う
		_, err = p.Parse([]string{"--help"})
		// help は Kong の特殊処理のためエラーが返される場合がある
		// ここでは panic せず処理できることを確認
		_ = err
	})

	t.Run("issue サブコマンドが認識される", func(t *testing.T) {
		var root cli.CLI
		p, err := kong.New(&root,
			kong.Name("logvalet"),
			kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
			kong.Exit(func(int) {}),
		)
		if err != nil {
			t.Fatalf("kong.New() エラー: %v", err)
		}
		_, err = p.Parse([]string{"issue", "--help"})
		_ = err
	})

	t.Run("未知のコマンドはエラーを返す", func(t *testing.T) {
		var root cli.CLI
		p, err := kong.New(&root,
			kong.Name("logvalet"),
			kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
			kong.Exit(func(int) {}),
		)
		if err != nil {
			t.Fatalf("kong.New() エラー: %v", err)
		}
		_, err = p.Parse([]string{"nonexistent-command"})
		if err == nil {
			t.Error("未知コマンドでエラーが返されなかった")
		}
	})
}

func TestRootCLI_VersionCommand(t *testing.T) {
	t.Run("version サブコマンドが認識される", func(t *testing.T) {
		var root cli.CLI
		p, err := kong.New(&root,
			kong.Name("logvalet"),
			kong.Writers(bytes.NewBuffer(nil), bytes.NewBuffer(nil)),
			kong.Exit(func(int) {}),
		)
		if err != nil {
			t.Fatalf("kong.New() エラー: %v", err)
		}
		_, err = p.Parse([]string{"version"})
		if err != nil {
			t.Errorf("version サブコマンドのパースに失敗: %v", err)
		}
	})
}

func TestRootCLI_VersionFlag(t *testing.T) {
	t.Run("--version フラグが認識される", func(t *testing.T) {
		var root cli.CLI
		var exitCalled bool
		stdout := bytes.NewBuffer(nil)
		p, err := kong.New(&root,
			kong.Name("logvalet"),
			kong.Writers(stdout, bytes.NewBuffer(nil)),
			kong.Exit(func(int) { exitCalled = true }),
			kong.Vars{"version": "test-version"},
		)
		if err != nil {
			t.Fatalf("kong.New() エラー: %v", err)
		}
		_, _ = p.Parse([]string{"--version"})
		if !exitCalled {
			t.Error("--version で Exit が呼ばれなかった")
		}
		output := stdout.String()
		if !strings.Contains(output, "test-version") {
			t.Errorf("--version 出力にバージョンが含まれていない: %s", output)
		}
	})
}

func TestNotImplementedError(t *testing.T) {
	t.Run("ErrNotImplemented がエラーを返す", func(t *testing.T) {
		err := cli.ErrNotImplemented("issue get")
		if err == nil {
			t.Fatal("ErrNotImplemented は nil を返してはならない")
		}
		if !strings.Contains(err.Error(), "not implemented") {
			t.Errorf("エラーメッセージに 'not implemented' が含まれていない: %v", err)
		}
	})
}

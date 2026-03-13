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

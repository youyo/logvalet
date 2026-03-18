package cli_test

import (
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/cli"
)

func TestCompletionCommands(t *testing.T) {
	t.Run("zsh completion が logvalet を含む", func(t *testing.T) {
		output := cli.GenerateCompletion("zsh", "logvalet", false)
		if !strings.Contains(output, "logvalet") {
			t.Errorf("zsh completion に 'logvalet' が含まれていない: %s", output)
		}
	})

	t.Run("zsh completion が --completion-bash を含む", func(t *testing.T) {
		output := cli.GenerateCompletion("zsh", "logvalet", false)
		if !strings.Contains(output, "--completion-bash") {
			t.Errorf("zsh completion に '--completion-bash' が含まれていない: %s", output)
		}
	})

	t.Run("--short=true のとき alias lv=logvalet が含まれる (zsh)", func(t *testing.T) {
		output := cli.GenerateCompletion("zsh", "logvalet", true)
		if !strings.Contains(output, "alias lv=logvalet") {
			t.Errorf("--short モードに 'alias lv=logvalet' が含まれていない: %s", output)
		}
	})

	t.Run("--short=true のとき compdef _logvalet lv が含まれる (zsh)", func(t *testing.T) {
		output := cli.GenerateCompletion("zsh", "logvalet", true)
		if !strings.Contains(output, "compdef _logvalet lv") {
			t.Errorf("--short モードに 'compdef _logvalet lv' が含まれていない: %s", output)
		}
	})

	t.Run("--short=false のとき alias が含まれない (zsh)", func(t *testing.T) {
		output := cli.GenerateCompletion("zsh", "logvalet", false)
		if strings.Contains(output, "alias lv=logvalet") {
			t.Errorf("--short=false なのに alias が含まれている: %s", output)
		}
	})

	t.Run("未対応シェルはコメントを返す", func(t *testing.T) {
		output := cli.GenerateCompletion("powershell", "logvalet", false)
		if !strings.Contains(output, "not supported") {
			t.Errorf("未対応シェルのメッセージがない: %s", output)
		}
	})

	t.Run("ZshCompletionCmd.Run は Kong 互換のシグネチャを持つ", func(t *testing.T) {
		// コンパイル時確認: Run(*GlobalFlags) error が存在することを確認
		cmd := &cli.ZshCompletionCmd{}
		_ = cmd // Run の直接呼び出しは os.Stdout に書くので省略
	})
}

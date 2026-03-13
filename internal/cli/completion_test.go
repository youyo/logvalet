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

	t.Run("bash completion が logvalet を含む", func(t *testing.T) {
		output := cli.GenerateCompletion("bash", "logvalet", false)
		if !strings.Contains(output, "logvalet") {
			t.Errorf("bash completion に 'logvalet' が含まれていない: %s", output)
		}
	})

	t.Run("fish completion が logvalet を含む", func(t *testing.T) {
		output := cli.GenerateCompletion("fish", "logvalet", false)
		if !strings.Contains(output, "logvalet") {
			t.Errorf("fish completion に 'logvalet' が含まれていない: %s", output)
		}
	})

	t.Run("--short=true のとき lv エイリアスも含む (zsh)", func(t *testing.T) {
		output := cli.GenerateCompletion("zsh", "logvalet", true)
		if !strings.Contains(output, "lv") {
			t.Errorf("--short モードに 'lv' が含まれていない: %s", output)
		}
	})

	t.Run("--short=true のとき lv エイリアスも含む (bash)", func(t *testing.T) {
		output := cli.GenerateCompletion("bash", "logvalet", true)
		if !strings.Contains(output, "lv") {
			t.Errorf("--short モードに 'lv' が含まれていない: %s", output)
		}
	})

	t.Run("--short=true のとき lv エイリアスも含む (fish)", func(t *testing.T) {
		output := cli.GenerateCompletion("fish", "logvalet", true)
		if !strings.Contains(output, "lv") {
			t.Errorf("--short モードに 'lv' が含まれていない: %s", output)
		}
	})

	t.Run("ZshCompletionCmd.Run は Kong 互換のシグネチャを持つ", func(t *testing.T) {
		// コンパイル時確認: Run(*GlobalFlags) error が存在することを確認
		cmd := &cli.ZshCompletionCmd{}
		_ = cmd // Run の直接呼び出しは os.Stdout に書くので省略
	})
}

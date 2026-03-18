package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/cli"
)

func TestVersionCmd_JSON(t *testing.T) {
	var buf bytes.Buffer
	cmd := cli.VersionCmd{Stdout: &buf}
	g := &cli.GlobalFlags{Format: "json"}

	if err := cmd.Run(g); err != nil {
		t.Fatalf("Run() エラー: %v", err)
	}

	var m map[string]string
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("JSON パースエラー: %v, output: %s", err, buf.String())
	}

	for _, key := range []string{"version", "commit", "date"} {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON に %q キーが存在しない", key)
		}
	}
}

func TestVersionCmd_PrettyJSON(t *testing.T) {
	var buf bytes.Buffer
	cmd := cli.VersionCmd{Stdout: &buf}
	g := &cli.GlobalFlags{Format: "json", Pretty: true}

	if err := cmd.Run(g); err != nil {
		t.Fatalf("Run() エラー: %v", err)
	}

	output := buf.String()
	// Pretty JSON はインデントを含む
	if !strings.Contains(output, "\n") {
		t.Errorf("Pretty JSON に改行が含まれていない: %s", output)
	}
}

func TestVersionCmd_YAML(t *testing.T) {
	var buf bytes.Buffer
	cmd := cli.VersionCmd{Stdout: &buf}
	g := &cli.GlobalFlags{Format: "yaml"}

	if err := cmd.Run(g); err != nil {
		t.Fatalf("Run() エラー: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "version:") {
		t.Errorf("YAML 出力に version: が含まれていない: %s", output)
	}
	if !strings.Contains(output, "commit:") {
		t.Errorf("YAML 出力に commit: が含まれていない: %s", output)
	}
}

func TestVersionCmd_DefaultFormat(t *testing.T) {
	var buf bytes.Buffer
	cmd := cli.VersionCmd{Stdout: &buf}
	g := &cli.GlobalFlags{} // Format 未指定

	if err := cmd.Run(g); err != nil {
		t.Fatalf("Run() エラー: %v", err)
	}

	// デフォルトは JSON
	var m map[string]string
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("デフォルト出力が JSON でない: %v, output: %s", err, buf.String())
	}
}

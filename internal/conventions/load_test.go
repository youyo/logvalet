package conventions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_UnknownFieldReturnsError(t *testing.T) {
	_, err := Load([]byte("schema_version: 1\nproject:\n  key: DEMO\n  typo: true\n"))
	if err == nil {
		t.Fatal("未知フィールドでエラーになりませんでした")
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "conventions.yaml")
	if err := os.WriteFile(path, []byte("schema_version: 1\nproject:\n  key: FILE\n"), 0o600); err != nil {
		t.Fatalf("テストファイルの作成に失敗しました: %v", err)
	}

	c, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile に失敗しました: %v", err)
	}
	if c.Project.Key != "FILE" {
		t.Fatalf("project.key = %q, want %q", c.Project.Key, "FILE")
	}
}

func TestLoadFile_MissingPathReturnsError(t *testing.T) {
	_, err := LoadFile(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("存在しないパスでエラーになりませんでした")
	}
}

func TestLoadFromIssueDescription_OneYAMLBlock(t *testing.T) {
	description := "運用ガイド\n\n```yaml\nschema_version: 1\nproject:\n  key: ONE\n```\n"

	c, err := LoadFromIssueDescription(description)
	if err != nil {
		t.Fatalf("YAML ブロックのロードに失敗しました: %v", err)
	}
	if c.Project.Key != "ONE" {
		t.Fatalf("project.key = %q, want %q", c.Project.Key, "ONE")
	}
}

func TestLoadFromIssueDescription_NoBlockReturnsError(t *testing.T) {
	_, err := LoadFromIssueDescription("運用ガイドのみ")
	if err == nil {
		t.Fatal("YAML ブロックなしでエラーになりませんでした")
	}
}

func TestLoadFromIssueDescription_UsesLastBlock(t *testing.T) {
	description := "```yaml\nschema_version: 1\nproject:\n  key: FIRST\n```\n\n```yaml\nschema_version: 1\nproject:\n  key: LAST\n```"

	c, err := LoadFromIssueDescription(description)
	if err != nil {
		t.Fatalf("複数 YAML ブロックのロードに失敗しました: %v", err)
	}
	if c.Project.Key != "LAST" {
		t.Fatalf("最後の project.key = %q, want %q", c.Project.Key, "LAST")
	}
}

func TestLoadFromIssueDescription_UnspecifiedLanguageIsIgnored(t *testing.T) {
	description := "```\nschema_version: 1\nproject:\n  key: NOPE\n```"
	if _, err := LoadFromIssueDescription(description); err == nil {
		t.Fatal("言語指定なしのフェンスを対象としてしまいました")
	}
}

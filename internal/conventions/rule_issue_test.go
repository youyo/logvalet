package conventions

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildRuleIssueDescription_RoundTripsConventions(t *testing.T) {
	yamlSource := []byte("schema_version: 1\nproject:\n  key: PROJ\n  name: Project\npriority:\n  high: |\n    high\n  normal: normal\n  low: low\nclose_policy:\n  low_untouched_days: 90\nstatuses:\n  - name: レビュー中\n    color: '#2da44e'\nissue_types:\n  - name: 規約\n    color: '#6f42c1'\n  - name: 案件\n    color: '#0969da'\n    template_description: |\n      Context & Goals\n      Scope\ninitiatives:\n  - name: 運用保守\n    description: 定常業務\nengagements:\n  - name: 顧客A\n    lead: 山田 太郎\n    initiative: 運用保守\n    start_date: 2026-10-01\n    due_date: 2026-10-31\n")
	want, err := Load(yamlSource)
	if err != nil {
		t.Fatalf("fixture Load() error = %v", err)
	}
	got, err := LoadFromIssueDescription(BuildRuleIssueDescription(yamlSource))
	if err != nil {
		t.Fatalf("LoadFromIssueDescription() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\n got %#v\nwant %#v", got, want)
	}
}

func TestBuildRuleIssueDescription_EscapesTablePipesAndEmbedsSource(t *testing.T) {
	source := []byte("schema_version: 1\nproject:\n  key: PROJ\n")
	description := BuildRuleIssueDescription(source)
	if !strings.Contains(description, "このプロジェクトの運用規約です。logvalet が読み書きするため、YAML ブロックは手で崩さないでください。") {
		t.Fatal("運用ガイドがありません")
	}
	if !strings.Contains(description, "Backlog 上の実体") || !strings.Contains(description, "| 規約課題（入力は conventions.yaml） |\n") {
		t.Fatal("用語集の Markdown 表がありません")
	}
	if got := escapeMarkdownTableCell("a|b"); got != `a\|b` {
		t.Fatalf("table pipe escape = %q, want %q", got, `a\|b`)
	}
	if !strings.Contains(description, "```yaml\n"+string(source)+"```") {
		t.Fatal("YAML コードブロックがありません")
	}
}

package conventions

import (
	"fmt"
	"strings"
)

// RuleIssueSummary は規約課題の件名。
const RuleIssueSummary = "[規約] 運用規約"

// BuildRuleIssueDescription は規約課題の説明欄を組み立てる。
// 人間向けの運用ガイドと用語集のあとに、conventions.yaml を yaml コードブロックで埋め込む。
// LoadFromIssueDescription が最後の yaml ブロックを読むため、この形式で往復できる。
func BuildRuleIssueDescription(yamlSource []byte) string {
	var builder strings.Builder
	builder.WriteString("このプロジェクトの運用規約です。logvalet が読み書きするため、YAML ブロックは手で崩さないでください。\n")
	builder.WriteString("規約を変えるときは conventions.yaml を直して `logvalet project apply` を実行するか、\n")
	builder.WriteString("この課題を直接編集してください。変更履歴は課題の更新履歴に残ります。\n\n")
	builder.WriteString("## 用語\n\n")
	builder.WriteString("| 用語 | 意味 | Backlog 上の実体 |\n")
	builder.WriteString("|---|---|---|\n")
	for _, entry := range Glossary() {
		fmt.Fprintf(&builder, "| %s | %s | %s |\n", escapeMarkdownTableCell(entry.Term), escapeMarkdownTableCell(entry.Meaning), escapeMarkdownTableCell(entry.BacklogForm))
	}
	builder.WriteString("\n## 規約 (conventions.yaml)\n\n```yaml\n")
	builder.Write(yamlSource)
	if len(yamlSource) == 0 || yamlSource[len(yamlSource)-1] != '\n' {
		builder.WriteByte('\n')
	}
	builder.WriteString("```\n")
	return builder.String()
}

func escapeMarkdownTableCell(value string) string {
	return strings.ReplaceAll(value, "|", `\|`)
}

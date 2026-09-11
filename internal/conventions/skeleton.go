package conventions

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	"gopkg.in/yaml.v3"
)

const defaultEngagementTemplate = `## Context & Goals
（なぜやるのか、何を達成するのか）
## Scope
（やること / やらないこと）
## Acceptance criteria
（何をもって完了とするか）
`

const unclassifiedInitiativeDescription = "自動生成の仮置き。案件を実際のテーマに割り当て直すこと"

const skeletonTemplate = `# logvalet conventions: Linear の思想を Backlog の語彙に翻訳した運用規約。
# 値の説明に見えて、書いているのは組織の優先順位に対する態度。埋めるのは自分たちの言葉。
schema_version: 1

project:
  # 適用先の Backlog プロジェクトキー。--project フラグで上書きできる。
  key: {{yamlScalar .ProjectKey 4}}
  # --create で新規作成するときの表示名。既存プロジェクトへの適用では無視される。
  name: {{yamlScalar .ProjectName 4}}

# 優先度の意味。Backlog は 高・中・低 の 3 段階固定なので段階は足さず、意味を定義する。
# 案件（engagements）との相対で書く。Initiative に紐づかない課題で「中」が増えるのは
# 優先度を決めきれていない裏返しとして扱う。
priority:
  high: "契約・SLA 上、他案件を止めてでも対応する"
  normal: "案件と同じ優先度。必ず担当者を割り当て、担当者が責任を持つ"
  low: "案件より劣後し、実行は保証されない。進むには担当者の熱量か 20% ルール的な仕組みが必要"

# クローズも決断のうち。低優先度を溜め続けないための規約。
close_policy:
  # この日数を超えて未着手の「低」課題を project health でクローズ候補として挙げる。
  low_untouched_days: 90

# 既定の 4 状態（未対応・処理中・処理済み・完了）に追加する状態。
# 追加できるのは最大 8 個で、スペース管理者権限が必要。不要なら空リストにする。
# color は Backlog が許可する状態用の色コードからのみ選べる。
statuses:{{if .Statuses}}
{{range .Statuses}}  - name: {{yamlScalar .Name 6}}
    color: {{yamlScalar .Color 6}}
{{end}}{{else}} []{{end}}
# 課題種別。「規約」はこの規約を保存する規約課題、「案件」は案件のヘッダー（親課題）に使う。
# どちらも必須。color は Backlog が許可する種別用の色コードからのみ選べる。
issue_types:{{if .IssueTypes}}
{{range .IssueTypes}}  - name: {{yamlScalar .Name 6}}
    color: {{yamlScalar .Color 6}}
{{if .TemplateSummary}}    # 種別ごとの要約を定型化し、起票時の判断を揃える。
    template_summary: {{yamlScalar .TemplateSummary 6}}
{{end}}{{if .TemplateDescription}}    # 案件親課題を起票するときに説明欄へ自動挿入されるテンプレート。
    # Context & Goals / Scope / Acceptance criteria を埋めないと案件を始められない構造にする。
    template_description: {{yamlScalar .TemplateDescription 6}}
{{end}}{{end}}{{else}} []{{end}}
# Initiative: 数か月規模の重点テーマ。Backlog には対応する概念がないので、この一覧で持つ。
# 並び順がそのまま優先度。横断テーマと顧客テーマのどちらが上かを明示せざるを得ない。
# 案件は必ずいずれかの Initiative に属する。定常業務も「運用保守」のように明示して置く。
initiatives:{{if .Initiatives}}
{{range .Initiatives}}  - name: {{yamlScalar .Name 6}}
    description: {{yamlScalar .Description 6}}
{{end}}{{else}} []{{end}}
# 案件: 数週間規模の取り組み。1 件ごとにカテゴリと「案件」種別の親課題を作る。
# 課題（子課題）は案件カテゴリをちょうど 1 つ持ち、案件親課題の子にする。
# 書き方:
#   - name: 顧客A 基盤更改        # 案件名。カテゴリ名と親課題の件名になる
#     lead: 山田 太郎             # Backlog 上の表示名。1 人だけ。空欄のまま apply すると警告
#     initiative: 運用保守        # initiatives[].name を参照する
#     start_date: "2026-10-01"    # 親課題の開始日。YYYY-MM-DD
#     due_date: "2026-10-31"      # 親課題の期限日。YYYY-MM-DD
engagements:{{if .Engagements}}
{{range .Engagements}}  - name: {{yamlScalar .Name 6}}
    lead: {{yamlScalar .Lead 6}}
    initiative: {{yamlScalar .Initiative 6}}
    start_date: {{yamlScalar .StartDate 6}}
    due_date: {{yamlScalar .DueDate 6}}
{{end}}{{else}} []{{end}}
`

type skeletonTemplateData struct {
	ProjectKey  string
	ProjectName string
	Statuses    []Status
	IssueTypes  []IssueType
	Initiatives []Initiative
	Engagements []Engagement
}

// Skeleton は既定の conventions スケルトンを返す。全項目にコメントが付く。
func Skeleton() ([]byte, error) {
	return renderSkeleton(skeletonTemplateData{
		ProjectKey:  "SANDBOX",
		ProjectName: "Sandbox",
		Statuses: []Status{{
			Name:  "レビュー中",
			Color: defaultStatusColor,
		}},
		IssueTypes: []IssueType{
			{Name: IssueTypeRule, Color: defaultIssueTypeColor},
			{Name: IssueTypeEngagement, Color: defaultEngagementColor, TemplateDescription: defaultEngagementTemplate},
		},
		Initiatives: []Initiative{{
			Name:        "運用保守",
			Description: "契約範囲内の定常対応。案件が属する Initiative を決めきれないときの逃げ場にしない",
		}},
		Engagements: []Engagement{},
	})
}

// BuildSkeletonFromProject は既存 Backlog プロジェクトの内容を反映したスケルトンを返す。
// 既存プロジェクトへ規約を導入するときの起点になる。
func BuildSkeletonFromProject(ctx context.Context, client backlog.Client, projectKey string) ([]byte, error) {
	if client == nil {
		return nil, fmt.Errorf("Backlog client が nil です")
	}

	project, err := client.GetProject(ctx, projectKey)
	if err != nil {
		return nil, fmt.Errorf("Backlog プロジェクトの取得に失敗しました: %w", err)
	}
	if project == nil {
		return nil, fmt.Errorf("Backlog プロジェクトの取得結果が nil です")
	}

	issueTypes, err := client.ListProjectIssueTypes(ctx, projectKey)
	if err != nil {
		return nil, fmt.Errorf("Backlog 課題種別の取得に失敗しました: %w", err)
	}
	statuses, err := client.ListProjectStatuses(ctx, projectKey)
	if err != nil {
		return nil, fmt.Errorf("Backlog 状態の取得に失敗しました: %w", err)
	}
	categories, err := client.ListProjectCategories(ctx, projectKey)
	if err != nil {
		return nil, fmt.Errorf("Backlog カテゴリの取得に失敗しました: %w", err)
	}

	data := skeletonTemplateData{
		ProjectKey:  project.ProjectKey,
		ProjectName: project.Name,
		Statuses:    conventionsStatuses(statuses),
		IssueTypes:  conventionsIssueTypes(issueTypes),
		Initiatives: []Initiative{{Name: "未分類", Description: unclassifiedInitiativeDescription}},
		Engagements: conventionsEngagements(categories),
	}
	data.IssueTypes = appendRequiredIssueTypes(data.IssueTypes)
	return renderSkeleton(data)
}

func renderSkeleton(data skeletonTemplateData) ([]byte, error) {
	tmpl, err := template.New("conventions-skeleton").
		Funcs(template.FuncMap{"yamlScalar": yamlScalar}).
		Option("missingkey=error").
		Parse(skeletonTemplate)
	if err != nil {
		return nil, fmt.Errorf("conventions スケルトンテンプレートの解析に失敗しました: %w", err)
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		return nil, fmt.Errorf("conventions スケルトンの生成に失敗しました: %w", err)
	}
	result := output.Bytes()
	conventions, err := Load(result)
	if err != nil {
		return nil, fmt.Errorf("生成した conventions スケルトンの自己検査に失敗しました: %w", err)
	}
	if violations := Validate(conventions); HasError(violations) {
		return nil, fmt.Errorf("生成した conventions スケルトンに error violation があります: %v", violations)
	}
	return result, nil
}

// yamlScalar は任意の文字列を YAML のスカラー値として安全に表現する。
// 単一行は yaml.Marshal に引用・エスケープを任せ、改行を含む場合は指定インデントのブロックスカラーにする。
func yamlScalar(s string, indent int) string {
	encoded, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Sprintf("%q", s)
	}
	value := strings.TrimSuffix(string(encoded), "\n")
	if !strings.Contains(s, "\n") {
		return value
	}

	if indent < 0 {
		indent = 0
	}
	indicator := blockIndicator(value, s)
	lines := strings.Split(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	var result strings.Builder
	result.WriteString(indicator)
	result.WriteByte('\n')
	for _, line := range lines {
		result.WriteString(strings.Repeat(" ", indent))
		result.WriteString(line)
		result.WriteByte('\n')
	}
	return strings.TrimSuffix(result.String(), "\n")
}

func blockIndicator(encoded, value string) string {
	firstLine, _, _ := strings.Cut(encoded, "\n")
	if strings.HasPrefix(firstLine, "|") {
		return firstLine
	}
	trailingNewlines := len(value) - len(strings.TrimRight(value, "\n"))
	if trailingNewlines == 0 {
		return "|-"
	}
	if trailingNewlines == 1 {
		return "|"
	}
	return "|+"
}

func conventionsIssueTypes(values []domain.IssueType) []IssueType {
	sorted := append([]domain.IssueType(nil), values...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].DisplayOrder != sorted[j].DisplayOrder {
			return sorted[i].DisplayOrder < sorted[j].DisplayOrder
		}
		return sorted[i].ID < sorted[j].ID
	})

	result := make([]IssueType, 0, len(sorted))
	for _, value := range sorted {
		result = append(result, IssueType{
			Name:                value.Name,
			Color:               fallbackIssueTypeColor(value.Color),
			TemplateSummary:     value.TemplateSummary,
			TemplateDescription: value.TemplateDescription,
		})
	}
	return result
}

const (
	// defaultIssueTypeColor は規約課題に使う既定の課題種別色。
	defaultIssueTypeColor = "#666665"
	// defaultEngagementColor は案件親課題に使う既定の課題種別色。
	defaultEngagementColor = "#7ea800"
	// defaultStatusColor はスケルトンが例示する既定の状態色。
	defaultStatusColor = "#e87758"
)

// fallbackIssueTypeColor は allowlist 外の色を既定色に置き換える。
// Backlog は allowlist 外の色を 400 で拒否するため、そのまま書き出すと apply が必ず失敗する。
func fallbackIssueTypeColor(color string) string {
	if IsValidIssueTypeColor(color) {
		return color
	}
	return defaultIssueTypeColor
}

// fallbackStatusColor は allowlist 外の色を既定色に置き換える。
func fallbackStatusColor(color string) string {
	if IsValidStatusColor(color) {
		return color
	}
	return defaultStatusColor
}

func conventionsStatuses(values []domain.Status) []Status {
	filtered := make([]domain.Status, 0, len(values))
	for _, value := range values {
		if value.ID >= 1 && value.ID <= 4 {
			continue
		}
		filtered = append(filtered, value)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].DisplayOrder != filtered[j].DisplayOrder {
			return filtered[i].DisplayOrder < filtered[j].DisplayOrder
		}
		return filtered[i].ID < filtered[j].ID
	})

	result := make([]Status, 0, len(filtered))
	for _, value := range filtered {
		result = append(result, Status{Name: value.Name, Color: fallbackStatusColor(value.Color)})
	}
	return result
}

func conventionsEngagements(values []domain.Category) []Engagement {
	sorted := append([]domain.Category(nil), values...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].DisplayOrder != sorted[j].DisplayOrder {
			return sorted[i].DisplayOrder < sorted[j].DisplayOrder
		}
		return sorted[i].ID < sorted[j].ID
	})

	result := make([]Engagement, 0, len(sorted))
	for _, value := range sorted {
		result = append(result, Engagement{
			Name:       value.Name,
			Initiative: "未分類",
		})
	}
	return result
}

func appendRequiredIssueTypes(values []IssueType) []IssueType {
	result := append([]IssueType(nil), values...)
	if !hasIssueType(result, IssueTypeRule) {
		result = append(result, IssueType{Name: IssueTypeRule, Color: defaultIssueTypeColor})
	}
	if !hasIssueType(result, IssueTypeEngagement) {
		result = append(result, IssueType{
			Name:                IssueTypeEngagement,
			Color:               defaultEngagementColor,
			TemplateDescription: defaultEngagementTemplate,
		})
	}
	return result
}

func hasIssueType(values []IssueType, name string) bool {
	for _, value := range values {
		if value.Name == name {
			return true
		}
	}
	return false
}

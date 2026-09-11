// Package conventions は組織の運用規約（conventions.yaml）の型・ローダー・検証を提供する。
package conventions

// SchemaVersion はこのパッケージが解釈できる conventions のスキーマバージョン。
const SchemaVersion = 1

const (
	// IssueTypeRule は規約の正本を保存する規約課題の種別名。
	IssueTypeRule = "規約"
	// IssueTypeEngagement は案件のヘッダーとなる親課題の種別名。
	IssueTypeEngagement = "案件"
)

// DefaultStatuses は Backlog の既定状態名。
var DefaultStatuses = []string{"未対応", "処理中", "処理済み", "完了"}

// Conventions は組織の運用規約。conventions.yaml の最上位。
type Conventions struct {
	SchemaVersion int          `yaml:"schema_version" json:"schema_version"`
	Project       Project      `yaml:"project" json:"project"`
	Priority      Priority     `yaml:"priority" json:"priority"`
	ClosePolicy   ClosePolicy  `yaml:"close_policy" json:"close_policy"`
	Statuses      []Status     `yaml:"statuses" json:"statuses"`
	IssueTypes    []IssueType  `yaml:"issue_types" json:"issue_types"`
	Initiatives   []Initiative `yaml:"initiatives" json:"initiatives"`
	Engagements   []Engagement `yaml:"engagements" json:"engagements"`
}

// Project は規約を適用する Backlog プロジェクト。
type Project struct {
	Key  string `yaml:"key" json:"key"`
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
}

// Priority は高・中・低の意味を組織の言葉で定義したもの。Backlog は 3 段階固定で段階は増やせない。
type Priority struct {
	High   string `yaml:"high" json:"high"`
	Normal string `yaml:"normal" json:"normal"`
	Low    string `yaml:"low" json:"low"`
}

// ClosePolicy は低優先度課題のクローズ候補検知に関する規約。
type ClosePolicy struct {
	// nil = 未指定（クローズ候補の検知を行わない）。0 以下の明示指定は violation。
	LowUntouchedDays *int `yaml:"low_untouched_days" json:"low_untouched_days"`
}

// Status は Backlog のカスタム状態。
type Status struct {
	Name  string `yaml:"name" json:"name"`
	Color string `yaml:"color" json:"color"`
}

// IssueType は Backlog の課題種別。
type IssueType struct {
	Name                string `yaml:"name" json:"name"`
	Color               string `yaml:"color" json:"color"`
	TemplateSummary     string `yaml:"template_summary,omitempty" json:"template_summary,omitempty"`
	TemplateDescription string `yaml:"template_description,omitempty" json:"template_description,omitempty"`
}

// Initiative は数か月規模の重点テーマ。
type Initiative struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// Engagement は数週間規模の取り組み。
type Engagement struct {
	Name       string `yaml:"name" json:"name"`
	Lead       string `yaml:"lead" json:"lead"`
	Initiative string `yaml:"initiative" json:"initiative"`
	StartDate  string `yaml:"start_date,omitempty" json:"start_date,omitempty"`
	DueDate    string `yaml:"due_date,omitempty" json:"due_date,omitempty"`
}

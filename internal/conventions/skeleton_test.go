package conventions

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	"gopkg.in/yaml.v3"
)

func TestSkeleton_LoadsAndHasNoErrorViolations(t *testing.T) {
	data, err := Skeleton()
	if err != nil {
		t.Fatalf("Skeleton() error = %v", err)
	}

	conventions, err := Load(data)
	if err != nil {
		t.Fatalf("Skeleton() の出力を Load できません: %v\n%s", err, data)
	}
	if violations := Validate(conventions); HasError(violations) {
		t.Fatalf("Skeleton() の出力に error violation があります: %#v", violations)
	}
	if conventions.Project != (Project{Key: "SANDBOX", Name: "Sandbox"}) {
		t.Fatalf("project = %#v", conventions.Project)
	}
	if len(conventions.Engagements) != 0 {
		t.Fatalf("engagements の件数 = %d, want 0", len(conventions.Engagements))
	}
}

func TestSkeleton_MatchesGolden(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "skeleton.golden.yaml"))
	if err != nil {
		t.Fatalf("golden の読み込みに失敗しました: %v", err)
	}
	got, err := Skeleton()
	if err != nil {
		t.Fatalf("Skeleton() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Skeleton() が golden と一致しません:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSkeleton_HasWhyComments(t *testing.T) {
	data, err := Skeleton()
	if err != nil {
		t.Fatalf("Skeleton() error = %v", err)
	}
	for _, comment := range []string{
		"# logvalet conventions: Linear の思想を Backlog の語彙に翻訳した運用規約。",
		"# 優先度の意味。Backlog は 高・中・低 の 3 段階固定なので段階は足さず、意味を定義する。",
		"# クローズも決断のうち。低優先度を溜め続けないための規約。",
		"# Initiative: 数か月規模の重点テーマ。Backlog には対応する概念がないので、この一覧で持つ。",
		"# 案件: 数週間規模の取り組み。1 件ごとにカテゴリと「案件」種別の親課題を作る。",
	} {
		if !strings.Contains(string(data), comment) {
			t.Errorf("コメントがありません: %q", comment)
		}
	}
}

func TestBuildSkeletonFromProject_EscapesDynamicValuesAndRoundTrips(t *testing.T) {
	const projectKey = "OPS"
	const projectName = "Ops: on-call #1"
	const categoryName = "- 顧客A \"基盤\" 更改"
	const templateDescription = "Context: #1\n- item\nquoted: \"value\""

	client := backlog.NewMockClient()
	client.GetProjectFunc = func(ctx context.Context, key string) (*domain.Project, error) {
		return &domain.Project{ProjectKey: projectKey, Name: projectName}, nil
	}
	client.ListProjectIssueTypesFunc = func(ctx context.Context, key string) ([]domain.IssueType, error) {
		return []domain.IssueType{{
			ID:                  10,
			Name:                "既存: 種別 #1",
			Color:               "#ffffff",
			DisplayOrder:        1,
			TemplateSummary:     "Summary: #1",
			TemplateDescription: templateDescription,
		}}, nil
	}
	client.ListProjectStatusesFunc = func(ctx context.Context, key string) ([]domain.Status, error) {
		return []domain.Status{{ID: 5, Name: "状態: #1", Color: StatusColors[0], DisplayOrder: 1}}, nil
	}
	client.ListProjectCategoriesFunc = func(ctx context.Context, key string) ([]domain.Category, error) {
		return []domain.Category{{ID: 3, Name: categoryName, DisplayOrder: 1}}, nil
	}

	data, err := BuildSkeletonFromProject(context.Background(), client, projectKey)
	if err != nil {
		t.Fatalf("BuildSkeletonFromProject() error = %v", err)
	}
	conventions, err := Load(data)
	if err != nil {
		t.Fatalf("生成結果を Load できません: %v\n%s", err, data)
	}
	if violations := Validate(conventions); HasError(violations) {
		t.Fatalf("生成結果に error violation があります: %#v\n%s", violations, data)
	}
	if conventions.Project.Key != projectKey || conventions.Project.Name != projectName {
		t.Fatalf("project = %#v", conventions.Project)
	}
	if len(conventions.IssueTypes) != 3 {
		t.Fatalf("issue_types の件数 = %d, want 3", len(conventions.IssueTypes))
	}
	var gotType *IssueType
	for i := range conventions.IssueTypes {
		if conventions.IssueTypes[i].Name == "既存: 種別 #1" {
			gotType = &conventions.IssueTypes[i]
		}
	}
	if gotType == nil {
		t.Fatal("動的な課題種別がありません")
	}
	if gotType.TemplateSummary != "Summary: #1" || gotType.TemplateDescription != templateDescription {
		t.Fatalf("動的なテンプレートが一致しません: %#v", gotType)
	}
	if len(conventions.Engagements) != 1 || conventions.Engagements[0].Name != categoryName || conventions.Engagements[0].Initiative != "未分類" || conventions.Engagements[0].Lead != "" {
		t.Fatalf("engagements = %#v", conventions.Engagements)
	}
}

// Backlog は allowlist 外の色を 400 で拒否するため、既存プロジェクトから取り込むときは
// 既定色に置き換える。そのまま書き出すと apply が必ず失敗するため。
func TestBuildSkeletonFromProject_ReplacesUnsupportedColors(t *testing.T) {
	client := backlog.NewMockClient()
	client.GetProjectFunc = func(ctx context.Context, key string) (*domain.Project, error) {
		return &domain.Project{ProjectKey: "PROJ", Name: "Project"}, nil
	}
	client.ListProjectIssueTypesFunc = func(ctx context.Context, key string) ([]domain.IssueType, error) {
		return []domain.IssueType{{ID: 10, Name: "既存種別", Color: "#ffffff", DisplayOrder: 1}}, nil
	}
	client.ListProjectStatusesFunc = func(ctx context.Context, key string) ([]domain.Status, error) {
		return []domain.Status{{ID: 5, Name: "既存状態", Color: "#000000", DisplayOrder: 1}}, nil
	}
	client.ListProjectCategoriesFunc = func(ctx context.Context, key string) ([]domain.Category, error) {
		return nil, nil
	}

	data, err := BuildSkeletonFromProject(context.Background(), client, "PROJ")
	if err != nil {
		t.Fatalf("BuildSkeletonFromProject() error = %v", err)
	}
	conventions, err := Load(data)
	if err != nil {
		t.Fatalf("生成結果を Load できません: %v", err)
	}
	if !containsIssueTypeColor(conventions.IssueTypes, "既存種別", defaultIssueTypeColor) {
		t.Fatalf("非 allowlist の課題種別色が既定色に置き換わっていません: %#v", conventions.IssueTypes)
	}
	if len(conventions.Statuses) != 1 || conventions.Statuses[0].Color != defaultStatusColor {
		t.Fatalf("非 allowlist の状態色が既定色に置き換わっていません: %#v", conventions.Statuses)
	}
	if violations := Validate(conventions); HasError(violations) {
		t.Fatalf("生成結果に error violation があります: %#v", violations)
	}
}

func TestBuildSkeletonFromProject_IsDeterministicAndAddsRequiredIssueTypes(t *testing.T) {
	client := backlog.NewMockClient()
	client.GetProjectFunc = func(ctx context.Context, key string) (*domain.Project, error) {
		return &domain.Project{ProjectKey: "PROJ", Name: "Project"}, nil
	}
	client.ListProjectIssueTypesFunc = func(ctx context.Context, key string) ([]domain.IssueType, error) {
		return []domain.IssueType{
			{ID: 20, Name: "後", Color: IssueTypeColors[0], DisplayOrder: 2},
			{ID: 10, Name: "前", Color: IssueTypeColors[1], DisplayOrder: 1},
		}, nil
	}
	client.ListProjectStatusesFunc = func(ctx context.Context, key string) ([]domain.Status, error) {
		return []domain.Status{
			{ID: 4, Name: "完了を改名", Color: StatusColors[0], DisplayOrder: 0},
			{ID: 6, Name: "後", Color: StatusColors[1], DisplayOrder: 2},
			{ID: 1, Name: "未対応を改名", Color: StatusColors[2], DisplayOrder: 1},
			{ID: 5, Name: "前", Color: StatusColors[3], DisplayOrder: 1},
		}, nil
	}
	client.ListProjectCategoriesFunc = func(ctx context.Context, key string) ([]domain.Category, error) {
		return []domain.Category{
			{ID: 2, Name: "カテゴリB", DisplayOrder: 2},
			{ID: 1, Name: "カテゴリA", DisplayOrder: 1},
		}, nil
	}

	first, err := BuildSkeletonFromProject(context.Background(), client, "PROJ")
	if err != nil {
		t.Fatalf("1 回目の生成に失敗しました: %v", err)
	}
	second, err := BuildSkeletonFromProject(context.Background(), client, "PROJ")
	if err != nil {
		t.Fatalf("2 回目の生成に失敗しました: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("同じ入力で生成結果が一致しません")
	}

	conventions, err := Load(first)
	if err != nil {
		t.Fatalf("生成結果を Load できません: %v", err)
	}
	if got := issueTypeNames(conventions.IssueTypes); !equalStrings(got, []string{"前", "後", "規約", "案件"}) {
		t.Errorf("issue_types の順序 = %v", got)
	}
	if got := statusNames(conventions.Statuses); !equalStrings(got, []string{"前", "後"}) {
		t.Errorf("statuses の順序 = %v", got)
	}
	if got := engagementNames(conventions.Engagements); !equalStrings(got, []string{"カテゴリA", "カテゴリB"}) {
		t.Errorf("engagements の順序 = %v", got)
	}
}

func TestBuildSkeletonFromProject_DoesNotDuplicateRequiredIssueTypes(t *testing.T) {
	client := backlog.NewMockClient()
	client.GetProjectFunc = func(ctx context.Context, key string) (*domain.Project, error) {
		return &domain.Project{ProjectKey: "PROJ", Name: "Project"}, nil
	}
	client.ListProjectIssueTypesFunc = func(ctx context.Context, key string) ([]domain.IssueType, error) {
		return []domain.IssueType{
			{ID: 2, Name: IssueTypeEngagement, Color: IssueTypeColors[0], DisplayOrder: 2},
			{ID: 1, Name: IssueTypeRule, Color: IssueTypeColors[1], DisplayOrder: 1},
		}, nil
	}
	client.ListProjectStatusesFunc = func(ctx context.Context, key string) ([]domain.Status, error) {
		return nil, nil
	}
	client.ListProjectCategoriesFunc = func(ctx context.Context, key string) ([]domain.Category, error) {
		return nil, nil
	}

	data, err := BuildSkeletonFromProject(context.Background(), client, "PROJ")
	if err != nil {
		t.Fatalf("BuildSkeletonFromProject() error = %v", err)
	}
	conventions, err := Load(data)
	if err != nil {
		t.Fatalf("生成結果を Load できません: %v", err)
	}
	if got := issueTypeNames(conventions.IssueTypes); !equalStrings(got, []string{IssueTypeRule, IssueTypeEngagement}) {
		t.Fatalf("required issue types が重複または順序違いです: %v", got)
	}
}

func TestBuildSkeletonFromProject_EmptyCategories(t *testing.T) {
	client := backlog.NewMockClient()
	client.GetProjectFunc = func(ctx context.Context, key string) (*domain.Project, error) {
		return &domain.Project{ProjectKey: "PROJ", Name: "Project"}, nil
	}
	client.ListProjectIssueTypesFunc = func(ctx context.Context, key string) ([]domain.IssueType, error) {
		return nil, nil
	}
	client.ListProjectStatusesFunc = func(ctx context.Context, key string) ([]domain.Status, error) {
		return nil, nil
	}
	client.ListProjectCategoriesFunc = func(ctx context.Context, key string) ([]domain.Category, error) {
		return []domain.Category{}, nil
	}

	data, err := BuildSkeletonFromProject(context.Background(), client, "PROJ")
	if err != nil {
		t.Fatalf("BuildSkeletonFromProject() error = %v", err)
	}
	if !strings.Contains(string(data), "engagements: []") {
		t.Fatalf("空の engagements が [] で出力されていません:\n%s", data)
	}
	conventions, err := Load(data)
	if err != nil {
		t.Fatalf("生成結果を Load できません: %v", err)
	}
	if len(conventions.Engagements) != 0 || len(conventions.Initiatives) != 1 || conventions.Initiatives[0].Name != "未分類" {
		t.Fatalf("空カテゴリ時の値が不正です: initiatives=%#v engagements=%#v", conventions.Initiatives, conventions.Engagements)
	}
}

func TestYamlScalar_RoundTripsValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
		block bool
	}{
		{name: "simple", value: "simple"},
		{name: "colon", value: "Ops: on-call"},
		{name: "hash", value: "on-call #1"},
		{name: "multiline", value: "line one\nline two\n# comment\n", block: true},
		{name: "multiline without trailing newline", value: "line one\nline two", block: true},
		{name: "single newline", value: "\n", block: true},
		{name: "blank lines", value: "\n\n", block: true},
		{name: "leading hyphen", value: "- item"},
		{name: "empty", value: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scalar := yamlScalar(tt.value, 4)
			var got string
			if err := yaml.Unmarshal([]byte("value: "+scalar+"\n"), &struct {
				Value *string `yaml:"value"`
			}{Value: &got}); err != nil {
				t.Fatalf("yamlScalar の出力をパースできません: %v\n%s", err, scalar)
			}
			if got != tt.value {
				t.Fatalf("復号値 = %q, want %q", got, tt.value)
			}
			if tt.block && !strings.HasPrefix(scalar, "|") {
				t.Fatalf("複数行値がブロックスカラーではありません: %q", scalar)
			}
		})
	}
}

func issueTypeNames(values []IssueType) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = value.Name
	}
	return result
}

func statusNames(values []Status) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = value.Name
	}
	return result
}

func engagementNames(values []Engagement) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = value.Name
	}
	return result
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func containsIssueTypeColor(values []IssueType, name, color string) bool {
	for _, value := range values {
		if value.Name == name && value.Color == color {
			return true
		}
	}
	return false
}

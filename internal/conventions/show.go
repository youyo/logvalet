package conventions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// ShowResult は規約課題から読み出した運用規約。
type ShowResult struct {
	ProjectKey  string          `json:"project_key"`
	IssueKey    string          `json:"issue_key"`
	Adopted     bool            `json:"adopted"`
	Conventions *Conventions    `json:"conventions,omitempty"`
	Glossary    []GlossaryEntry `json:"glossary"`
}

// Show は指定プロジェクトの規約課題から運用規約を読み出す。
func Show(ctx context.Context, client backlog.Client, projectKey string) (*ShowResult, error) {
	if client == nil {
		return nil, errors.New("Backlog client が nil です")
	}

	projectKey = strings.TrimSpace(projectKey)
	if projectKey == "" {
		return nil, errors.New("project key is required")
	}

	project, err := client.GetProject(ctx, projectKey)
	if err != nil {
		return nil, fmt.Errorf("プロジェクト %q の取得に失敗しました: %w", projectKey, err)
	}
	if project == nil {
		return nil, fmt.Errorf("プロジェクト %q の取得結果が nil です", projectKey)
	}

	// BuildPlan と同じ探索経路に揃え、課題種別の取得エラーも隠さない。
	if _, err := listIssueTypes(ctx, client, projectKey); err != nil {
		return nil, fmt.Errorf("課題種別の取得に失敗しました: %w", err)
	}
	issues, err := client.ListIssues(ctx, backlog.ListIssuesOptions{ProjectIDs: []int{project.ID}})
	if errors.Is(err, backlog.ErrNotFound) {
		issues = nil
	} else if err != nil {
		return nil, fmt.Errorf("プロジェクト課題の取得に失敗しました: %w", err)
	}

	ruleIssues := make([]domain.Issue, 0, 1)
	for _, issue := range issues {
		if issue.IssueType != nil && sameName(issue.IssueType.Name, IssueTypeRule) {
			ruleIssues = append(ruleIssues, issue)
		}
	}

	result := &ShowResult{
		ProjectKey: projectKey,
		Glossary:   Glossary(),
	}
	switch len(ruleIssues) {
	case 0:
		return result, nil
	case 1:
		conventions, err := LoadFromIssueDescription(ruleIssues[0].Description)
		if err != nil {
			return nil, fmt.Errorf("規約課題 %q の説明欄の読み込みに失敗しました: %w", ruleIssues[0].IssueKey, err)
		}
		result.IssueKey = ruleIssues[0].IssueKey
		result.Adopted = true
		result.Conventions = conventions
		return result, nil
	default:
		return nil, fmt.Errorf("issue %q（規約課題）が複数あります（%d 件）", IssueTypeRule, len(ruleIssues))
	}
}

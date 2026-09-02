package conventions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// EngagementRef は案件名から解決した、課題に設定すべきカテゴリと親課題。
type EngagementRef struct {
	Name          string
	CategoryID    int
	ParentIssueID int
}

// ResolveEngagement は案件名から案件カテゴリと案件親課題を解決する。
// 課題を案件に紐づけるには「カテゴリ」と「親課題」の 2 つを設定する必要があるが、
// 規約上この 2 つは常に対で決まるため、案件名 1 つから両方を解決する。
func ResolveEngagement(ctx context.Context, client backlog.Client, projectKey, name string) (*EngagementRef, error) {
	projectKey = strings.TrimSpace(projectKey)
	name = strings.TrimSpace(name)

	result, err := Show(ctx, client, projectKey)
	if err != nil {
		return nil, err
	}
	if !result.Adopted || result.Conventions == nil {
		return nil, fmt.Errorf("プロジェクト %q には運用規約が導入されていません", projectKey)
	}

	available := make([]string, 0, len(result.Conventions.Engagements))
	for _, engagement := range result.Conventions.Engagements {
		engagementName := strings.TrimSpace(engagement.Name)
		available = append(available, engagementName)
		if !sameName(engagementName, name) {
			continue
		}

		categories, err := listCategories(ctx, client, projectKey)
		if err != nil {
			return nil, fmt.Errorf("プロジェクト %q のカテゴリ取得に失敗しました: %w", projectKey, err)
		}
		categoryID := 0
		for _, category := range categories {
			if sameName(category.Name, name) {
				categoryID = category.ID
				break
			}
		}
		if categoryID == 0 {
			return nil, fmt.Errorf("案件 %q に対応するカテゴリがプロジェクトにありません", name)
		}

		project, err := client.GetProject(ctx, projectKey)
		if err != nil {
			return nil, fmt.Errorf("プロジェクト %q の取得に失敗しました: %w", projectKey, err)
		}
		if project == nil {
			return nil, fmt.Errorf("プロジェクト %q の取得結果が nil です", projectKey)
		}

		issues, err := client.ListIssues(ctx, backlog.ListIssuesOptions{ProjectIDs: []int{project.ID}})
		if errors.Is(err, backlog.ErrNotFound) {
			issues = nil
		} else if err != nil {
			return nil, fmt.Errorf("プロジェクト課題の取得に失敗しました: %w", err)
		}

		ref := &EngagementRef{Name: name, CategoryID: categoryID}
		for _, issue := range matchingEngagementIssues(issues, name) {
			ref.ParentIssueID = issue.ID
			break
		}
		return ref, nil
	}

	return nil, fmt.Errorf("案件 %q は規約にありません。利用可能な案件: %v", name, available)
}

// CountEngagementCategories は課題が持つカテゴリのうち、規約の案件カテゴリの数を返す。
// 規約は「案件カテゴリをちょうど 1 つ持つ」ことを求めるので、0 個や 2 個以上は
// 規約違反として警告する材料になる。
func CountEngagementCategories(c *Conventions, categories []domain.IDName) int {
	if c == nil {
		return 0
	}

	engagementNames := make(map[string]struct{}, len(c.Engagements))
	for _, engagement := range c.Engagements {
		engagementNames[normalize(engagement.Name)] = struct{}{}
	}

	count := 0
	for _, category := range categories {
		if _, ok := engagementNames[normalize(category.Name)]; ok {
			count++
		}
	}
	return count
}

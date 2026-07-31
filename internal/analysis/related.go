package analysis

import (
	"context"
	"fmt"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// RelatedIssueRef は digest 系サーフェス（issue context / triage-materials 等）に埋め込む
// 関連課題の軽量射影。domain.RelatedIssue（Issue 埋め込み）から必要フィールドのみを
// 抽出し、description / customFields 等の重量フィールドは含めない。
type RelatedIssueRef struct {
	IssueKey  string          `json:"issue_key"`
	Summary   string          `json:"summary"`
	Status    *domain.IDName  `json:"status,omitempty"`
	Priority  *domain.IDName  `json:"priority,omitempty"`
	IssueType *domain.IDName  `json:"issue_type,omitempty"`
	Assignee  *domain.UserRef `json:"assignee,omitempty"`
	Type      string          `json:"type"`
	Created   *time.Time      `json:"created,omitempty"`
	Updated   *time.Time      `json:"updated,omitempty"`
}

// buildRelatedIssueRefs は domain.RelatedIssue のスライスを RelatedIssueRef に射影する。
// 入力が nil または空でも、JSON マーシャル時に null にならないよう常に空スライス
// （nilではない）を返す。
func buildRelatedIssueRefs(related []domain.RelatedIssue) []RelatedIssueRef {
	refs := make([]RelatedIssueRef, 0, len(related))
	for _, r := range related {
		refs = append(refs, RelatedIssueRef{
			IssueKey:  r.IssueKey,
			Summary:   r.Summary,
			Status:    r.Status,
			Priority:  r.Priority,
			IssueType: r.IssueType,
			Assignee:  toUserRef(r.Assignee),
			Type:      r.Type,
			Created:   r.Created,
			Updated:   r.Updated,
		})
	}
	return refs
}

// fetchRelatedIssues は指定課題の関連課題を取得し RelatedIssueRef に射影する。
// 取得に失敗した場合はエラーを返さず、空スライスと domain.Warning を返す
// （呼び出し元の課題本体取得を失敗させない graceful degradation）。
//
// この関数自体は同期関数として実装されており、内部にロックを持たない。
// errgroup 等の並行 goroutine から呼び出す場合、戻り値（RelatedIssues スライスや
// warnings への追加）の共有変数への反映は呼び出し側 goroutine が mutex 下で
// 行う契約とする。
func fetchRelatedIssues(ctx context.Context, client backlog.Client, issueKey string) ([]RelatedIssueRef, *domain.Warning) {
	related, err := client.ListRelatedIssues(ctx, issueKey)
	if err != nil {
		return []RelatedIssueRef{}, &domain.Warning{
			Code:      "related_issues_fetch_failed",
			Message:   fmt.Sprintf("failed to list related issues: %v", err),
			Component: "related_issues",
			Retryable: true,
		}
	}
	return buildRelatedIssueRefs(related), nil
}

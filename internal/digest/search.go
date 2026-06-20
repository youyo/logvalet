package digest

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

const defaultSearchCount = 20
const maxSearchCount = 100

// SearchOptions は横断検索 digest の入力オプション。
type SearchOptions struct {
	Keyword     string
	ProjectKeys []string
	Detail      string
	Count       int
	Offset      int
}

// SearchResult は横断検索結果の共通表現。
type SearchResult struct {
	ResourceType string          `json:"resource_type"`
	ID           string          `json:"id,omitempty"`
	Key          string          `json:"key,omitempty"`
	ProjectID    int             `json:"project_id,omitempty"`
	ProjectKey   string          `json:"project_key,omitempty"`
	Title        string          `json:"title"`
	URL          string          `json:"url,omitempty"`
	Snippet      string          `json:"snippet,omitempty"`
	Created      *time.Time      `json:"created,omitempty"`
	Updated      *time.Time      `json:"updated,omitempty"`
	Assignee     *domain.UserRef `json:"assignee,omitempty"`
	Reporter     *domain.UserRef `json:"reporter,omitempty"`
	CreatedUser  *domain.UserRef `json:"created_user,omitempty"`
	UpdatedUser  *domain.UserRef `json:"updated_user,omitempty"`
}

// SearchCounts は resource type ごとの返却件数。
type SearchCounts struct {
	Issues    int `json:"issues"`
	Documents int `json:"documents"`
	Wikis     int `json:"wikis"`
}

// SearchDigest は横断検索 digest のトップレベル。
type SearchDigest struct {
	Keyword            string         `json:"keyword"`
	Detail             string         `json:"detail"`
	ProjectKeys        []string       `json:"project_keys,omitempty"`
	CountPerResource   int            `json:"count_per_resource"`
	Offset             int            `json:"offset"`
	TotalReturned      int            `json:"total_returned"`
	ReturnedByType     SearchCounts   `json:"returned_by_type"`
	PossiblyMore       bool           `json:"possibly_more"`
	PossiblyMoreByType SearchCounts   `json:"possibly_more_by_type"`
	NextOffset         int            `json:"next_offset,omitempty"`
	Items              []SearchResult `json:"items"`
}

// SearchBuilder は Backlog の複数リソースを keyword で横断検索する。
type SearchBuilder interface {
	Build(ctx context.Context, opt SearchOptions) (*domain.DigestEnvelope, error)
}

// DefaultSearchBuilder は SearchBuilder の標準実装。
type DefaultSearchBuilder struct {
	BaseDigestBuilder
}

// NewDefaultSearchBuilder は DefaultSearchBuilder を生成する。
func NewDefaultSearchBuilder(client backlog.Client, profile, space, baseURL string) *DefaultSearchBuilder {
	return &DefaultSearchBuilder{BaseDigestBuilder{client: client, profile: profile, space: space, baseURL: baseURL}}
}

// Build は issue / document / wiki を同じ keyword で検索し、共通 digest に正規化する。
func (b *DefaultSearchBuilder) Build(ctx context.Context, opt SearchOptions) (*domain.DigestEnvelope, error) {
	if strings.TrimSpace(opt.Keyword) == "" {
		return nil, fmt.Errorf("keyword is required")
	}

	detail := opt.Detail
	if detail == "" {
		detail = "snippet"
	}
	count := normalizeSearchCount(opt.Count)
	offset := opt.Offset
	if offset < 0 {
		offset = 0
	}

	projects, projectKeyMap, projectIDFilter, warnings, err := b.resolveSearchProjects(ctx, opt.ProjectKeys)
	if err != nil {
		return nil, err
	}

	var items []SearchResult
	var counts SearchCounts
	var possiblyMore SearchCounts

	issues, err := b.client.ListIssues(ctx, backlog.ListIssuesOptions{
		ProjectIDs: projectIDFilter,
		Keyword:    opt.Keyword,
		Limit:      count,
		Offset:     offset,
		Sort:       "updated",
		Order:      "desc",
	})
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "issue_search_failed",
			Message:   fmt.Sprintf("failed to search issues: %v", err),
			Component: "issues",
			Retryable: true,
		})
	} else {
		counts.Issues = len(issues)
		if len(issues) >= count {
			possiblyMore.Issues = 1
		}
		items = append(items, b.issueResults(issues, projectKeyMap, opt.Keyword, detail)...)
	}

	docs, err := b.client.SearchDocuments(ctx, backlog.SearchDocumentsOptions{
		Keyword:    opt.Keyword,
		ProjectIDs: projectIDFilter,
		Count:      count,
		Offset:     offset,
		Sort:       "updated",
		Order:      "desc",
	})
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "document_search_failed",
			Message:   fmt.Sprintf("failed to search documents: %v", err),
			Component: "documents",
			Retryable: true,
		})
	} else {
		counts.Documents = len(docs)
		if len(docs) >= count {
			possiblyMore.Documents = 1
		}
		items = append(items, b.documentResults(docs, projectKeyMap, opt.Keyword, detail)...)
	}

	wikiPages, wikiPossiblyMore, wikiWarnings := b.searchWikis(ctx, projects, opt.Keyword, count, offset, detail, projectKeyMap)
	warnings = append(warnings, wikiWarnings...)
	counts.Wikis = len(wikiPages)
	if wikiPossiblyMore {
		possiblyMore.Wikis = 1
	}
	items = append(items, wikiPages...)

	possiblyMoreAny := possiblyMore.Issues > 0 || possiblyMore.Documents > 0 || possiblyMore.Wikis > 0
	nextOffset := 0
	if possiblyMoreAny {
		nextOffset = offset + count
	}

	d := &SearchDigest{
		Keyword:            opt.Keyword,
		Detail:             detail,
		ProjectKeys:        opt.ProjectKeys,
		CountPerResource:   count,
		Offset:             offset,
		TotalReturned:      len(items),
		ReturnedByType:     counts,
		PossiblyMore:       possiblyMoreAny,
		PossiblyMoreByType: possiblyMore,
		NextOffset:         nextOffset,
		Items:              items,
	}
	return b.newEnvelope("search", d, warnings), nil
}

func normalizeSearchCount(count int) int {
	if count <= 0 {
		return defaultSearchCount
	}
	if count > maxSearchCount {
		return maxSearchCount
	}
	return count
}

func (b *DefaultSearchBuilder) resolveSearchProjects(ctx context.Context, projectKeys []string) ([]domain.Project, map[int]string, []int, []domain.Warning, error) {
	projects := make([]domain.Project, 0)
	projectKeyMap := make(map[int]string)
	projectIDFilter := make([]int, 0, len(projectKeys))
	var warnings []domain.Warning

	if len(projectKeys) > 0 {
		for _, key := range projectKeys {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			p, err := b.client.GetProject(ctx, key)
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to resolve project key %q: %w", key, err)
			}
			projects = append(projects, *p)
			projectKeyMap[p.ID] = p.ProjectKey
			projectIDFilter = append(projectIDFilter, p.ID)
		}
		return projects, projectKeyMap, projectIDFilter, warnings, nil
	}

	allProjects, err := b.client.ListProjects(ctx)
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "project_list_failed",
			Message:   fmt.Sprintf("failed to list projects for wiki search and URL construction: %v", err),
			Component: "projects",
			Retryable: true,
		})
		return nil, projectKeyMap, nil, warnings, nil
	}
	for _, p := range allProjects {
		projects = append(projects, p)
		projectKeyMap[p.ID] = p.ProjectKey
	}
	return projects, projectKeyMap, nil, warnings, nil
}

func (b *DefaultSearchBuilder) issueResults(issues []domain.Issue, projectKeyMap map[int]string, keyword, detail string) []SearchResult {
	baseURL := strings.TrimRight(b.baseURL, "/")
	results := make([]SearchResult, 0, len(issues))
	for _, issue := range issues {
		projectKey := projectKeyMap[issue.ProjectID]
		if projectKey == "" {
			projectKey = extractProjectKey(issue.IssueKey)
		}
		item := SearchResult{
			ResourceType: "issue",
			ID:           fmt.Sprintf("%d", issue.ID),
			Key:          issue.IssueKey,
			ProjectID:    issue.ProjectID,
			ProjectKey:   projectKey,
			Title:        issue.Summary,
			Created:      issue.Created,
			Updated:      issue.Updated,
			Assignee:     toUserRef(issue.Assignee),
			Reporter:     toUserRef(issue.Reporter),
		}
		if baseURL != "" && issue.IssueKey != "" {
			item.URL = fmt.Sprintf("%s/view/%s", baseURL, url.PathEscape(issue.IssueKey))
		}
		if detail != "meta" {
			item.Snippet = extractSnippet(issue.Description, keyword)
		}
		results = append(results, item)
	}
	return results
}

func (b *DefaultSearchBuilder) documentResults(docs []domain.Document, projectKeyMap map[int]string, keyword, detail string) []SearchResult {
	baseURL := strings.TrimRight(b.baseURL, "/")
	results := make([]SearchResult, 0, len(docs))
	for _, doc := range docs {
		projectKey := projectKeyMap[doc.ProjectID]
		item := SearchResult{
			ResourceType: "document",
			ID:           doc.ID,
			ProjectID:    doc.ProjectID,
			ProjectKey:   projectKey,
			Title:        doc.Title,
			Created:      doc.Created,
			Updated:      doc.Updated,
			CreatedUser:  toUserRef(doc.CreatedUser),
			UpdatedUser:  toUserRef(doc.UpdatedUser),
		}
		if baseURL != "" && projectKey != "" && doc.ID != "" {
			item.URL = fmt.Sprintf("%s/document/%s/%s", baseURL, url.PathEscape(projectKey), url.PathEscape(doc.ID))
		}
		if detail != "meta" {
			item.Snippet = extractSnippet(doc.Plain, keyword)
		}
		results = append(results, item)
	}
	return results
}

func (b *DefaultSearchBuilder) searchWikis(ctx context.Context, projects []domain.Project, keyword string, count, offset int, detail string, projectKeyMap map[int]string) ([]SearchResult, bool, []domain.Warning) {
	var warnings []domain.Warning
	if len(projects) == 0 {
		return nil, false, nil
	}

	all := make([]SearchResult, 0)
	baseURL := strings.TrimRight(b.baseURL, "/")
	for _, project := range projects {
		pages, err := b.client.ListWikis(ctx, project.ProjectKey, backlog.ListWikisOptions{Keyword: keyword})
		if err != nil {
			warnings = append(warnings, domain.Warning{
				Code:      "wiki_search_failed",
				Message:   fmt.Sprintf("failed to search wikis in project %s: %v", project.ProjectKey, err),
				Component: "wikis",
				Retryable: true,
			})
			continue
		}
		for _, page := range pages {
			projectKey := project.ProjectKey
			if projectKey == "" {
				projectKey = projectKeyMap[page.ProjectID]
			}
			item := SearchResult{
				ResourceType: "wiki",
				ID:           fmt.Sprintf("%d", page.ID),
				ProjectID:    page.ProjectID,
				ProjectKey:   projectKey,
				Title:        page.Name,
				Created:      page.Created,
				Updated:      page.Updated,
				CreatedUser:  toUserRef(page.CreatedUser),
				UpdatedUser:  toUserRef(page.UpdatedUser),
			}
			if baseURL != "" && projectKey != "" && page.Name != "" {
				item.URL = fmt.Sprintf("%s/wiki/%s/%s", baseURL, url.PathEscape(projectKey), url.PathEscape(page.Name))
			}
			if detail != "meta" {
				item.Snippet = extractSnippet(page.Content, keyword)
			}
			all = append(all, item)
		}
	}

	if offset >= len(all) {
		return []SearchResult{}, false, warnings
	}
	end := offset + count
	possiblyMore := false
	if end < len(all) {
		possiblyMore = true
	} else {
		end = len(all)
	}
	return all[offset:end], possiblyMore, warnings
}

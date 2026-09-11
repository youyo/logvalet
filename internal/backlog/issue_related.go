package backlog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/youyo/logvalet/internal/domain"
)

// AddRelatedIssueRequest は AddRelatedIssue リクエストのパラメータ。
type AddRelatedIssueRequest struct {
	// TargetIssueID は関連付ける対象課題の内部 ID（課題キーではない）。
	TargetIssueID int64
}

// ListRelatedIssues は指定課題の関連課題一覧を返す。
//
// 未公開 API（GET /api/v2/issues/{issueIdOrKey}/relatedIssues）。
// レスポンス形状の根拠は domain.RelatedIssue のコメントを参照。
func (c *HTTPClient) ListRelatedIssues(ctx context.Context, issueKey string) ([]domain.RelatedIssue, error) {
	apiPath := fmt.Sprintf("/api/v2/issues/%s/relatedIssues", url.PathEscape(issueKey))
	req, err := c.newRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, err
	}
	var related []domain.RelatedIssue
	if err := c.do(req, &related); err != nil {
		return nil, err
	}
	return related, nil
}

// AddRelatedIssue は指定課題に関連課題を追加する。
//
// 未公開 API（POST /api/v2/issues/{issueIdOrKey}/relatedIssues、body: targetIssueId）。
func (c *HTTPClient) AddRelatedIssue(ctx context.Context, issueKey string, reqBody AddRelatedIssueRequest) (*domain.RelatedIssue, error) {
	q := url.Values{}
	q.Set("targetIssueId", strconv.FormatInt(reqBody.TargetIssueID, 10))
	apiPath := fmt.Sprintf("/api/v2/issues/%s/relatedIssues", url.PathEscape(issueKey))
	req, err := c.newBodyRequest(ctx, http.MethodPost, apiPath, q)
	if err != nil {
		return nil, err
	}
	var related domain.RelatedIssue
	if err := c.do(req, &related); err != nil {
		return nil, err
	}
	return &related, nil
}

// DeleteRelatedIssue は指定課題の関連課題を削除し、削除された関連課題情報を返す。
//
// 未公開 API（DELETE /api/v2/issues/{issueIdOrKey}/relatedIssues/{relatedIssueId}）。
// 未公開のため 204 No Content や 200 + 空 body を返す可能性を排除できず、共通の
// c.do() ではなく空 body を許容する専用経路（doAllowEmptyBody）を使う。
func (c *HTTPClient) DeleteRelatedIssue(ctx context.Context, issueKey string, relatedIssueID int64) (*domain.RelatedIssue, error) {
	apiPath := fmt.Sprintf("/api/v2/issues/%s/relatedIssues/%d", url.PathEscape(issueKey), relatedIssueID)
	req, err := c.newRequest(ctx, http.MethodDelete, apiPath, nil)
	if err != nil {
		return nil, err
	}
	var related domain.RelatedIssue
	if err := c.doAllowEmptyBody(req, &related); err != nil {
		return nil, err
	}
	return &related, nil
}

// doAllowEmptyBody は c.do() と同じ手順で HTTP リクエストを実行するが、レスポンス
// body が空の場合（204 No Content や 200 + 空 body）は json.Unmarshal をスキップする。
// c.do() は out が非 nil だと空 body でも Unmarshal を試みてエラーになるため、
// レスポンス形状が未確定な未公開 API 向けにこの専用経路を用意する。
func (c *HTTPClient) doAllowEmptyBody(req *http.Request, out interface{}) (retErr error) {
	defer func() { retErr = c.redactAPIKey(retErr) }()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("backlog: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseBytes))
	if err != nil {
		return fmt.Errorf("backlog: failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return c.normalizeError(resp.StatusCode, body)
	}

	if out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("backlog: failed to parse response: %w", err)
		}
	}
	return nil
}

package domain

import "encoding/json"

// RelatedIssue は Backlog の関連課題(relatedIssues) API レスポンスの1要素を表す。
//
// この API は Backlog の公式ドキュメントに記載のない未公開エンドポイントである
// （GET/POST /api/v2/issues/:issueIdOrKey/relatedIssues, DELETE .../relatedIssues/:relatedIssueId）。
// レスポンス形状は logvalet issue #63 に記録された実測結果（backlog-js PR #181・
// backlog-mcp-server の getRelatedIssues 実装を根拠とする一次情報）に基づく。
// 実測されたレスポンスは Issue 相当のフィールドがフラットに並んだ配列で、末尾に
// 関連種別を表す "type" フィールドが付与される（実測値は "RELATES" のみ確認）。
// 未公開 API のため将来的にネスト形状 {"relatedIssue": {...}, "type": "..."} へ
// 変更される可能性を排除できず、UnmarshalJSON はフラット・ネスト両形状を受容する
// 防御的パースを行う。
type RelatedIssue struct {
	Issue
	// Type は関連種別。実測値は "RELATES" のみだが、未公開 API のため他の値
	// （例: "PRECEDES" 等）が将来追加される可能性がある。enum 化はせず string の
	// まま保持し、未知値でもエラーにしない。
	Type string `json:"type"`
}

// relatedIssueNested はネスト形状 {"relatedIssue": {...}, "type": "..."} 用の中間表現。
// 実測未確認だが将来の API 変更に備えた防御的パースのために用意する。
type relatedIssueNested struct {
	RelatedIssue *Issue `json:"relatedIssue"`
	Type         string `json:"type"`
}

// UnmarshalJSON はフラット形状（実測スキーマ）とネスト形状の両方を受容する。
func (r *RelatedIssue) UnmarshalJSON(data []byte) error {
	var nested relatedIssueNested
	if err := json.Unmarshal(data, &nested); err == nil && nested.RelatedIssue != nil {
		r.Issue = *nested.RelatedIssue
		r.Type = nested.Type
		return nil
	}

	// relatedIssueNested にエイリアスを使わない型で再パースし、UnmarshalJSON の
	// 無限再帰を避ける。
	type flatRelatedIssue RelatedIssue
	var f flatRelatedIssue
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	*r = RelatedIssue(f)
	return nil
}

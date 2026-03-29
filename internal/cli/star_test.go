package cli

import (
	"strings"
	"testing"
)

// CLI-ST-1: star add 排他バリデーション (複数指定 → エラー出力確認)
func TestStarAddCmd_multiple_flags_error(t *testing.T) {
	issueID := 1
	commentID := 2
	cmd := &StarAddCmd{
		IssueID:   &issueID,
		CommentID: &commentID,
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Fatal("Run() should return error when multiple flags are specified")
	}
	if !strings.Contains(err.Error(), "only one of") {
		t.Errorf("error message = %q, want to contain 'only one of'", err.Error())
	}
}

// star add: フラグ未指定 → エラー
func TestStarAddCmd_no_flags_error(t *testing.T) {
	cmd := &StarAddCmd{}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Fatal("Run() should return error when no flag is specified")
	}
	if !strings.Contains(err.Error(), "at least one of") {
		t.Errorf("error message = %q, want to contain 'at least one of'", err.Error())
	}
}

// star add: 単一フラグ指定 → バリデーション通過（buildRunContext でエラーになる）
func TestStarAddCmd_single_flag_passes_validation(t *testing.T) {
	issueID := 42
	cmd := &StarAddCmd{
		IssueID: &issueID,
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	// バリデーションは通過するが buildRunContext で config エラーになる
	if err == nil {
		t.Fatal("Run() should return error (config not available)")
	}
	// バリデーションエラーではないことを確認
	if strings.Contains(err.Error(), "at least one of") || strings.Contains(err.Error(), "only one of") {
		t.Errorf("unexpected validation error: %v", err)
	}
}

// star add: 5種類のフラグを個別に確認 (バリデーション通過を確認)
func TestStarAddCmd_each_flag_passes_validation(t *testing.T) {
	wikiID := 10
	prID := 20
	prCommentID := 30

	cases := []struct {
		name string
		cmd  *StarAddCmd
	}{
		{"wiki-id", &StarAddCmd{WikiID: &wikiID}},
		{"pr-id", &StarAddCmd{PrID: &prID}},
		{"pr-comment-id", &StarAddCmd{PrCommentID: &prCommentID}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := &GlobalFlags{}
			err := tc.cmd.Run(g)
			// バリデーションは通過するが buildRunContext で config エラーになる
			if err == nil {
				t.Fatal("Run() should return error (config not available)")
			}
			if strings.Contains(err.Error(), "at least one of") || strings.Contains(err.Error(), "only one of") {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

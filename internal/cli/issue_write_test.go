package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- validateDescriptionFlags テスト ----

func TestValidateDescriptionFlags_bothSet(t *testing.T) {
	err := validateDescriptionFlags("テキスト", "/some/file.txt")
	if err == nil {
		t.Fatal("両方指定時はエラーが返るべき")
	}
}

func TestValidateDescriptionFlags_onlyDescription(t *testing.T) {
	err := validateDescriptionFlags("テキスト", "")
	if err != nil {
		t.Fatalf("description のみ指定は OK のはず: %v", err)
	}
}

func TestValidateDescriptionFlags_onlyFile(t *testing.T) {
	err := validateDescriptionFlags("", "/some/file.txt")
	if err != nil {
		t.Fatalf("description-file のみ指定は OK のはず: %v", err)
	}
}

func TestValidateDescriptionFlags_noneSet(t *testing.T) {
	err := validateDescriptionFlags("", "")
	if err != nil {
		t.Fatalf("両方未指定も OK のはず: %v", err)
	}
}

// ---- validateContentFlags テスト ----

func TestValidateContentFlags_bothSet(t *testing.T) {
	err := validateContentFlags("テキスト", "/some/file.txt")
	if err == nil {
		t.Fatal("両方指定時はエラーが返るべき")
	}
}

func TestValidateContentFlags_noneSet(t *testing.T) {
	err := validateContentFlags("", "")
	if err == nil {
		t.Fatal("両方未指定時はエラーが返るべき（content 必須）")
	}
}

func TestValidateContentFlags_onlyContent(t *testing.T) {
	err := validateContentFlags("テキスト", "")
	if err != nil {
		t.Fatalf("content のみ指定は OK のはず: %v", err)
	}
}

func TestValidateContentFlags_onlyFile(t *testing.T) {
	err := validateContentFlags("", "/some/file.txt")
	if err != nil {
		t.Fatalf("content-file のみ指定は OK のはず: %v", err)
	}
}

// ---- validateAtLeastOneUpdateFlag テスト ----

func TestValidateAtLeastOneUpdateFlag_allNil(t *testing.T) {
	err := validateAtLeastOneUpdateFlag(nil, nil, nil, nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("全フィールド nil の場合はエラーが返るべき")
	}
}

func TestValidateAtLeastOneUpdateFlag_summarySet(t *testing.T) {
	s := "新しいサマリー"
	err := validateAtLeastOneUpdateFlag(&s, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("summary が設定されていれば OK のはず: %v", err)
	}
}

func TestValidateAtLeastOneUpdateFlag_statusSet(t *testing.T) {
	s := "完了"
	err := validateAtLeastOneUpdateFlag(nil, nil, &s, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("status が設定されていれば OK のはず: %v", err)
	}
}

// ---- readContentFromFile テスト ----

func TestReadContentFromFile_success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "content.txt")
	expected := "ファイルの内容\n2行目"
	if err := os.WriteFile(path, []byte(expected), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := readContentFromFile(path)
	if err != nil {
		t.Fatalf("読み込みエラー: %v", err)
	}
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestReadContentFromFile_notFound(t *testing.T) {
	_, err := readContentFromFile("/nonexistent/path/file.txt")
	if err == nil {
		t.Fatal("存在しないファイルはエラーが返るべき")
	}
}

// ---- formatDryRun テスト ----

func TestFormatDryRun_createIssue(t *testing.T) {
	params := map[string]interface{}{
		"project_key": "PROJ",
		"summary":     "テスト課題",
	}
	data, err := formatDryRun("create_issue", params)
	if err != nil {
		t.Fatalf("フォーマットエラー: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("出力が空")
	}
	// JSON に dry_run: true が含まれること
	if !strings.Contains(string(data), `"dry_run"`) {
		t.Errorf("出力に dry_run フィールドがない: %s", string(data))
	}
	if !strings.Contains(string(data), `"create_issue"`) {
		t.Errorf("出力に operation フィールドがない: %s", string(data))
	}
}

// ---- IssueCreateCmd.Run() テスト ----

func TestIssueCreateCmd_run_descriptionExclusive(t *testing.T) {
	cmd := &IssueCreateCmd{
		WriteFlags:      WriteFlags{DryRun: false},
		ProjectKey:      "PROJ",
		Summary:         "テスト",
		IssueType:       "Bug",
		Description:     "説明",
		DescriptionFile: "/some/file.txt",
	}
	err := cmd.Run(&GlobalFlags{})
	if err == nil {
		t.Fatal("--description と --description-file 両指定はエラーが返るべき")
	}
}

func TestIssueCreateCmd_run_dryRun(t *testing.T) {
	cmd := &IssueCreateCmd{
		WriteFlags: WriteFlags{DryRun: true},
		ProjectKey: "PROJ",
		Summary:    "テスト",
		IssueType:  "Bug",
	}
	err := cmd.Run(&GlobalFlags{})
	if err != nil {
		t.Fatalf("dry-run は成功するはず: %v", err)
	}
}

// ---- IssueUpdateCmd.Run() テスト ----

func TestIssueUpdateCmd_run_noFlags(t *testing.T) {
	cmd := &IssueUpdateCmd{
		WriteFlags:   WriteFlags{DryRun: false},
		IssueIDOrKey: "PROJ-1",
	}
	err := cmd.Run(&GlobalFlags{})
	if err == nil {
		t.Fatal("更新フラグなしはエラーが返るべき")
	}
}

func TestIssueUpdateCmd_run_descriptionExclusive(t *testing.T) {
	s := "新しい説明"
	f := "/some/file.txt"
	cmd := &IssueUpdateCmd{
		WriteFlags:      WriteFlags{DryRun: false},
		IssueIDOrKey:    "PROJ-1",
		Description:     &s,
		DescriptionFile: f,
	}
	err := cmd.Run(&GlobalFlags{})
	if err == nil {
		t.Fatal("--description と --description-file 両指定はエラーが返るべき")
	}
}

func TestIssueUpdateCmd_run_dryRunWithSummary(t *testing.T) {
	s := "新しいサマリー"
	cmd := &IssueUpdateCmd{
		WriteFlags:   WriteFlags{DryRun: true},
		IssueIDOrKey: "PROJ-1",
		Summary:      &s,
	}
	err := cmd.Run(&GlobalFlags{})
	if err != nil {
		t.Fatalf("dry-run with summary は成功するはず: %v", err)
	}
}

// ---- IssueCommentAddCmd.Run() テスト ----

func TestIssueCommentAddCmd_run_noContent(t *testing.T) {
	cmd := &IssueCommentAddCmd{
		WriteFlags:   WriteFlags{DryRun: false},
		IssueIDOrKey: "PROJ-1",
	}
	err := cmd.Run(&GlobalFlags{})
	if err == nil {
		t.Fatal("content も content-file もない場合はエラーが返るべき")
	}
}

func TestIssueCommentAddCmd_run_bothContent(t *testing.T) {
	cmd := &IssueCommentAddCmd{
		WriteFlags:   WriteFlags{DryRun: false},
		IssueIDOrKey: "PROJ-1",
		Content:      "コメント",
		ContentFile:  "/some/file.txt",
	}
	err := cmd.Run(&GlobalFlags{})
	if err == nil {
		t.Fatal("content と content-file 両指定はエラーが返るべき")
	}
}

func TestIssueCommentAddCmd_run_dryRunWithContent(t *testing.T) {
	cmd := &IssueCommentAddCmd{
		WriteFlags:   WriteFlags{DryRun: true},
		IssueIDOrKey: "PROJ-1",
		Content:      "コメントテキスト",
	}
	err := cmd.Run(&GlobalFlags{})
	if err != nil {
		t.Fatalf("dry-run with content は成功するはず: %v", err)
	}
}

func TestIssueCommentAddCmd_run_contentFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "comment.txt")
	if err := os.WriteFile(path, []byte("ファイルからのコメント"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := &IssueCommentAddCmd{
		WriteFlags:   WriteFlags{DryRun: true},
		IssueIDOrKey: "PROJ-1",
		ContentFile:  path,
	}
	err := cmd.Run(&GlobalFlags{})
	if err != nil {
		t.Fatalf("content-file からの dry-run は成功するはず: %v", err)
	}
}

// ---- IssueCommentUpdateCmd.Run() テスト ----

func TestIssueCommentUpdateCmd_run_noContent(t *testing.T) {
	cmd := &IssueCommentUpdateCmd{
		WriteFlags:   WriteFlags{DryRun: false},
		IssueIDOrKey: "PROJ-1",
		CommentID:    42,
	}
	err := cmd.Run(&GlobalFlags{})
	if err == nil {
		t.Fatal("content も content-file もない場合はエラーが返るべき")
	}
}

func TestIssueCommentUpdateCmd_run_bothContent(t *testing.T) {
	cmd := &IssueCommentUpdateCmd{
		WriteFlags:   WriteFlags{DryRun: false},
		IssueIDOrKey: "PROJ-1",
		CommentID:    42,
		Content:      "コメント",
		ContentFile:  "/some/file.txt",
	}
	err := cmd.Run(&GlobalFlags{})
	if err == nil {
		t.Fatal("content と content-file 両指定はエラーが返るべき")
	}
}

func TestIssueCommentUpdateCmd_run_dryRunWithContent(t *testing.T) {
	cmd := &IssueCommentUpdateCmd{
		WriteFlags:   WriteFlags{DryRun: true},
		IssueIDOrKey: "PROJ-1",
		CommentID:    42,
		Content:      "更新コメント",
	}
	err := cmd.Run(&GlobalFlags{})
	if err != nil {
		t.Fatalf("dry-run with content は成功するはず: %v", err)
	}
}


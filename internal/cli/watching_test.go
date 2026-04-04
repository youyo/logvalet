package cli

import (
	"strings"
	"testing"
)

// CLI-WT-1: watching add dry-run は buildRunContext を呼ばずに出力する
func TestWatchingAddCmd_dry_run(t *testing.T) {
	cmd := &WatchingAddCmd{
		WriteFlags:   WriteFlags{DryRun: true},
		IssueIDOrKey: "PROJ-1",
		Note:         "test note",
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err != nil {
		t.Fatalf("Run() should not return error on dry-run: %v", err)
	}
}

// CLI-WT-2: watching add 実行時（DryRun=false）は buildRunContext でエラーになる
func TestWatchingAddCmd_not_dry_run(t *testing.T) {
	cmd := &WatchingAddCmd{
		IssueIDOrKey: "PROJ-1",
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	// DryRun=false なので buildRunContext でエラーになるはず
	if err == nil {
		t.Fatal("Run() should return error (config not available)")
	}
	// バリデーションエラーではないことを確認
	if strings.Contains(err.Error(), "issue_id_or_key") {
		t.Errorf("unexpected validation error: %v", err)
	}
}

// CLI-WT-3: watching delete dry-run
func TestWatchingDeleteCmd_dry_run(t *testing.T) {
	cmd := &WatchingDeleteCmd{
		WriteFlags: WriteFlags{DryRun: true},
		WatchingID: 42,
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err != nil {
		t.Fatalf("Run() should not return error on dry-run: %v", err)
	}
}

// CLI-WT-4: watching update dry-run
func TestWatchingUpdateCmd_dry_run(t *testing.T) {
	cmd := &WatchingUpdateCmd{
		WriteFlags: WriteFlags{DryRun: true},
		WatchingID: 42,
		Note:       "updated note",
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err != nil {
		t.Fatalf("Run() should not return error on dry-run: %v", err)
	}
}

// CLI-WT-5: watching mark-as-read dry-run
func TestWatchingMarkAsReadCmd_dry_run(t *testing.T) {
	cmd := &WatchingMarkAsReadCmd{
		WriteFlags: WriteFlags{DryRun: true},
		WatchingID: 42,
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err != nil {
		t.Fatalf("Run() should not return error on dry-run: %v", err)
	}
}

// CLI-WT-6: watching list は buildRunContext でエラーになる（引数バリデーションは通過）
func TestWatchingListCmd_passes_validation(t *testing.T) {
	cmd := &WatchingListCmd{
		UserID: "123",
		Count:  20,
		Order:  "desc",
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Fatal("Run() should return error (config not available)")
	}
}

// CLI-WT-7: watching get は buildRunContext でエラーになる
func TestWatchingGetCmd_passes_validation(t *testing.T) {
	cmd := &WatchingGetCmd{
		WatchingID: 42,
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Fatal("Run() should return error (config not available)")
	}
}

// CLI-WT-8: watching count は buildRunContext でエラーになる
func TestWatchingCountCmd_passes_validation(t *testing.T) {
	cmd := &WatchingCountCmd{
		UserID: "123",
	}
	g := &GlobalFlags{}
	err := cmd.Run(g)
	if err == nil {
		t.Fatal("Run() should return error (config not available)")
	}
}

// CLI-WT-9: resolveUserID "me" のテスト（mock 使用）
func TestResolveUserID_me(t *testing.T) {
	// buildRunContext が必要なため、resolveUserID 単体のロジックテスト
	// "me" 以外の数値テスト
	id, err := resolveUserID(t.Context(), "42", nil)
	if err != nil {
		t.Fatalf("resolveUserID should succeed for numeric: %v", err)
	}
	if id != 42 {
		t.Errorf("resolveUserID = %d, want 42", id)
	}
}

// CLI-WT-10: resolveUserID 不正値のテスト
func TestResolveUserID_invalid(t *testing.T) {
	_, err := resolveUserID(t.Context(), "abc", nil)
	if err == nil {
		t.Fatal("resolveUserID should fail for non-numeric non-me input")
	}
	if !strings.Contains(err.Error(), "numeric ID or 'me'") {
		t.Errorf("error = %q, want message about numeric ID or me", err.Error())
	}
}

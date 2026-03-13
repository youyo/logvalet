package cli

import (
	"encoding/json"
	"fmt"
	"os"
)

// validateDescriptionFlags は --description と --description-file の排他チェックを行う。
// 両方指定されている場合は exit code 2 相当のエラーを返す。
func validateDescriptionFlags(description, descriptionFile string) error {
	if description != "" && descriptionFile != "" {
		return fmt.Errorf("--description と --description-file は同時に指定できません (exit 2)")
	}
	return nil
}

// validateContentFlags は --content と --content-file の排他チェックを行う。
// 両方指定またはどちらも未指定の場合はエラーを返す。
func validateContentFlags(content, contentFile string) error {
	if content != "" && contentFile != "" {
		return fmt.Errorf("--content と --content-file は同時に指定できません (exit 2)")
	}
	if content == "" && contentFile == "" {
		return fmt.Errorf("--content または --content-file のどちらか一方を指定してください (exit 2)")
	}
	return nil
}

// validateAtLeastOneUpdateFlag は issue update コマンドの更新フィールドチェックを行う。
// 全フィールドが nil / 空の場合は exit code 2 相当のエラーを返す。
// パラメータ: summary, description, status, priority, assignee, dueDate, startDate, descriptionFile
// スライスパラメータ: categories, versions, milestones
func validateAtLeastOneUpdateFlag(
	summary *string,
	description *string,
	status *string,
	priority *string,
	assignee *string,
	dueDate *string,
	startDate *string,
	descriptionFile *string,
	slices ...[]string,
) error {
	if summary != nil || description != nil || status != nil || priority != nil ||
		assignee != nil || dueDate != nil || startDate != nil || descriptionFile != nil {
		return nil
	}
	// スライスフィールド（categories/versions/milestones）もチェック
	for _, s := range slices {
		if len(s) > 0 {
			return nil
		}
	}
	return fmt.Errorf("更新するフィールドを少なくとも1つ指定してください (exit 2)")
}

// readContentFromFile はファイルパスからテキスト内容を読み込む。
// ファイルが存在しない / 読み込み失敗の場合はエラーを返す。
func readContentFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("ファイルの読み込みに失敗しました (%s): %w", path, err)
	}
	return string(data), nil
}

// dryRunOutput は dry-run 時の出力構造体。
type dryRunOutput struct {
	DryRun    bool                   `json:"dry_run"`
	Operation string                 `json:"operation"`
	Params    map[string]interface{} `json:"params"`
}

// formatDryRun は dry-run 出力の JSON バイト列を返す。
func formatDryRun(operation string, params map[string]interface{}) ([]byte, error) {
	out := dryRunOutput{
		DryRun:    true,
		Operation: operation,
		Params:    params,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("dry-run 出力のフォーマットに失敗しました: %w", err)
	}
	return data, nil
}

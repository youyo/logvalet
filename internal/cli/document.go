package cli

import (
	"fmt"
)

// DocumentCmd は document コマンド群のルート。
type DocumentCmd struct {
	Get    DocumentGetCmd    `cmd:"" help:"ドキュメントを取得する"`
	List   DocumentListCmd   `cmd:"" help:"ドキュメント一覧を取得する"`
	Tree   DocumentTreeCmd   `cmd:"" help:"ドキュメントツリーを取得する"`
	Digest DocumentDigestCmd `cmd:"" help:"ドキュメントのダイジェストを生成する"`
	Create DocumentCreateCmd `cmd:"" help:"ドキュメントを作成する"`
}

// DocumentGetCmd は document get コマンド（spec §14.18）。
// lv document get <document_id>
type DocumentGetCmd struct {
	DocumentID int64 `arg:"" required:"" help:"ドキュメントID"`
}

func (c *DocumentGetCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("document get")
}

// DocumentListCmd は document list コマンド（spec §14.19）。
// lv document list --project <key>
type DocumentListCmd struct {
	ListFlags
	ProjectKey string `required:"" help:"プロジェクトキー"`
}

func (c *DocumentListCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("document list")
}

// DocumentTreeCmd は document tree コマンド（spec §14.20）。
// lv document tree --project <key>
type DocumentTreeCmd struct {
	ProjectKey string `required:"" help:"プロジェクトキー"`
}

func (c *DocumentTreeCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("document tree")
}

// DocumentDigestCmd は document digest コマンド（spec §14.21）。
// lv document digest <document_id>
type DocumentDigestCmd struct {
	DigestFlags
	DocumentID int64 `arg:"" required:"" help:"ドキュメントID"`
}

func (c *DocumentDigestCmd) Run(g *GlobalFlags) error {
	return ErrNotImplemented("document digest")
}

// DocumentCreateCmd は document create コマンド（spec §14.22）。
// lv document create --project <key> --title <text> (--content <text> | --content-file <path>)
type DocumentCreateCmd struct {
	WriteFlags
	ProjectKey  string `required:"" help:"プロジェクトキー"`
	Title       string `required:"" help:"ドキュメントのタイトル"`
	Content     string `help:"ドキュメントの本文（--content-file と排他）"`
	ContentFile string `help:"ドキュメント本文のファイルパス（--content と排他）" type:"existingfile"`
	ParentID    *int64 `help:"親ドキュメントID（任意）"`
}

func (c *DocumentCreateCmd) Run(g *GlobalFlags) error {
	// --content と --content-file の排他バリデーション（どちらか必須）
	if err := validateContentFlags(c.Content, c.ContentFile); err != nil {
		return err
	}

	// --content-file が指定された場合はファイルから本文を読み込む
	content := c.Content
	if c.ContentFile != "" {
		fileContent, err := readContentFromFile(c.ContentFile)
		if err != nil {
			return fmt.Errorf("--content-file の読み込みに失敗しました: %w", err)
		}
		content = fileContent
	}

	if c.DryRun {
		params := map[string]interface{}{
			"project_key": c.ProjectKey,
			"title":       c.Title,
			"content":     content,
		}
		if c.ParentID != nil {
			params["parent_id"] = *c.ParentID
		}
		out, err := formatDryRun("document_create", params)
		if err != nil {
			return fmt.Errorf("dry-run 出力のフォーマットに失敗しました: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}

	return ErrNotImplemented("document create")
}

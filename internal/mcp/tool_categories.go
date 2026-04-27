package mcp

// ToolCategory は MCP ツールの annotation カテゴリを表す。
type ToolCategory int

const (
	CategoryReadOnly          ToolCategory = iota
	CategoryWriteNonIdempotent              // create 系（同じ操作を繰り返すと副作用が積み上がる）
	CategoryWriteIdempotent                 // update / add / mark_as_read 系（同じ操作を繰り返しても副作用は変わらない）
	CategoryDestructive                     // delete 系
)

// ToolCategorySpec はツールのカテゴリと日本語 UI タイトルを保持する。
type ToolCategorySpec struct {
	Category ToolCategory
	Title    string // 日本語 UI タイトル（MCP クライアントが表示する）
}

// toolCategories は全 65 MCP ツールの annotation カテゴリと日本語 title を宣言する。
// 実ツール登録との一致は annotations_test.go / TestToolCategories_CoversAllRegisteredTools が保証する。
var toolCategories = map[string]ToolCategorySpec{
	// Read-only (32+14=46)
	"logvalet_space_info":             {CategoryReadOnly, "スペース情報取得"},
	"logvalet_project_list":           {CategoryReadOnly, "プロジェクト一覧取得"},
	"logvalet_project_get":            {CategoryReadOnly, "プロジェクト詳細取得"},
	"logvalet_project_health":         {CategoryReadOnly, "プロジェクトヘルス取得"},
	"logvalet_project_blockers":       {CategoryReadOnly, "プロジェクトブロッカー一覧取得"},
	"logvalet_team_list":              {CategoryReadOnly, "チーム一覧取得"},
	"logvalet_team_get":               {CategoryReadOnly, "チーム詳細取得"},
	"logvalet_user_list":              {CategoryReadOnly, "ユーザー一覧取得"},
	"logvalet_user_get":               {CategoryReadOnly, "ユーザー詳細取得"},
	"logvalet_user_workload":          {CategoryReadOnly, "ユーザー稼働状況取得"},
	"logvalet_activity_list":          {CategoryReadOnly, "アクティビティ一覧取得"},
	"logvalet_activity_stats":         {CategoryReadOnly, "アクティビティ統計取得"},
	"logvalet_issue_list":             {CategoryReadOnly, "課題一覧取得"},
	"logvalet_issue_get":              {CategoryReadOnly, "課題詳細取得"},
	"logvalet_issue_context":          {CategoryReadOnly, "課題コンテキスト取得"},
	"logvalet_issue_timeline":         {CategoryReadOnly, "課題タイムライン取得"},
	"logvalet_issue_triage_materials": {CategoryReadOnly, "課題トリアージ材料取得"},
	"logvalet_issue_stale":            {CategoryReadOnly, "停滞課題一覧取得"},
	"logvalet_issue_attachment_list":  {CategoryReadOnly, "課題添付ファイル一覧取得"},
	"logvalet_issue_comment_list":     {CategoryReadOnly, "課題コメント一覧取得"},
	"logvalet_my_tasks":               {CategoryReadOnly, "自分のタスク一覧取得"},
	"logvalet_digest_daily":           {CategoryReadOnly, "日次ダイジェスト取得"},
	"logvalet_digest_weekly":          {CategoryReadOnly, "週次ダイジェスト取得"},
	"logvalet_document_list":          {CategoryReadOnly, "ドキュメント一覧取得"},
	"logvalet_document_get":           {CategoryReadOnly, "ドキュメント取得"},
	"logvalet_shared_file_list":       {CategoryReadOnly, "共有ファイル一覧取得"},
	"logvalet_meta_categories":        {CategoryReadOnly, "カテゴリ一覧取得"},
	"logvalet_meta_issue_types":       {CategoryReadOnly, "課題種別一覧取得"},
	"logvalet_meta_statuses":          {CategoryReadOnly, "ステータス一覧取得"},
	"logvalet_watching_list":          {CategoryReadOnly, "ウォッチ一覧取得"},
	"logvalet_watching_get":           {CategoryReadOnly, "ウォッチ詳細取得"},
	"logvalet_watching_count":         {CategoryReadOnly, "ウォッチ数取得"},
	// M02 新規追加 (14)
	"logvalet_user_me":                    {CategoryReadOnly, "認証ユーザー情報取得"},
	"logvalet_user_activity":              {CategoryReadOnly, "ユーザーアクティビティ取得"},
	"logvalet_digest_unified":             {CategoryReadOnly, "統合ダイジェスト生成"},
	"logvalet_activity_digest":            {CategoryReadOnly, "アクティビティダイジェスト生成"},
	"logvalet_document_tree":              {CategoryReadOnly, "ドキュメントツリー取得"},
	"logvalet_document_digest":            {CategoryReadOnly, "ドキュメントダイジェスト生成"},
	"logvalet_space_digest":               {CategoryReadOnly, "スペースダイジェスト生成"},
	"logvalet_space_disk_usage":           {CategoryReadOnly, "スペースディスク使用量取得"},
	"logvalet_meta_version":               {CategoryReadOnly, "バージョン一覧取得"},
	"logvalet_meta_custom_field":          {CategoryReadOnly, "カスタムフィールド一覧取得"},
	"logvalet_team_project":               {CategoryReadOnly, "プロジェクトチーム一覧取得"},
	"logvalet_issue_attachment_download":  {CategoryReadOnly, "添付ファイルダウンロード"},
	"logvalet_shared_file_download":       {CategoryReadOnly, "共有ファイルダウンロード"},
	// Wiki (8)
	"logvalet_wiki_list":             {CategoryReadOnly, "Wiki ページ一覧取得"},
	"logvalet_wiki_get":              {CategoryReadOnly, "Wiki ページ取得"},
	"logvalet_wiki_count":            {CategoryReadOnly, "Wiki ページ件数取得"},
	"logvalet_wiki_tags":             {CategoryReadOnly, "Wiki タグ一覧取得"},
	"logvalet_wiki_history":          {CategoryReadOnly, "Wiki 履歴取得"},
	"logvalet_wiki_stars":            {CategoryReadOnly, "Wiki スター一覧取得"},
	"logvalet_wiki_attachment_list":  {CategoryReadOnly, "Wiki 添付ファイル一覧取得"},
	"logvalet_wiki_sharedfile_list":  {CategoryReadOnly, "Wiki 共有ファイル一覧取得"},

	// Write non-idempotent (4)
	"logvalet_issue_create":             {CategoryWriteNonIdempotent, "課題作成"},
	"logvalet_issue_comment_add":        {CategoryWriteNonIdempotent, "課題コメント追加"},
	"logvalet_document_create":          {CategoryWriteNonIdempotent, "ドキュメント作成"},
	"logvalet_issue_attachment_upload":  {CategoryWriteNonIdempotent, "添付ファイルアップロード"},

	// Write idempotent (6)
	"logvalet_issue_update":          {CategoryWriteIdempotent, "課題更新"},
	"logvalet_issue_comment_update":  {CategoryWriteIdempotent, "課題コメント更新"},
	"logvalet_star_add":              {CategoryWriteIdempotent, "スター追加"},
	"logvalet_watching_add":          {CategoryWriteIdempotent, "ウォッチ追加"},
	"logvalet_watching_update":       {CategoryWriteIdempotent, "ウォッチ更新"},
	"logvalet_watching_mark_as_read": {CategoryWriteIdempotent, "ウォッチ既読化"},

	// Destructive (2)
	"logvalet_watching_delete":          {CategoryDestructive, "ウォッチ削除"},
	"logvalet_issue_attachment_delete":  {CategoryDestructive, "添付ファイル削除"},
}

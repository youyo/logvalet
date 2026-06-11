package digest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

var updateGolden = flag.Bool("update", false, "golden ファイルを更新する")

// ---- extractSnippet ユニットテスト ----

// ES1: ASCII keyword、中央ヒット → keyword 周辺 ±100 rune
func TestExtractSnippet_ASCIIKeywordCenter(t *testing.T) {
	prefix := string(make([]rune, 120)) // 120 rune のパディング
	suffix := string(make([]rune, 120))
	plain := prefix + "hello" + suffix
	got := extractSnippet(plain, "hello")
	runes := []rune(got)
	// snippetRadius*2 + len("hello") = 205 以下であること
	if len(runes) > snippetRadius*2+5 {
		t.Errorf("snippet too long: %d runes, want <= %d", len(runes), snippetRadius*2+5)
	}
	// "hello" が含まれること
	if !containsRune(got, "hello") {
		t.Errorf("snippet does not contain keyword 'hello': %q", got)
	}
}

// ES2: 日本語 keyword → []rune ベースで正しい切り出し（plain > 200 rune で実際に切り出しを検証）
func TestExtractSnippet_JapaneseKeyword(t *testing.T) {
	// 各部分が 110 rune ずつ → 合計 110 + 2 + 110 = 222 rune > snippetRadius*2(200)
	prefix := make([]rune, 110)
	for i := range prefix {
		prefix[i] = 'あ' + rune(i%46) // ひらがな範囲でローテーション
	}
	plain := string(prefix) + "認証" + string(prefix)
	got := extractSnippet(plain, "認証")

	// "認証" が snippet に含まれること
	if !containsRune(got, "認証") {
		t.Errorf("snippet does not contain '認証': %q", got)
	}
	// plain より短く切り出されていること（rune-slicing が機能している）
	gotRunes := len([]rune(got))
	plainRunes := len([]rune(plain))
	if gotRunes >= plainRunes {
		t.Errorf("snippet was not truncated: got %d runes, plain has %d runes", gotRunes, plainRunes)
	}
	// snippetRadius*2 以内であること
	if gotRunes > snippetRadius*2 {
		t.Errorf("snippet too long: %d runes (want <= %d)", gotRunes, snippetRadius*2)
	}
}

// ES3: ケースインセンシティブ → "oauth" で "OAuth" にマッチ
func TestExtractSnippet_CaseInsensitive(t *testing.T) {
	plain := "ここで OAuth トークンを使います"
	got := extractSnippet(plain, "oauth")
	if !containsRune(got, "OAuth") {
		t.Errorf("case-insensitive match failed, got: %q", got)
	}
}

// ES4: keyword なし → 先頭 200 rune のリード抜粋
func TestExtractSnippet_EmptyKeyword(t *testing.T) {
	plain := string(make([]rune, 300))
	got := extractSnippet(plain, "")
	runeLen := len([]rune(got))
	if runeLen != snippetRadius*2 {
		t.Errorf("lead excerpt len = %d, want %d", runeLen, snippetRadius*2)
	}
}

// ES4b: keyword ヒットなし → 先頭 200 rune のリード抜粋
func TestExtractSnippet_NoMatch(t *testing.T) {
	plain := string(make([]rune, 300))
	got := extractSnippet(plain, "zzz")
	runeLen := len([]rune(got))
	if runeLen != snippetRadius*2 {
		t.Errorf("lead excerpt len = %d, want %d", runeLen, snippetRadius*2)
	}
}

// ES5: 複数語 keyword（スペース区切り）→ キーワード文字列の先頭語をアンカー
// "alpha beta" なら "alpha" が先頭語 → "alpha" の周辺を抜粋する。
// plain 内では "beta" が先に登場し "alpha" が後に登場するが、
// アンカーはテキスト出現順ではなく keyword 先頭語で決まる。
func TestExtractSnippet_MultiWord(t *testing.T) {
	// snippetRadius*2 = 200 rune を超えるよう構築する。
	// 構造: pad(110) + "beta" + sep(110) + "alpha" + tail(10)
	// "beta" と "alpha" は 110+ rune 離れているため、"alpha" アンカーの snippet に "beta" は入らない。
	pad := string(make([]rune, 110))
	sep := string(make([]rune, 110))
	tail := string(make([]rune, 10))
	plain := pad + "beta" + sep + "alpha" + tail

	// keyword は "alpha beta" → 先頭語 "alpha" が最初に試みられアンカーになる
	got := extractSnippet(plain, "alpha beta")

	// "alpha" がアンカーなので snippet に含まれること
	if !containsRune(got, "alpha") {
		t.Errorf("multi-word: keyword-first word 'alpha' should be anchor, got: %q", got)
	}
	// "beta" は "alpha" から 110+ rune 離れているため snippet に含まれないこと
	// （アンカーが "beta" だった場合と区別するための検証）
	if containsRune(got, "beta") {
		t.Errorf("multi-word: 'beta' should NOT be in snippet when 'alpha' is anchor (words are 110+ runes apart), got: %q", got)
	}
}

// ES6: plain が radius*2 より短い → plain 全体を返す
func TestExtractSnippet_ShortPlain(t *testing.T) {
	plain := "短い本文"
	got := extractSnippet(plain, "本文")
	if got != plain {
		t.Errorf("short plain: got %q, want %q", got, plain)
	}
}

// containsRune は s が substr を含むか確認するヘルパー
func containsRune(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	runesS := []rune(s)
	runesSubstr := []rune(substr)
	for i := 0; i <= len(runesS)-len(runesSubstr); i++ {
		match := true
		for j, r := range runesSubstr {
			if runesS[i+j] != r {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// ---- DocumentSearchBuilder.Build() ユニットテスト ----

func newTestDocumentSearchBuilder() *DefaultDocumentSearchBuilder {
	mock := backlog.NewMockClient()
	mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
		return []domain.Project{{ID: 10, ProjectKey: "PROJ"}}, nil
	}
	return NewDefaultDocumentSearchBuilder(mock, "default", "myspace", "https://example.backlog.com")
}

func makeTestDoc(id string, title, plain string) domain.Document {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return domain.Document{
		ID:        id,
		ProjectID: 10,
		Title:     title,
		Plain:     plain,
		Created:   &now,
		Updated:   &now,
	}
}

// B1: 空スライス → Items=[], total_returned=0, possibly_more=false
func TestDocumentSearchBuilder_Build_Empty(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	env := b.Build(context.Background(), []domain.Document{}, DocumentSearchOptions{Keyword: "test"})
	d := env.Digest.(*DocumentSearchDigest)
	if d.TotalReturned != 0 {
		t.Errorf("TotalReturned = %d, want 0", d.TotalReturned)
	}
	if d.PossiblyMore {
		t.Error("PossiblyMore = true, want false")
	}
	if d.Items == nil {
		t.Error("Items = nil, want empty slice []")
	}
	if len(d.Items) != 0 {
		t.Errorf("len(Items) = %d, want 0", len(d.Items))
	}
}

// B2: snippet モード → Snippet 非空・Plain 空
func TestDocumentSearchBuilder_Build_SnippetMode(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	docs := []domain.Document{
		makeTestDoc("doc-001", "OAuth ドキュメント", "OAuth トークンの説明テキストが含まれます"),
		makeTestDoc("doc-002", "別ドキュメント", "OAuth に関する別の説明です"),
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "OAuth", Detail: "snippet"})
	d := env.Digest.(*DocumentSearchDigest)
	for i, item := range d.Items {
		if item.Snippet == "" {
			t.Errorf("Items[%d].Snippet is empty, want non-empty (snippet mode)", i)
		}
		if item.Plain != "" {
			t.Errorf("Items[%d].Plain = %q, want empty (snippet mode)", i, item.Plain)
		}
	}
}

// B3: meta モード → Snippet 空・Plain 空
func TestDocumentSearchBuilder_Build_MetaMode(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	docs := []domain.Document{
		makeTestDoc("doc-001", "タイトル", "本文テキスト"),
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "k", Detail: "meta"})
	d := env.Digest.(*DocumentSearchDigest)
	for i, item := range d.Items {
		if item.Snippet != "" {
			t.Errorf("Items[%d].Snippet = %q, want empty (meta mode)", i, item.Snippet)
		}
		if item.Plain != "" {
			t.Errorf("Items[%d].Plain = %q, want empty (meta mode)", i, item.Plain)
		}
	}
}

// B4: full モード → Snippet 非空・Plain が全文
func TestDocumentSearchBuilder_Build_FullMode(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	plain := "テスト本文。ここにキーワードが含まれます。"
	docs := []domain.Document{
		makeTestDoc("doc-001", "タイトル", plain),
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "キーワード", Detail: "full"})
	d := env.Digest.(*DocumentSearchDigest)
	if len(d.Items) != 1 {
		t.Fatalf("Items len = %d, want 1", len(d.Items))
	}
	if d.Items[0].Snippet == "" {
		t.Error("Items[0].Snippet is empty, want non-empty (full mode)")
	}
	if d.Items[0].Plain != plain {
		t.Errorf("Items[0].Plain = %q, want %q (full mode)", d.Items[0].Plain, plain)
	}
}

// B5: 100件 → PossiblyMore=true
func TestDocumentSearchBuilder_Build_PossiblyMore_True(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	docs := make([]domain.Document, 100)
	for i := range docs {
		docs[i] = makeTestDoc("doc", "t", "p")
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: ""})
	d := env.Digest.(*DocumentSearchDigest)
	if !d.PossiblyMore {
		t.Error("PossiblyMore = false, want true (100 items)")
	}
}

// B6: 99件 → PossiblyMore=false
func TestDocumentSearchBuilder_Build_PossiblyMore_False(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	docs := make([]domain.Document, 99)
	for i := range docs {
		docs[i] = makeTestDoc("doc", "t", "p")
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: ""})
	d := env.Digest.(*DocumentSearchDigest)
	if d.PossiblyMore {
		t.Error("PossiblyMore = true, want false (99 items)")
	}
}

// B7: keyword ヒットなし → リード抜粋（先頭 200 rune 以内）
func TestDocumentSearchBuilder_Build_KeywordNoMatch_LeadExcerpt(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	plain := string(make([]rune, 300)) // 300 rune の長いテキスト
	docs := []domain.Document{
		makeTestDoc("doc-001", "タイトル", plain),
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "zzz", Detail: "snippet"})
	d := env.Digest.(*DocumentSearchDigest)
	if len(d.Items) == 0 {
		t.Fatal("Items is empty")
	}
	runeLen := len([]rune(d.Items[0].Snippet))
	if runeLen > snippetRadius*2 {
		t.Errorf("snippet len = %d runes, want <= %d (lead excerpt)", runeLen, snippetRadius*2)
	}
}

// B8: DigestEnvelope 構造 → Resource="document_search"、Warnings=[]
func TestDocumentSearchBuilder_Build_EnvelopeStructure(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	env := b.Build(context.Background(), []domain.Document{}, DocumentSearchOptions{})
	if env.Resource != "document_search" {
		t.Errorf("Resource = %q, want %q", env.Resource, "document_search")
	}
	if env.Warnings == nil {
		t.Error("Warnings = nil, want []")
	}
	if len(env.Warnings) != 0 {
		t.Errorf("Warnings len = %d, want 0", len(env.Warnings))
	}
	if _, ok := env.Digest.(*DocumentSearchDigest); !ok {
		t.Errorf("Digest type = %T, want *DocumentSearchDigest", env.Digest)
	}
}

// ---- golden test ----

// GoldenTest: 2件・snippet モードの Build() JSON 出力が golden ファイルと一致すること
func TestDocumentSearchBuilder_Build_Golden(t *testing.T) {
	b := newTestDocumentSearchBuilder()

	now := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	docs := []domain.Document{
		{
			ID:          "doc-golden-001",
			ProjectID:   10,
			Title:       "OAuth 認証ガイド",
			Plain:       "OAuth 2.0 を使った認証フローの説明です。クライアントIDとシークレットを設定してください。",
			Created:     &now,
			Updated:     &now,
			CreatedUser: &domain.User{ID: 1, Name: "Alice"},
			UpdatedUser: &domain.User{ID: 2, Name: "Bob"},
		},
		{
			ID:        "doc-golden-002",
			ProjectID: 10,
			Title:     "API リファレンス",
			Plain:     "OAuth トークンを Authorization ヘッダーに付与してリクエストします。",
			Created:   &now,
			Updated:   &now,
		},
	}

	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "OAuth", Detail: "snippet"})
	digestData := env.Digest.(*DocumentSearchDigest)

	got, err := json.MarshalIndent(digestData, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent: %v", err)
	}

	goldenPath := "testdata/document_search_snippet.json"
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		t.Logf("golden ファイルを更新しました: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden ファイルの読み込みに失敗（-update で作成してください）: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("golden mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// B_DetailDefault: Detail が空文字の場合は "snippet" として扱われること
func TestDocumentSearchBuilder_Build_DetailDefault(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	docs := []domain.Document{
		makeTestDoc("doc-001", "タイトル", "キーワードを含む本文テキスト"),
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "キーワード"}) // Detail 未指定
	d := env.Digest.(*DocumentSearchDigest)
	if d.Detail != "snippet" {
		t.Errorf("Detail = %q, want %q (default should be snippet)", d.Detail, "snippet")
	}
}

// ---- URL フィールドテスト（M6）----

// U1: ListProjects 成功 → url が "{baseURL}/document/{projectKey}/{docID}" 形式で構築される
func TestDocumentSearchBuilder_Build_URL_Constructed(t *testing.T) {
	b := newTestDocumentSearchBuilder() // ListProjects → {ID:10, ProjectKey:"PROJ"}
	docs := []domain.Document{
		makeTestDoc("doc-001", "タイトル", "本文"),
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "k", Detail: "snippet"})
	d := env.Digest.(*DocumentSearchDigest)
	if len(d.Items) != 1 {
		t.Fatalf("Items len = %d, want 1", len(d.Items))
	}
	want := "https://example.backlog.com/document/PROJ/doc-001"
	if d.Items[0].URL != want {
		t.Errorf("URL = %q, want %q", d.Items[0].URL, want)
	}
	if len(env.Warnings) != 0 {
		t.Errorf("Warnings len = %d, want 0 (no failure)", len(env.Warnings))
	}
}

// U2: ListProjects 失敗 → url が空・warnings が1件（partial success）
func TestDocumentSearchBuilder_Build_URL_ListProjectsFailed(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
		return nil, errors.New("api error")
	}
	b := NewDefaultDocumentSearchBuilder(mock, "default", "myspace", "https://example.backlog.com")
	docs := []domain.Document{
		makeTestDoc("doc-001", "タイトル", "本文"),
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "k", Detail: "snippet"})
	d := env.Digest.(*DocumentSearchDigest)
	if d.Items[0].URL != "" {
		t.Errorf("URL = %q, want empty (ListProjects failed)", d.Items[0].URL)
	}
	if len(env.Warnings) != 1 {
		t.Fatalf("Warnings len = %d, want 1 (partial success)", len(env.Warnings))
	}
	// digest 本体は返ること（partial success）
	if d.TotalReturned != 1 {
		t.Errorf("TotalReturned = %d, want 1 (digest still returned)", d.TotalReturned)
	}
}

// U3: projectKey 不在（マップに projectID がない）→ url が空・エラーなし
func TestDocumentSearchBuilder_Build_URL_ProjectKeyMissing(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
		// projectID=10 を含まないプロジェクトのみ返す
		return []domain.Project{{ID: 999, ProjectKey: "OTHER"}}, nil
	}
	b := NewDefaultDocumentSearchBuilder(mock, "default", "myspace", "https://example.backlog.com")
	docs := []domain.Document{
		makeTestDoc("doc-001", "タイトル", "本文"), // ProjectID=10
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "k", Detail: "snippet"})
	d := env.Digest.(*DocumentSearchDigest)
	if d.Items[0].URL != "" {
		t.Errorf("URL = %q, want empty (projectKey missing)", d.Items[0].URL)
	}
	if len(env.Warnings) != 0 {
		t.Errorf("Warnings len = %d, want 0 (no error when key missing)", len(env.Warnings))
	}
}

// U4: baseURL 空 → url が空・ListProjects 呼ばれず・エラーなし
func TestDocumentSearchBuilder_Build_URL_EmptyBaseURL(t *testing.T) {
	mock := backlog.NewMockClient()
	called := false
	mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
		called = true
		return []domain.Project{{ID: 10, ProjectKey: "PROJ"}}, nil
	}
	b := NewDefaultDocumentSearchBuilder(mock, "default", "myspace", "")
	docs := []domain.Document{
		makeTestDoc("doc-001", "タイトル", "本文"),
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "k", Detail: "snippet"})
	d := env.Digest.(*DocumentSearchDigest)
	if d.Items[0].URL != "" {
		t.Errorf("URL = %q, want empty (baseURL empty)", d.Items[0].URL)
	}
	if called {
		t.Error("ListProjects should NOT be called when baseURL is empty (AD12)")
	}
	if len(env.Warnings) != 0 {
		t.Errorf("Warnings len = %d, want 0", len(env.Warnings))
	}
}

// U5: verbosity 非依存 → detail="meta" でも url が設定される
func TestDocumentSearchBuilder_Build_URL_VerbosityIndependent(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	docs := []domain.Document{
		makeTestDoc("doc-001", "タイトル", "本文"),
	}
	for _, detail := range []string{"snippet", "meta", "full"} {
		env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "k", Detail: detail})
		d := env.Digest.(*DocumentSearchDigest)
		want := "https://example.backlog.com/document/PROJ/doc-001"
		if d.Items[0].URL != want {
			t.Errorf("detail=%q: URL = %q, want %q (url is verbosity independent)", detail, d.Items[0].URL, want)
		}
	}
}

// ---- M7: ページネーション改善テスト ----

// P1: バグ修正確認 - count=50 で 50件返却時 PossiblyMore=true（旧実装は偽陰性）
func TestDocumentSearchBuilder_Build_PossiblyMore_Count50_ExactlyFull(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	docs := make([]domain.Document, 50)
	for i := range docs {
		docs[i] = makeTestDoc("doc", "t", "p")
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "", RequestedCount: 50})
	d := env.Digest.(*DocumentSearchDigest)
	if !d.PossiblyMore {
		t.Error("PossiblyMore = false, want true (50 docs == requestedCount=50)")
	}
	if d.NextOffset != 50 {
		t.Errorf("NextOffset = %d, want 50", d.NextOffset)
	}
}

// P2: count=50 で 49件返却 → PossiblyMore=false
func TestDocumentSearchBuilder_Build_PossiblyMore_Count50_LessThan(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	docs := make([]domain.Document, 49)
	for i := range docs {
		docs[i] = makeTestDoc("doc", "t", "p")
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "", RequestedCount: 50})
	d := env.Digest.(*DocumentSearchDigest)
	if d.PossiblyMore {
		t.Error("PossiblyMore = true, want false (49 docs < requestedCount=50)")
	}
}

// P3: AD7 維持 - count=100 で 100件返却 → PossiblyMore=true
func TestDocumentSearchBuilder_Build_PossiblyMore_Count100_Full(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	docs := make([]domain.Document, 100)
	for i := range docs {
		docs[i] = makeTestDoc("doc", "t", "p")
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "", RequestedCount: 100})
	d := env.Digest.(*DocumentSearchDigest)
	if !d.PossiblyMore {
		t.Error("PossiblyMore = false, want true (100 docs == requestedCount=100)")
	}
	if d.NextOffset != 100 {
		t.Errorf("NextOffset = %d, want 100", d.NextOffset)
	}
}

// P4: offset=50 で 50件 → NextOffset=100
func TestDocumentSearchBuilder_Build_NextOffset_WithOffset(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	docs := make([]domain.Document, 50)
	for i := range docs {
		docs[i] = makeTestDoc("doc", "t", "p")
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "", RequestedCount: 50, Offset: 50})
	d := env.Digest.(*DocumentSearchDigest)
	if !d.PossiblyMore {
		t.Error("PossiblyMore = false, want true")
	}
	if d.NextOffset != 100 {
		t.Errorf("NextOffset = %d, want 100 (offset=50 + len=50)", d.NextOffset)
	}
}

// P5: possibly_more=false の場合 NextOffset=0
func TestDocumentSearchBuilder_Build_NextOffset_Zero_WhenNotPossiblyMore(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	docs := make([]domain.Document, 99)
	for i := range docs {
		docs[i] = makeTestDoc("doc", "t", "p")
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "", RequestedCount: 100})
	d := env.Digest.(*DocumentSearchDigest)
	if d.PossiblyMore {
		t.Error("PossiblyMore = true, want false (99 docs < 100)")
	}
	if d.NextOffset != 0 {
		t.Errorf("NextOffset = %d, want 0 (possibly_more=false)", d.NextOffset)
	}
}

// P6: RequestedCount=0 フォールバック → 0は100として扱う → 100件で PossiblyMore=true
func TestDocumentSearchBuilder_Build_RequestedCount_Zero_Fallback(t *testing.T) {
	b := newTestDocumentSearchBuilder()
	docs := make([]domain.Document, 100)
	for i := range docs {
		docs[i] = makeTestDoc("doc", "t", "p")
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "", RequestedCount: 0})
	d := env.Digest.(*DocumentSearchDigest)
	if !d.PossiblyMore {
		t.Error("PossiblyMore = false, want true (RequestedCount=0 falls back to 100)")
	}
}

// U6: baseURL 末尾スラッシュ → 二重スラッシュにならない
func TestDocumentSearchBuilder_Build_URL_TrailingSlash(t *testing.T) {
	mock := backlog.NewMockClient()
	mock.ListProjectsFunc = func(ctx context.Context) ([]domain.Project, error) {
		return []domain.Project{{ID: 10, ProjectKey: "PROJ"}}, nil
	}
	b := NewDefaultDocumentSearchBuilder(mock, "default", "myspace", "https://example.backlog.com/")
	docs := []domain.Document{
		makeTestDoc("doc-001", "タイトル", "本文"),
	}
	env := b.Build(context.Background(), docs, DocumentSearchOptions{Keyword: "k", Detail: "snippet"})
	d := env.Digest.(*DocumentSearchDigest)
	want := "https://example.backlog.com/document/PROJ/doc-001"
	if d.Items[0].URL != want {
		t.Errorf("URL = %q, want %q (no double slash)", d.Items[0].URL, want)
	}
}

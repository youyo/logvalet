package digest

import (
	"time"
	"unicode"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// snippetRadius は keyword 前後に含める rune 数。
const snippetRadius = 100

// DocumentSearchDetail は検索結果の1件エントリ。
type DocumentSearchDetail struct {
	ID          string          `json:"id"`
	ProjectID   int             `json:"project_id"`
	Title       string          `json:"title"`
	Snippet     string          `json:"snippet,omitempty"`   // snippet/full のみ
	Plain       string          `json:"plain,omitempty"`     // full のみ
	Created     *time.Time      `json:"created,omitempty"`
	Updated     *time.Time      `json:"updated,omitempty"`
	CreatedUser *domain.UserRef `json:"created_user,omitempty"`
	UpdatedUser *domain.UserRef `json:"updated_user,omitempty"`
}

// DocumentSearchDigest は document search digest のトップレベル。
type DocumentSearchDigest struct {
	Keyword       string                 `json:"keyword"`
	Detail        string                 `json:"detail"`         // "snippet" | "meta" | "full"
	TotalReturned int                    `json:"total_returned"`
	PossiblyMore  bool                   `json:"possibly_more"` // true = 100件ちょうど返却
	Items         []DocumentSearchDetail `json:"items"`
}

// DocumentSearchOptions は DocumentSearchBuilder.Build() のオプション。
type DocumentSearchOptions struct {
	Keyword string // スニペット抽出のアンカー語（空可）
	Detail  string // "snippet"（既定）| "meta" | "full"
}

// DocumentSearchBuilder は []domain.Document から DigestEnvelope を生成するインターフェース。
type DocumentSearchBuilder interface {
	Build(docs []domain.Document, opt DocumentSearchOptions) *domain.DigestEnvelope
}

// DefaultDocumentSearchBuilder は DocumentSearchBuilder の標準実装。
type DefaultDocumentSearchBuilder struct {
	BaseDigestBuilder
}

// NewDefaultDocumentSearchBuilder は DefaultDocumentSearchBuilder を生成する。
func NewDefaultDocumentSearchBuilder(client backlog.Client, profile, space, baseURL string) *DefaultDocumentSearchBuilder {
	return &DefaultDocumentSearchBuilder{BaseDigestBuilder{client: client, profile: profile, space: space, baseURL: baseURL}}
}

// Build は []domain.Document から DigestEnvelope を構築する。
// API コールは行わず、渡されたデータから純粋に digest を構築する。
func (b *DefaultDocumentSearchBuilder) Build(docs []domain.Document, opt DocumentSearchOptions) *domain.DigestEnvelope {
	// Detail のデフォルトは "snippet"
	detail := opt.Detail
	if detail == "" {
		detail = "snippet"
	}

	items := make([]DocumentSearchDetail, 0, len(docs))
	for _, doc := range docs {
		item := DocumentSearchDetail{
			ID:          doc.ID,
			ProjectID:   doc.ProjectID,
			Title:       doc.Title,
			Created:     doc.Created,
			Updated:     doc.Updated,
			CreatedUser: toUserRef(doc.CreatedUser),
			UpdatedUser: toUserRef(doc.UpdatedUser),
		}

		switch detail {
		case "snippet":
			item.Snippet = extractSnippet(doc.Plain, opt.Keyword)
		case "full":
			item.Snippet = extractSnippet(doc.Plain, opt.Keyword)
			item.Plain = doc.Plain
		// "meta": Snippet も Plain も返さない（ゼロ値のまま）
		}

		items = append(items, item)
	}

	digestData := &DocumentSearchDigest{
		Keyword:       opt.Keyword,
		Detail:        detail,
		TotalReturned: len(docs),
		PossiblyMore:  len(docs) >= 100,
		Items:         items,
	}

	return b.newEnvelope("document_search", digestData, nil)
}

// extractSnippet は plain からキーワード周辺 ±snippetRadius rune を切り出す。
//
// - []rune ベース（マルチバイト安全、[]byte 禁止）
// - ケースインセンシティブ（unicode.ToLower で1:1 rune インデックスを保持）
// - 複数語（スペース区切り）: 最初にマッチした語をアンカーにする
// - keyword が空 or ヒットなし: 先頭 snippetRadius*2 rune をリード抜粋
// - plain が snippetRadius*2 以下: plain 全体を返す
func extractSnippet(plain, keyword string) string {
	runes := []rune(plain)
	total := len(runes)

	// plain が radius*2 以下なら全体を返す
	if total <= snippetRadius*2 {
		return plain
	}

	// keyword が空の場合はリード抜粋
	if keyword == "" {
		return string(runes[:snippetRadius*2])
	}

	// lower 変換（1:1 インデックス保持）
	lower := make([]rune, total)
	for i, r := range runes {
		lower[i] = unicode.ToLower(r)
	}

	// keyword をスペースで分割し、各語を検索（最初にマッチした語をアンカー）
	words := splitKeyword(keyword)
	anchorIdx := -1
	for _, word := range words {
		wordRunes := []rune(word)
		wLen := len(wordRunes)
		// word を lower 変換
		wordLower := make([]rune, wLen)
		for i, r := range wordRunes {
			wordLower[i] = unicode.ToLower(r)
		}
		idx := runeIndex(lower, wordLower)
		if idx >= 0 {
			anchorIdx = idx
			break
		}
	}

	// ヒットなし → リード抜粋
	if anchorIdx < 0 {
		return string(runes[:snippetRadius*2])
	}

	// アンカーを中心に ±snippetRadius を切り出す
	start := anchorIdx - snippetRadius
	if start < 0 {
		start = 0
	}
	end := anchorIdx + snippetRadius
	if end > total {
		end = total
	}

	return string(runes[start:end])
}

// splitKeyword はキーワードをスペースで分割し空文字列を除去する。
func splitKeyword(keyword string) []string {
	words := make([]string, 0)
	current := make([]rune, 0)
	for _, r := range keyword {
		if r == ' ' || r == '\t' {
			if len(current) > 0 {
				words = append(words, string(current))
				current = current[:0]
			}
		} else {
			current = append(current, r)
		}
	}
	if len(current) > 0 {
		words = append(words, string(current))
	}
	return words
}

// runeIndex は haystack 内で needle の最初の出現位置を返す。見つからない場合は -1。
func runeIndex(haystack, needle []rune) int {
	nLen := len(needle)
	if nLen == 0 {
		return 0
	}
	hLen := len(haystack)
	for i := 0; i <= hLen-nLen; i++ {
		match := true
		for j := 0; j < nLen; j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

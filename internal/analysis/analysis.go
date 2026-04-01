// Package analysis は logvalet の分析機能を提供する。
//
// digest パッケージが「何があるか（what）」を返すのに対し、
// analysis パッケージは「だから何か（so what）」を返す。
// AnalysisEnvelope は DigestEnvelope と同構造で、LLM が同パターンで処理可能。
package analysis

import (
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// AnalysisEnvelope は全 analysis コマンドの共通ラッパー。
// DigestEnvelope と同構造（LLM が同パターンで処理可能）。
type AnalysisEnvelope struct {
	SchemaVersion string           `json:"schema_version"`
	Resource      string           `json:"resource"`
	GeneratedAt   time.Time        `json:"generated_at"`
	Profile       string           `json:"profile"`
	Space         string           `json:"space"`
	BaseURL       string           `json:"base_url"`
	Warnings      []domain.Warning `json:"warnings"`
	Analysis      any              `json:"analysis"`
}

// BaseAnalysisBuilder は全 AnalysisBuilder に共通するフィールドと helper を提供する。
type BaseAnalysisBuilder struct {
	client  backlog.Client
	profile string
	space   string
	baseURL string
	now     func() time.Time
}

// Option は BaseAnalysisBuilder のオプション設定関数型。
type Option func(*BaseAnalysisBuilder)

// WithClock はテスト用の clock injection オプション。
func WithClock(now func() time.Time) Option {
	return func(b *BaseAnalysisBuilder) {
		b.now = now
	}
}

// NewBaseAnalysisBuilder は BaseAnalysisBuilder を生成する。
func NewBaseAnalysisBuilder(client backlog.Client, profile, space, baseURL string, opts ...Option) BaseAnalysisBuilder {
	b := BaseAnalysisBuilder{
		client:  client,
		profile: profile,
		space:   space,
		baseURL: baseURL,
		now:     time.Now,
	}
	for _, opt := range opts {
		opt(&b)
	}
	return b
}

// newEnvelope は AnalysisEnvelope を組み立てる共通 helper。
func (b *BaseAnalysisBuilder) newEnvelope(resource string, analysis any, warnings []domain.Warning) *AnalysisEnvelope {
	if warnings == nil {
		warnings = []domain.Warning{}
	}
	return &AnalysisEnvelope{
		SchemaVersion: "1",
		Resource:      resource,
		GeneratedAt:   b.now().UTC(),
		Profile:       b.profile,
		Space:         b.space,
		BaseURL:       b.baseURL,
		Warnings:      warnings,
		Analysis:      analysis,
	}
}

// toUserRef は domain.User を domain.UserRef に変換する（nil 安全）。
func toUserRef(user *domain.User) *domain.UserRef {
	if user == nil {
		return nil
	}
	return &domain.UserRef{ID: user.ID, Name: user.Name}
}

// extractProjectKey は issueKey（例: "PROJ-123"）からプロジェクトキー（"PROJ"）を抽出する。
func extractProjectKey(issueKey string) string {
	for i, c := range issueKey {
		if c == '-' {
			return issueKey[:i]
		}
	}
	return issueKey
}

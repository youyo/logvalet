package digest

import (
	"context"
	"fmt"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// SpaceDigestOptions は SpaceDigestBuilder.Build() のオプション。
// 将来の拡張のためのプレースホルダー。
type SpaceDigestOptions struct{}

// SpaceDigestBuilder はインターフェース（spec §13.7）。
type SpaceDigestBuilder interface {
	Build(ctx context.Context, opt SpaceDigestOptions) (*domain.DigestEnvelope, error)
}

// DefaultSpaceDigestBuilder は SpaceDigestBuilder の標準実装。
// backlog.Client を使って必要なデータを収集し DigestEnvelope を構築する。
type DefaultSpaceDigestBuilder struct {
	BaseDigestBuilder
}

// NewDefaultSpaceDigestBuilder は DefaultSpaceDigestBuilder を生成する。
func NewDefaultSpaceDigestBuilder(client backlog.Client, profile, space, baseURL string) *DefaultSpaceDigestBuilder {
	return &DefaultSpaceDigestBuilder{BaseDigestBuilder{client: client, profile: profile, space: space, baseURL: baseURL}}
}

// SpaceDigest は digest フィールドに格納されるスペースダイジェスト構造体（spec §13.7）。
type SpaceDigest struct {
	Space     DigestSpace          `json:"space"`
	DiskUsage *domain.DiskUsage    `json:"disk_usage,omitempty"`
	Summary   SpaceDigestSummary   `json:"summary"`
	LLMHints  DigestLLMHints       `json:"llm_hints"`
}

// DigestSpace はスペースダイジェスト内のスペース情報（spec §13.7）。
type DigestSpace struct {
	SpaceKey           string `json:"space_key"`
	Name               string `json:"name"`
	OwnerID            int    `json:"owner_id"`
	Lang               string `json:"lang"`
	Timezone           string `json:"timezone"`
	TextFormattingRule string `json:"text_formatting_rule"`
}

// SpaceDigestSummary はスペースダイジェストの決定論的サマリー（spec §13.7）。
type SpaceDigestSummary struct {
	Headline      string `json:"headline"`
	HasDiskUsage  bool   `json:"has_disk_usage"`
	CapacityBytes int64  `json:"capacity_bytes,omitempty"`
	UsedBytes     int64  `json:"used_bytes,omitempty"`
}

// Build はスペースダイジェストを構築する。
// スペース情報の取得に失敗した場合はエラーを返す（必須）。
// ディスク使用量の取得失敗は warning として記録し、
// 部分成功として DigestEnvelope を返す（spec §13.7 / partial success behavior）。
func (b *DefaultSpaceDigestBuilder) Build(ctx context.Context, opt SpaceDigestOptions) (*domain.DigestEnvelope, error) {
	var warnings []domain.Warning

	// 1. スペース情報取得（必須）
	spaceInfo, err := b.client.GetSpace(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetSpace: %w", err)
	}

	// 2. ディスク使用量取得（オプション）
	diskUsage, err := b.client.GetSpaceDiskUsage(ctx)
	if err != nil {
		warnings = append(warnings, domain.Warning{
			Code:      "disk_usage_fetch_failed",
			Message:   fmt.Sprintf("ディスク使用量の取得に失敗しました: %v", err),
			Component: "disk_usage",
			Retryable: true,
		})
		diskUsage = nil
	}

	// 3. DigestSpace 組み立て
	ds := DigestSpace{
		SpaceKey:           spaceInfo.SpaceKey,
		Name:               spaceInfo.Name,
		OwnerID:            spaceInfo.OwnerID,
		Lang:               spaceInfo.Lang,
		Timezone:           spaceInfo.Timezone,
		TextFormattingRule: spaceInfo.TextFormattingRule,
	}

	// 4. SpaceDigestSummary 組み立て（決定論的）
	summary := buildSpaceDigestSummary(spaceInfo, diskUsage)

	// 5. LLMHints 組み立て
	hints := buildSpaceDigestLLMHints(spaceInfo)

	digestData := &SpaceDigest{
		Space:     ds,
		DiskUsage: diskUsage,
		Summary:   summary,
		LLMHints:  hints,
	}

	return b.newEnvelope("space", digestData, warnings), nil
}

// buildSpaceDigestSummary は決定論的スペースサマリーを構築する（spec §13.7）。
func buildSpaceDigestSummary(space *domain.Space, diskUsage *domain.DiskUsage) SpaceDigestSummary {
	headline := fmt.Sprintf("スペース %s（%s）", space.Name, space.SpaceKey)

	summary := SpaceDigestSummary{
		Headline:     headline,
		HasDiskUsage: diskUsage != nil,
	}

	if diskUsage != nil {
		summary.CapacityBytes = diskUsage.Capacity
		// 使用量合計（issue + wiki + file + subversion + git + gitLFS）
		usedBytes := diskUsage.Issue + diskUsage.Wiki + diskUsage.File +
			diskUsage.Subversion + diskUsage.Git + diskUsage.GitLFS
		summary.UsedBytes = usedBytes
	}

	return summary
}

// buildSpaceDigestLLMHints は LLM ヒントを構築する（spec §13.7）。
func buildSpaceDigestLLMHints(space *domain.Space) DigestLLMHints {
	entities := []string{space.SpaceKey, space.Name}
	return DigestLLMHints{
		PrimaryEntities:      entities,
		OpenQuestions:        []string{},
		SuggestedNextActions: []string{},
	}
}

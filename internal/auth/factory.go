package auth

import (
	"context"

	"github.com/youyo/logvalet/internal/backlog"
)

// ClientFactory は context からリクエスト固有の資格情報を取り出し、
// その資格情報を使った backlog.Client を生成する関数型。
// 実装は passthrough.go の NewPassthroughClientFactory を参照。
type ClientFactory func(ctx context.Context) (backlog.Client, error)

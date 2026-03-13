package backlog_test

import (
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
)

// TestClientInterface は Client interface が HTTPClient と MockClient で実装されていることを
// コンパイル時に確認するテスト。
func TestClientInterface(t *testing.T) {
	// コンパイルエラーになれば interface の未実装メソッドが分かる
	var _ backlog.Client = (*backlog.HTTPClient)(nil)
	var _ backlog.Client = (*backlog.MockClient)(nil)
	t.Log("Client interface is correctly implemented by HTTPClient and MockClient")
}

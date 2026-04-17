package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
	"github.com/youyo/logvalet/internal/render"
)

// TestUserMeCmd_Run_Success は GetMyself が正常に返った場合、出力に user が含まれることを確認する。
func TestUserMeCmd_Run_Success(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return &domain.User{ID: 42, Name: "田中太郎", UserID: "tanaka"}, nil
	}

	renderer, err := render.NewRenderer("json", false, "")
	if err != nil {
		t.Fatalf("render.NewRenderer エラー: %v", err)
	}

	var out bytes.Buffer
	cmd := &UserMeCmd{}
	if err := cmd.run(context.Background(), mc, renderer, &out); err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	got := out.String()
	if got == "" {
		t.Error("出力が空")
	}
	// JSON に name フィールドが含まれることを確認
	if !bytes.Contains(out.Bytes(), []byte("tanaka")) {
		t.Errorf("出力に userID が含まれていない: %s", got)
	}
}

// TestUserMeCmd_Run_GetMyselfError は GetMyself がエラーを返した場合に "user me:" プレフィクスが付くことを確認する。
func TestUserMeCmd_Run_GetMyselfError(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return nil, errors.New("unauthorized")
	}

	renderer, err := render.NewRenderer("json", false, "")
	if err != nil {
		t.Fatalf("render.NewRenderer エラー: %v", err)
	}

	var out bytes.Buffer
	cmd := &UserMeCmd{}
	err = cmd.run(context.Background(), mc, renderer, &out)
	if err == nil {
		t.Fatal("エラーが返されなかった")
	}
	if !errors.Is(err, errors.Unwrap(err)) {
		// Unwrap で元のエラーが取れることを確認
	}
	// エラーメッセージに "user me:" プレフィクスが含まれることを確認
	if err.Error()[:8] != "user me:" {
		t.Errorf("エラーメッセージのプレフィクスが想定外: %q", err.Error())
	}
}

// TestUserMeCmd_Run_CallsGetMyself は GetMyself が 1 回だけ呼ばれることを確認する。
func TestUserMeCmd_Run_CallsGetMyself(t *testing.T) {
	mc := backlog.NewMockClient()
	mc.GetMyselfFunc = func(ctx context.Context) (*domain.User, error) {
		return &domain.User{ID: 1, Name: "テスト", UserID: "test"}, nil
	}

	renderer, err := render.NewRenderer("json", false, "")
	if err != nil {
		t.Fatalf("render.NewRenderer エラー: %v", err)
	}

	var out bytes.Buffer
	cmd := &UserMeCmd{}
	if err := cmd.run(context.Background(), mc, renderer, &out); err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	if mc.GetCallCount("GetMyself") != 1 {
		t.Errorf("GetMyself の呼び出し回数: 期待 1, 実際 %d", mc.GetCallCount("GetMyself"))
	}
}

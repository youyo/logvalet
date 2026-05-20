package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/domain"
)

// TestRunFanoutWrite_NoSpaces は --spaces 未指定・--all-spaces false の場合、
// defaultClient で fn を実行することを確認する。
func TestRunFanoutWrite_NoSpaces(t *testing.T) {
	called := false
	var usedClient backlog.Client

	srv := staticIssueServer(t, []domain.Issue{{ID: 1}})
	defer srv.Close()

	factory := buildTestFactory()
	defaultReg := newTestSpaceReg("default", srv.URL)
	defaultClient, err := factory(context.Background(), defaultReg)
	if err != nil {
		t.Fatal(err)
	}

	g := &GlobalFlags{Spaces: "", AllSpaces: false}

	_, err = runFanoutWriteWithFactory(
		context.Background(),
		g,
		func(ctx context.Context, client backlog.Client) (any, error) {
			called = true
			usedClient = client
			return nil, nil
		},
		defaultClient,
		nil, // factory 不要（--spaces 未指定）
	)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !called {
		t.Fatal("fn が呼ばれていない")
	}
	if usedClient != defaultClient {
		t.Fatal("defaultClient で fn が実行されていない")
	}
}

// TestRunFanoutWrite_SingleSpaceOK は --spaces foo（1件）指定の場合、
// 指定スペースの client で fn を実行することを確認する。
func TestRunFanoutWrite_SingleSpaceOK(t *testing.T) {
	srv := staticIssueServer(t, []domain.Issue{{ID: 99}})
	defer srv.Close()

	called := false
	factory := buildTestFactory()
	// foo スペースの reg
	fooReg := newTestSpaceReg("foo", srv.URL)

	// 単一スペース用のテスト Resolver（Resolve を呼ばずに factory + reg を渡す）
	g := &GlobalFlags{Spaces: "foo", AllSpaces: false}

	// factory + reg slice を直接渡す版
	_, err := runFanoutWriteWithFactory(
		context.Background(),
		g,
		func(ctx context.Context, client backlog.Client) (any, error) {
			called = true
			return nil, nil
		},
		nil, // defaultClient は使われない
		func(_ context.Context, alias string) (backlog.Client, error) {
			if alias != "foo" {
				return nil, errors.New("unexpected alias: " + alias)
			}
			return factory(context.Background(), fooReg)
		},
	)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !called {
		t.Fatal("fn が呼ばれていない")
	}
}

// TestRunFanoutWrite_MultiSpaceError は --spaces foo,bar（複数）指定の場合、
// エラーを返すことを確認する。
func TestRunFanoutWrite_MultiSpaceError(t *testing.T) {
	g := &GlobalFlags{Spaces: "foo,bar", AllSpaces: false}

	_, err := runFanoutWriteWithFactory(
		context.Background(),
		g,
		func(ctx context.Context, client backlog.Client) (any, error) {
			return nil, nil
		},
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("複数スペース指定時はエラーが返るべき")
	}
	if err.Error() != "--spaces: multiple spaces not supported for write operations, specify exactly one" {
		t.Errorf("エラーメッセージ不一致: %v", err)
	}
}

// TestRunFanoutWrite_AllSpacesError は --all-spaces 指定の場合、
// エラーを返すことを確認する。
func TestRunFanoutWrite_AllSpacesError(t *testing.T) {
	g := &GlobalFlags{Spaces: "", AllSpaces: true}

	_, err := runFanoutWriteWithFactory(
		context.Background(),
		g,
		func(ctx context.Context, client backlog.Client) (any, error) {
			return nil, nil
		},
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("--all-spaces 指定時はエラーが返るべき")
	}
	if err.Error() != "--all-spaces is not supported for write operations" {
		t.Errorf("エラーメッセージ不一致: %v", err)
	}
}

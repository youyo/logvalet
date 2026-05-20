package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/youyo/logvalet/internal/app"
	"github.com/youyo/logvalet/internal/auth"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/credentials"
	"github.com/youyo/logvalet/internal/space"
)

// buildCLIClientFactory は CLI モード用の space.ClientFactory を構築する。
// APIKey 認証のみサポート（OAuth は MCP モードで使用）。
func buildCLIClientFactory() (space.ClientFactory, error) {
	tokensPath := credentials.DefaultTokensPath(os.Getenv)
	store := credentials.NewStore(tokensPath)
	credResolver := credentials.NewResolver(store)
	factory := auth.NewSpaceAwareClientFactory(nil, credResolver)
	return space.ClientFactory(factory), nil
}

// buildSpacesAndFactory は GlobalFlags から対象 SpaceRegistration 一覧と
// ClientFactory を構築するヘルパー。
// --spaces / --all-spaces が未指定の場合は nil, nil, nil を返す（既存動作）。
func buildSpacesAndFactory(g *GlobalFlags) ([]space.SpaceRegistration, space.ClientFactory, error) {
	if g.Spaces == "" && !g.AllSpaces {
		return nil, nil, nil
	}

	// SpaceStore を構築
	spaceStore, err := buildSpaceStore()
	if err != nil {
		return nil, nil, err
	}
	defer spaceStore.Close()

	// Scope を構築
	var sc space.Scope
	if g.AllSpaces {
		sc = space.Scope{AllSpaces: true}
	} else {
		aliases, err := ParseSpacesFlag(g.Spaces)
		if err != nil {
			return nil, nil, err
		}
		sc = space.Scope{Aliases: aliases}
	}

	// Resolver で SpaceRegistration 一覧を解決
	resolver := space.NewResolver(spaceStore)
	regs, err := resolver.Resolve(context.Background(), cliUserID, sc)
	if err != nil {
		return nil, nil, err
	}

	// ClientFactory を構築
	factory, err := buildCLIClientFactory()
	if err != nil {
		return nil, nil, err
	}

	return regs, factory, nil
}

// runAcrossSpacesWithFactory は regs の各スペースに対して fn を実行し、
// []space.Result[T] を JSON で w に出力する。
// 全成功 → exit code 0, err=nil
// partial failure → exit code 8, err=*partialFailureError
// 全失敗 → exit code 1, err=*allFailureError
func runAcrossSpacesWithFactory[T any](
	ctx context.Context,
	factory space.ClientFactory,
	regs []space.SpaceRegistration,
	w io.Writer,
	fn func(context.Context, space.SpaceRegistration, backlog.Client) (T, error),
) (int, error) {
	executor := &space.Executor{Factory: factory}
	results := space.ExecuteAcrossSpaces(ctx, executor, regs, fn)

	// 出力
	enc := json.NewEncoder(w)
	if err := enc.Encode(results); err != nil {
		return app.ExitGenericError, err
	}

	// exit code 判定
	successCount := 0
	for _, r := range results {
		if r.OK {
			successCount++
		}
	}
	switch {
	case successCount == len(results):
		return app.ExitSuccess, nil
	case successCount == 0:
		return app.ExitGenericError, &allFailureError{msg: "all spaces failed"}
	default:
		return app.ExitPartialFailure, &partialFailureError{msg: "some spaces failed"}
	}
}

// aliasClientFunc は alias から backlog.Client を構築する関数型。write fanout のテスト注入用。
type aliasClientFunc func(ctx context.Context, alias string) (backlog.Client, error)

// runFanoutWriteWithFactory は write コマンドの --spaces 単一指定対応のコア実装。
// aliasFn が nil の場合は buildCLIClientFactory + buildSpaceStore を使う（本番パス）。
// テストでは aliasFn にモックを渡す。
func runFanoutWriteWithFactory(
	ctx context.Context,
	g *GlobalFlags,
	fn func(ctx context.Context, client backlog.Client) (any, error),
	defaultClient backlog.Client,
	aliasToClient aliasClientFunc,
) (any, error) {
	// --all-spaces は write では非対応
	if g.AllSpaces {
		return nil, fmt.Errorf("--all-spaces is not supported for write operations")
	}

	// --spaces 未指定 → defaultClient で実行
	if g.Spaces == "" {
		return fn(ctx, defaultClient)
	}

	// --spaces の解析
	aliases, err := ParseSpacesFlag(g.Spaces)
	if err != nil {
		return nil, err
	}
	if len(aliases) > 1 {
		return nil, fmt.Errorf("--spaces: multiple spaces not supported for write operations, specify exactly one")
	}

	// aliasToClient が nil の場合は本番パスで client を構築
	if aliasToClient == nil {
		factory, err := buildCLIClientFactory()
		if err != nil {
			return nil, err
		}
		spaceStore, err := buildSpaceStore()
		if err != nil {
			return nil, err
		}
		defer spaceStore.Close()
		resolver := space.NewResolver(spaceStore)
		sc := space.Scope{Aliases: aliases}
		regs, err := resolver.Resolve(ctx, cliUserID, sc)
		if err != nil {
			return nil, err
		}
		if len(regs) == 0 {
			return nil, fmt.Errorf("space %q not found", aliases[0])
		}
		client, err := factory(ctx, regs[0])
		if err != nil {
			return nil, err
		}
		return fn(ctx, client)
	}

	// テスト注入パス
	client, err := aliasToClient(ctx, aliases[0])
	if err != nil {
		return nil, err
	}
	return fn(ctx, client)
}

// runFanoutWrite は GlobalFlags から write fan-out を実行するエントリポイント。
// --spaces が1件指定された場合は指定スペースの client で fn を実行する。
// 複数指定・--all-spaces はエラーを返す。
// 未指定の場合は既存動作（fn(ctx, defaultClient) を呼ぶ）。
func runFanoutWrite(
	ctx context.Context,
	g *GlobalFlags,
	fn func(ctx context.Context, client backlog.Client) (any, error),
	defaultClient backlog.Client,
) (any, error) {
	return runFanoutWriteWithFactory(ctx, g, fn, defaultClient, nil)
}

// runFanout は GlobalFlags から fan-out を実行するエントリポイント。
// --spaces / --all-spaces が未指定の場合は false, nil を返し、呼び出し元が既存動作を実行する。
// 指定がある場合は fan-out を実行して true + error を返す。
func runFanout[T any](
	g *GlobalFlags,
	fn func(context.Context, space.SpaceRegistration, backlog.Client) (T, error),
) (bool, error) {
	regs, factory, err := buildSpacesAndFactory(g)
	if err != nil {
		return true, err
	}
	if regs == nil {
		return false, nil
	}

	_, err = runAcrossSpacesWithFactory(context.Background(), factory, regs, os.Stdout, fn)
	if err != nil {
		return true, err
	}
	return true, nil
}

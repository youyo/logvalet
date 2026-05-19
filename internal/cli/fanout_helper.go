package cli

import (
	"context"
	"encoding/json"
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

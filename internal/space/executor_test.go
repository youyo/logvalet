package space_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/space"
)

// mockFactory は SpaceAwareClientFactory のモック実装。
type mockFactory struct {
	clients map[string]backlog.Client
	err     error
}

func (m *mockFactory) call(ctx context.Context, reg space.SpaceRegistration) (backlog.Client, error) {
	if m.err != nil {
		return nil, m.err
	}
	if c, ok := m.clients[reg.Alias]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("no client for space %q", reg.Alias)
}

func makeSpaces(aliases ...string) []space.SpaceRegistration {
	regs := make([]space.SpaceRegistration, len(aliases))
	for i, a := range aliases {
		regs[i] = space.SpaceRegistration{
			Alias:   a,
			Tenant:  a + ".backlog.com",
			BaseURL: "https://" + a + ".backlog.com",
		}
	}
	return regs
}

func makeClientsMap(aliases []string) map[string]backlog.Client {
	m := make(map[string]backlog.Client, len(aliases))
	for _, a := range aliases {
		m[a] = backlog.NewMockClient()
	}
	return m
}

// T1: 全スペース成功
func TestExecuteAcrossSpaces_AllSuccess(t *testing.T) {
	spaces := makeSpaces("foo", "bar")
	factory := &mockFactory{
		clients: makeClientsMap([]string{"foo", "bar"}),
	}

	ex := &space.Executor{
		Factory:        factory.call,
		MaxConcurrency: 2,
	}

	results := space.ExecuteAcrossSpaces(
		context.Background(),
		ex,
		spaces,
		func(ctx context.Context, reg space.SpaceRegistration, c backlog.Client) (string, error) {
			return "ok:" + reg.Alias, nil
		},
	)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].SpaceAlias != "foo" || !results[0].OK {
		t.Errorf("results[0] = %+v, want OK=true, alias=foo", results[0])
	}
	if results[1].SpaceAlias != "bar" || !results[1].OK {
		t.Errorf("results[1] = %+v, want OK=true, alias=bar", results[1])
	}
}

// T2: 一部失敗 → partial failure
func TestExecuteAcrossSpaces_PartialFailure(t *testing.T) {
	spaces := makeSpaces("foo", "bar")
	// foo のみ clients に含める。bar は factory がエラーを返す。
	factory := &mockFactory{
		clients: makeClientsMap([]string{"foo"}),
	}

	ex := &space.Executor{
		Factory:        factory.call,
		MaxConcurrency: 2,
	}

	results := space.ExecuteAcrossSpaces(
		context.Background(),
		ex,
		spaces,
		func(ctx context.Context, reg space.SpaceRegistration, c backlog.Client) (string, error) {
			return "ok", nil
		},
	)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].OK {
		t.Errorf("results[0] should be OK=true")
	}
	if results[1].OK {
		t.Errorf("results[1] should be OK=false")
	}
	if results[1].ErrorCode == "" {
		t.Errorf("results[1].ErrorCode should not be empty")
	}
}

// T3: 入力順序保持
func TestExecuteAcrossSpaces_InputOrderPreserved(t *testing.T) {
	aliases := []string{"a", "b", "c", "d", "e"}
	spaces := makeSpaces(aliases...)
	factory := &mockFactory{clients: makeClientsMap(aliases)}

	ex := &space.Executor{
		Factory:        factory.call,
		MaxConcurrency: 3,
	}

	results := space.ExecuteAcrossSpaces(
		context.Background(),
		ex,
		spaces,
		func(ctx context.Context, reg space.SpaceRegistration, c backlog.Client) (string, error) {
			// ランダム遅延をシミュレート
			time.Sleep(time.Duration(len(reg.Alias)) * time.Millisecond)
			return reg.Alias, nil
		},
	)

	if len(results) != len(aliases) {
		t.Fatalf("expected %d results, got %d", len(aliases), len(results))
	}
	for i, a := range aliases {
		if results[i].SpaceAlias != a {
			t.Errorf("results[%d].SpaceAlias = %q, want %q", i, results[i].SpaceAlias, a)
		}
	}
}

// T4: MaxConcurrency=0 → デフォルト 4 にフォールバック（deadlock なし）
func TestExecuteAcrossSpaces_MaxConcurrency_Zero_Fallback(t *testing.T) {
	aliases := make([]string, 10)
	for i := range aliases {
		aliases[i] = fmt.Sprintf("sp%02d", i)
	}
	spaces := makeSpaces(aliases...)
	factory := &mockFactory{clients: makeClientsMap(aliases)}

	ex := &space.Executor{
		Factory:        factory.call,
		MaxConcurrency: 0, // デフォルト 4 にフォールバック
	}

	done := make(chan []space.Result[string], 1)
	go func() {
		results := space.ExecuteAcrossSpaces(
			context.Background(),
			ex,
			spaces,
			func(ctx context.Context, reg space.SpaceRegistration, c backlog.Client) (string, error) {
				return reg.Alias, nil
			},
		)
		done <- results
	}()

	select {
	case results := <-done:
		if len(results) != 10 {
			t.Errorf("expected 10 results, got %d", len(results))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock detected: ExecuteAcrossSpaces did not complete within 5 seconds")
	}
}

// T5: MaxConcurrency=-1 → デフォルト 4 にフォールバック
func TestExecuteAcrossSpaces_MaxConcurrency_Negative_Fallback(t *testing.T) {
	spaces := makeSpaces("x")
	factory := &mockFactory{
		clients: makeClientsMap([]string{"x"}),
	}

	ex := &space.Executor{
		Factory:        factory.call,
		MaxConcurrency: -1,
	}

	results := space.ExecuteAcrossSpaces(
		context.Background(),
		ex,
		spaces,
		func(ctx context.Context, reg space.SpaceRegistration, c backlog.Client) (string, error) {
			return "ok", nil
		},
	)

	if len(results) != 1 || !results[0].OK {
		t.Errorf("expected 1 OK result, got %+v", results)
	}
}

// T6: context キャンセルで early exit（goroutine leak なし）
func TestExecuteAcrossSpaces_ContextCancellation(t *testing.T) {
	aliases := make([]string, 26)
	for i := range aliases {
		aliases[i] = fmt.Sprintf("space%d", i)
	}
	spaces := makeSpaces(aliases...)
	factory := &mockFactory{clients: makeClientsMap(aliases)}

	ctx, cancel := context.WithCancel(context.Background())

	ex := &space.Executor{
		Factory:        factory.call,
		MaxConcurrency: 2,
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		space.ExecuteAcrossSpaces(
			ctx,
			ex,
			spaces,
			func(ctx context.Context, reg space.SpaceRegistration, c backlog.Client) (string, error) {
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(100 * time.Millisecond):
					return reg.Alias, nil
				}
			},
		)
	}()

	// 少し待ってからキャンセル
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// goroutine leak なし
	case <-time.After(3 * time.Second):
		t.Fatal("goroutine leak: ExecuteAcrossSpaces did not return after context cancellation")
	}
}

// T7: 空スペース → 空スライス（nil でなく）
func TestExecuteAcrossSpaces_EmptySpaces(t *testing.T) {
	factory := &mockFactory{}
	ex := &space.Executor{
		Factory:        factory.call,
		MaxConcurrency: 4,
	}

	results := space.ExecuteAcrossSpaces(
		context.Background(),
		ex,
		[]space.SpaceRegistration{},
		func(ctx context.Context, reg space.SpaceRegistration, c backlog.Client) (string, error) {
			return "", nil
		},
	)

	if results == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// T8: factory がエラーを返す → ErrorCode が設定される
func TestExecuteAcrossSpaces_FactoryError_Mapped_To_Result(t *testing.T) {
	spaces := makeSpaces("z")
	factory := &mockFactory{
		err: errors.New("some factory error"),
	}

	ex := &space.Executor{
		Factory:        factory.call,
		MaxConcurrency: 1,
	}

	results := space.ExecuteAcrossSpaces(
		context.Background(),
		ex,
		spaces,
		func(ctx context.Context, reg space.SpaceRegistration, c backlog.Client) (string, error) {
			return "ok", nil
		},
	)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].OK {
		t.Error("expected OK=false")
	}
	if results[0].ErrorCode == "" {
		t.Error("expected non-empty ErrorCode")
	}
}

// T9: 失敗時 Result フィールドは nil（*T）
func TestResult_FailureCase_ResultIsNil(t *testing.T) {
	spaces := makeSpaces("fail")
	// clients が空なので factory がエラーを返す
	factory := &mockFactory{}

	ex := &space.Executor{
		Factory:        factory.call,
		MaxConcurrency: 1,
	}

	results := space.ExecuteAcrossSpaces(
		context.Background(),
		ex,
		spaces,
		func(ctx context.Context, reg space.SpaceRegistration, c backlog.Client) (string, error) {
			return "ok", nil
		},
	)

	if results[0].OK {
		t.Error("expected OK=false")
	}
	if results[0].Result != nil {
		t.Errorf("expected Result to be nil, got %v", results[0].Result)
	}
}

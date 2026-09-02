package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/youyo/logvalet/internal/app"
	"github.com/youyo/logvalet/internal/backlog"
	"github.com/youyo/logvalet/internal/conventions"
)

// ProjectApplyCmd は project apply コマンド。
// conventions.yaml を Backlog プロジェクトへ冪等に差分適用する。
type ProjectApplyCmd struct {
	File    string `required:"" help:"path to conventions.yaml" type:"path"`
	Project string `help:"override project.key in the conventions file"`
	DryRun  bool   `help:"show the plan without applying anything"`
	Create  bool   `help:"create the project if it does not exist"`
}

// Run は conventions.yaml の差分計画を表示、または Backlog に適用する。
func (c *ProjectApplyCmd) Run(g *GlobalFlags) error {
	conv, err := conventions.LoadFile(c.File)
	if err != nil {
		return err
	}

	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	plan, err := conventions.BuildPlan(context.Background(), rc.Client, conv, conventions.PlanOptions{
		ProjectKey: c.Project,
		Create:     c.Create,
	})
	if err != nil {
		return newProjectApplyExitError(err, projectApplyBuildPlanExitCode(err))
	}

	if c.DryRun {
		if err := writeProjectApplyJSON(plan); err != nil {
			return err
		}
		_, _ = fmt.Fprint(os.Stderr, conventions.RenderPlan(plan))
		return nil
	}

	result, applyErr := conventions.Apply(context.Background(), rc.Client, plan)
	if result == nil {
		if applyErr != nil {
			return applyErr
		}
		return errors.New("conventions の適用結果が nil です")
	}
	if err := writeProjectApplyJSON(result); err != nil {
		return err
	}
	_, _ = fmt.Fprint(os.Stderr, conventions.RenderResult(result))

	if result.Summary.Failed == 0 {
		return nil
	}

	return &quietExitError{
		code: projectApplyFailureExitCode(result, applyErr),
		msg:  "conventions の適用に失敗しました",
	}
}

func writeProjectApplyJSON(value any) error {
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		return fmt.Errorf("project apply 結果の JSON 出力に失敗しました: %w", err)
	}
	return nil
}

func projectApplyBuildPlanExitCode(err error) int {
	if errors.Is(err, backlog.ErrNotFound) {
		return app.ExitNotFoundError
	}
	return app.ExitArgumentError
}

func projectApplyFailureExitCode(result *conventions.ApplyResult, err error) int {
	if result.Summary.Applied > 0 {
		return app.ExitPartialFailure
	}
	switch {
	case errors.Is(err, backlog.ErrUnauthorized):
		return app.ExitAuthenticationError
	case errors.Is(err, backlog.ErrForbidden):
		return app.ExitPermissionError
	case errors.Is(err, backlog.ErrNotFound):
		return app.ExitNotFoundError
	default:
		return app.ExitAPIError
	}
}

type projectApplyExitError struct {
	err  error
	code int
}

func newProjectApplyExitError(err error, code int) error {
	return &projectApplyExitError{err: err, code: code}
}

func (e *projectApplyExitError) Error() string { return e.err.Error() }
func (e *projectApplyExitError) Unwrap() error { return e.err }
func (e *projectApplyExitError) ExitCode() int { return e.code }

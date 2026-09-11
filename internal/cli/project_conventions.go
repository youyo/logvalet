package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/youyo/logvalet/internal/app"
	"github.com/youyo/logvalet/internal/conventions"
)

// ProjectConventionsCmd は project conventions コマンド群のルート。
type ProjectConventionsCmd struct {
	Init     ProjectConventionsInitCmd     `cmd:"" help:"generate a conventions.yaml skeleton"`
	Validate ProjectConventionsValidateCmd `cmd:"" help:"validate a conventions.yaml"`
	Show     ProjectConventionsShowCmd     `cmd:"" help:"show the conventions adopted by a project"`
}

// ProjectConventionsInitCmd は conventions.yaml のスケルトンを生成するコマンド。
type ProjectConventionsInitCmd struct {
	Out         string `help:"output file path (default: stdout)" type:"path"`
	FromProject string `help:"seed the skeleton from an existing project key"`
}

// ProjectConventionsValidateCmd は conventions.yaml を検証するコマンド。
type ProjectConventionsValidateCmd struct {
	File   string `required:"" help:"path to conventions.yaml" type:"path"`
	Strict bool   `help:"treat warnings as errors"`
}

// ProjectConventionsShowCmd は project conventions show コマンド。
type ProjectConventionsShowCmd struct {
	Project string `required:"" help:"project key"`
	File    string `help:"read from a local conventions.yaml instead of the rule issue" type:"path"`
}

// Run は conventions.yaml のスケルトンを生成する。
func (c *ProjectConventionsInitCmd) Run(g *GlobalFlags) error {
	var (
		data []byte
		err  error
	)
	if c.FromProject == "" {
		data, err = conventions.Skeleton()
	} else {
		rc, buildErr := buildRunContext(g)
		if buildErr != nil {
			return buildErr
		}
		data, err = conventions.BuildSkeletonFromProject(context.Background(), rc.Client, c.FromProject)
	}
	if err != nil {
		return err
	}

	if c.Out == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			return fmt.Errorf("conventions スケルトンの stdout 出力に失敗しました: %w", err)
		}
		return nil
	}

	file, err := os.OpenFile(c.Out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return &argumentError{msg: fmt.Sprintf("出力先 %q は既に存在します", c.Out)}
		}
		return fmt.Errorf("conventions スケルトンの出力ファイルを作成できませんでした: %w", err)
	}

	written, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("conventions スケルトンの書き込みに失敗しました: %w", writeErr)
	}
	if written != len(data) {
		return fmt.Errorf("conventions スケルトンの書き込みに失敗しました: %w", io.ErrShortWrite)
	}
	if closeErr != nil {
		return fmt.Errorf("conventions スケルトンの出力ファイルを閉じられませんでした: %w", closeErr)
	}

	fmt.Fprintf(os.Stderr, "conventions スケルトンを書き出しました: %s\n", c.Out)
	return nil
}

type projectConventionsValidationResult struct {
	Valid      bool                    `json:"valid"`
	Violations []conventions.Violation `json:"violations"`
}

// Run は conventions.yaml を検証し、結果を JSON で出力する。
func (c *ProjectConventionsValidateCmd) Run(_ *GlobalFlags) error {
	loaded, err := conventions.LoadFile(c.File)
	if err != nil {
		return &argumentError{msg: err.Error()}
	}

	violations := conventions.Validate(loaded)
	if violations == nil {
		violations = []conventions.Violation{}
	}
	result := projectConventionsValidationResult{
		Valid:      !conventions.HasError(violations),
		Violations: violations,
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return fmt.Errorf("conventions 検証結果の JSON 出力に失敗しました: %w", err)
	}

	writeConventionsValidationSummary(violations)
	if conventions.HasError(violations) || (c.Strict && len(violations) > 0) {
		return &quietExitError{
			code: app.ExitArgumentError,
			msg:  "conventions.yaml の検証に失敗しました",
		}
	}
	return nil
}

func writeConventionsValidationSummary(violations []conventions.Violation) {
	errorCount := 0
	warningCount := 0
	for _, violation := range violations {
		fmt.Fprintf(os.Stderr, "%s %s %s\n", violation.Severity, violation.Path, violation.Message)
		switch violation.Severity {
		case conventions.SeverityError:
			errorCount++
		case conventions.SeverityWarning:
			warningCount++
		}
	}
	fmt.Fprintf(os.Stderr, "%d 件の違反（error %d、warning %d）\n", len(violations), errorCount, warningCount)
}

// Run はプロジェクトが採用している運用規約を表示する。
func (c *ProjectConventionsShowCmd) Run(g *GlobalFlags) error {
	rc, err := buildRunContext(g)
	if err != nil {
		return err
	}

	var result *conventions.ShowResult
	if c.File != "" {
		loaded, loadErr := conventions.LoadFile(c.File)
		if loadErr != nil {
			return loadErr
		}
		result = &conventions.ShowResult{
			ProjectKey:  c.Project,
			Adopted:     true,
			Conventions: loaded,
			Glossary:    conventions.Glossary(),
		}
	} else {
		result, err = conventions.Show(context.Background(), rc.Client, c.Project)
		if err != nil {
			return err
		}
	}

	return rc.Renderer.Render(os.Stdout, result)
}

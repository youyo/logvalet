package app_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/youyo/logvalet/internal/app"
	"github.com/youyo/logvalet/internal/domain"
)

// testExitCoderError は ExitCoder を実装するテスト用エラー。
type testExitCoderError struct {
	msg      string
	exitCode int
}

func (e *testExitCoderError) Error() string   { return e.msg }
func (e *testExitCoderError) ExitCode() int   { return e.exitCode }

// testFullError は ExitCoder, ErrorCoder, Retryabler を全て実装するテスト用エラー。
type testFullError struct {
	msg       string
	exitCode  int
	errorCode string
	retryable bool
}

func (e *testFullError) Error() string    { return e.msg }
func (e *testFullError) ExitCode() int    { return e.exitCode }
func (e *testFullError) ErrorCode() string { return e.errorCode }
func (e *testFullError) Retryable() bool  { return e.retryable }

func TestNewErrorEnvelope(t *testing.T) {
	env := app.NewErrorEnvelope("not_found", "Issue PROJ-999 was not found.", false)

	if env.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want %q", env.SchemaVersion, "1")
	}
	if env.Error.Code != "not_found" {
		t.Errorf("Error.Code = %q, want %q", env.Error.Code, "not_found")
	}
	if env.Error.Message != "Issue PROJ-999 was not found." {
		t.Errorf("Error.Message = %q, want %q", env.Error.Message, "Issue PROJ-999 was not found.")
	}
	if env.Error.Retryable != false {
		t.Error("Error.Retryable = true, want false")
	}
}

func TestNewErrorEnvelope_Retryable(t *testing.T) {
	env := app.NewErrorEnvelope("rate_limited", "Too many requests.", true)

	if env.Error.Retryable != true {
		t.Error("Error.Retryable = false, want true")
	}
}

func TestNewErrorEnvelope_JSONShape(t *testing.T) {
	env := app.NewErrorEnvelope("issue_not_found", "Issue PROJ-999 was not found.", false)

	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if _, ok := m["schema_version"]; !ok {
		t.Error("JSON に schema_version フィールドが存在しない")
	}
	errObj, ok := m["error"].(map[string]interface{})
	if !ok {
		t.Fatal("JSON に error フィールドが存在しない")
	}
	if _, ok := errObj["code"]; !ok {
		t.Error("error に code フィールドが存在しない")
	}
	if _, ok := errObj["message"]; !ok {
		t.Error("error に message フィールドが存在しない")
	}
	if _, ok := errObj["retryable"]; !ok {
		t.Error("error に retryable フィールドが存在しない")
	}
}

func TestExitCodeToErrorCode(t *testing.T) {
	tests := []struct {
		exitCode int
		want     string
	}{
		{app.ExitGenericError, "generic_error"},
		{app.ExitArgumentError, "argument_error"},
		{app.ExitAuthenticationError, "authentication_error"},
		{app.ExitPermissionError, "permission_error"},
		{app.ExitNotFoundError, "not_found"},
		{app.ExitAPIError, "api_error"},
		{app.ExitDigestError, "digest_error"},
		{app.ExitConfigError, "config_error"},
		{99, "generic_error"}, // 未知の exit code
	}

	for _, tt := range tests {
		got := app.ExitCodeToErrorCode(tt.exitCode)
		if got != tt.want {
			t.Errorf("ExitCodeToErrorCode(%d) = %q, want %q", tt.exitCode, got, tt.want)
		}
	}
}

func TestExitCodeRetryable(t *testing.T) {
	tests := []struct {
		exitCode int
		want     bool
	}{
		{app.ExitGenericError, false},
		{app.ExitArgumentError, false},
		{app.ExitAuthenticationError, false},
		{app.ExitAPIError, true},
		{app.ExitNotFoundError, false},
		{app.ExitConfigError, false},
	}

	for _, tt := range tests {
		got := app.ExitCodeRetryable(tt.exitCode)
		if got != tt.want {
			t.Errorf("ExitCodeRetryable(%d) = %v, want %v", tt.exitCode, got, tt.want)
		}
	}
}

func TestHandleError_GenericError(t *testing.T) {
	var buf bytes.Buffer
	err := errors.New("something failed")

	exitCode := app.HandleError(&buf, err, app.ExitGenericError)

	if exitCode != app.ExitGenericError {
		t.Errorf("exit code = %d, want %d", exitCode, app.ExitGenericError)
	}

	var env domain.ErrorEnvelope
	if jsonErr := json.Unmarshal(buf.Bytes(), &env); jsonErr != nil {
		t.Fatalf("出力が valid JSON でない: %v\n出力: %s", jsonErr, buf.String())
	}

	if env.SchemaVersion != "1" {
		t.Errorf("schema_version = %q, want %q", env.SchemaVersion, "1")
	}
	if env.Error.Code != "generic_error" {
		t.Errorf("error.code = %q, want %q", env.Error.Code, "generic_error")
	}
	if env.Error.Message != "something failed" {
		t.Errorf("error.message = %q, want %q", env.Error.Message, "something failed")
	}
	if env.Error.Retryable != false {
		t.Error("error.retryable = true, want false")
	}
}

func TestHandleError_ExitCoderError(t *testing.T) {
	var buf bytes.Buffer
	err := &testExitCoderError{msg: "not found", exitCode: app.ExitNotFoundError}

	exitCode := app.HandleError(&buf, err, app.ExitGenericError)

	if exitCode != app.ExitNotFoundError {
		t.Errorf("exit code = %d, want %d", exitCode, app.ExitNotFoundError)
	}

	var env domain.ErrorEnvelope
	if jsonErr := json.Unmarshal(buf.Bytes(), &env); jsonErr != nil {
		t.Fatalf("出力が valid JSON でない: %v", jsonErr)
	}

	if env.Error.Code != "not_found" {
		t.Errorf("error.code = %q, want %q", env.Error.Code, "not_found")
	}
}

func TestHandleError_FullError(t *testing.T) {
	var buf bytes.Buffer
	err := &testFullError{
		msg:       "Issue PROJ-999 was not found.",
		exitCode:  app.ExitNotFoundError,
		errorCode: "issue_not_found",
		retryable: false,
	}

	exitCode := app.HandleError(&buf, err, app.ExitGenericError)

	if exitCode != app.ExitNotFoundError {
		t.Errorf("exit code = %d, want %d", exitCode, app.ExitNotFoundError)
	}

	var env domain.ErrorEnvelope
	if jsonErr := json.Unmarshal(buf.Bytes(), &env); jsonErr != nil {
		t.Fatalf("出力が valid JSON でない: %v", jsonErr)
	}

	if env.Error.Code != "issue_not_found" {
		t.Errorf("error.code = %q, want %q", env.Error.Code, "issue_not_found")
	}
	if env.Error.Message != "Issue PROJ-999 was not found." {
		t.Errorf("error.message = %q, want %q", env.Error.Message, "Issue PROJ-999 was not found.")
	}
	if env.Error.Retryable != false {
		t.Error("error.retryable = true, want false")
	}
}

func TestHandleError_RetryableError(t *testing.T) {
	var buf bytes.Buffer
	err := &testFullError{
		msg:       "rate limited",
		exitCode:  app.ExitAPIError,
		errorCode: "rate_limited",
		retryable: true,
	}

	exitCode := app.HandleError(&buf, err, app.ExitGenericError)

	if exitCode != app.ExitAPIError {
		t.Errorf("exit code = %d, want %d", exitCode, app.ExitAPIError)
	}

	var env domain.ErrorEnvelope
	if jsonErr := json.Unmarshal(buf.Bytes(), &env); jsonErr != nil {
		t.Fatalf("出力が valid JSON でない: %v", jsonErr)
	}

	if env.Error.Retryable != true {
		t.Error("error.retryable = false, want true")
	}
}

func TestHandleError_DefaultExitCode(t *testing.T) {
	var buf bytes.Buffer
	err := errors.New("config broken")

	exitCode := app.HandleError(&buf, err, app.ExitConfigError)

	if exitCode != app.ExitConfigError {
		t.Errorf("exit code = %d, want %d", exitCode, app.ExitConfigError)
	}

	var env domain.ErrorEnvelope
	if jsonErr := json.Unmarshal(buf.Bytes(), &env); jsonErr != nil {
		t.Fatalf("出力が valid JSON でない: %v", jsonErr)
	}

	if env.Error.Code != "config_error" {
		t.Errorf("error.code = %q, want %q", env.Error.Code, "config_error")
	}
}

func TestHandleError_OutputEndsWithNewline(t *testing.T) {
	var buf bytes.Buffer
	err := errors.New("test error")

	app.HandleError(&buf, err, app.ExitGenericError)

	output := buf.String()
	if len(output) == 0 {
		t.Fatal("出力が空")
	}
	if output[len(output)-1] != '\n' {
		t.Error("出力が改行で終わっていない")
	}
}

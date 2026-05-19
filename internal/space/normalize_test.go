package space

import (
	"testing"
)

// --- NormalizeBaseURL ---

func TestNormalizeBaseURL_NoScheme(t *testing.T) {
	got, err := NormalizeBaseURL("foo.backlog.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://foo.backlog.com"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNormalizeBaseURL_TrailingSlash(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"https://foo.backlog.com/", "https://foo.backlog.com"},
		{"https://foo.backlog.com///", "https://foo.backlog.com"},
	}
	for _, c := range cases {
		got, err := NormalizeBaseURL(c.input)
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", c.input, err)
		}
		if got != c.want {
			t.Errorf("input %q: got %q, want %q", c.input, got, c.want)
		}
	}
}

func TestNormalizeBaseURL_AlreadyNormalized(t *testing.T) {
	input := "https://foo.backlog.com"
	got, err := NormalizeBaseURL(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != input {
		t.Errorf("got %q, want %q", got, input)
	}
}

func TestNormalizeBaseURL_BacklogJP(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"https://foo.backlog.jp", "https://foo.backlog.jp"},
		{"foo.backlog.jp", "https://foo.backlog.jp"},
	}
	for _, c := range cases {
		got, err := NormalizeBaseURL(c.input)
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", c.input, err)
		}
		if got != c.want {
			t.Errorf("input %q: got %q, want %q", c.input, got, c.want)
		}
	}
}

func TestNormalizeBaseURL_PathRejected(t *testing.T) {
	_, err := NormalizeBaseURL("https://foo.backlog.com/api/v2")
	if err == nil {
		t.Error("expected error for URL with path, got nil")
	}
}

func TestNormalizeBaseURL_QueryRejected(t *testing.T) {
	_, err := NormalizeBaseURL("https://foo.backlog.com?x=1")
	if err == nil {
		t.Error("expected error for URL with query, got nil")
	}
}

func TestNormalizeBaseURL_FragmentRejected(t *testing.T) {
	_, err := NormalizeBaseURL("https://foo.backlog.com#section")
	if err == nil {
		t.Error("expected error for URL with fragment, got nil")
	}
}

func TestNormalizeBaseURL_EmptyRejected(t *testing.T) {
	_, err := NormalizeBaseURL("")
	if err == nil {
		t.Error("expected error for empty URL, got nil")
	}
}

func TestNormalizeBaseURL_CustomDomain(t *testing.T) {
	got, err := NormalizeBaseURL("https://backlog.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://backlog.example.com"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- DeriveAliasFromBaseURL ---

func TestDeriveAliasFromBaseURL_BacklogCom(t *testing.T) {
	got, err := DeriveAliasFromBaseURL("https://foo.backlog.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "foo" {
		t.Errorf("got %q, want %q", got, "foo")
	}
}

func TestDeriveAliasFromBaseURL_BacklogJP(t *testing.T) {
	got, err := DeriveAliasFromBaseURL("https://foo.backlog.jp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "foo" {
		t.Errorf("got %q, want %q", got, "foo")
	}
}

func TestDeriveAliasFromBaseURL_Uppercase(t *testing.T) {
	got, err := DeriveAliasFromBaseURL("https://FOO.backlog.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "foo" {
		t.Errorf("got %q, want %q", got, "foo")
	}
}

func TestDeriveAliasFromBaseURL_CustomDomain(t *testing.T) {
	got, err := DeriveAliasFromBaseURL("https://backlog.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty string for custom domain", got)
	}
}

func TestDeriveAliasFromBaseURL_HyphenInName(t *testing.T) {
	got, err := DeriveAliasFromBaseURL("https://foo-bar.backlog.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "foo-bar" {
		t.Errorf("got %q, want %q", got, "foo-bar")
	}
}

// --- DeriveInitialTenant ---

func TestDeriveInitialTenant_BacklogCom(t *testing.T) {
	got, err := DeriveInitialTenant("https://foo.backlog.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "foo" {
		t.Errorf("got %q, want %q", got, "foo")
	}
}

func TestDeriveInitialTenant_BacklogJP(t *testing.T) {
	got, err := DeriveInitialTenant("https://foo.backlog.jp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "foo" {
		t.Errorf("got %q, want %q", got, "foo")
	}
}

func TestDeriveInitialTenant_CustomDomain(t *testing.T) {
	got, err := DeriveInitialTenant("https://backlog.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty string for custom domain", got)
	}
}

func TestDeriveInitialTenant_CaseSensitivity(t *testing.T) {
	got, err := DeriveInitialTenant("https://FOO.backlog.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "foo" {
		t.Errorf("got %q, want %q", got, "foo")
	}
}

// --- ValidateAlias ---

func TestValidateAlias_Valid(t *testing.T) {
	cases := []string{"foo", "foo-bar", "foo_bar", "foo.bar", "foo123"}
	for _, alias := range cases {
		if err := ValidateAlias(alias); err != nil {
			t.Errorf("alias %q: unexpected error: %v", alias, err)
		}
	}
}

func TestValidateAlias_Empty(t *testing.T) {
	if err := ValidateAlias(""); err == nil {
		t.Error("expected error for empty alias, got nil")
	}
}

func TestValidateAlias_InvalidChars(t *testing.T) {
	cases := []string{"foo bar", "foo/bar", "foo@bar"}
	for _, alias := range cases {
		if err := ValidateAlias(alias); err == nil {
			t.Errorf("alias %q: expected error for invalid chars, got nil", alias)
		}
	}
}

func TestValidateAlias_TooLong(t *testing.T) {
	long := make([]byte, 65)
	for i := range long {
		long[i] = 'a'
	}
	if err := ValidateAlias(string(long)); err == nil {
		t.Error("expected error for alias exceeding 64 chars, got nil")
	}
}

func TestValidateAlias_StartsWithHyphen(t *testing.T) {
	if err := ValidateAlias("-foo"); err == nil {
		t.Error("expected error for alias starting with hyphen, got nil")
	}
}

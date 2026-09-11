package cli_test

import (
	"strings"
	"testing"

	"github.com/youyo/logvalet/internal/cli"
)

// multi-space 撤去 (v0.40) 後の期待:
// --spaces / --all-spaces は移行案内付きで fail-fast すること。
func TestGlobalFlags_RemovedSpacesFlags(t *testing.T) {
	tests := []struct {
		name string
		g    cli.GlobalFlags
		flag string
	}{
		{"spaces", cli.GlobalFlags{RemovedSpaces: "foo,bar"}, "--spaces"},
		{"all-spaces", cli.GlobalFlags{RemovedAllSpaces: true}, "--all-spaces"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.g.Validate()
			if err == nil {
				t.Fatalf("%s 指定時はエラーになるべき", tc.flag)
			}
			if !strings.Contains(err.Error(), tc.flag) {
				t.Errorf("エラーに %s を含むべき: %v", tc.flag, err)
			}
			if !strings.Contains(err.Error(), "Portals") {
				t.Errorf("エラーに移行先 (Portals) の案内を含むべき: %v", err)
			}
		})
	}
}

// 未指定なら従来どおり通ること。
func TestGlobalFlags_NoRemovedFlags_OK(t *testing.T) {
	g := cli.GlobalFlags{}
	if err := g.Validate(); err != nil {
		t.Fatalf("未指定ならエラーにならないべき: %v", err)
	}
}

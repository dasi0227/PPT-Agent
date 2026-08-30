package designsystem_test

import (
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
)

// DS-TOKENS-002：必需 token 全集校验——齐备则无缺失。
func TestLintTokensComplete(t *testing.T) {
	var css []byte
	for _, tk := range designsystem.RequiredTokens() {
		css = append(css, []byte(tk+": x;\n")...)
	}
	if missing := designsystem.LintTokens(css); len(missing) != 0 {
		t.Fatalf("complete token set reported missing: %v", missing)
	}
}

// AC-ASSET-002：缺某必需 token → 校验失败并指出缺失。
func TestLintTokensReportsMissing(t *testing.T) {
	css := []byte("--color-bg: #000; --color-fg: #fff;")
	missing := designsystem.LintTokens(css)
	if len(missing) == 0 {
		t.Fatal("expected missing tokens for incomplete css")
	}
	// 至少应指出 --stage-w 缺失。
	found := false
	for _, m := range missing {
		if m == "--stage-w" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --stage-w in missing set, got %v", missing)
	}
}

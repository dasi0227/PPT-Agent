package designsystem_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
)

// docPath 定位 docs 权威文档（从本包目录回溯到仓库根）。
func docPath(parts ...string) string {
	base := []string{"..", "..", "..", "docs", "v1"}
	return filepath.Join(append(base, parts...)...)
}

// AC-LAYOUTS-001：layouts.md 版式目录 MUST 与 slide-json schema 的 layout enum 完全一致（无遗漏/多余）。
func TestLayoutCatalogMatchesSchemaEnum(t *testing.T) {
	md, err := os.ReadFile(docPath("60-design-system", "layouts.md"))
	if err != nil {
		t.Fatalf("read layouts.md: %v", err)
	}
	catalog := designsystem.ParseLayoutCatalog(md)
	enum := slidejson.LayoutEnum()

	if len(catalog) == 0 {
		t.Fatal("parsed empty layout catalog from layouts.md")
	}
	cset := toSet(catalog)
	eset := toSet(enum)

	for l := range cset {
		if !eset[l] {
			t.Errorf("layout %q in layouts.md but NOT in schema enum", l)
		}
	}
	for l := range eset {
		if !cset[l] {
			t.Errorf("layout %q in schema enum but NOT in layouts.md", l)
		}
	}
	if len(cset) != len(eset) {
		t.Errorf("layout count mismatch: catalog=%d enum=%d", len(cset), len(eset))
	}
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

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

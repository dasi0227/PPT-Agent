package runtimehtml

import (
	"strings"
	"testing"
)

func TestNormalizeOwnsBaseAndThemeLinks(t *testing.T) {
	raw := []byte(`<!doctype html><html><head><link id="theme-link" href="/wrong.css"><link href="../../common/base.css"></head><body><main class="slide-stage"></main></body></html>`)
	normalized, err := Normalize(raw, "tokyo-night")
	if err != nil {
		t.Fatal(err)
	}
	html := string(normalized)
	for _, expected := range []string{
		`id="base-link"`, `href="/api/v1/runtime/base.css"`,
		`id="theme-link"`, `href="/api/v1/themes/tokyo-night/css"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("missing %q in %s", expected, html)
		}
	}
	if strings.Contains(html, "/wrong.css") || strings.Contains(html, "../../common/base.css") {
		t.Fatalf("legacy or agent-owned links remain: %s", html)
	}
}

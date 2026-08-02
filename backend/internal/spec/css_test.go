package spec

import (
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
)

func TestDesignTokensCSSProvidesProjectRuntimeContract(t *testing.T) {
	design := Design{
		Palette:    []string{"#101010", "#F0F0F0", "#3366FF", "#FFAA00", "#222222"},
		Typography: TypographySpec{Body: FontSpec{Family: "Inter, sans-serif"}},
		Spacing:    SpacingSpec{Unit: 8}, Radius: RadiusSpec{Card: 12},
		Shadows: ShadowSpec{Card: "0 4px 16px rgba(0,0,0,.15)"},
	}
	css := DesignTokensCSS(design)
	if missing := designsystem.LintTokens(css); len(missing) != 0 {
		t.Fatalf("generated CSS misses required tokens: %v", missing)
	}
	for _, expected := range []string{"--stage-w: 1600", "--stage-h: 900", "--color-primary: #3366FF"} {
		if !strings.Contains(string(css), expected) {
			t.Fatalf("generated CSS missing %q", expected)
		}
	}
}

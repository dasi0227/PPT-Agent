package spec

import (
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
)

func TestDesignTokensCSSProvidesProjectRuntimeContract(t *testing.T) {
	design := Design{
		Theme: "swiss-modern",
	}
	css := DesignTokensCSS(design)
	if missing := designsystem.LintTokens(css); len(missing) != 0 {
		t.Fatalf("generated CSS misses required tokens: %v", missing)
	}
	for _, expected := range []string{"--stage-w:", "--stage-h:", "--color-primary:"} {
		if !strings.Contains(string(css), expected) {
			t.Fatalf("generated CSS missing %q", expected)
		}
	}
}

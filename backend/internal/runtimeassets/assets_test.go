package runtimeassets

import (
	"regexp"
	"testing"
)

func TestBaseCSSDefinesAllPublicComponentDefaults(t *testing.T) {
	css := string(BaseCSS())
	for _, selector := range []string{
		".kicker",
		".metric",
		".metric-value",
		".metric-label",
		".quote",
		".data-table",
	} {
		pattern := regexp.MustCompile(regexp.QuoteMeta(selector) + `\s*\{`)
		if !pattern.MatchString(css) {
			t.Errorf("base CSS missing default style for %s", selector)
		}
	}
}

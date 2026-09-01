package contextengine

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

var themeTokenPattern = regexp.MustCompile(`(?m)(--[A-Za-z0-9_-]+)\s*:\s*([^;{}]+);`)
var cssCommentPattern = regexp.MustCompile(`(?s)/\*.*?\*/`)

var themeAllowedSelectors = []string{
	"html",
	"body",
	".slide-scaler",
	".slide-stage",
	".slide-content",
	".slide-title",
	".slide-subtitle",
	".slide-body",
	".card",
	".kicker",
	".metric",
	".metric-value",
	".metric-label",
	".quote",
	".data-table",
	"a",
}

func buildThemeContext(theme model.Theme) (ThemeContext, error) {
	tokens := make([]ThemeToken, 0)
	seen := map[string]bool{}
	css := cssCommentPattern.ReplaceAllString(theme.CSS, "")
	for _, match := range themeTokenPattern.FindAllStringSubmatch(css, -1) {
		name := strings.TrimSpace(match[1])
		value := strings.TrimSpace(match[2])
		if name == "" || value == "" || seen[name] {
			continue
		}
		seen[name] = true
		tokens = append(tokens, ThemeToken{Name: name, Value: value})
	}
	if len(tokens) == 0 {
		return ThemeContext{}, fmt.Errorf("theme %q has no CSS token declarations", theme.ID)
	}
	return ThemeContext{
		ID:               theme.ID,
		Name:             theme.Name,
		Description:      theme.Description,
		Tokens:           tokens,
		AllowedSelectors: append([]string(nil), themeAllowedSelectors...),
		Source:           "theme_repository",
		Trust:            "untrusted_read_only_reference",
	}, nil
}

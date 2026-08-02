package spec

import (
	"fmt"
	"strings"
)

// DesignTokensCSS materializes the model-visible Design resource into the
// project-local CSS contract consumed by slide HTML. The CSS is derived
// runtime state, not an additional model-visible resource.
func DesignTokensCSS(design Design) []byte {
	palette := append([]string{}, design.Palette...)
	for len(palette) < 5 {
		palette = append(palette, palette[len(palette)-1])
	}
	unit := design.Spacing.Unit
	if unit < 2 {
		unit = 8
	}
	font := strings.TrimSpace(design.Typography.Body.Family)
	if font == "" {
		font = "Inter, \"Noto Sans SC\", system-ui, sans-serif"
	}
	return []byte(fmt.Sprintf(`/* Generated from design.json. Runtime-managed; do not edit directly. */
:root {
  --color-bg: %s;
  --color-fg: %s;
  --color-primary: %s;
  --color-accent: %s;
  --color-muted: %s;
  --font-sans: %s;
  --font-serif: "Noto Serif SC", Georgia, serif;
  --font-mono: "JetBrains Mono", "SFMono-Regular", monospace;
  --text-title: 72px;
  --text-h1: 52px;
  --text-body: 28px;
  --text-caption: 20px;
  --space-1: %dpx;
  --space-2: %dpx;
  --space-3: %dpx;
  --space-4: %dpx;
  --space-6: %dpx;
  --space-8: %dpx;
  --radius-sm: %dpx;
  --radius-md: %dpx;
  --radius-lg: %dpx;
  --shadow-card: %s;
  --shadow-pop: %s;
  --stage-w: 1600;
  --stage-h: 900;
}
`,
		palette[0], palette[1], palette[2], palette[3], palette[4], font,
		maxCSSInt(unit/2, 2), unit, unit*2, unit*3, unit*4, unit*6,
		maxCSSInt(design.Radius.Card/2, 0), design.Radius.Card, design.Radius.Card*2,
		design.Shadows.Card, design.Shadows.Card,
	))
}

func maxCSSInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

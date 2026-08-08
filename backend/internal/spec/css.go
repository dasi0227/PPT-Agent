package spec

import (
	"io/fs"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/seed"
)

// DesignTokensCSS materializes the model-visible Design resource into the
// project-local CSS contract consumed by slide HTML. The CSS is derived
// runtime state, not an additional model-visible resource.
func DesignTokensCSS(design Design) []byte {
	theme := strings.TrimSpace(design.Theme)
	if theme == "" {
		theme = "swiss-modern"
	}
	raw, err := fs.ReadFile(seed.FS(), "assets/themes/"+theme+"/tokens.css")
	if err == nil {
		return raw
	}
	fallback, fallbackErr := fs.ReadFile(seed.FS(), "assets/themes/swiss-modern/tokens.css")
	if fallbackErr == nil {
		return fallback
	}
	return []byte(`:root {
  --color-bg: #ffffff;
  --color-fg: #111418;
  --color-primary: #d0021b;
  --color-accent: #1c1c1c;
  --color-muted: #f2f3f5;
  --font-sans: "Inter", "Noto Sans SC", system-ui, sans-serif;
  --text-title: 84px;
  --text-body: 32px;
  --space-4: 16px;
  --radius-md: 4px;
  --shadow-card: none;
  --stage-w: 1920;
  --stage-h: 1080;
}
`)
}

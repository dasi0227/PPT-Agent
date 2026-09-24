package designsystem

// Appearance is a read-only runtime dependency, never authored into design.json.
type Appearance struct {
	// ThemeID is carried only inside Runtime to build the frame; the public
	// appearance descriptor exposes the fingerprint and CSS URL separately.
	ThemeID          string            `json:"-"`
	Hash             string            `json:"hash"`
	ThemeCSSURL      string            `json:"theme_css_url"`
	DecorationTokens map[string]string `json:"decoration_tokens"`
}

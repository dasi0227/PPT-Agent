package designsystem

// Appearance is a read-only runtime dependency, never authored into design.json.
type Appearance struct {
	Hash         string            `json:"hash"`
	ThemeCSSURL  string            `json:"theme_css_url"`
	ChromeTokens map[string]string `json:"chrome_tokens"`
}

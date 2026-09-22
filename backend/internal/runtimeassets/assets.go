// Package runtimeassets embeds files required by the slide rendering runtime.
package runtimeassets

import _ "embed"

//go:embed base.css
var baseCSS []byte

//go:embed chrome.js
var chromeJS []byte

// ChromeJS owns the shared outer-canvas decoration rendering and its styles.
func ChromeJS() []byte { return chromeJS }

// BaseCSS returns the theme-neutral slide stage and typography rules.
func BaseCSS() []byte {
	return baseCSS
}

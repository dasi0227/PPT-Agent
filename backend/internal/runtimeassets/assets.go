// Package runtimeassets embeds files required by the slide rendering runtime.
package runtimeassets

import _ "embed"

//go:embed base.css
var baseCSS []byte

// BaseCSS returns the theme-neutral slide stage and typography rules.
func BaseCSS() []byte {
	return baseCSS
}

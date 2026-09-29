package service

import "strings"

// Bound reference excerpts without cutting UTF-8 or losing a final correction.
// User drafts and revision feedback are validated separately and never clipped.
func commandExcerpt(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	const marker = "\n[…省略…]\n"
	available := limit - len([]rune(marker))
	head := available * 2 / 3
	return string(runes[:head]) + marker + string(runes[len(runes)-(available-head):])
}

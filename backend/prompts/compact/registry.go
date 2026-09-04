package compactprompts

import (
	"embed"
	"strings"
)

//go:embed policy.md
var promptFiles embed.FS

func Load() string {
	raw, err := promptFiles.ReadFile("policy.md")
	if err != nil {
		panic(err)
	}
	body := strings.TrimSpace(string(raw))
	if body == "" {
		panic("compact prompt is empty")
	}
	return body
}

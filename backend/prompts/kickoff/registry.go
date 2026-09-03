package kickoffprompts

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"strings"
)

//go:embed policy.md
var promptFiles embed.FS

const Version = "2026-09-04.v1"

type Prompt struct {
	Version string
	Body    string
	Hash    string
}

func Load() Prompt {
	raw, err := promptFiles.ReadFile("policy.md")
	if err != nil {
		panic(err)
	}
	body := strings.TrimSpace(string(raw))
	if body == "" {
		panic("kickoff prompt is empty")
	}
	sum := sha256.Sum256([]byte(body))
	return Prompt{Version: Version, Body: body, Hash: hex.EncodeToString(sum[:])}
}

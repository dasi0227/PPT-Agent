package polishprompts

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"strings"
)

//go:embed policy.md
var promptFiles embed.FS

const Version = "2026-08-23.v1"

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
		panic("polish prompt is empty")
	}
	sum := sha256.Sum256([]byte(body))
	return Prompt{Version: Version, Body: body, Hash: hex.EncodeToString(sum[:])}
}

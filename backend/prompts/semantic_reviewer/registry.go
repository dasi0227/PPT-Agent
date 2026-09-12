package semanticprompts

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
)

//go:embed core/*.md rubrics/*.md schemas/*.md
var promptFiles embed.FS

const Version = "2026-09-12.v2"

type Module struct {
	ID      string
	Version string
	Path    string
	Body    string
	Hash    string
}

var modulePaths = map[string]string{
	"semantic_reviewer_policy": "core/semantic_reviewer_policy.md",
	"ppt_completion_rubric":    "rubrics/ppt_completion_rubric.md",
	"review_output_contract":   "schemas/review_output_contract.md",
}

func Load(id string) (Module, error) {
	rel, ok := modulePaths[id]
	if !ok {
		return Module{}, fmt.Errorf("semantic reviewer prompt module %q is not registered", id)
	}
	raw, err := promptFiles.ReadFile(path.Clean(rel))
	if err != nil {
		return Module{}, fmt.Errorf("semantic reviewer prompt module %q cannot be read: %w", id, err)
	}
	body := strings.TrimSpace(string(raw))
	if body == "" {
		return Module{}, fmt.Errorf("semantic reviewer prompt module %q is empty", id)
	}
	sum := sha256.Sum256([]byte(body))
	return Module{ID: id, Version: Version, Path: rel, Body: body, Hash: hex.EncodeToString(sum[:])}, nil
}

func MustLoad(id string) Module {
	module, err := Load(id)
	if err != nil {
		panic(err)
	}
	return module
}

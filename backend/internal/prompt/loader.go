package prompt

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"strings"

	embedded "github.com/dasi0227/PPT-Agent/backend"
)

type Module struct {
	ID      string
	Version string
	Path    string
	Hash    string
	Body    string
}

func Load(id string) (Module, error) { return load(embedded.PromptFiles, id) }

func load(files fs.FS, id string) (Module, error) {
	entry, ok := catalog[id]
	if !ok {
		return Module{}, fmt.Errorf("unknown prompt %q", id)
	}
	raw, err := fs.ReadFile(files, entry.Path)
	if err != nil {
		return Module{}, fmt.Errorf("load prompt %s: %w", id, err)
	}
	return (Module{ID: id, Path: entry.Path, Version: entry.Version}).WithBody(string(raw))
}

// WithBody normalizes and hashes the actual injected body, including rendered schemas.
func (m Module) WithBody(body string) (Module, error) {
	m.Body = strings.TrimSpace(body)
	if m.Body == "" {
		return Module{}, fmt.Errorf("prompt %s is empty", m.ID)
	}
	m.Hash = fmt.Sprintf("%x", sha256.Sum256([]byte(m.Body)))
	return m, nil
}

func MustLoad(id string) Module {
	m, err := Load(id)
	if err != nil {
		panic(err)
	}
	return m
}

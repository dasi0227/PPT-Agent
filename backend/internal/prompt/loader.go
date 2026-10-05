package prompt

import (
	"crypto/sha256"
	"fmt"
	"html"
	"io/fs"
	"regexp"
	"strings"

	embedded "github.com/dasi0227/PPT-Agent/backend"
	"gopkg.in/yaml.v3"
)

type Module struct {
	ID          string
	Description string
	Scope       string
	Version     string
	Path        string
	Hash        string
	Body        string
}

var frontMatterPattern = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---(?:\r?\n|\z)`)

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
	module := Module{ID: id, Path: entry.Path, Version: entry.Version}
	body := string(raw)
	firstLine, _, _ := strings.Cut(body, "\n")
	if strings.TrimSuffix(firstLine, "\r") == "---" {
		match := frontMatterPattern.FindStringSubmatchIndex(body)
		if match == nil {
			return Module{}, fmt.Errorf("prompt %s has unterminated YAML front matter", id)
		}
		var metadata struct {
			ID          string `yaml:"id"`
			Description string `yaml:"description"`
			Scope       string `yaml:"scope"`
		}
		decoder := yaml.NewDecoder(strings.NewReader(body[match[2]:match[3]]))
		decoder.KnownFields(true)
		if err := decoder.Decode(&metadata); err != nil {
			return Module{}, fmt.Errorf("parse prompt %s front matter: %w", id, err)
		}
		if metadata.ID != id {
			return Module{}, fmt.Errorf("prompt %s front matter declares ID %q", id, metadata.ID)
		}
		if strings.TrimSpace(metadata.Description) == "" {
			return Module{}, fmt.Errorf("prompt %s front matter requires a non-empty description", id)
		}
		module.ID = metadata.ID
		module.Description = strings.TrimSpace(metadata.Description)
		module.Scope = strings.TrimSpace(metadata.Scope)
		body = body[match[1]:]
	}
	return module.WithBody(body)
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

// SystemPrompt renders model-facing metadata without exposing YAML or source paths.
// Scope describes applicability; it does not select modules or grant permissions.
func (m Module) SystemPrompt() string {
	if strings.TrimSpace(m.Body) == "" {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<system_prompt id="%s"`, html.EscapeString(m.ID))
	if m.Description != "" {
		fmt.Fprintf(&b, ` desc="%s"`, html.EscapeString(m.Description))
	}
	if m.Scope != "" {
		fmt.Fprintf(&b, ` scope="%s"`, html.EscapeString(m.Scope))
	}
	fmt.Fprintf(&b, ">\n%s\n</system_prompt>", strings.TrimSpace(m.Body))
	return b.String()
}

// PublicPolicy gives user-visible command results the same disclosure rules as
// the main agent, without pulling execution protocols into a side command.
func PublicPolicy(id string) string {
	return MustLoad("core.output").Body + "\n\n" + MustLoad(id).Body
}

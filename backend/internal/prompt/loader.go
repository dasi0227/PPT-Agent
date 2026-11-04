package prompt

import (
	"crypto/sha256"
	"fmt"
	"html"
	"io/fs"
	"regexp"
	"strings"

	embedded "github.com/dasi0227/PPT-Agent/backend/prompts"
	"gopkg.in/yaml.v3"
)

type Module struct {
	ID      string
	Desc    string
	Version string
	Path    string
	Hash    string
	Body    string
}

var frontMatterPattern = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---(?:\r?\n|\z)`)
var moduleIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)

func Load(id string) (Module, error) { return load(embedded.PromptFiles, id) }

func load(files fs.FS, id string) (Module, error) {
	if !moduleIDPattern.MatchString(id) {
		return Module{}, fmt.Errorf("invalid prompt ID %q: use lowercase snake_case", id)
	}
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
	match := frontMatterPattern.FindStringSubmatchIndex(body)
	if match == nil {
		return Module{}, fmt.Errorf("prompt %s requires YAML front matter with id and desc", id)
	}
	var metadata struct {
		ID   string `yaml:"id"`
		Desc string `yaml:"desc"`
	}
	decoder := yaml.NewDecoder(strings.NewReader(body[match[2]:match[3]]))
	decoder.KnownFields(true)
	if err := decoder.Decode(&metadata); err != nil {
		return Module{}, fmt.Errorf("parse prompt %s front matter: %w", id, err)
	}
	if metadata.ID != id {
		return Module{}, fmt.Errorf("prompt %s front matter declares ID %q", id, metadata.ID)
	}
	if strings.TrimSpace(metadata.Desc) == "" {
		return Module{}, fmt.Errorf("prompt %s front matter requires a non-empty desc", id)
	}
	module.Desc = strings.TrimSpace(metadata.Desc)
	return module.WithBody(body[match[1]:])
}

// WithBody normalizes the body and hashes the complete model-facing module.
func (m Module) WithBody(body string) (Module, error) {
	m.Body = strings.TrimSpace(body)
	if m.Body == "" {
		return Module{}, fmt.Errorf("prompt %s is empty", m.ID)
	}
	m.Hash = fmt.Sprintf("%x", sha256.Sum256([]byte(m.Render())))
	return m, nil
}

func MustLoad(id string) Module {
	m, err := Load(id)
	if err != nil {
		panic(err)
	}
	return m
}

// Render uses the stable ID as the tag, independently of the source filename.
func (m Module) Render() string {
	if strings.TrimSpace(m.Body) == "" {
		return ""
	}
	return fmt.Sprintf("<%s desc=\"%s\">\n%s\n</%s>", m.ID, html.EscapeString(m.Desc), strings.TrimSpace(m.Body), m.ID)
}

// RenderGroup wraps ordered modules in one root without exposing source metadata.
func RenderGroup(tag string, modules ...Module) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<%s>\n", tag)
	for _, module := range modules {
		if rendered := module.Render(); rendered != "" {
			b.WriteString(rendered)
			b.WriteByte('\n')
		}
	}
	fmt.Fprintf(&b, "</%s>", tag)
	return b.String()
}

package prompt

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	embedded "github.com/dasi0227/PPT-Agent/backend"
)

// LoadTool reads a tool's plain Markdown description and JSON argument descriptions.
func LoadTool(name string) (string, map[string]string, error) {
	return loadTool(embedded.PromptFiles, name)
}

func loadTool(files fs.FS, name string) (string, map[string]string, error) {
	if !moduleIDPattern.MatchString(name) {
		return "", nil, fmt.Errorf("invalid tool name %q: use lowercase snake_case", name)
	}
	base := "prompts/tool/" + name
	raw, err := fs.ReadFile(files, base+".md")
	if err != nil {
		return "", nil, fmt.Errorf("load tool description %s: %w", name, err)
	}
	description := strings.TrimSpace(string(raw))
	if description == "" {
		return "", nil, fmt.Errorf("tool description %s is empty", name)
	}
	path := base + ".json"
	raw, err = fs.ReadFile(files, path)
	if err != nil {
		return "", nil, fmt.Errorf("load tool arguments %s: %w", path, err)
	}
	var descriptions map[string]string
	if err := json.Unmarshal(raw, &descriptions); err != nil {
		return "", nil, fmt.Errorf("parse tool arguments %s: %w", path, err)
	}
	if len(descriptions) == 0 {
		return "", nil, fmt.Errorf("tool arguments %s are empty", path)
	}
	for field, description := range descriptions {
		if strings.TrimSpace(description) == "" {
			return "", nil, fmt.Errorf("tool arguments %s: description for %q is empty", path, field)
		}
	}
	return description, descriptions, nil
}

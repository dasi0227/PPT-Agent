package prompt

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	embedded "github.com/dasi0227/PPT-Agent/backend"
)

// LoadToolArgs reads the argument descriptions alongside a tool's Markdown prompt.
func LoadToolArgs(id string) (map[string]string, error) {
	return loadToolArgs(embedded.PromptFiles, id)
}

func loadToolArgs(files fs.FS, id string) (map[string]string, error) {
	entry, ok := catalog[id]
	if !ok || !strings.HasPrefix(id, "tool_") {
		return nil, fmt.Errorf("unknown tool prompt %q", id)
	}
	path := strings.TrimSuffix(entry.Path, ".md") + ".json"
	raw, err := fs.ReadFile(files, path)
	if err != nil {
		return nil, fmt.Errorf("load tool arguments %s: %w", path, err)
	}
	var descriptions map[string]string
	if err := json.Unmarshal(raw, &descriptions); err != nil {
		return nil, fmt.Errorf("parse tool arguments %s: %w", path, err)
	}
	if len(descriptions) == 0 {
		return nil, fmt.Errorf("tool arguments %s are empty", path)
	}
	for field, description := range descriptions {
		if strings.TrimSpace(description) == "" {
			return nil, fmt.Errorf("tool arguments %s: description for %q is empty", path, field)
		}
	}
	return descriptions, nil
}

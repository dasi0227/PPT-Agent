package tooldefinition

import "github.com/dasi0227/PPT-Agent/backend/internal/prompt"

type Schema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Get combines the tool's Markdown description with a fresh parameter schema.
func Get(id string) Schema {
	return Schema{
		Name:        id,
		Description: prompt.MustLoad("tool." + id).Body,
		Parameters:  inputSchema(id),
	}
}

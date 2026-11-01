package tooldefinition

import (
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/prompt"
)

type Schema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Get combines the tool's Markdown and JSON descriptions with a fresh parameter schema.
func Get(id string) Schema {
	description, arguments, err := prompt.LoadTool(id)
	if err != nil {
		panic(err)
	}
	parameters := inputSchema(id)
	if err := applyArgumentDescriptions(parameters, arguments); err != nil {
		panic(fmt.Errorf("tool %s arguments: %w", id, err))
	}
	return Schema{
		Name:        id,
		Description: description,
		Parameters:  parameters,
	}
}

package tooldefinition

type Schema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Get constructs a fresh definition. Descriptions await approval in tools.md.
func Get(id string) Schema {
	return Schema{Name: id, Description: "", Parameters: inputSchema(id)}
}

package tooldefinition

type Schema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Get constructs a fresh definition.
func Get(id string) Schema {
	description := ""
	if id == "load_component" {
		description = "Load component HTML sources as references for constructing presentation pages.\n" +
			"Cases:\n" +
			"- You need to reuse an existing HTML component or read it as a reference.\n" +
			"Rules:\n" +
			"- A successful call adds the complete source to `active_components` for the current run and returns only `status=success`.\n" +
			"- Do not load a component again if it is already present in `active_components`."
	} else if id == "load_skill" {
		description = "Load skill instructions to guide presentation creation.\n" +
			"Cases:\n" +
			"- You need to consult an existing skill's workflows, methods, or guidelines to improve the quality of your work.\n" +
			"Rules:\n" +
			"- A successful call adds the complete skill instructions to `active_skills` for the current run and returns only `status=success`.\n" +
			"- Do not load a skill again if it is already present in `active_skills`."
	}
	return Schema{Name: id, Description: description, Parameters: inputSchema(id)}
}

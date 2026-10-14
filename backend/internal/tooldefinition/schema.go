package tooldefinition

// Resource authoring schemas are independent copies; retain their constraints
// while tool descriptions await approval.
func clearSchemaDescriptions(node map[string]any) {
	node["description"] = ""
	delete(node, "examples")
	for _, keyword := range []string{"properties", "patternProperties", "$defs", "definitions", "dependentSchemas"} {
		if children, ok := node[keyword].(map[string]any); ok {
			for _, child := range children {
				if schema, ok := child.(map[string]any); ok {
					clearSchemaDescriptions(schema)
				}
			}
		}
	}
	for _, keyword := range []string{"items", "additionalProperties", "contains", "if", "then", "else", "not", "propertyNames"} {
		if child, ok := node[keyword].(map[string]any); ok {
			clearSchemaDescriptions(child)
		}
	}
	for _, keyword := range []string{"oneOf", "anyOf", "allOf", "prefixItems"} {
		switch branches := node[keyword].(type) {
		case []any:
			for _, branch := range branches {
				if child, ok := branch.(map[string]any); ok {
					clearSchemaDescriptions(child)
				}
			}
		case []map[string]any:
			for _, child := range branches {
				clearSchemaDescriptions(child)
			}
		}
	}
}

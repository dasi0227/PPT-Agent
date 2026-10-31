package tooldefinition

import "fmt"

// Dot paths address properties inside objects and array items; schema conditions
// are constraints, not additional parameter definitions.
func applyArgumentDescriptions(schema map[string]any, descriptions map[string]string) error {
	fields := map[string]map[string]any{}
	var collect func(map[string]any, string)
	collect = func(node map[string]any, prefix string) {
		if items, ok := node["items"].(map[string]any); ok {
			collect(items, prefix)
		}
		properties, _ := node["properties"].(map[string]any)
		for name, value := range properties {
			field := value.(map[string]any)
			path := name
			if prefix != "" {
				path = prefix + "." + name
			}
			fields[path] = field
			collect(field, path)
		}
	}
	collect(schema, "")
	for path := range descriptions {
		if _, exists := fields[path]; !exists {
			return fmt.Errorf("unknown parameter path %q", path)
		}
	}
	for path, field := range fields {
		description, exists := descriptions[path]
		if !exists {
			return fmt.Errorf("missing description for parameter %q", path)
		}
		field["description"] = description
	}
	return nil
}

// Remove source-document annotations from independent authoring schema copies
// before applying the tool's argument descriptions. Validation constraints stay intact.
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

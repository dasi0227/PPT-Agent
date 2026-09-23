package workflow

import (
	"regexp"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
)

// Typed patch paths share the same domain fragments as whole-resource writes.
// Path authorization still comes from pptmutation; payload validation is not a
// substitute for validating the final, patched resource.
func mutationPatchSchema(operation, definition string, schema map[string]any) map[string]any {
	variants := []any{}
	for _, op := range []string{"add", "remove", "replace"} {
		rules := pptmutation.PatchPathRules(operation, op)
		var walk func(map[string]any, string, string, string)
		walk = func(node map[string]any, pattern, sample, ref string) {
			if sample != "" {
				allowed := false
				for _, rule := range rules {
					if regexp.MustCompile(rule.Pattern).MatchString(sample) {
						allowed = true
						break
					}
				}
				if allowed {
					props := map[string]any{"op": map[string]any{"const": op}, "path": map[string]any{"type": "string", "pattern": "^" + pattern + "$"}}
					required := []string{"op", "path"}
					if op != "remove" {
						required = append(required, "value")
						props["value"] = map[string]any{"$ref": ref}
					}
					variants = append(variants, objectSchema(required, props))
				}
			}
			if properties, ok := node["properties"].(map[string]any); ok {
				keys := make([]string, 0, len(properties))
				for key := range properties {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					child, ok := properties[key].(map[string]any)
					if !ok {
						continue
					}
					pointer := strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
					walk(child, pattern+"/"+regexp.QuoteMeta(pointer), sample+"/"+pointer, ref+"/properties/"+pointer)
				}
			}
			if items, ok := node["items"].(map[string]any); ok {
				walk(items, pattern+"/(?:0|[1-9][0-9]*)", sample+"/0", ref+"/items")
				if op == "add" {
					for _, rule := range rules {
						if regexp.MustCompile(rule.Pattern).MatchString(sample + "/-") {
							variants = append(variants, objectSchema([]string{"op", "path", "value"}, map[string]any{"op": map[string]any{"const": op}, "path": map[string]any{"type": "string", "pattern": "^" + pattern + "/-$"}, "value": map[string]any{"$ref": ref + "/items"}}))
							break
						}
					}
				}
			}
		}
		walk(schema, "", "", "#/$defs/"+definition)
	}
	return map[string]any{"type": "array", "minItems": 1, "maxItems": 32, "items": map[string]any{"oneOf": variants}}
}

func pruneSchemaDefinitions(schema map[string]any) {
	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		return
	}
	used := map[string]bool{}
	var visit func(any)
	visit = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if ref, ok := v["$ref"].(string); ok && strings.HasPrefix(ref, "#/$defs/") {
				name := strings.Split(strings.TrimPrefix(ref, "#/$defs/"), "/")[0]
				if !used[name] {
					used[name] = true
					visit(defs[name])
				}
			}
			for key, child := range v {
				if key != "$defs" {
					visit(child)
				}
			}
		case []any:
			for _, child := range v {
				visit(child)
			}
		}
	}
	visit(schema)
	for name := range defs {
		if !used[name] {
			delete(defs, name)
		}
	}
	if len(defs) == 0 {
		delete(schema, "$defs")
	}
}

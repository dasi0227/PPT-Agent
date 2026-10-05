package schemas

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

const (
	ManifestName  = "manifest"
	OutlineName   = "outline"
	DesignName    = "design"
	SlideSpecName = "slide-spec"
)

//go:embed *.schema.json
var schemaFS embed.FS

var (
	compiledOnce sync.Once
	compiled     map[string]*jsonschema.Schema
	compileErr   error
)

func schemaFilename(name string) (string, error) {
	switch name {
	case ManifestName:
		return "manifest.schema.json", nil
	case OutlineName:
		return "outline.schema.json", nil
	case DesignName:
		return "design.schema.json", nil
	case SlideSpecName:
		return "slide-spec.schema.json", nil
	default:
		return "", fmt.Errorf("unknown PPT domain schema %q", name)
	}
}

func compileAll() {
	compiled = map[string]*jsonschema.Schema{}
	compiler := jsonschema.NewCompiler()
	for _, name := range []string{ManifestName, OutlineName, DesignName, SlideSpecName} {
		filename, _ := schemaFilename(name)
		raw, err := schemaFS.ReadFile(filename)
		if err != nil {
			compileErr = err
			return
		}
		if err := compiler.AddResource(filename, io.NopCloser(bytes.NewReader(raw))); err != nil {
			compileErr = err
			return
		}
	}
	for _, name := range []string{ManifestName, OutlineName, DesignName, SlideSpecName} {
		filename, _ := schemaFilename(name)
		value, err := compiler.Compile(filename)
		if err != nil {
			compileErr = err
			return
		}
		compiled[name] = value
	}
}

func Validate(name string, value any) error {
	compiledOnce.Do(compileAll)
	if compileErr != nil {
		return compileErr
	}
	schema := compiled[name]
	if schema == nil {
		return fmt.Errorf("unknown PPT domain schema %q", name)
	}
	if err := schema.Validate(value); err != nil {
		return fmt.Errorf("%s schema: %w", name, err)
	}
	if name == DesignName {
		return validateDecorationPositions(value)
	}
	return nil
}

func Raw(name string) ([]byte, error) {
	filename, err := schemaFilename(name)
	if err != nil {
		return nil, err
	}
	raw, err := schemaFS.ReadFile(filename)
	return append([]byte(nil), raw...), err
}

// RuntimeContract returns an independent copy of the complete authoritative
// schema used by the runtime validator, including runtime-managed fields.
func RuntimeContract(name string) (map[string]any, error) {
	raw, err := Raw(name)
	if err != nil {
		return nil, err
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, err
	}
	return schema, nil
}

// AuthoringSchema resolves local references and returns an independent schema
// for tool payloads. The persisted resource contract is never modified.
func AuthoringSchema(name string) map[string]any {
	root, err := RuntimeContract(name)
	if err != nil {
		panic(err)
	}
	var resolve func(any) any
	resolve = func(value any) any {
		switch v := value.(type) {
		case map[string]any:
			out := map[string]any{}
			if ref, ok := v["$ref"].(string); ok {
				var target any = root
				for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
					target = target.(map[string]any)[part]
				}
				out = resolve(target).(map[string]any)
			}
			// Keep field-specific annotations alongside a resolved local reference.
			for k, item := range v {
				if k != "$ref" && k != "$defs" && k != "$id" && k != "$schema" && k != "x-agent-example" {
					out[k] = resolve(item)
				}
			}
			if example, ok := v["x-agent-example"]; ok {
				out["examples"] = []any{cloneValue(example)}
			}
			return out
		case []any:
			out := make([]any, len(v))
			for i, item := range v {
				out[i] = resolve(item)
			}
			return out
		default:
			return value
		}
	}
	resolved := resolve(root).(map[string]any)
	resolved = stripRuntimeManaged(resolved).(map[string]any)
	properties := resolved["properties"].(map[string]any)
	fields := []string{}
	for key, raw := range properties {
		field := raw.(map[string]any)
		if field["x-agent-readonly"] == true {
			delete(properties, key)
		} else {
			fields = append(fields, key)
		}
	}
	resolved["required"] = filterStrings(stringList(resolved["required"]), fields)
	if examples, ok := resolved["examples"].([]any); ok {
		for i, example := range examples {
			if m, ok := example.(map[string]any); ok {
				examples[i] = filterMap(m, fields)
			}
		}
	}
	return resolved
}

func cloneValue(value any) any {
	raw, _ := json.Marshal(value)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

func stripRuntimeManaged(value any) any {
	cloned := cloneValue(value)
	return stripRuntimeManagedInPlace(cloned)
}

func stripRuntimeManagedInPlace(value any) any {
	switch v := value.(type) {
	case map[string]any:
		properties, _ := v["properties"].(map[string]any)
		if len(properties) > 0 {
			removed := map[string]bool{}
			for key, raw := range properties {
				property, _ := raw.(map[string]any)
				if managed, _ := property["x-runtime-managed"].(bool); managed {
					delete(properties, key)
					removed[key] = true
					continue
				}
				properties[key] = stripRuntimeManagedInPlace(raw)
			}
			if len(removed) > 0 {
				required := stringList(v["required"])
				filtered := make([]any, 0, len(required))
				for _, field := range required {
					if !removed[field] {
						filtered = append(filtered, field)
					}
				}
				v["required"] = filtered
			}
		}
		if items, ok := v["items"]; ok {
			v["items"] = stripRuntimeManagedInPlace(items)
		}
		if defs, ok := v["$defs"].(map[string]any); ok {
			for key, raw := range defs {
				defs[key] = stripRuntimeManagedInPlace(raw)
			}
		}
		return v
	case []any:
		for i, item := range v {
			v[i] = stripRuntimeManagedInPlace(item)
		}
		return v
	default:
		return value
	}
}

func filterMap(value map[string]any, allowed []string) map[string]any {
	out := make(map[string]any, len(allowed))
	for _, key := range allowed {
		if item, ok := value[key]; ok {
			out[key] = cloneValue(item)
		}
	}
	return out
}

func filterStrings(values, allowed []string) []string {
	allowedSet := make(map[string]bool, len(allowed))
	for _, value := range allowed {
		allowedSet[value] = true
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if allowedSet[value] {
			out = append(out, value)
		}
	}
	return out
}

func stringList(value any) []string {
	raw, _ := value.([]any)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

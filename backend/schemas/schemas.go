package schemas

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

const (
	ManifestName        = "manifest"
	OutlineName         = "outline"
	DesignName          = "design"
	SlideSpecName       = "slide-spec"
	MaterializationName = "materialization"
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
	case MaterializationName:
		return "materialization.schema.json", nil
	default:
		return "", fmt.Errorf("unknown PPT domain schema %q", name)
	}
}

func compileAll() {
	compiled = map[string]*jsonschema.Schema{}
	compiler := jsonschema.NewCompiler()
	for _, name := range []string{ManifestName, OutlineName, DesignName, SlideSpecName, MaterializationName} {
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
	for _, name := range []string{ManifestName, OutlineName, DesignName, SlideSpecName, MaterializationName} {
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
	return nil
}

func ValidateJSON(name string, raw []byte) error {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("%s JSON parse: %w", name, err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("%s JSON parse: trailing content", name)
	}
	return Validate(name, value)
}

func Raw(name string) ([]byte, error) {
	filename, err := schemaFilename(name)
	if err != nil {
		return nil, err
	}
	raw, err := schemaFS.ReadFile(filename)
	return append([]byte(nil), raw...), err
}

type Contract struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Fields      []string       `json:"agent_fields"`
	Required    []string       `json:"required"`
	FieldSchema map[string]any `json:"field_schema"`
	Example     map[string]any `json:"example"`
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

// AgentContract is compiled from the authoritative domain schema. Runtime
// fields are excluded because the model cannot set them.
func AgentContract(name string) (Contract, error) {
	schema, err := RuntimeContract(name)
	if err != nil {
		return Contract{}, err
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return Contract{}, errors.New("schema properties are missing")
	}
	fields := make([]string, 0, len(properties))
	fieldSchema := make(map[string]any, len(properties))
	for key, value := range properties {
		property, _ := value.(map[string]any)
		managed, _ := property["x-runtime-managed"].(bool)
		readOnly, _ := property["x-agent-readonly"].(bool)
		if !managed && !readOnly {
			fields = append(fields, key)
			fieldSchema[key] = stripRuntimeManaged(value)
		}
	}
	sort.Strings(fields)
	example, _ := schema["x-agent-example"].(map[string]any)
	example = filterMap(example, fields)
	required := filterStrings(stringList(schema["required"]), fields)
	return Contract{
		Name: name, Description: strings.TrimSpace(stringValue(schema["description"])),
		Fields: fields, Required: required, FieldSchema: fieldSchema, Example: example,
	}, nil
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
			if ref, ok := v["$ref"].(string); ok {
				var target any = root
				for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
					target = target.(map[string]any)[part]
				}
				return resolve(target)
			}
			out := map[string]any{}
			for k, item := range v {
				if k != "$defs" && k != "$id" && k != "$schema" && k != "x-agent-example" {
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

// OutlineDraftSchema retains the canonical field/array constraints, replacing
// server identities with per-operation client references.
func OutlineDraftSchema(kind string) map[string]any {
	outline := AuthoringSchema(OutlineName)
	section := outline["properties"].(map[string]any)["sections"].(map[string]any)["items"].(map[string]any)
	subsection := section["properties"].(map[string]any)["subsections"].(map[string]any)["items"].(map[string]any)
	slide := section["properties"].(map[string]any)["slides"].(map[string]any)["items"].(map[string]any)
	node := map[string]map[string]any{"section": section, "subsection": subsection, "slide": slide}[kind]
	if node == nil {
		panic("invalid outline draft kind")
	}
	var convert func(any)
	convert = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if props, ok := v["properties"].(map[string]any); ok {
				for _, id := range []string{"id", "slide_id"} {
					if _, exists := props[id]; exists {
						delete(props, id)
						props["client_ref"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 120}
						required := stringList(v["required"])
						for i := range required {
							if required[i] == id {
								required[i] = "client_ref"
							}
						}
						v["required"] = required
					}
				}
			}
			for _, child := range v {
				convert(child)
			}
		case []any:
			for _, child := range v {
				convert(child)
			}
		}
	}
	convert(node)
	return node
}

func RuntimeManagedFields(name string) (map[string]bool, error) {
	raw, err := Raw(name)
	if err != nil {
		return nil, err
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, err
	}
	out := map[string]bool{}
	properties, _ := schema["properties"].(map[string]any)
	for key, value := range properties {
		property, _ := value.(map[string]any)
		if managed, _ := property["x-runtime-managed"].(bool); managed {
			out[key] = true
		}
	}
	return out, nil
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
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

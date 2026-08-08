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
	for _, name := range []string{OutlineName, DesignName, SlideSpecName} {
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
	for _, name := range []string{OutlineName, DesignName, SlideSpecName} {
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
		if !managed {
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

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	raw, _ := json.Marshal(value)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
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

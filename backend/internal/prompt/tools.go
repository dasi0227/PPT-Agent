package prompt

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"sort"
	"strings"

	embedded "github.com/dasi0227/PPT-Agent/backend"
)

const ToolsPath = "prompts/tools.json"

// ToolDocumentation is prose only. Runtime contracts own fields and constraints.
type ToolDocumentation struct {
	Function string            `json:"function"`
	Cases    []string          `json:"cases"`
	Rules    []string          `json:"rules"`
	Args     map[string]string `json:"args"`
	Returns  map[string]string `json:"returns"`
	Errors   map[string]string `json:"errors"`
}

type ToolContract struct {
	Name       string
	Input      map[string]any
	Output     map[string]any
	ErrorCodes []string
}

// DocumentedTool is the provider-facing result; output contracts stay internal.
type DocumentedTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

func LoadTools(contracts map[string]ToolContract) (map[string]DocumentedTool, error) {
	return loadTools(embedded.PromptFiles, contracts)
}

func loadTools(files fs.FS, contracts map[string]ToolContract) (map[string]DocumentedTool, error) {
	raw, err := fs.ReadFile(files, ToolsPath)
	if err != nil {
		return nil, fmt.Errorf("load tools: %w", err)
	}
	if err := rejectDuplicateKeys(json.NewDecoder(strings.NewReader(string(raw)))); err != nil {
		return nil, fmt.Errorf("parse tools: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var docs map[string]ToolDocumentation
	if err := decoder.Decode(&docs); err != nil {
		return nil, fmt.Errorf("parse tools: %w", err)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("tools catalog is empty")
	}
	out := make(map[string]DocumentedTool, len(contracts))
	for _, id := range sortedKeys(docs) {
		contract, ok := contracts[id]
		if !ok {
			return nil, fmt.Errorf("tool documentation %q has no bound contract", id)
		}
		tool, err := assembleTool(id, docs[id], contract)
		if err != nil {
			return nil, err
		}
		out[id] = tool
	}
	for _, id := range sortedKeys(contracts) {
		if _, ok := docs[id]; !ok {
			return nil, fmt.Errorf("tool %q is missing documentation", id)
		}
	}
	return out, nil
}

func assembleTool(id string, doc ToolDocumentation, contract ToolContract) (DocumentedTool, error) {
	if contract.Name == "" || strings.TrimSpace(doc.Function) == "" || doc.Cases == nil || doc.Rules == nil || doc.Args == nil || doc.Returns == nil || doc.Errors == nil {
		return DocumentedTool{}, fmt.Errorf("tool %q requires function, cases, rules, args, returns and errors", id)
	}
	input := CopyToolSchema(contract.Input)
	for _, path := range sortedKeys(doc.Args) {
		nodes, err := documentedFields(input, path)
		if err != nil {
			return DocumentedTool{}, fmt.Errorf("tool %q args %q: %w", id, path, err)
		}
		if strings.TrimSpace(doc.Args[path]) == "" {
			return DocumentedTool{}, fmt.Errorf("tool %q args %q has empty description", id, path)
		}
		for _, node := range nodes {
			if previous, ok := node["description"].(string); ok && previous != doc.Args[path] {
				return DocumentedTool{}, fmt.Errorf("tool %q args %q conflicts with another description of the same referenced field", id, path)
			}
			node["description"] = doc.Args[path]
		}
	}
	for field := range topLevelFields(input, input, map[string]bool{}) {
		if _, ok := doc.Args[field]; !ok {
			return DocumentedTool{}, fmt.Errorf("tool %q args is missing top-level field %q", id, field)
		}
	}
	for _, path := range sortedKeys(doc.Returns) {
		if _, err := documentedFields(contract.Output, path); err != nil {
			return DocumentedTool{}, fmt.Errorf("tool %q returns %q: %w", id, path, err)
		}
		if strings.TrimSpace(doc.Returns[path]) == "" {
			return DocumentedTool{}, fmt.Errorf("tool %q returns %q has empty description", id, path)
		}
	}
	expected := map[string]bool{}
	for _, code := range contract.ErrorCodes {
		if expected[code] {
			return DocumentedTool{}, fmt.Errorf("tool %q declares duplicate error code %q", id, code)
		}
		expected[code] = true
	}
	for _, code := range sortedKeys(doc.Errors) {
		if !expected[code] || strings.TrimSpace(doc.Errors[code]) == "" {
			return DocumentedTool{}, fmt.Errorf("tool %q errors %q is undeclared or empty", id, code)
		}
		delete(expected, code)
	}
	if len(expected) != 0 {
		return DocumentedTool{}, fmt.Errorf("tool %q errors is missing codes %v", id, sortedKeys(expected))
	}
	var description strings.Builder
	description.WriteString(strings.TrimSpace(doc.Function))
	for _, section := range []struct {
		name string
		rows []string
	}{{"Cases", doc.Cases}, {"Rules", doc.Rules}, {"Returns", documentationRows(doc.Returns)}, {"Errors", documentationRows(doc.Errors)}} {
		if len(section.rows) == 0 {
			continue
		}
		description.WriteString("\n\n" + section.name + ":")
		for _, row := range section.rows {
			if strings.TrimSpace(row) == "" {
				return DocumentedTool{}, fmt.Errorf("tool %q %s contains an empty description", id, section.name)
			}
			description.WriteString("\n- " + row)
		}
	}
	return DocumentedTool{Name: contract.Name, Description: description.String(), Parameters: input}, nil
}

var fieldPath = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

func documentedFields(root map[string]any, path string) ([]map[string]any, error) {
	if !fieldPath.MatchString(path) {
		return nil, fmt.Errorf("use dot-separated field names without array or schema indices")
	}
	nodes := findFields(root, root, strings.Split(path, "."), map[string]bool{})
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no corresponding field exists in the contract")
	}
	return nodes, nil
}

func findFields(root, node map[string]any, parts []string, refs map[string]bool) []map[string]any {
	if node == nil || node["type"] == "null" {
		return nil
	}
	var found []map[string]any
	if len(parts) == 0 {
		found = append(found, node)
	}
	if ref, ok := node["$ref"].(string); ok {
		key := ref + "|" + strings.Join(parts, ".")
		if !refs[key] {
			refs[key] = true
			found = append(found, findFields(root, localReference(root, ref), parts, refs)...)
			delete(refs, key)
		}
	}
	for _, keyword := range []string{"oneOf", "anyOf", "allOf"} {
		for _, branch := range schemaBranches(node[keyword]) {
			found = append(found, findFields(root, branch, parts, refs)...)
		}
	}
	if len(parts) != 0 {
		if items, ok := node["items"].(map[string]any); ok {
			found = append(found, findFields(root, items, parts, refs)...)
		}
		if properties, ok := node["properties"].(map[string]any); ok {
			if child, ok := properties[parts[0]].(map[string]any); ok {
				found = append(found, findFields(root, child, parts[1:], refs)...)
			}
		}
	}
	return found
}

func topLevelFields(root, node map[string]any, refs map[string]bool) map[string]bool {
	out := map[string]bool{}
	if properties, ok := node["properties"].(map[string]any); ok {
		for name := range properties {
			out[name] = true
		}
	}
	if ref, ok := node["$ref"].(string); ok && !refs[ref] {
		refs[ref] = true
		for name := range topLevelFields(root, localReference(root, ref), refs) {
			out[name] = true
		}
		delete(refs, ref)
	}
	for _, keyword := range []string{"oneOf", "anyOf", "allOf"} {
		for _, branch := range schemaBranches(node[keyword]) {
			for name := range topLevelFields(root, branch, refs) {
				out[name] = true
			}
		}
	}
	return out
}

func localReference(root map[string]any, ref string) map[string]any {
	if ref == "#" {
		return root
	}
	if !strings.HasPrefix(ref, "#/") {
		return nil
	}
	var value any = root
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		value = object[part]
	}
	object, _ := value.(map[string]any)
	return object
}

func schemaBranches(value any) []map[string]any {
	var out []map[string]any
	switch branches := value.(type) {
	case []any:
		for _, branch := range branches {
			if object, ok := branch.(map[string]any); ok {
				out = append(out, object)
			}
		}
	case []map[string]any:
		out = append(out, branches...)
	}
	return out
}

// CopyToolSchema preserves structural value types and strips prose recursively.
// Resource schemas remain unchanged for their non-tool consumers.
func CopyToolSchema(source map[string]any) map[string]any {
	out := CloneToolSchema(source)
	stripSchemaProse(out)
	return out
}

func CloneToolSchema(source map[string]any) map[string]any {
	return copySchemaValue(source).(map[string]any)
}

func stripSchemaProse(node map[string]any) {
	delete(node, "description")
	delete(node, "examples")
	for _, keyword := range []string{"properties", "patternProperties", "$defs", "definitions", "dependentSchemas"} {
		if children, ok := node[keyword].(map[string]any); ok {
			for _, child := range children {
				if schema, ok := child.(map[string]any); ok {
					stripSchemaProse(schema)
				}
			}
		}
	}
	for _, keyword := range []string{"items", "additionalProperties", "contains", "if", "then", "else", "not", "propertyNames"} {
		if child, ok := node[keyword].(map[string]any); ok {
			stripSchemaProse(child)
		}
	}
	for _, keyword := range []string{"oneOf", "anyOf", "allOf", "prefixItems"} {
		for _, child := range schemaBranches(node[keyword]) {
			stripSchemaProse(child)
		}
	}
}

func copySchemaValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			out[key] = copySchemaValue(child)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			out[i] = copySchemaValue(child)
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, len(typed))
		for i, child := range typed {
			out[i] = CloneToolSchema(child)
		}
		return out
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}

func documentationRows(values map[string]string) []string {
	rows := make([]string, 0, len(values))
	for _, key := range sortedKeys(values) {
		rows = append(rows, key+": "+values[key])
	}
	return rows
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// A normal JSON decode silently overwrites duplicate object keys.
func rejectDuplicateKeys(decoder *json.Decoder) error {
	var value func(string) error
	value = func(path string) error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				token, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := token.(string)
				if !ok || seen[key] {
					return fmt.Errorf("duplicate or invalid key at %s.%v", path, token)
				}
				seen[key] = true
				if err := value(path + "." + key); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := value(path); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unexpected delimiter %q", delim)
		}
		_, err = decoder.Token()
		return err
	}
	if err := value("tools"); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON content")
	}
	return nil
}

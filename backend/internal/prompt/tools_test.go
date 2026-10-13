package prompt

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestToolDescriptionsTraverseArraysReferencesAndNullableBranches(t *testing.T) {
	leaf := map[string]any{"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "null"}}}
	input := map[string]any{"type": "object", "properties": map[string]any{
		"questions": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/$defs/question"}},
	}, "$defs": map[string]any{"question": map[string]any{"type": "object", "properties": map[string]any{"description": leaf}}}}
	output := map[string]any{"oneOf": []any{map[string]any{"properties": map[string]any{"decision": map[string]any{"type": "string"}}}, map[string]any{"properties": map[string]any{"content": map[string]any{"type": "object"}}}}}
	doc := ToolDocumentation{Function: "Ask.", Cases: []string{}, Rules: []string{"Wait."}, Args: map[string]string{"questions": "Questions.", "questions.description": "User-visible explanation."}, Returns: map[string]string{"decision": "Normal decision.", "content": "Complete content."}, Errors: map[string]string{"INVALID": "Invalid input."}}
	before, _ := json.Marshal(input)
	tool, err := assembleTool("ask", doc, ToolContract{Name: "ask", Input: input, Output: output, BusinessErrorCodes: []string{"INVALID"}})
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := documentedFields(tool.Parameters, "questions.description")
	if err != nil || len(nodes) != 2 {
		t.Fatalf("missing field or non-null branch: %v %v", nodes, err)
	}
	for _, node := range nodes {
		if node["description"] != "User-visible explanation." {
			t.Fatal("field description missing")
		}
	}
	after, _ := json.Marshal(input)
	if string(before) != string(after) || strings.Contains(tool.Description, "Args:") || strings.Contains(tool.Description, "Cases:") {
		t.Fatal("source contract mutated or empty/duplicate sections disclosed")
	}
	if !strings.Contains(tool.Description, "Returns:\n- content: Complete content.\n- decision: Normal decision.") {
		t.Fatal("unstable return formatting")
	}
	copy := CloneToolSchema(tool.Parameters)
	if !reflect.DeepEqual(copy, tool.Parameters) {
		t.Fatal("cloning dropped annotations or constraints")
	}
	doc.Errors = map[string]string{}
	withoutErrors, err := assembleTool("ask", doc, ToolContract{Name: "ask", Input: input, Output: output})
	if err != nil || strings.Contains(withoutErrors.Description, "Errors:") {
		t.Fatalf("empty business errors were rejected or disclosed: %v %s", err, withoutErrors.Description)
	}
}

func TestToolCatalogRejectsUnknownPathsCodesMissingEntriesAndDuplicateKeys(t *testing.T) {
	base := `{"inspect":{"function":"Inspect.","cases":[],"rules":[],"args":{"id":"ID."},"returns":{"content":"Body."},"errors":{"FAILED":"Failure."}}}`
	contracts := map[string]ToolContract{"inspect": {Name: "inspect", Input: map[string]any{"properties": map[string]any{"id": map[string]any{"type": "string"}}}, Output: map[string]any{"properties": map[string]any{"content": map[string]any{"type": "string"}}}, BusinessErrorCodes: []string{"FAILED"}}}
	for name, raw := range map[string]string{
		"input path":     strings.Replace(base, `"id":"ID."`, `"ids":"ID."`, 1),
		"output path":    strings.Replace(base, `"content":"Body."`, `"contents":"Body."`, 1),
		"array index":    strings.Replace(base, `"id":"ID."`, `"id[]":"ID."`, 1),
		"unknown code":   strings.Replace(base, `"FAILED":"Failure."`, `"OTHER":"Failure."`, 1),
		"framework code": strings.Replace(base, `"FAILED":"Failure."`, `"RUN_CANCELED":"Canceled."`, 1),
		"missing code":   strings.Replace(base, `"FAILED":"Failure."`, "", 1),
		"empty error":    strings.Replace(base, `"FAILED":"Failure."`, `"FAILED":" "`, 1),
		"unknown entry":  strings.Replace(base, `"inspect":`, `"extra":`, 1),
		"missing entry":  `{}`,
		"duplicate key":  strings.Replace(base, `"id":"ID."`, `"id":"ID.","id":"Other."`, 1),
		"duplicate code": strings.Replace(base, `"FAILED":"Failure."`, `"FAILED":"Failure.","FAILED":"Other."`, 1),
		"unknown config": strings.Replace(base, `"function":`, `"extra":`, 1),
		"trailing JSON":  base + `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadTools(fstest.MapFS{ToolsPath: &fstest.MapFile{Data: []byte(raw)}}, contracts); err == nil {
				t.Fatal("invalid documentation accepted")
			}
		})
	}
}

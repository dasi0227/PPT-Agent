package schemas

import (
	"encoding/json"
	"testing"
)

func TestAuthoringSchemasComeFromSchemas(t *testing.T) {
	for _, name := range []string{MetaName, OutlineName, ConstraintsName, SlideSpecName} {
		contract := AuthoringSchema(name)
		properties := contract["properties"].(map[string]any)
		required := contract["required"].([]string)
		if len(properties) == 0 || len(required) == 0 {
			t.Fatalf("%s contract is empty", name)
		}
	}
}

func TestSlideSpecAuthoringSchemaExcludesOutlinePlacement(t *testing.T) {
	contract := AuthoringSchema(SlideSpecName)
	properties := contract["properties"].(map[string]any)
	required := contract["required"].([]string)
	for _, field := range []string{"core", "purpose", "layout"} {
		if properties[field] == nil || !containsString(required, field) {
			t.Fatalf("%s must be required in a saved spec", field)
		}
	}
	if _, exists := properties["approach"]; exists {
		t.Fatal("removed approach field remains in the authoring schema")
	}
	for _, field := range []string{"project_id", "slide_id"} {
		if properties[field] != nil {
			t.Fatalf("resource identity %q leaked into slide content contract", field)
		}
	}
	for _, field := range []string{"section" + "_id", "subsection" + "_id", "title"} {
		if properties[field] != nil {
			t.Fatalf("outline-owned field %q leaked into slide agent contract", field)
		}
	}
}

func TestOutlineRuntimeContractOwnsStableNodeIDs(t *testing.T) {
	contract, err := RuntimeContract(OutlineName)
	if err != nil {
		t.Fatal(err)
	}
	defs := contract["$defs"].(map[string]any)
	for _, name := range []string{"section", "subsection", "slide"} {
		properties := defs[name].(map[string]any)["properties"].(map[string]any)
		field := "id"
		if _, ok := properties[field]; !ok {
			t.Fatalf("%s stable ID missing", name)
		}
	}
}

func TestAuthoringSchemasExcludeResourceIdentity(t *testing.T) {
	for _, name := range []string{MetaName, OutlineName, ConstraintsName, SlideSpecName} {
		contract := AuthoringSchema(name)
		properties := contract["properties"].(map[string]any)
		for _, field := range []string{"project", "project_id", "slide_id", "version", "created_at", "updated_at"} {
			if _, exists := properties[field]; exists {
				t.Errorf("%s content contract still contains %s", name, field)
			}
		}
	}
}

func TestConstraintsOwnPresentationRulesAndMetaAndOutlineDoNot(t *testing.T) {
	constraints := AuthoringSchema(ConstraintsName)["properties"].(map[string]any)
	for _, domain := range []string{"content", "visual"} {
		properties := constraints[domain].(map[string]any)["properties"].(map[string]any)
		for _, field := range []string{"requirements", "prohibitions"} {
			if properties[field] == nil {
				t.Errorf("%s constraints do not contain %q", domain, field)
			}
		}
	}
	for _, name := range []string{MetaName, OutlineName} {
		properties := AuthoringSchema(name)["properties"].(map[string]any)
		for _, field := range []string{"requirements", "prohibitions"} {
			if properties[field] != nil {
				t.Errorf("%s still owns %q", name, field)
			}
		}
	}
	if AuthoringSchema(OutlineName)["properties"].(map[string]any)["slide"+"_order"] != nil {
		t.Error("outline still contains a second ordering field")
	}
}

func TestSchemasRejectUnknownFields(t *testing.T) {
	raw, err := Raw(OutlineName)
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	if schema["additionalProperties"] != false {
		t.Fatal("outline must reject unknown fields")
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

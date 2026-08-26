package schemas

import (
	"encoding/json"
	"testing"
)

func TestAgentContractsComeFromSchemas(t *testing.T) {
	for _, name := range []string{DeckName, OutlineName, DesignName, SlideSpecName} {
		contract, err := AgentContract(name)
		if err != nil {
			t.Fatalf("%s contract: %v", name, err)
		}
		if len(contract.Fields) == 0 || len(contract.Required) == 0 ||
			len(contract.FieldSchema) == 0 || len(contract.Example) == 0 {
			t.Fatalf("%s contract is empty", name)
		}
		managed, err := RuntimeManagedFields(name)
		if err != nil {
			t.Fatal(err)
		}
		for field := range managed {
			if containsString(contract.Fields, field) {
				t.Errorf("%s managed field %q leaked into agent_fields", name, field)
			}
			if containsString(contract.Required, field) {
				t.Errorf("%s managed field %q leaked into required", name, field)
			}
			if _, leaked := contract.FieldSchema[field]; leaked {
				t.Errorf("%s managed field %q leaked into field_schema", name, field)
			}
			if _, leaked := contract.Example[field]; leaked {
				t.Errorf("%s managed field %q leaked into example", name, field)
			}
		}
	}
}

func TestRuntimeContractsContainManagedFields(t *testing.T) {
	for _, name := range []string{DeckName, OutlineName, DesignName, SlideSpecName, MaterializationName} {
		contract, err := RuntimeContract(name)
		if err != nil {
			t.Fatal(err)
		}
		managed, err := RuntimeManagedFields(name)
		if err != nil {
			t.Fatal(err)
		}
		for field := range managed {
			if _, exists := contract["properties"].(map[string]any)[field]; !exists {
				t.Errorf("%s runtime contract no longer contains %q", name, field)
			}
		}
	}
}

func TestSlideSpecAgentContractExcludesOutlinePlacement(t *testing.T) {
	contract, err := AgentContract(SlideSpecName)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"project_id", "slide_id"} {
		if containsString(contract.Fields, field) {
			t.Fatalf("managed field %q leaked into slide agent contract", field)
		}
	}
	for _, field := range []string{"section" + "_id", "subsection" + "_id", "role"} {
		if containsString(contract.Fields, field) {
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
		if name == "slide" {
			field = "slide_id"
		}
		if _, ok := properties[field]; !ok {
			t.Fatalf("%s stable ID missing", name)
		}
	}
}

func TestAuthoringSchemasUseProjectID(t *testing.T) {
	for _, name := range []string{DeckName, OutlineName, DesignName, SlideSpecName} {
		contract, err := RuntimeContract(name)
		if err != nil {
			t.Fatal(err)
		}
		properties := contract["properties"].(map[string]any)
		if _, exists := properties["project_id"]; !exists {
			t.Errorf("%s runtime contract does not contain project_id", name)
		}
		if _, exists := properties["project"]; exists {
			t.Errorf("%s runtime contract still contains obsolete project field", name)
		}
	}
}

func TestDeckOwnsPresentationRulesAndOutlineDoesNot(t *testing.T) {
	deck, err := RuntimeContract(DeckName)
	if err != nil {
		t.Fatal(err)
	}
	deckProperties := deck["properties"].(map[string]any)
	outline, err := RuntimeContract(OutlineName)
	if err != nil {
		t.Fatal(err)
	}
	outlineProperties := outline["properties"].(map[string]any)
	for _, field := range []string{"requirements", "prohibitions"} {
		if _, exists := deckProperties[field]; !exists {
			t.Errorf("deck runtime contract does not contain %q", field)
		}
		if _, exists := outlineProperties[field]; exists {
			t.Errorf("outline still owns %q", field)
		}
	}
	if _, exists := outlineProperties["slide"+"_order"]; exists {
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

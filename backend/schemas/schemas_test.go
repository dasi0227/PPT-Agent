package schemas

import (
	"encoding/json"
	"testing"
)

func TestAgentContractsComeFromSchemas(t *testing.T) {
	for _, name := range []string{OutlineName, DesignName, SlideSpecName} {
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
	for _, name := range []string{OutlineName, DesignName, SlideSpecName, MaterializationName} {
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

func TestSlideSpecAgentContractHidesPlacementIDs(t *testing.T) {
	contract, err := AgentContract(SlideSpecName)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"project_id", "slide_id", "section_id", "subsection_id"} {
		if containsString(contract.Fields, field) {
			t.Fatalf("managed field %q leaked into slide agent contract", field)
		}
	}
}

func TestAuthoringSchemasUseProjectID(t *testing.T) {
	for _, name := range []string{OutlineName, DesignName, SlideSpecName} {
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

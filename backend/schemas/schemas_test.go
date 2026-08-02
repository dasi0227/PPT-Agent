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

func TestRuntimeContractsStillRequireManagedFields(t *testing.T) {
	for _, name := range []string{OutlineName, DesignName, SlideSpecName} {
		contract, err := RuntimeContract(name)
		if err != nil {
			t.Fatal(err)
		}
		required := stringList(contract["required"])
		managed, err := RuntimeManagedFields(name)
		if err != nil {
			t.Fatal(err)
		}
		for field := range managed {
			if !containsString(required, field) {
				t.Errorf("%s runtime contract no longer requires %q", name, field)
			}
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

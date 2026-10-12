package workflow

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
)

func TestModelObservationKeepsBusinessResultsAndHidesInternalMetadata(t *testing.T) {
	for _, success := range []bool{true, false} {
		result := SuccessfulToolResult("saved")
		result.OK = success
		result.Code = CodeContentConflict
		result.Data = map[string]any{"content": map[string]any{"title": "Deck", "ok": true, "content_hash": "private"}, "source_hash": "private"}
		result.ChangedTargets = []ChangedTarget{{Type: "deck", Part: "manifest", Hash: "private"}}
		raw := modelToolObservation(result)
		var value map[string]any
		if json.Unmarshal([]byte(raw), &value) != nil {
			t.Fatal(raw)
		}
		for _, forbidden := range []string{`"ok"`, `"content_hash"`, `"source_hash"`, `"changed_targets"`, `"data"`} {
			if strings.Contains(raw, forbidden) {
				t.Fatalf("metadata leaked: %s", raw)
			}
		}
		if !success && value["reason"] == nil {
			t.Fatal("error lost reason")
		}
	}
}

func TestContentPrecheckContractsAndDescriptionsPreserveScoreMeaning(t *testing.T) {
	for _, name := range []string{"edit_html", "run_command"} {
		schema := tooldefinition.OutputContract(name)
		if name == "run_command" {
			schema = schema["oneOf"].([]any)[0].(map[string]any)
		}
		precheck := schema["properties"].(map[string]any)["content_precheck"].(map[string]any)
		completed := precheck["oneOf"].([]any)[0].(map[string]any)
		scores := completed["properties"].(map[string]any)["scores"].(map[string]any)["properties"].(map[string]any)
		tool := tooldefinition.Get(name)
		for _, rubric := range model.ContentPrecheckRubrics() {
			field := scores[rubric.Dimension].(map[string]any)
			if field["type"] != "number" || field["minimum"] != 0 || field["maximum"] != 3 {
				t.Fatal("fractional score constraints changed")
			}
			if !strings.Contains(tool.Description, rubric.Dimension) {
				t.Fatalf("%s lost score meaning for %s", name, rubric.Dimension)
			}
		}
		if !strings.Contains(tool.Description, "probability-weighted") || strings.Contains(tool.Description, "Output contract (") {
			t.Fatal("score interpretation missing or legacy schema projection disclosed")
		}
	}
}

func TestContentPrecheckObservationOmitsUnusableScores(t *testing.T) {
	scores := map[string]model.ContentScore{model.ContentCoverage: {Score: 2, MaxScore: 3, Confidence: 1}}
	for _, status := range []string{"unavailable", "skipped", "stale", "pending", "completed"} {
		assessment := contentPrecheckObservation(model.ContentPrecheck{Status: status, Reason: "material_changed", Scores: scores})
		if assessment["scores"] != nil || assessment["reason"] == nil || assessment["status"] == "pending" || assessment["status"] == "completed" {
			t.Fatalf("incomplete assessment exposed usable scores: %+v", assessment)
		}
	}
}

func TestApprovalDescriptionsAndContractsPreserveNormalDecisions(t *testing.T) {
	for _, tc := range []struct {
		name      string
		decisions []string
	}{
		{"create_plan", []string{"approve", "refuse"}}, {"request_privilege", []string{"approve", "revise", "refuse"}},
		{"run_command", nil}, {"edit_manifest", nil}, {"edit_design", nil}, {"edit_outline", nil},
	} {
		schema := tooldefinition.OutputContract(tc.name)
		tool := tooldefinition.Get(tc.name)
		var props map[string]any
		if tc.decisions == nil {
			props = schema["oneOf"].([]any)[1].(map[string]any)["properties"].(map[string]any)
			if props["decision"].(map[string]any)["const"] != "refuse" {
				t.Fatal("normal refusal changed")
			}
			result := RefusedToolResult("not applied")
			var observation map[string]any
			_ = json.Unmarshal([]byte(modelToolObservation(result)), &observation)
			if err := validateToolArguments(ToolSchema{Parameters: schema}, observation); err != nil {
				t.Fatal(err)
			}
		} else {
			props = schema["properties"].(map[string]any)
			if !reflect.DeepEqual(props["decision"].(map[string]any)["enum"], tc.decisions) {
				t.Fatal("decision constraints changed")
			}
		}
		if props["feedback"].(map[string]any)["type"] != "string" || !strings.Contains(tool.Description, "verbatim") || !strings.Contains(tool.Description, "Returns:") {
			t.Fatal("normal feedback meaning missing")
		}
	}
}

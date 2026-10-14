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
		result.Code = model.CodeContentConflict
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
		wantStatus := "success"
		if !success {
			wantStatus = "error"
		}
		if value["status"] != wantStatus || result.Data["status"] != nil {
			t.Fatal("shared status missing or injected into durable business data")
		}
	}
}

func TestModelAcknowledgmentDoesNotExposeInternalSummary(t *testing.T) {
	result := SuccessfulToolResult("private runtime acknowledgement")
	if got := modelToolObservation(result); got != `{"status":"success"}` {
		t.Fatalf("unexpected acknowledgement: %s", got)
	}
}

func TestPreviewContractPreservesScoreMeaningAndJudges(t *testing.T) {
	schema := tooldefinition.OutputContract("render_slide")
	preview := schema["properties"].(map[string]any)["preview"].(map[string]any)
	props := preview["properties"].(map[string]any)
	scores := props["scores"].(map[string]any)["properties"].(map[string]any)
	tool := tooldefinition.Get("render_slide")
	for _, rubric := range model.PreviewRubrics() {
		field := scores[rubric.Dimension].(map[string]any)
		if field["type"] != "number" || field["minimum"] != 0 || field["maximum"] != 3 {
			t.Fatal("score contract changed")
		}
		if !strings.Contains(tool.Description, rubric.Dimension) {
			t.Fatal("score meaning missing")
		}
	}
	if len(props) != 2 || props["judges"] == nil || !strings.Contains(tool.Description, "probability-weighted") {
		t.Fatal("preview contract is incomplete")
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
			result := RefusedToolResult("not applied")
			if tc.name == "edit_manifest" || tc.name == "edit_design" || tc.name == "edit_outline" {
				result = refusedResourceEditResult()
			}
			if props["decision"].(map[string]any)["const"] != "refuse" || props["status"].(map[string]any)["const"] != "success" {
				t.Fatal("normal refusal changed")
			}
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
			if len(props) != 3 || props["summary"] != nil || strings.Contains(tool.Description, "summary:") {
				t.Fatal("approval result contains redundant fields")
			}
		}
		if props["feedback"].(map[string]any)["type"] != "string" || !strings.Contains(tool.Description, "feedback:") || !strings.Contains(tool.Description, "Returns:") {
			t.Fatal("normal feedback meaning missing")
		}
	}
}

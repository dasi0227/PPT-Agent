package workflow

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
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

func TestModelObservationPreservesNormalRefusal(t *testing.T) {
	for _, name := range []string{"run_command", "edit_manifest", "edit_design", "edit_outline"} {
		result := RefusedToolResult("not applied")
		if name != "run_command" {
			result = refusedResourceEditResult()
		}
		var observation map[string]any
		if err := json.Unmarshal([]byte(modelToolObservation(result)), &observation); err != nil {
			t.Fatal(err)
		}
		if !result.OK || result.Applied || observation["status"] != "success" || observation["decision"] != "refuse" || observation["content"] != nil {
			t.Fatalf("%s normal refusal changed: %+v", name, observation)
		}
	}
}

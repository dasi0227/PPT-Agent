package workflow

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
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
	for _, name := range []string{"run_command", "edit_manifest", "edit_design", "edit_outline", "create_plan", "request_privilege"} {
		result := RefusedToolResult("not applied")
		if name == "run_command" {
			result = refusedCommandResult(ToolDecision{Command: "cat .env"})
		}
		result.Observation = `{"status":"success"}`
		result.ObservationParts = []llm.ContentPart{{Type: "text", Text: result.Observation}, {Type: "image", ImageRef: "retained-image"}}
		result.Retryable = true
		call := llm.ToolCall{ID: "refused", Name: name}
		bound := bindToolObservation(result, call)
		if !bound.OK || bound.Applied || bound.Retryable {
			t.Fatalf("%s refusal changed execution state: %+v", name, bound)
		}
		for _, messages := range [][]llm.Message{
			appendToolObservation(nil, call, "", result),
			appendBatchObservations(nil, []llm.ToolCall{call}, "", []ToolResult{result}),
		} {
			reply := messages[len(messages)-1]
			var observation map[string]any
			if err := json.Unmarshal([]byte(reply.Text()), &observation); err != nil {
				t.Fatal(err)
			}
			if observation["status"] != "refuse" || observation["decision"] != nil || observation["content"] != nil || observation["code"] != nil || len(reply.Content) != 2 || reply.Content[1].ImageRef != "retained-image" {
				t.Fatalf("%s normal refusal changed: %+v", name, observation)
			}
		}
		if result.Data["status"] != nil || result.Data["decision"] != nil {
			t.Fatal("observation mutated durable business data")
		}
	}
}

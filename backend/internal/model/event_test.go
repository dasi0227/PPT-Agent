package model

import (
	"strings"
	"testing"
)

func TestPublicPayloadValidationRejectsInternalAndUnsafeData(t *testing.T) {
	base := NewPublicEventBase("r1")
	valid := ToolStartedPayload{
		PublicEventBase: base, CallID: "c1", Tool: "edit_spec",
		Display: PublicDisplay{Label: "生成第 3 页"},
	}
	if err := ValidatePublicEvent(EventToolStarted, valid); err != nil {
		t.Fatal(err)
	}
	manual := valid
	manual.Tool = "git_commit"
	if err := ValidatePublicEvent(EventToolStarted, manual); err == nil {
		t.Fatal("manual commit was accepted as an Agent tool event")
	}
	if err := ValidatePublicEvent(EventToolCompleted, ToolCompletedPayload{
		PublicEventBase: base, CallID: "c1", Tool: "git_commit", Status: "completed", Display: valid.Display,
	}); err == nil {
		t.Fatal("manual commit was accepted as an Agent tool result")
	}
	for _, payload := range []map[string]any{
		{
			"schema_version": 6, "run_id": "r1", "occurred_at": base.OccurredAt,
			"call_id": "c1", "tool": "edit_spec", "display": map[string]any{"label": "生成"},
			"args": map[string]any{"html": "<section />"},
		},
		{
			"schema_version": 6, "run_id": "r1", "occurred_at": base.OccurredAt,
			"message_id": "m1", "text": "安全摘要", "reasoning_content": "hidden",
		},
	} {
		event := EventToolStarted
		if _, ok := payload["message_id"]; ok {
			event = EventMessageThinking
		}
		err := ValidatePublicEvent(event, payload)
		if err == nil || !strings.Contains(err.Error(), "forbidden") {
			t.Fatalf("payload accepted: %+v err=%v", payload, err)
		}
	}
	if err := ValidatePublicEvent(EventType("context.assembled"), base); err == nil {
		t.Fatal("internal trace event was accepted as public")
	}
}

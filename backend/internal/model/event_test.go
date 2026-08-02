package model

import (
	"strings"
	"testing"
)

func TestPublicEventTypeSetContainsExactlyElevenEvents(t *testing.T) {
	want := []EventType{
		EventRunStarted, EventRunProgress, EventRunFinished, EventPlanUpdated,
		EventMessageReasoning, EventMessageMilestone, EventMessageFinal,
		EventToolStarted, EventToolCompleted, EventQuestionAsked, EventQuestionAnswered,
	}
	if len(PublicEventTypes) != 11 {
		t.Fatalf("public event count=%d", len(PublicEventTypes))
	}
	for index, event := range want {
		if PublicEventTypes[index] != event {
			t.Fatalf("event[%d]=%s want=%s", index, PublicEventTypes[index], event)
		}
		if event.Terminal() != (event == EventRunFinished) {
			t.Fatalf("terminal(%s)=%v", event, event.Terminal())
		}
	}
}

func TestPublicPayloadValidationRejectsInternalAndUnsafeData(t *testing.T) {
	base := NewPublicEventBase("r1")
	valid := ToolStartedPayload{
		PublicEventBase: base, CallID: "c1", Tool: "write_ppt",
		Display: PublicDisplay{Label: "生成第 3 页"},
	}
	if err := ValidatePublicEvent(EventToolStarted, valid); err != nil {
		t.Fatal(err)
	}
	for _, payload := range []map[string]any{
		{
			"schema_version": 1, "run_id": "r1", "occurred_at": base.OccurredAt,
			"call_id": "c1", "tool": "write_ppt", "display": map[string]any{"label": "生成"},
			"args": map[string]any{"html": "<section />"},
		},
		{
			"schema_version": 1, "run_id": "r1", "occurred_at": base.OccurredAt,
			"message_id": "m1", "text": "安全摘要", "reasoning_content": "hidden",
		},
	} {
		event := EventToolStarted
		if _, ok := payload["message_id"]; ok {
			event = EventMessageReasoning
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

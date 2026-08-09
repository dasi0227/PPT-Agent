package model

import (
	"encoding/json"
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
			"schema_version": 3, "run_id": "r1", "occurred_at": base.OccurredAt,
			"call_id": "c1", "tool": "write_ppt", "display": map[string]any{"label": "生成"},
			"args": map[string]any{"html": "<section />"},
		},
		{
			"schema_version": 3, "run_id": "r1", "occurred_at": base.OccurredAt,
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

func TestRunStartedPayloadUsesV3RunCommandFields(t *testing.T) {
	payload := RunStartedPayload{
		PublicEventBase: NewPublicEventBase("r1"),
		Scope:           RunScope{Artifact: ArtifactPPT, Level: ScopeSlide, SlideID: "s1"},
		Intent:          IntentExecute,
		UserInput:       "revise",
	}
	if err := ValidatePublicEvent(EventRunStarted, payload); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	value := string(raw)
	for _, want := range []string{`"schema_version":3`, `"scope":`, `"artifact":"ppt"`, `"intent":"execute"`} {
		if !strings.Contains(value, want) {
			t.Fatalf("run.started missing %s: %s", want, value)
		}
	}
	for _, legacy := range []string{`"target":`, `"interaction":`, `"presentation"`} {
		if strings.Contains(value, legacy) {
			t.Fatalf("run.started contains legacy field %s: %s", legacy, value)
		}
	}
}

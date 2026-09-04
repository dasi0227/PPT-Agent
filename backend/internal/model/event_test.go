package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublicEventTypeSetContainsAllEvents(t *testing.T) {
	want := []EventType{
		EventRunStarted, EventRunProgress, EventRunCompleted, EventRunFailed, EventRunError, EventRunCanceled,
		EventRunResumed,
		EventPlanUpdated, EventPlanApprovalRequested, EventPlanApprovalAnswered,
		EventCommandPermissionRequested, EventCommandPermissionAnswered, EventRunModeChanged,
		EventMessageReasoning, EventMessageMilestone, EventMessageFinal,
		EventToolStarted, EventToolCompleted, EventQuestionAsked, EventQuestionAnswered,
		EventContextWindowUpdated,
		EventContextCompacted,
	}
	if len(PublicEventTypes) != 22 {
		t.Fatalf("public event count=%d", len(PublicEventTypes))
	}
	for index, event := range want {
		if PublicEventTypes[index] != event {
			t.Fatalf("event[%d]=%s want=%s", index, PublicEventTypes[index], event)
		}
		wantTerminal := event == EventRunCompleted || event == EventRunFailed || event == EventRunError || event == EventRunCanceled
		if event.Terminal() != wantTerminal {
			t.Fatalf("terminal(%s)=%v", event, event.Terminal())
		}
	}
}

func TestPublicPayloadValidationRejectsInternalAndUnsafeData(t *testing.T) {
	base := NewPublicEventBase("r1")
	valid := ToolStartedPayload{
		PublicEventBase: base, CallID: "c1", Tool: "mutate_ppt",
		Display: PublicDisplay{Label: "生成第 3 页"},
	}
	if err := ValidatePublicEvent(EventToolStarted, valid); err != nil {
		t.Fatal(err)
	}
	for _, payload := range []map[string]any{
		{
			"schema_version": 3, "run_id": "r1", "occurred_at": base.OccurredAt,
			"call_id": "c1", "tool": "mutate_ppt", "display": map[string]any{"label": "生成"},
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
		Mode:            ModeExecute,
		UserInput:       "revise",
		Skills: []PublicSkill{{
			ID: "story", Name: "演示叙事", Description: "梳理页面叙事。",
			LocalPath: "/tmp/skills/story/SKILL.md", OpenURL: "vscode://file/tmp/skills/story/SKILL.md",
		}},
		Resources: []PublicLoadedResource{{
			Kind: "component", ID: "feature-card", Name: "能力卡片",
			OpenURL: "vscode://file/tmp/components/feature-card/index.html",
		}},
	}
	if err := ValidatePublicEvent(EventRunStarted, payload); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	value := string(raw)
	for _, want := range []string{`"schema_version":3`, `"scope":`, `"artifact":"ppt"`, `"mode":"execute"`} {
		if !strings.Contains(value, want) {
			t.Fatalf("run.started missing %s: %s", want, value)
		}
	}
	if !strings.Contains(value, `"kind":"component"`) || strings.Contains(value, `"html"`) {
		t.Fatalf("run.started component projection is unsafe: %s", value)
	}
	for _, legacy := range []string{`"target":`, `"interaction":`, `"presentation"`} {
		if strings.Contains(value, legacy) {
			t.Fatalf("run.started contains legacy field %s: %s", legacy, value)
		}
	}
}

func TestRunLifecyclePayloadsValidate(t *testing.T) {
	if err := ValidatePublicEvent(EventRunResumed, RunResumedPayload{
		PublicEventBase: NewPublicEventBase("r1"),
	}); err != nil {
		t.Fatal(err)
	}
	canceled := NewRunTerminalPayload("r1", 10, nil, nil)
	canceled.Reason = RunCancelSuperseded
	if err := ValidatePublicEvent(EventRunCanceled, canceled); err != nil {
		t.Fatal(err)
	}
	canceled.Reason = "unexpected"
	if err := ValidatePublicEvent(EventRunCanceled, canceled); err == nil {
		t.Fatal("invalid cancellation reason was accepted")
	}
}

func TestCommandToolProjectionValidation(t *testing.T) {
	base := NewPublicEventBase("r1")
	started := ToolStartedPayload{
		PublicEventBase: base,
		CallID:          "c1",
		Tool:            "run_command",
		Display:         PublicDisplay{Label: "正在执行命令"},
		Command:         &CommandProjection{Text: "git status --short"},
	}
	if err := ValidatePublicEvent(EventToolStarted, started); err != nil {
		t.Fatal(err)
	}
	exitCode := 0
	duration := int64(12)
	completed := ToolCompletedPayload{
		PublicEventBase: base,
		CallID:          "c1",
		Tool:            "run_command",
		Status:          "completed",
		Display:         PublicDisplay{Label: "已执行命令"},
		Command: &CommandProjection{
			Text:       "git status --short",
			Status:     "completed",
			ExitCode:   &exitCode,
			DurationMS: &duration,
		},
	}
	if err := ValidatePublicEvent(EventToolCompleted, completed); err != nil {
		t.Fatal(err)
	}
	started.Command.Status = "completed"
	if err := ValidatePublicEvent(EventToolStarted, started); err == nil {
		t.Fatal("started command projection accepted terminal fields")
	}
	completed.Command = nil
	if err := ValidatePublicEvent(EventToolCompleted, completed); err == nil {
		t.Fatal("run_command completion accepted without command projection")
	}
}

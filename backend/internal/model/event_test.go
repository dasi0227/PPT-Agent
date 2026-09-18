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
		EventCommandPermissionRequested, EventCommandPermissionAnswered,
		EventScopeExpansionRequested, EventScopeExpansionAnswered, EventScopeUpdated, EventRunModeChanged,
		EventMessageReasoning, EventMessageMilestone, EventMessageFinal,
		EventToolStarted, EventToolCompleted, EventQuestionAsked, EventQuestionAnswered,
		EventContextWindowUpdated,
		EventContextCompacted,
	}
	if len(PublicEventTypes) != 25 {
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
			"schema_version": 5, "run_id": "r1", "occurred_at": base.OccurredAt,
			"call_id": "c1", "tool": "mutate_ppt", "display": map[string]any{"label": "生成"},
			"args": map[string]any{"html": "<section />"},
		},
		{
			"schema_version": 5, "run_id": "r1", "occurred_at": base.OccurredAt,
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

func TestRunStartedPayloadUsesV5RunCommandFields(t *testing.T) {
	payload := RunStartedPayload{
		PublicEventBase: NewPublicEventBase("r1"),
		Scope:           NewRunScope(ScopeObjectPresentation, ScopeCurrentPage, "sli_1"),
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
	for _, want := range []string{`"schema_version":5`, `"scope":`, `"object":"presentation"`, `"mode":"execute"`} {
		if !strings.Contains(value, want) {
			t.Fatalf("run.started missing %s: %s", want, value)
		}
	}
	if !strings.Contains(value, `"kind":"component"`) || strings.Contains(value, `"html"`) {
		t.Fatalf("run.started component projection is unsafe: %s", value)
	}
	for _, legacy := range []string{`"target":`, `"interaction":`, `"artifact":`, `"level":`} {
		if strings.Contains(value, legacy) {
			t.Fatalf("run.started contains legacy field %s: %s", legacy, value)
		}
	}
}

func TestRunProgressAcceptsOnlyStructuredActivity(t *testing.T) {
	base := NewPublicEventBase("r1")
	if err := ValidatePublicEvent(EventRunProgress, RunProgressPayload{
		PublicEventBase: base, Activity: ActivitySlideLayoutChecking,
	}); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePublicEvent(EventRunProgress, map[string]any{
		"schema_version": PublicEventSchemaVersion,
		"run_id":         "r1",
		"occurred_at":    base.OccurredAt,
		"activity":       "unknown",
	}); err == nil {
		t.Fatal("unknown run activity was accepted")
	}
	if err := ValidatePublicEvent(EventRunProgress, map[string]any{
		"schema_version": PublicEventSchemaVersion,
		"run_id":         "r1",
		"occurred_at":    base.OccurredAt,
		"activity":       string(ActivityRunAnalyzing),
		"text":           "legacy progress text",
	}); err == nil {
		t.Fatal("legacy run progress field was accepted")
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

func TestContextWindowStatusOnlyTracksCompaction(t *testing.T) {
	payload := ContextWindowUpdatedPayload{
		PublicEventBase: NewPublicEventBase("r1"),
		Total:           10, Max: 100, Ratio: 0.1, Status: "idle",
		Buckets: map[string]int{
			"system_prompt": 0, "runtime": 0, "chat_history": 0,
			"read_file": 0, "run_command": 0, "other": 10,
		},
		Details: map[string][]ContextWindowBucketDetail{
			"system_prompt": {{Name: "system prompts"}, {Name: "tool definitions"}},
			"runtime":       {{Name: "runtime state"}, {Name: "runtime resources"}, {Name: "runtime messages"}},
			"chat_history":  {{Name: "user messages"}, {Name: "assistant messages"}, {Name: "other tools"}, {Name: "context summary"}},
			"read_file":     {{Name: "read_ppt"}, {Name: "read_image"}, {Name: "read_project"}},
			"run_command":   {{Name: "run_command"}},
			"other":         {{Name: "other", Tokens: 10}},
		},
	}
	if err := ValidatePublicEvent(EventContextWindowUpdated, payload); err != nil {
		t.Fatal(err)
	}
	payload.Status = "compacting"
	if err := ValidatePublicEvent(EventContextWindowUpdated, payload); err != nil {
		t.Fatal(err)
	}
	payload.Status = "running"
	if err := ValidatePublicEvent(EventContextWindowUpdated, payload); err == nil {
		t.Fatal("running context window status was accepted")
	}
	payload.Status = "idle"
	raw, _ := json.Marshal(payload)
	var legacyDetail map[string]any
	_ = json.Unmarshal(raw, &legacyDetail)
	groups := legacyDetail["details"].(map[string]any)
	systemDetails := groups["system_prompt"].([]any)
	systemDetails[0].(map[string]any)["source"] = "legacy"
	if err := ValidatePublicEvent(EventContextWindowUpdated, legacyDetail); err == nil {
		t.Fatal("legacy context window detail source was accepted")
	}
	payload.Total = 110
	payload.Buckets["run_command"] = 100
	payload.Details["run_command"] = []ContextWindowBucketDetail{
		{Name: "ls", Tokens: 60},
		{Name: "rg", Tokens: 30},
		{Name: "other command", Tokens: 10},
	}
	if err := ValidatePublicEvent(EventContextWindowUpdated, payload); err != nil {
		t.Fatalf("dynamic run command details were rejected: %v", err)
	}
	payload.Details["run_command"][1].Tokens = 70
	payload.Buckets["run_command"] = 140
	payload.Total = 150
	if err := ValidatePublicEvent(EventContextWindowUpdated, payload); err == nil {
		t.Fatal("unsorted run command details were accepted")
	}
	payload.Buckets["user_prompt"] = 0
	if err := ValidatePublicEvent(EventContextWindowUpdated, payload); err == nil {
		t.Fatal("legacy context window bucket was accepted")
	}
}

func TestContextCompactedRequiresSafeTitleAndCompleteMetrics(t *testing.T) {
	payload := ContextCompactedPayload{
		PublicEventBase: NewPublicEventBase("r1"),
		Compaction: ContextCompaction{
			ID: "cmp_1", ThreadID: "t1", ProjectID: "p1", RunID: "r1",
			Trigger: ContextCompactionAuto, Title: "收敛上下文协议与前端实现",
			Summary: "## 目标与意图\n继续", BeforeTokens: 56000,
			AfterTokens: 30000, MaxTokens: 65536, Reclaimed: 26000,
			DurationMS: 4200, CreatedAt: 1,
		},
	}
	if err := ValidatePublicEvent(EventContextCompacted, payload); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(payload)
	var data map[string]any
	_ = json.Unmarshal(raw, &data)
	compaction := data["compaction"].(map[string]any)
	for _, invalid := range []any{"", "bad\ntitle", strings.Repeat("长", 49)} {
		compaction["title"] = invalid
		if err := ValidatePublicEvent(EventContextCompacted, data); err == nil {
			t.Fatalf("invalid title accepted: %q", invalid)
		}
	}
	compaction["title"] = "合法标题"
	delete(compaction, "before_tokens")
	if err := ValidatePublicEvent(EventContextCompacted, data); err == nil {
		t.Fatal("missing compaction metrics were accepted")
	}
}

func TestMessageFinalV4RequiresBoundedSuggestionsAndHistoryRevision(t *testing.T) {
	payload := MessageFinalPayload{
		PublicEventBase: NewPublicEventBase("r1"), MessageID: "m1", Text: "完成。",
		AffectedTargets: []PublicTarget{}, SuggestedNextInputs: []string{"继续优化第 2 页"}, ProjectHistoryRevision: 4,
	}
	if err := ValidatePublicEvent(EventMessageFinal, payload); err != nil {
		t.Fatal(err)
	}
	payload.SuggestedNextInputs = []string{"重复", "重复"}
	if err := ValidatePublicEvent(EventMessageFinal, payload); err == nil {
		t.Fatal("duplicate suggestions were accepted")
	}
	payload.SuggestedNextInputs = []string{}
	payload.ProjectHistoryRevision = 0
	if err := ValidatePublicEvent(EventMessageFinal, payload); err == nil {
		t.Fatal("missing history revision was accepted")
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

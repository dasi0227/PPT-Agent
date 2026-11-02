package model

import (
	"encoding/json"
	"testing"
)

func TestPublicReviewResultSeparatesAssessmentFromExecutionStatus(t *testing.T) {
	payload := ToolCompletedPayload{PublicEventBase: NewPublicEventBase("review_run"), CallID: "call_review", Tool: "review_ppt", Status: "completed", Display: PublicDisplay{Label: "已审查演示文稿"}, Review: &ReviewResult{Result: "第 3 页正文被截断，需要修正。"}}
	if err := ValidatePublicEvent(EventToolCompleted, payload); err != nil {
		t.Fatal(err)
	}
	for _, extra := range []map[string]any{
		{"result": "private raw result"},
		{"other": map[string]any{"result": "private nested result"}},
		{"review": map[string]any{"result": "正文", "arguments": map[string]any{}}},
		{"review": map[string]any{"result": map[string]any{"text": "invalid"}}},
	} {
		raw, _ := json.Marshal(payload)
		var fields map[string]any
		_ = json.Unmarshal(raw, &fields)
		for key, value := range extra {
			fields[key] = value
		}
		if err := ValidatePublicEvent(EventToolCompleted, fields); err == nil {
			t.Fatal("review exception allowed a private or malformed result")
		}
	}
	payload.Review.Result = " \n "
	if err := ValidatePublicEvent(EventToolCompleted, payload); err == nil {
		t.Fatal("completed review requires nonblank text")
	}
	payload.Review = nil
	if err := ValidatePublicEvent(EventToolCompleted, payload); err == nil {
		t.Fatal("completed review requires an assessment")
	}
	payload.Status = "failed"
	payload.Error = &PublicError{Code: "REVIEW_FAILED", Message: "未完成成果审查。"}
	if err := ValidatePublicEvent(EventToolCompleted, payload); err != nil {
		t.Fatal(err)
	}
	payload.Review = &ReviewResult{Result: "服务异常。"}
	if err := ValidatePublicEvent(EventToolCompleted, payload); err == nil {
		t.Fatal("execution failure must not carry an assessment")
	}
}

func TestPlanRefusalEventAllowsNoFeedbackAndRejectsRemovedDecisions(t *testing.T) {
	payload := PlanApprovalAnsweredPayload{PublicEventBase: NewPublicEventBase("run"), InteractionID: "interaction", Decision: "approve"}
	if err := ValidatePublicEvent(EventPlanApprovalAnswered, payload); err != nil {
		t.Fatal(err)
	}
	payload.Decision = "refuse"
	if err := ValidatePublicEvent(EventPlanApprovalAnswered, payload); err != nil {
		t.Fatal(err)
	}
	for _, decision := range []string{"revise", "cancel"} {
		payload.Decision = decision
		if err := ValidatePublicEvent(EventPlanApprovalAnswered, payload); err == nil {
			t.Fatal("accepted removed plan decision")
		}
	}
}

func TestPublicPreviewResultAllowsOnlyValidatedProjectionPaths(t *testing.T) {
	for event, field := range map[EventType]string{EventToolCompleted: "preview_assessment", EventPreviewUpdated: "preview"} {
		result := map[string]any{"scores": map[string]any{"content_coverage": 3, "expression_clarity": 2, "requirement_adherence": 3},
			"judges": map[string]any{"mismatch": "none", "issue": "none"}}
		payload := map[string]any{"schema_version": PublicEventSchemaVersion, "run_id": "run", "occurred_at": "2026-11-01T00:00:00Z",
			"call_id": "render", "tool": "render_slide", "status": "completed", "source_run_id": "run", "display": map[string]any{"label": "已渲染"},
			field: map[string]any{"assessment_id": "assessment", "slide_id": "sli_test", "rubric": PreviewRubricVersion, "status": "completed", "result": result}}
		if err := ValidatePublicEvent(event, payload); err != nil {
			t.Fatal(err)
		}
		result["arguments"] = map[string]any{"private": true}
		if err := ValidatePublicEvent(event, payload); err == nil {
			t.Fatal("typed result allowed private nested fields")
		}
	}
}

func TestPublicApprovalEventsPreserveOptionalFeedbackAndRejectWrongTypes(t *testing.T) {
	const feedback = "  原文理由\n修改建议  "
	for event, fields := range map[EventType]map[string]any{
		EventPlanApprovalAnswered:         {"interaction_id": "id", "decision": "refuse"},
		EventCommandPermissionAnswered:    {"interaction_id": "id", "call_id": "call", "command_hash": "hash", "decision": "deny"},
		EventScopeExpansionAnswered:       {"interaction_id": "id", "call_id": "call", "base_revision": 1, "decision": "refuse"},
		EventResourceEditApprovalAnswered: {"interaction_id": "id", "call_id": "call", "resource": "meta", "revision": 1, "decision": "reject"},
	} {
		fields["schema_version"], fields["run_id"], fields["occurred_at"] = PublicEventSchemaVersion, "run", "2026-10-09T10:00:00Z"
		fields["feedback"] = feedback
		if err := ValidatePublicEvent(event, fields); err != nil {
			t.Fatal(err)
		}
		fields["feedback"] = []string{"invalid"}
		if err := ValidatePublicEvent(event, fields); err == nil {
			t.Fatal("invalid feedback accepted")
		}
	}
}

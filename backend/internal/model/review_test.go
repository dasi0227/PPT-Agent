package model

import "testing"

func TestPublicReviewResultSeparatesVerdictFromExecutionStatus(t *testing.T) {
	for _, kind := range []string{"approve", "revise", "refuse"} {
		payload := ToolCompletedPayload{PublicEventBase: NewPublicEventBase("review_run"), CallID: "call_review", Tool: "review_task", Status: "completed", Display: PublicDisplay{Label: "已审查演示文稿"}, Review: &ReviewResult{Decision: kind, Reasons: []string{"第 3 页内容已核对。"}}}
		if err := ValidatePublicEvent(EventToolCompleted, payload); err != nil {
			t.Fatal(err)
		}
		payload.Review.Reasons = nil
		if err := ValidatePublicEvent(EventToolCompleted, payload); err == nil {
			t.Fatal("approval and other verdicts require reasons")
		}
		payload.Review = nil
		if err := ValidatePublicEvent(EventToolCompleted, payload); err == nil {
			t.Fatal("completed review requires a verdict")
		}
		payload.Status = "failed"
		payload.Error = &PublicError{Code: "REVIEW_FAILED", Message: "未完成成果审查。"}
		if err := ValidatePublicEvent(EventToolCompleted, payload); err != nil {
			t.Fatal(err)
		}
		payload.Review = &ReviewResult{Decision: "revise", Reasons: []string{"服务异常。"}}
		if err := ValidatePublicEvent(EventToolCompleted, payload); err == nil {
			t.Fatal("execution failure must not carry an assessment")
		}
	}
}

func TestPlanRefusalEventAllowsNoFeedbackAndRejectsRemovedDecisions(t *testing.T) {
	payload := PlanApprovalAnsweredPayload{PublicEventBase: NewPublicEventBase("run"), InteractionID: "interaction", PlanID: "plan", Decision: "approve"}
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

func TestPublicApprovalEventsPreserveOptionalFeedbackAndRejectWrongTypes(t *testing.T) {
	const feedback = "  原文理由\n修改建议  "
	for event, fields := range map[EventType]map[string]any{
		EventPlanApprovalAnswered:         {"interaction_id": "id", "plan_id": "plan", "decision": "refuse"},
		EventCommandPermissionAnswered:    {"interaction_id": "id", "call_id": "call", "command_hash": "hash", "decision": "deny"},
		EventScopeExpansionAnswered:       {"interaction_id": "id", "call_id": "call", "base_revision": 1, "decision": "refuse"},
		EventResourceEditApprovalAnswered: {"interaction_id": "id", "call_id": "call", "resource": "manifest", "revision": 1, "decision": "reject"},
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

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

func TestPlanRevisionEventAllowsNoFeedbackAndRejectsOldDecision(t *testing.T) {
	payload := PlanApprovalAnsweredPayload{PublicEventBase: NewPublicEventBase("run"), InteractionID: "interaction", PlanID: "plan", Decision: "revise"}
	if err := ValidatePublicEvent(EventPlanApprovalAnswered, payload); err != nil {
		t.Fatal(err)
	}
	payload.Decision = "refuse"
	if err := ValidatePublicEvent(EventPlanApprovalAnswered, payload); err != nil {
		t.Fatal(err)
	}
	payload.Decision = "cancel"
	if err := ValidatePublicEvent(EventPlanApprovalAnswered, payload); err == nil {
		t.Fatal("accepted old approval enum")
	}
}

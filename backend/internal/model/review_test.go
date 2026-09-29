package model

import "testing"

func TestPublicReviewResultSeparatesVerdictFromExecutionStatus(t *testing.T) {
	for _, kind := range []string{"approve", "check", "refuse"} {
		payload := ToolCompletedPayload{PublicEventBase: NewPublicEventBase("review_run"), CallID: "call_review", Tool: "review_task", Status: "completed", Display: PublicDisplay{Label: "已审查 PPT 成果"}, Review: &ReviewResult{Type: kind, Reasons: []string{"第 3 页内容已核对。"}}}
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
		payload.Error = &PublicError{Code: "REVIEW_FAILED", Message: "成果审查未完成。"}
		if err := ValidatePublicEvent(EventToolCompleted, payload); err != nil {
			t.Fatal(err)
		}
		payload.Review = &ReviewResult{Type: "check", Reasons: []string{"服务异常。"}}
		if err := ValidatePublicEvent(EventToolCompleted, payload); err == nil {
			t.Fatal("execution failure must not carry an assessment")
		}
	}
}

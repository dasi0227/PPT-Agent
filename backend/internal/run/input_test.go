package run

import (
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestValidateQuestionAnswer(t *testing.T) {
	question := model.QuestionAskedPayload{
		QuestionID: "q1",
		Questions: []model.QuestionField{
			{
				ID: "type", Title: "选择题型",
				Options: []model.QuestionOption{{ID: "single", Label: "单选"}},
			},
			{
				ID: "icon", Title: "选择图标", AllowCustom: true,
				Options: []model.QuestionOption{{ID: "msg", Label: "MessageCircleQuestion"}},
			},
			{ID: "note", Title: "补充说明", AllowCustom: true},
		},
	}
	answer, display, ok := validateQuestionAnswer(question, `{"answers":[{"question_id":"type","selected_option_id":"single"},{"question_id":"icon","custom_text":"自定义图标"},{"question_id":"note","custom_text":"保持简洁"}]}`)
	if !ok {
		t.Fatal("answer rejected")
	}
	if len(answer.Answers) != 3 || display == "" {
		t.Fatalf("answer=%+v display=%q", answer, display)
	}
}

func TestValidateQuestionAnswerRejectsIncompleteOrInvalidCustom(t *testing.T) {
	question := model.QuestionAskedPayload{
		QuestionID: "q1",
		Questions: []model.QuestionField{{
			ID: "type", Title: "选择题型",
			Options: []model.QuestionOption{{ID: "single", Label: "单选"}},
		}},
	}
	if _, _, ok := validateQuestionAnswer(question, `{"answers":[]}`); ok {
		t.Fatal("accepted incomplete answer")
	}
	if _, _, ok := validateQuestionAnswer(question, `{"answers":[{"question_id":"type","custom_text":"自定义"}]}`); ok {
		t.Fatal("accepted custom answer when allow_custom is false")
	}
	if _, _, ok := validateQuestionAnswer(question, `{"selected_option_ids":[],"answers":[{"question_id":"type","selected_option_id":"a"}]}`); ok {
		t.Fatal("accepted legacy answer fields")
	}
}

func TestPlanApprovalReplyIsIdempotentOnlyForIdenticalSubmission(t *testing.T) {
	queue := NewInputQueue()
	queue.MarkPlanApproval(model.PlanApprovalRequestedPayload{
		InteractionID: "interaction-1",
		Plan:          model.PublicPlan{PlanID: "plan-1", Revision: 3},
	})
	answer := model.PlanApprovalAnswer{
		InteractionID: "interaction-1", PlanID: "plan-1", ExpectedRevision: 3, Decision: "approve",
	}
	if !queue.ReplyPlanApproval(answer) {
		t.Fatal("first approval was rejected")
	}
	if !queue.ReplyPlanApproval(answer) {
		t.Fatal("identical approval replay was not idempotent")
	}
	changed := answer
	changed.Decision = "cancel"
	if queue.ReplyPlanApproval(changed) {
		t.Fatal("conflicting replay was accepted")
	}
}

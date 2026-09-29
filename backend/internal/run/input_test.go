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
		Plan:          model.PublicPlan{PlanID: "plan-1"},
	})
	answer := model.PlanApprovalAnswer{
		InteractionID: "interaction-1", PlanID: "plan-1", Decision: "approve",
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

func TestCommandPermissionReplyRequiresExactMatchAndIsIdempotent(t *testing.T) {
	queue := NewInputQueue()
	queue.MarkCommandPermission(model.CommandPermissionRequestedPayload{
		InteractionID: "command-1",
		CallID:        "call-1",
		Command:       "cat .env",
		CommandHash:   "hash-1",
	})
	answer := model.CommandPermissionAnswer{
		InteractionID: "command-1",
		CallID:        "call-1",
		CommandHash:   "hash-1",
		Decision:      "allow_once",
	}
	mismatch := answer
	mismatch.CommandHash = "hash-2"
	if queue.ReplyCommandPermission(mismatch) {
		t.Fatal("accepted a mismatched command hash")
	}
	invalid := answer
	invalid.Decision = "allow_always"
	if queue.ReplyCommandPermission(invalid) {
		t.Fatal("accepted an unsupported command decision")
	}
	if !queue.ReplyCommandPermission(answer) {
		t.Fatal("first command permission answer was rejected")
	}
	if !queue.ReplyCommandPermission(answer) {
		t.Fatal("identical command permission replay was not idempotent")
	}
	changed := answer
	changed.Decision = "deny"
	if queue.ReplyCommandPermission(changed) {
		t.Fatal("conflicting command permission replay was accepted")
	}
}

func TestReplacementProposalInvalidatesOldApproval(t *testing.T) {
	queue := NewInputQueue()
	queue.MarkPlanApproval(model.PlanApprovalRequestedPayload{InteractionID: "old", Plan: model.PublicPlan{PlanID: "plan"}})
	queue.MarkPlanApproval(model.PlanApprovalRequestedPayload{InteractionID: "new", Plan: model.PublicPlan{PlanID: "plan"}})
	if queue.ReplyPlanApproval(model.PlanApprovalAnswer{InteractionID: "old", PlanID: "plan", Decision: "approve"}) {
		t.Fatal("superseded proposal accepted")
	}
	if !queue.ReplyPlanApproval(model.PlanApprovalAnswer{InteractionID: "new", PlanID: "plan", Decision: "approve"}) {
		t.Fatal("current proposal rejected")
	}
	select {
	case answer := <-queue.PlanApprovalSignal():
		if answer.InteractionID != "new" {
			t.Fatal(answer)
		}
	default:
		t.Fatal("approval not delivered")
	}
}

func TestQuestionSkipIsExplicitAndCustomAnswerPreservesWhitespace(t *testing.T) {
	q := model.QuestionAskedPayload{Questions: []model.QuestionField{{ID: "one", Title: "Choice", Options: []model.QuestionOption{{ID: "a", Label: "A"}}}, {ID: "two", Title: "Text", AllowCustom: true}}}
	got, _, ok := validateQuestionAnswer(q, `{"answers":[{"question_id":"one","skipped":true},{"question_id":"two","custom_text":"  original\n "}]}`)
	if !ok || !got.Answers[0].Skipped || got.Answers[1].CustomText != "  original\n " {
		t.Fatalf("answer=%+v accepted=%v", got, ok)
	}
	if _, _, ok := validateQuestionAnswer(q, `{"answers":[{"question_id":"one","skipped":true,"selected_option_id":"a"},{"question_id":"two","custom_text":"value"}]}`); ok {
		t.Fatal("skip also selected an option")
	}
}

func TestScopeReviseAndPlanRefuseAreNormalPersistedAnswers(t *testing.T) {
	q := NewInputQueue()
	q.MarkScopeExpansion(model.ScopeExpansionRequestedPayload{InteractionID: "scope", CallID: "call", BaseRevision: 1})
	scope := model.ScopeExpansionAnswer{InteractionID: "scope", CallID: "call", BaseRevision: 1, Decision: "revise"}
	if !q.ReplyScopeExpansion(scope) || !q.ReplyScopeExpansion(scope) {
		t.Fatal("revise should not require an adjusted scope payload")
	}
	for _, decision := range []string{"revise", "refuse"} {
		queue := NewInputQueue()
		queue.MarkPlanApproval(model.PlanApprovalRequestedPayload{InteractionID: "plan", Plan: model.PublicPlan{PlanID: "id"}})
		answer := model.PlanApprovalAnswer{InteractionID: "plan", PlanID: "id", Decision: decision}
		if !queue.ReplyPlanApproval(answer) || !queue.ReplyPlanApproval(answer) {
			t.Fatalf("%s without feedback rejected", decision)
		}
	}
}

package sqlite

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
)

func conversationJournal(t *testing.T) (*[]threadjournal.Event, func(string, string, any)) {
	t.Helper()
	events := []threadjournal.Event{}
	return &events, func(runID, kind string, payload any) {
		t.Helper()
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, threadjournal.Event{RunID: runID, Type: kind, Payload: raw})
	}
}

func TestConversationPreservesAllRoundsAndUnfinishedInput(t *testing.T) {
	events, appendEvent := conversationJournal(t)
	for i := 0; i < 7; i++ {
		id := fmt.Sprintf("run-%d", i)
		appendEvent(id, "run.accepted", runAcceptance{ClientRequestID: id, Command: model.RunCommand{Instruction: "请求" + id}})
		for j := 0; j < 3; j++ {
			appendEvent(id, "steering.accepted", model.SteeringMessage{ClientMessageID: fmt.Sprintf("%s-add-%d", id, j), Content: fmt.Sprintf("补充%d", j)})
		}
		appendEvent(id, string(model.EventMessageReply), model.MessageReplyPayload{Text: "收到补充"})
		if i > 0 {
			appendEvent(id, string(model.EventMessageFinal), model.MessageFinalPayload{Text: "完成" + id})
		}
		appendEvent(id, string(model.EventRunCompleted), model.RunTerminalPayload{Reply: &model.InputReply{Text: "完成" + id}})
	}
	appendEvent("unfinished", "run.accepted", runAcceptance{ClientRequestID: "unfinished", Command: model.RunCommand{Instruction: "尚无最终回复"}})
	source, err := conversationContextFromEvents(*events)
	if err != nil || len(source.Rounds) != 8 {
		t.Fatalf("lost original conversation rounds: %+v %v", source, err)
	}
	for i, round := range source.Rounds[:7] {
		if len(round.Messages) != 6 || round.Messages[0].Text != fmt.Sprintf("请求run-%d", i) || round.Messages[4].Text != "收到补充" || round.Messages[5].Text != fmt.Sprintf("完成run-%d", i) {
			t.Fatalf("round was split, reordered or replaced by incomplete input: %+v", round)
		}
	}
	if last := source.Rounds[7].Messages; len(last) != 1 || last[0].Role != "user" || last[0].Text != "尚无最终回复" {
		t.Fatalf("lost unfinished user input: %+v", last)
	}
}

func TestConversationDeduplicatesTransferredInputsAndResetsReplacedPlan(t *testing.T) {
	events, appendEvent := conversationJournal(t)
	appendEvent("old", "run.accepted", runAcceptance{ClientRequestID: "original", Command: model.RunCommand{Instruction: "原始任务"}})
	appendEvent("old", string(model.EventPlanUpdated), model.PlanUpdatedPayload{Plan: model.PublicPlan{Title: "旧计划", Status: "active"}})
	appendEvent("old", "steering.accepted", model.SteeringMessage{ClientMessageID: "transfer", Content: "后续任务"})
	appendEvent("old", string(model.EventMessageFinal), model.MessageFinalPayload{Text: "原始任务完成"})
	appendEvent("new", "run.accepted", runAcceptance{ClientRequestID: "new-request", SourceRunID: "old", SourceMessageIDs: []string{"transfer"}, Command: model.RunCommand{Instruction: "后续任务"}})
	appendEvent("new", string(model.EventRunResumed), map[string]any{})
	appendEvent("new", string(model.EventMessageFinal), model.MessageFinalPayload{Text: "后续任务完成"})
	// A delayed old-plan publication cannot replace the newer Run's state.
	appendEvent("old", string(model.EventPlanUpdated), model.PlanUpdatedPayload{Plan: model.PublicPlan{Title: "旧计划再次发布", Status: "completed"}})
	source, err := conversationContextFromEvents(*events)
	if err != nil || source.Plan != nil || len(source.Rounds) != 2 || len(source.Rounds[0].Messages) != 2 || len(source.Rounds[1].Messages) != 2 {
		t.Fatalf("transfer duplicated input or retained a replaced plan: %+v %v", source, err)
	}
	if source.Rounds[1].Messages[0].Text != "后续任务" {
		t.Fatal("transferred input was lost")
	}
}

func TestConversationExcludesQuestionsApprovalsAndToolTraces(t *testing.T) {
	events, appendEvent := conversationJournal(t)
	appendEvent("r", "run.accepted", runAcceptance{ClientRequestID: "original", Command: model.RunCommand{Instruction: "调整市场分析"}})
	appendEvent("r", string(model.EventQuestionAsked), model.QuestionAskedPayload{QuestionID: "group", Questions: []model.QuestionField{{ID: "q", Question: "这次直接修改还是先讨论？", Options: []model.QuestionOption{{ID: "A", Label: "先讨论", Description: "确认方案后再改"}}}}})
	appendEvent("r", string(model.EventQuestionAnswered), model.QuestionAnsweredPayload{QuestionID: "group", Answer: model.QuestionAnswer{Answers: []model.QuestionFieldAnswer{{QuestionID: "q", SelectedOptionID: "A", CustomText: "先确认图表"}}}})
	plan := model.PublicPlan{PlanID: "private-plan", Title: "完成市场分析", Status: "awaiting_approval", Content: "PRIVATE_PLAN_BODY", Steps: []model.PublicPlanStep{{ID: "private-step", Title: "制作图表", Status: "pending"}}}
	appendEvent("r", string(model.EventPlanApprovalRequested), model.PlanApprovalRequestedPayload{InteractionID: "plan-approval", Plan: plan})
	appendEvent("r", string(model.EventPlanApprovalAnswered), model.PlanApprovalAnsweredPayload{InteractionID: "plan-approval", PlanID: plan.PlanID, Decision: "refuse", Feedback: "先讨论，不写代码"})
	appendEvent("r", string(model.EventCommandPermissionRequested), model.CommandPermissionRequestedPayload{InteractionID: "command-approval", Command: "PRIVATE_SHELL", Stdout: "PRIVATE_LOG", Changes: []model.PublicTarget{{DisplayName: "第 2 页"}}})
	appendEvent("r", string(model.EventCommandPermissionAnswered), model.CommandPermissionAnsweredPayload{InteractionID: "command-approval", Decision: "deny", Feedback: "保留原数据"})
	appendEvent("r", string(model.EventResourceEditApprovalRequested), model.ResourceEditApprovalRequestedPayload{InteractionID: "resource-approval", Target: model.PublicTarget{Part: "design"}})
	appendEvent("r", string(model.EventResourceEditApprovalAnswered), model.ResourceEditApprovalAnsweredPayload{ResourceEditApprovalAnswer: model.ResourceEditApprovalAnswer{InteractionID: "resource-approval", Decision: "approve"}})
	appendEvent("r", string(model.EventScopeExpansionRequested), model.ScopeExpansionRequestedPayload{InteractionID: "scope-approval", AffectedPageCount: 3, Reason: "统一市场分析的三页图表"})
	appendEvent("r", string(model.EventScopeExpansionAnswered), model.ScopeExpansionAnsweredPayload{InteractionID: "scope-approval", Decision: "revise", Feedback: "只修改竞争分析"})
	appendEvent("r", string(model.EventToolCompleted), map[string]any{"tool": "PRIVATE_TOOL", "result": "PRIVATE_RESULT"})
	appendEvent("r", string(model.EventMessageFinal), model.MessageFinalPayload{Text: "已整理方案"})
	source, err := conversationContextFromEvents(*events)
	if err != nil || len(source.Rounds) != 1 || source.Plan == nil || source.Plan.Title != plan.Title {
		t.Fatalf("lost conversation or separate current plan: %+v %v", source, err)
	}
	raw, _ := json.Marshal(source.Rounds)
	text := string(raw)
	if messages := source.Rounds[0].Messages; len(messages) != 2 || messages[0].Text != "调整市场分析" || messages[1].Text != "已整理方案" {
		t.Fatalf("conversation included extra events: %+v", messages)
	}
	for _, forbidden := range []string{"这次直接修改还是先讨论", "先确认图表", "完成市场分析", "先讨论，不写代码", "保留原数据", "只修改竞争分析", "PRIVATE_TOOL", "PRIVATE_RESULT", "PRIVATE_SHELL", "PRIVATE_LOG", "PRIVATE_PLAN_BODY", "private-plan", "private-step", "selected_option_id"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("raw execution data entered history: %s", forbidden)
		}
	}
}

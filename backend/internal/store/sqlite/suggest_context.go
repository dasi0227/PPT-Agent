package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
)

const suggestHistoryRoundLimit = 5

func (s *Store) LoadThreadSuggestContext(ctx context.Context, threadID string) (model.ThreadSuggestContextSource, error) {
	events, err := s.ThreadEvents(ctx, threadID, 0)
	if err != nil {
		return model.ThreadSuggestContextSource{}, err
	}
	return suggestContextFromEvents(events)
}

func suggestContextFromEvents(events []threadjournal.Event) (model.ThreadSuggestContextSource, error) {
	source := model.ThreadSuggestContextSource{}
	// Keep the last acceptance of an input, including inputs transferred to a
	// new Run. This is the same origin rule as naming, without its normal filter.
	selected := map[int]bool{}
	seen := map[string]bool{}
	for i := len(events) - 1; i >= 0; i-- {
		var key string
		switch events[i].Type {
		case "run.accepted":
			var input runAcceptance
			if err := json.Unmarshal(events[i].Payload, &input); err != nil {
				return source, err
			}
			key = namingInputOrigin(input, "initial:"+input.ClientRequestID)
		case "steering.accepted":
			var input model.SteeringMessage
			if err := json.Unmarshal(events[i].Payload, &input); err != nil {
				return source, err
			}
			key = input.ClientMessageID
		default:
			continue
		}
		if !seen[key] {
			selected[i], seen[key] = true, true
		}
	}
	pending := map[string]*model.ThreadSuggestRound{}
	appendMessage := func(runID, role, text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		if pending[runID] == nil {
			pending[runID] = &model.ThreadSuggestRound{}
		}
		pending[runID].Messages = append(pending[runID].Messages, model.ThreadNamingActivity{Role: role, Text: text})
	}
	finish := func(runID, reply string) {
		if strings.TrimSpace(reply) == "" || pending[runID] == nil {
			return
		}
		hasUser := false
		for _, message := range pending[runID].Messages {
			hasUser = hasUser || message.Role == "user"
		}
		if hasUser {
			appendMessage(runID, "assistant", reply)
			source.Rounds = append(source.Rounds, *pending[runID])
			if len(source.Rounds) > suggestHistoryRoundLimit {
				source.Rounds = source.Rounds[len(source.Rounds)-suggestHistoryRoundLimit:]
			}
		}
		delete(pending, runID)
	}
	decisions := suggestDecisionContext{questions: map[string][]model.QuestionField{}, objects: map[string]string{}, answered: map[string]bool{}}
	currentRun := ""
	for i, event := range events {
		switch event.Type {
		case "run.accepted":
			// Plans belong to a Run. A fresh task must not inherit a previous
			// task's plan just because its own history contains no plan event.
			currentRun, source.Plan = event.RunID, nil
			if selected[i] {
				var input runAcceptance
				if err := json.Unmarshal(event.Payload, &input); err != nil {
					return source, err
				}
				appendMessage(event.RunID, "user", suggestUserText(input.Command.Instruction, input.Command.DOMSelections))
			}
		case "steering.accepted":
			if selected[i] {
				var input model.SteeringMessage
				if err := json.Unmarshal(event.Payload, &input); err != nil {
					return source, err
				}
				appendMessage(event.RunID, "user", suggestUserText(input.Content, input.DOMSelections))
			}
		case string(model.EventPlanUpdated), string(model.EventPlanApprovalRequested):
			if event.RunID == currentRun {
				var payload struct {
					Plan model.PublicPlan `json:"plan"`
				}
				if err := json.Unmarshal(event.Payload, &payload); err != nil {
					return source, err
				}
				source.Plan = nil
				if strings.TrimSpace(payload.Plan.Title) != "" {
					source.Plan = &payload.Plan
				}
			}
		case string(model.EventMessageFinal):
			var reply model.MessageFinalPayload
			if err := json.Unmarshal(event.Payload, &reply); err != nil {
				return source, err
			}
			finish(event.RunID, reply.Text)
		case string(model.EventMessageReply):
			var reply model.MessageReplyPayload
			if err := json.Unmarshal(event.Payload, &reply); err != nil {
				return source, err
			}
			// A reply to one addition does not finish the whole business round.
			appendMessage(event.RunID, "assistant", reply.Text)
		case string(model.EventRunCompleted), string(model.EventRunFailed), string(model.EventRunCanceled):
			var terminal model.RunTerminalPayload
			if err := json.Unmarshal(event.Payload, &terminal); err != nil {
				return source, err
			}
			if terminal.Reply != nil {
				finish(event.RunID, terminal.Reply.Text)
			}
		}
		feedback, err := decisions.consume(event)
		if err != nil {
			return source, err
		}
		appendMessage(event.RunID, "user", feedback)
	}
	return source, nil
}

func suggestUserText(text string, selections []model.DOMSelection) string {
	parts := []string{}
	if strings.TrimSpace(text) != "" {
		parts = append(parts, text)
	}
	for _, selection := range selections {
		if strings.TrimSpace(selection.Comment) != "" {
			parts = append(parts, fmt.Sprintf("页面批注 %d：%s", selection.MarkerNo, selection.Comment))
		}
	}
	return strings.Join(parts, "\n")
}

type suggestDecisionContext struct {
	questions map[string][]model.QuestionField
	objects   map[string]string
	answered  map[string]bool
}

func suggestInteractionKey(runID, id string) string { return runID + ":" + id }

func (c *suggestDecisionContext) consume(event threadjournal.Event) (string, error) {
	switch event.Type {
	case string(model.EventQuestionAsked):
		var question model.QuestionAskedPayload
		if err := json.Unmarshal(event.Payload, &question); err != nil {
			return "", err
		}
		c.questions[suggestInteractionKey(event.RunID, question.QuestionID)] = question.Questions
	case string(model.EventQuestionAnswered):
		var answer model.QuestionAnsweredPayload
		if err := json.Unmarshal(event.Payload, &answer); err != nil {
			return "", err
		}
		key := suggestInteractionKey(event.RunID, answer.QuestionID)
		if c.answered[key] {
			return "", nil
		}
		c.answered[key] = true
		parts := []string{}
		for _, field := range answer.Answer.Answers {
			for _, question := range c.questions[key] {
				if question.ID != field.QuestionID {
					continue
				}
				values := []string{}
				for _, option := range question.Options {
					if option.ID == field.SelectedOptionID {
						value := option.Label
						if option.Description != "" {
							value += "（" + option.Description + "）"
						}
						values = append(values, value)
					}
				}
				if field.CustomText != "" {
					values = append(values, field.CustomText)
				}
				if field.Skipped {
					values = append(values, "跳过此问题")
				}
				if len(values) > 0 {
					parts = append(parts, "问题："+question.Question+"\n用户回答："+strings.Join(values, "；"))
				}
			}
		}
		return strings.Join(parts, "\n"), nil
	case string(model.EventPlanApprovalRequested):
		var request model.PlanApprovalRequestedPayload
		if err := json.Unmarshal(event.Payload, &request); err != nil {
			return "", err
		}
		c.objects[suggestInteractionKey(event.RunID, request.InteractionID)] = "计划「" + request.Plan.Title + "」"
	case string(model.EventResourceEditApprovalRequested), string(model.EventResourceEditApprovalUpdated):
		var request model.ResourceEditApprovalRequestedPayload
		if err := json.Unmarshal(event.Payload, &request); err != nil {
			return "", err
		}
		c.objects[suggestInteractionKey(event.RunID, request.InteractionID)] = "修改「" + suggestTargetText(request.Target) + "」"
	case string(model.EventCommandPermissionRequested):
		var request model.CommandPermissionRequestedPayload
		if err := json.Unmarshal(event.Payload, &request); err != nil {
			return "", err
		}
		labels := []string{}
		for _, change := range request.Changes {
			labels = append(labels, suggestTargetText(change))
		}
		c.objects[suggestInteractionKey(event.RunID, request.InteractionID)] = "项目文件修改（" + strings.Join(labels, "、") + "）"
	case string(model.EventScopeExpansionRequested):
		var request model.ScopeExpansionRequestedPayload
		if err := json.Unmarshal(event.Payload, &request); err != nil {
			return "", err
		}
		object := fmt.Sprintf("扩大修改范围（涉及 %d 页）", request.AffectedPageCount)
		if strings.TrimSpace(request.Reason) != "" {
			object += "，申请说明：" + request.Reason
		}
		c.objects[suggestInteractionKey(event.RunID, request.InteractionID)] = object
	case string(model.EventPlanApprovalAnswered), string(model.EventResourceEditApprovalAnswered), string(model.EventCommandPermissionAnswered), string(model.EventScopeExpansionAnswered):
		var answer struct {
			InteractionID string `json:"interaction_id"`
			Decision      string `json:"decision"`
			Feedback      string `json:"feedback"`
			Revision      int64  `json:"revision"`
		}
		if err := json.Unmarshal(event.Payload, &answer); err != nil {
			return "", err
		}
		key := suggestInteractionKey(event.RunID, answer.InteractionID)
		identity := fmt.Sprintf("%s:%s:%d", event.Type, key, answer.Revision)
		if c.answered[identity] {
			return "", nil
		}
		c.answered[identity] = true
		object := c.objects[key]
		if object == "" {
			// Do not turn an unpaired approval into an unexplained yes/no.
			return "", nil
		}
		decision := map[string]string{"approve": "同意", "allow_once": "同意本次操作", "refuse": "拒绝", "reject": "拒绝", "deny": "拒绝", "revise": "要求调整"}[answer.Decision]
		if decision == "" {
			return "", nil
		}
		text := "关于" + object + "：用户" + decision
		if strings.TrimSpace(answer.Feedback) != "" {
			text += "\n用户反馈：" + answer.Feedback
		}
		return text, nil
	}
	return "", nil
}

func suggestTargetText(target model.PublicTarget) string {
	if strings.TrimSpace(target.DisplayName) != "" {
		return target.DisplayName
	}
	if text := map[string]string{"manifest": "内容要求", "outline": "目录结构", "design": "视觉要求", "spec": "页面设计稿", "html": "幻灯片"}[target.Part]; text != "" {
		return text
	}
	return "项目文件"
}

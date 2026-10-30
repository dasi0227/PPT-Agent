package sqlite

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
)

func (s *Store) LoadThreadConversationContext(ctx context.Context, threadID string) (model.ThreadConversationContext, error) {
	events, err := s.ThreadEvents(ctx, threadID, 0)
	if err != nil {
		return model.ThreadConversationContext{}, err
	}
	return conversationContextFromEvents(events)
}

func conversationContextFromEvents(events []threadjournal.Event) (model.ThreadConversationContext, error) {
	source := model.ThreadConversationContext{}
	// A transferred input keeps its identity; retain its latest acceptance.
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

	openRound, openRun := false, ""
	appendMessage := func(runID, role, text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		if !openRound || openRun != runID {
			source.Rounds = append(source.Rounds, model.ConversationRound{})
			openRound, openRun = true, runID
		}
		round := &source.Rounds[len(source.Rounds)-1]
		round.Messages = append(round.Messages, model.ConversationMessage{Role: role, Text: text})
		if role == "assistant" {
			openRound = false
		}
	}
	currentRun := ""
	lastFinal := map[string]string{}
	for i, event := range events {
		switch event.Type {
		case "run.accepted":
			// The current plan belongs to the newest Run, independently of history.
			currentRun, source.Plan = event.RunID, nil
			if selected[i] {
				var input runAcceptance
				if err := json.Unmarshal(event.Payload, &input); err != nil {
					return source, err
				}
				appendMessage(event.RunID, "user", input.Command.Instruction)
			}
		case "steering.accepted":
			if selected[i] {
				var input model.SteeringMessage
				if err := json.Unmarshal(event.Payload, &input); err != nil {
					return source, err
				}
				appendMessage(event.RunID, "user", input.Content)
			}
		case string(model.EventMessageFinal):
			var reply model.MessageFinalPayload
			if err := json.Unmarshal(event.Payload, &reply); err != nil {
				return source, err
			}
			appendMessage(event.RunID, "assistant", reply.Text)
			lastFinal[event.RunID] = reply.Text
		case string(model.EventRunCompleted), string(model.EventRunFailed), string(model.EventRunCanceled):
			var terminal model.RunTerminalPayload
			if err := json.Unmarshal(event.Payload, &terminal); err != nil {
				return source, err
			}
			// A terminal event may carry the final reply without message.final.
			if terminal.Reply != nil && terminal.Reply.Text != lastFinal[event.RunID] {
				appendMessage(event.RunID, "assistant", terminal.Reply.Text)
				lastFinal[event.RunID] = terminal.Reply.Text
			}
			if openRun == event.RunID {
				openRound = false
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
		}
	}
	return source, nil
}

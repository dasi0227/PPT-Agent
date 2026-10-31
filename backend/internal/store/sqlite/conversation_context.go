package sqlite

import (
	"context"
	"encoding/json"
	"errors"
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

// LoadThreadConversationContextBeforeInput validates the durable acceptance and
// projects only its strict event prefix, including any unfinished latest round.
func (s *Store) LoadThreadConversationContextBeforeInput(ctx context.Context, threadID, runID, inputID string) (model.ThreadConversationContext, error) {
	events, err := s.ThreadEvents(ctx, threadID, 0)
	if err != nil {
		return model.ThreadConversationContext{}, err
	}
	prefix, input, err := conversationEventsBeforeInput(events, runID, inputID)
	if err != nil {
		return model.ThreadConversationContext{}, err
	}
	// Deduplicate only after truncation: a later handoff must not suppress an
	// earlier acceptance in the historical view for this input.
	source, err := conversationContextFromEvents(prefix)
	if err == nil {
		source.Input = &input
	}
	return source, err
}

func conversationEventsBeforeInput(events []threadjournal.Event, runID, inputID string) ([]threadjournal.Event, model.ConversationInput, error) {
	accepted := model.ConversationInput{RunID: runID, InputID: inputID}
	for i, event := range events {
		if event.RunID != runID {
			continue
		}
		switch event.Type {
		case "run.accepted":
			var input runAcceptance
			if err := json.Unmarshal(event.Payload, &input); err != nil {
				return nil, accepted, err
			}
			if inputID == "initial:"+input.ClientRequestID {
				// A handoff reaccepts an existing user input. Anchor the snapshot
				// to that original acceptance, so its copy cannot enter history.
				if input.SourceRunID != "" && len(input.SourceMessageIDs) > 0 {
					return conversationEventsBeforeInput(events[:i], input.SourceRunID, input.SourceMessageIDs[0])
				}
				accepted.EventSeq, accepted.Text = event.Seq, input.Command.Instruction
				return events[:i], accepted, nil
			}
		case "steering.accepted":
			var input model.SteeringMessage
			if err := json.Unmarshal(event.Payload, &input); err != nil {
				return nil, accepted, err
			}
			if inputID == input.ClientMessageID {
				accepted.EventSeq, accepted.Text = event.Seq, input.Content
				return events[:i], accepted, nil
			}
		}
	}
	return nil, accepted, errors.New("conversation input is not durably accepted")
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
			if strings.TrimSpace(reply.Text) != "" {
				openRound = false
			}
		case string(model.EventMessageReply):
			var reply model.MessageReplyPayload
			if err := json.Unmarshal(event.Payload, &reply); err != nil {
				return source, err
			}
			// A fixed reply answers one input, not the original task. Keep its
			// unfinished round open so subsequent steering retains that context.
			appendMessage(event.RunID, "assistant", reply.Text)
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

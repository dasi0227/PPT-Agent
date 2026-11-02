package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
	"gorm.io/gorm"
)

func (s *Store) LoadThreadConversationContext(ctx context.Context, threadID string) (model.ThreadConversationContext, error) {
	events, err := s.ThreadEvents(ctx, threadID, 0)
	if err != nil {
		return model.ThreadConversationContext{}, err
	}
	return conversationContextFromEvents(events, nil)
}

// LoadThreadConversationContextBeforeInput validates the durable acceptance and
// projects only its strict event prefix, including any unfinished latest round.
func (s *Store) LoadThreadConversationContextBeforeInput(ctx context.Context, threadID, runID, inputID string) (model.ThreadConversationContext, error) {
	return s.conversationBeforeInput(ctx, threadID, runID, inputID, false)
}

// LoadTaskConversationBeforeInput feeds tool selection only. The intent router
// and the public timeline continue to use the complete conversation.
func (s *Store) LoadTaskConversationBeforeInput(ctx context.Context, threadID, runID, inputID string) (model.ThreadConversationContext, error) {
	return s.conversationBeforeInput(ctx, threadID, runID, inputID, true)
}

type conversationInputKey struct {
	RunID   string
	InputID string
}

func (s *Store) conversationBeforeInput(ctx context.Context, threadID, runID, inputID string, taskOnly bool) (model.ThreadConversationContext, error) {
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
	var excluded map[conversationInputKey]bool
	if taskOnly {
		var rows []conversationInputKey
		err := s.db.WithContext(ctx).Table("idempotency_records AS d").
			Select("d.owner_id AS run_id, d.key AS input_id").
			Joins("JOIN runs ON runs.id = d.owner_id").
			Where("runs.thread_id = ? AND d.scope = 'input_decision' AND d.status = 'completed'", threadID).
			Where("json_extract(d.result_json, '$.disposition') IN ?", []string{model.InputChat, model.InputIrrelevant, model.InputBypass, model.InputDisclosure}).
			Scan(&rows).Error
		if err != nil {
			return model.ThreadConversationContext{}, err
		}
		excluded = make(map[conversationInputKey]bool, len(rows))
		for _, row := range rows {
			excluded[row] = true
		}
		if excluded[conversationInputKey{runID, inputID}] {
			return model.ThreadConversationContext{}, errors.New("tool selection requires a task input")
		}
	}
	source, err := conversationContextFromEvents(prefix, excluded)
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
				cmd := input.Command
				accepted.HasExplicitReferences = len(cmd.Attachments)+len(cmd.DOMSelections)+len(cmd.ReferenceOrder)+len(cmd.Skills)+len(cmd.Components)+len(cmd.MentionedPages)+len(cmd.DroppedMentionedSlideIDs) > 0
				return events[:i], accepted, nil
			}
		case "steering.accepted":
			var input model.SteeringMessage
			if err := json.Unmarshal(event.Payload, &input); err != nil {
				return nil, accepted, err
			}
			if inputID == input.ClientMessageID {
				accepted.EventSeq, accepted.Text = event.Seq, input.Content
				accepted.HasExplicitReferences = len(input.Attachments)+len(input.DOMSelections)+len(input.ReferenceOrder) > 0
				return events[:i], accepted, nil
			}
		}
	}
	return nil, accepted, errors.New("conversation input is not durably accepted")
}

func conversationContextFromEvents(events []threadjournal.Event, excluded map[conversationInputKey]bool) (model.ThreadConversationContext, error) {
	source := model.ThreadConversationContext{}
	// A transferred input keeps its identity; retain its latest acceptance.
	selected := map[int]bool{}
	seen := map[string]bool{}
	for i := len(events) - 1; i >= 0; i-- {
		var key, inputID string
		switch events[i].Type {
		case "run.accepted":
			var input runAcceptance
			if err := json.Unmarshal(events[i].Payload, &input); err != nil {
				return source, err
			}
			inputID = "initial:" + input.ClientRequestID
			key = namingInputOrigin(input, inputID)
		case "steering.accepted":
			var input model.SteeringMessage
			if err := json.Unmarshal(events[i].Payload, &input); err != nil {
				return source, err
			}
			inputID, key = input.ClientMessageID, input.ClientMessageID
		default:
			continue
		}
		if !seen[key] {
			selected[i] = !excluded[conversationInputKey{events[i].RunID, inputID}]
			seen[key] = true
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
	lastFinal := map[string]string{}
	for i, event := range events {
		switch event.Type {
		case "run.accepted":
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
			if id, direct := strings.CutPrefix(reply.MessageID, "input_"); direct && excluded[conversationInputKey{event.RunID, "initial:" + id}] {
				continue
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
			if excluded[conversationInputKey{event.RunID, reply.InputID}] {
				continue
			}
			// Keep the task's round open after replying to an individual input.
			appendMessage(event.RunID, "assistant", reply.Text)
		case string(model.EventRunCompleted), string(model.EventRunFailed), string(model.EventRunCanceled):
			var terminal model.RunTerminalPayload
			if err := json.Unmarshal(event.Payload, &terminal); err != nil {
				return source, err
			}
			// A terminal event may carry the final reply without message.final.
			skipRejection := excluded != nil && terminal.Reason == model.RunCancelInputRejected
			if !skipRejection && terminal.Reply != nil && terminal.Reply.Text != lastFinal[event.RunID] {
				appendMessage(event.RunID, "assistant", terminal.Reply.Text)
				lastFinal[event.RunID] = terminal.Reply.Text
			}
			if openRun == event.RunID {
				openRound = false
			}

		}
	}
	snapshot, err := latestThreadPlan(events)
	source.Plan = snapshot.Plan
	return source, err
}

func latestThreadPlan(events []threadjournal.Event) (model.ThreadPlanSnapshot, error) {
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.Type != string(model.EventPlanUpdated) {
			continue
		}
		// Use the same public projection as history and SSE.
		if model.ValidatePublicEvent(model.EventPlanUpdated, event.Payload) != nil {
			continue
		}
		var payload model.PlanUpdatedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return model.ThreadPlanSnapshot{}, err
		}
		return model.ThreadPlanSnapshot{Plan: payload.Plan, RunID: event.RunID, Seq: event.Seq}, nil
	}
	return model.ThreadPlanSnapshot{}, nil
}

func (s *Store) LatestThreadPlan(ctx context.Context, threadID string) (model.ThreadPlanSnapshot, error) {
	events, err := s.ThreadEvents(ctx, threadID, 0)
	if err != nil {
		return model.ThreadPlanSnapshot{}, err
	}
	return latestThreadPlan(events)
}

// ClearThreadPlan fences the read against both run state and journal writes.
func (s *Store) ClearThreadPlan(ctx context.Context, threadID string) (int64, error) {
	for {
		events, err := s.ThreadEvents(ctx, threadID, 0)
		if err != nil {
			return 0, err
		}
		snapshot, err := latestThreadPlan(events)
		if err != nil {
			return 0, err
		}
		sequence := snapshot.Seq
		var changed bool
		err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := s.checkAcceptance(tx, threadID); err != nil {
				return err
			}
			var thread journalThreadPO
			if err := tx.First(&thread, "id = ?", threadID).Error; err != nil {
				return mapErr(err)
			}
			var busy int64
			if err := tx.Model(&runPO{}).Where("thread_id = ? AND status IN ?", threadID,
				[]model.RunStatus{model.RunPending, model.RunRunning, model.RunWaiting, model.RunRecovering}).Count(&busy).Error; err != nil {
				return err
			}
			if busy > 0 {
				return model.ErrThreadPlanBusy
			}
			if thread.NextEventSeq != int64(len(events))+1 {
				changed = true
				return nil
			}
			if snapshot.Plan == nil {
				return nil
			}
			payload, err := json.Marshal(model.PlanUpdatedPayload{PublicEventBase: model.NewPublicEventBase("")})
			if err != nil {
				return err
			}
			event, err := s.enqueueEvent(tx, threadID, threadjournal.Event{Type: string(model.EventPlanUpdated), Payload: payload})
			sequence = event.Seq
			return err
		})
		if err != nil {
			return 0, err
		}
		if changed {
			continue
		}
		return sequence, s.FlushThreadEvents(ctx, threadID)
	}
}

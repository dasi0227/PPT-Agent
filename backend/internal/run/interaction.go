package run

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type interactionStore interface {
	AcceptInteractionAnswer(context.Context, string, string, string, json.RawMessage) error
	InteractionAnswer(context.Context, string, string, string) (json.RawMessage, error)
}

func (e *Engine) durableInputQueue(runID string) *InputQueue {
	q := NewInputQueue()
	if store, ok := e.store.(interactionStore); ok {
		q.persist = func(kind, id string, answer any) error {
			raw, err := json.Marshal(answer)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return store.AcceptInteractionAnswer(ctx, runID, kind, id, raw)
		}
	}
	return q
}
func (c *checkpoint) replayAnswer(ctx context.Context, kind, id string) error {
	store, ok := c.engine.store.(interactionStore)
	if !ok {
		return nil
	}
	raw, err := store.InteractionAnswer(ctx, c.runID, kind, id)
	if err == ErrRunNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	switch kind {
	case "question":
		var reply AcceptedReply
		if err := json.Unmarshal(raw, &reply); err != nil {
			return err
		}
		content, _ := json.Marshal(reply.Answer)
		if !c.queue.Reply(id, string(content)) {
			return ErrReplyMismatch
		}
	case "plan":
		var answer model.PlanApprovalAnswer
		if err := json.Unmarshal(raw, &answer); err != nil {
			return err
		}
		if !c.queue.ReplyPlanApproval(answer) {
			return ErrReplyMismatch
		}
	case "command":
		var answer model.CommandPermissionAnswer
		if err := json.Unmarshal(raw, &answer); err != nil {
			return err
		}
		if !c.queue.ReplyCommandPermission(answer) {
			return ErrReplyMismatch
		}
	case "scope":
		var answer model.ScopeExpansionAnswer
		if err := json.Unmarshal(raw, &answer); err != nil {
			return err
		}
		if !c.queue.ReplyScopeExpansion(answer) {
			return ErrReplyMismatch
		}
	}
	return nil
}

// Interaction identity is restored from the original event rather than issued
// a second time. Answers already delivered publicly are consumed without replay.
func (c *checkpoint) interactionEventExists(ctx context.Context, eventType model.EventType, id string) (bool, error) {
	events, err := c.engine.store.EventsSince(ctx, c.runID, 0)
	if err != nil {
		return false, err
	}
	for _, event := range events {
		if event.Type != eventType {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			return false, err
		}
		if payload["interaction_id"] == id || payload["question_id"] == id {
			return true, nil
		}
	}
	return false, nil
}
func (c *checkpoint) emitInteraction(ctx context.Context, eventType model.EventType, id string, payload any) error {
	exists, err := c.interactionEventExists(ctx, eventType, id)
	if err != nil || exists {
		return err
	}
	return c.bus.Emit(ctx, eventType, payload)
}

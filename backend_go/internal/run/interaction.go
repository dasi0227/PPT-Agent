package run

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type interactionStore interface {
	AcceptInteractionAnswer(context.Context, string, string, string, json.RawMessage) (sessionstate.Receipt, error)
	Interaction(context.Context, string, string, string) (model.Interaction, error)
}

// Accept before notifying a worker. A paused or restarting run does not need
// an in-memory waiter for the user's answer to become durable.
func (e *Engine) submitInteraction(ctx context.Context, runID, kind, id string, answer any) (sessionstate.Receipt, error) {
	store, ok := e.store.(interactionStore)
	if !ok {
		return sessionstate.Receipt{}, errors.New("durable interaction store is required")
	}
	raw, err := json.Marshal(answer)
	if err != nil {
		return sessionstate.Receipt{}, err
	}
	receipt, err := store.AcceptInteractionAnswer(ctx, runID, kind, id, raw)
	if err != nil {
		return sessionstate.Receipt{}, err
	}
	if active, ok := e.lookup(runID); ok {
		// HTTP disconnection cannot undo acceptance or suppress the wakeup.
		notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		row, notifyErr := store.Interaction(notifyCtx, runID, kind, id)
		if notifyErr == nil {
			notifyErr = active.queue.deliverInteraction(row)
		}
		if notifyErr != nil {
			e.log.Error("accepted interaction could not wake worker", zap.String("run_id", runID), zap.String("interaction_id", id), zap.Error(notifyErr))
			(&inputCheckpoint{store: e.store, runID: runID, active: active}).PersistenceFailed(notifyCtx, "interaction_delivery_failed")
		}
	}
	// The receipt always describes the committed answer; delivery is recoverable
	// from that row and must not turn successful acceptance into an HTTP error.
	return receipt, nil
}

// Requests were saved with the waiting checkpoint. Registering an in-process
// waiter neither writes a second request nor replays historical events.
func (c *checkpoint) interaction(ctx context.Context, kind, id string, request any) (model.Interaction, error) {
	store, ok := c.engine.store.(interactionStore)
	if !ok {
		return model.Interaction{}, errors.New("durable interaction store is required")
	}
	row, err := store.Interaction(ctx, c.runID, kind, id)
	if err != nil {
		return row, err
	}
	if row.State != "pending" && row.State != "answered" {
		return row, ErrReplyMismatch
	}
	if err := json.Unmarshal(row.Request, request); err != nil {
		return row, err
	}
	return row, nil
}

func (c *checkpoint) deliverAcceptedAnswer(ctx context.Context, row model.Interaction) error {
	store, ok := c.engine.store.(interactionStore)
	if !ok {
		return errors.New("durable interaction store is required")
	}
	current, err := store.Interaction(ctx, c.runID, row.Kind, row.ID)
	if err != nil {
		return err
	}
	return c.queue.deliverInteraction(current)
}

func (q *InputQueue) deliverInteraction(row model.Interaction) error {
	if row.State == "pending" || row.State == "applied" {
		return nil
	}
	if row.State != "answered" || len(row.Answer) == 0 {
		return ErrReplyMismatch
	}
	accepted := false
	switch row.Kind {
	case "question":
		var request model.QuestionAskedPayload
		if err := json.Unmarshal(row.Request, &request); err != nil {
			return err
		}
		q.MarkQuestion(request)
		var reply model.QuestionReply
		if err := json.Unmarshal(row.Answer, &reply); err != nil {
			return err
		}
		content, err := json.Marshal(reply.Answer)
		if err != nil {
			return err
		}
		accepted = q.Reply(row.ID, string(content))
	case "plan":
		var request model.PlanApprovalRequestedPayload
		if err := json.Unmarshal(row.Request, &request); err != nil {
			return err
		}
		q.MarkPlanApproval(request)
		var answer model.PlanApprovalAnswer
		if err := json.Unmarshal(row.Answer, &answer); err != nil {
			return err
		}
		accepted = q.ReplyPlanApproval(answer)
	case "command":
		var request model.CommandPermissionRequestedPayload
		if err := json.Unmarshal(row.Request, &request); err != nil {
			return err
		}
		q.MarkCommandPermission(request)
		var answer model.CommandPermissionAnswer
		if err := json.Unmarshal(row.Answer, &answer); err != nil {
			return err
		}
		accepted = q.ReplyCommandPermission(answer)
	case "scope":
		var request model.ScopeExpansionRequestedPayload
		if err := json.Unmarshal(row.Request, &request); err != nil {
			return err
		}
		q.MarkScopeExpansion(request)
		var answer model.ScopeExpansionAnswer
		if err := json.Unmarshal(row.Answer, &answer); err != nil {
			return err
		}
		accepted = q.ReplyScopeExpansion(answer)
	case "resource":
		var request model.ResourceEditApprovalRequestedPayload
		if err := json.Unmarshal(row.Request, &request); err != nil {
			return err
		}
		q.MarkResourceApproval(request)
		var answer model.ResourceEditApprovalAnswer
		if err := json.Unmarshal(row.Answer, &answer); err != nil {
			return err
		}
		accepted = q.ReplyResourceApproval(answer)
	}
	if !accepted {
		return q.ReplyError()
	}
	return nil
}

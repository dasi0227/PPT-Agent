package run

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

// Bus submits current public facts. Lifecycle and idempotency checks belong to
// the database transaction, so a restarted worker never replays display events.
type Bus struct {
	scene     int64
	owner     string
	execution int64
	runID     string
	store     Store
}

func NewBus(current model.Run, store Store) *Bus {
	return &Bus{runID: current.ID, scene: current.SceneRevision, owner: current.OwnerInstanceID, execution: current.ExecutionRevision, store: store}
}

func (b *Bus) Emit(ctx context.Context, kind model.EventType, payload any) error {
	update, err := sessionstate.NewRunUpdate(kind, payload)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrEventProtocol, err)
	}
	ctx = sessionstate.WithRequestScene(ctx, b.scene)
	if b.execution > 0 && kind != model.EventRunUsageUpdated {
		ctx = workflow.WithCheckpointLease(ctx, b.owner, b.execution, 0)
	}
	_, err = b.store.ApplyRunUpdate(ctx, b.runID, update)
	return err
}

func (b *Bus) emitDirectReply(ctx context.Context, decision model.InputDecision, kind model.EventType) error {
	if !decision.RepliesDirectly() {
		return errors.New("direct reply requires a resolved input")
	}
	if decision.ReplyMessageID != "" {
		return nil
	}
	writer, ok := b.store.(InputReplyStore)
	if !ok {
		return errors.New("input reply store is required")
	}
	var payload any
	base := model.NewPublicEventBase(b.runID)
	if kind == model.EventMessageFinal {
		payload = model.MessageFinalPayload{PublicEventBase: base, MessageID: "input_" + strings.TrimPrefix(decision.InputID, "initial:"), Text: decision.Reply(), AffectedTargets: []model.PublicTarget{}}
	} else {
		payload = model.MessageReplyPayload{PublicEventBase: base, MessageID: "input_" + decision.InputID, InputID: decision.InputID, Text: decision.Reply()}
	}
	update, err := sessionstate.NewRunUpdate(kind, payload)
	if err != nil {
		return err
	}
	ctx = sessionstate.WithRequestScene(ctx, b.scene)
	if b.execution > 0 {
		ctx = workflow.WithCheckpointLease(ctx, b.owner, b.execution, 0)
	}
	_, err = writer.CommitInputReply(ctx, b.runID, decision.InputID, update)
	return err
}

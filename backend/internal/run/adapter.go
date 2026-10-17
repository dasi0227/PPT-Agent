package run

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type workflowEmitter struct {
	ctx            context.Context
	bus            *Bus
	cancel         context.CancelFunc
	active         *active
	mu             sync.Mutex
	err            error
	pausePersisted bool
}

func (e *workflowEmitter) Emit(evt model.EventType, payload any) {
	if e.active != nil {
		if evt.Terminal() {
			if stop := e.active.inputGate.resolvedStop(); stop != nil {
				e.active.mu.Lock()
				e.active.inputStop = stop
				e.active.mu.Unlock()
			}
		}
		e.active.mu.Lock()
		paused := e.active.pauseRequested
		inputStopped := e.active.inputStop != nil
		e.active.mu.Unlock()
		if paused {
			return
		}
		if inputStopped && (evt.Terminal() || evt == model.EventMessageFinal) {
			if terminal, ok := payload.(model.RunTerminalPayload); ok {
				e.active.mu.Lock()
				e.active.inputStopTargets = terminal.AffectedTargets
				e.active.mu.Unlock()
			}
			return
		}
		if evt == model.EventRunCanceled {
			if terminal, ok := payload.(model.RunTerminalPayload); ok {
				e.active.mu.Lock()
				reason := e.active.cancelReason
				e.active.mu.Unlock()
				if reason != "" {
					terminal.Reason = reason
					payload = terminal
				}
			}
		}
	}
	ctx := e.ctx
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
	}
	if err := e.bus.Emit(ctx, evt, payload); err != nil {
		e.mu.Lock()
		if e.err == nil {
			e.err = err
		}
		e.mu.Unlock()
		pauseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		_ = e.pauseAfterFailure(pauseCtx)
		cancel()
		if e.cancel != nil {
			e.cancel()
		}
	}
}

func (e *workflowEmitter) Err() error { e.mu.Lock(); defer e.mu.Unlock(); return e.err }

// Retry a failed pause at scheduler exit, but never pause an already paused run.
func (e *workflowEmitter) pauseAfterFailure(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.active == nil || e.pausePersisted || e.err == nil {
		return nil
	}
	e.active.mu.Lock()
	e.active.pauseRequested = true
	e.active.mu.Unlock()
	if lifecycle, ok := e.bus.store.(LifecycleStore); ok {
		reason := eventFailurePauseReason(e.err)
		if _, err := lifecycle.PauseRun(ctx, e.active.run.ID, e.active.run.OwnerInstanceID, reason, time.Now().Unix()); err != nil {
			return err
		}
		e.pausePersisted = true
	}
	return nil
}

func eventFailurePauseReason(err error) string {
	if errors.Is(err, ErrEventProtocol) {
		return "event_protocol_invalid"
	}
	return "journal_write_failed"
}

type Checkpointer interface {
	workflow.SteeringSource
	workflow.LifecycleObserver
	workflow.CheckpointSink
}

type inputCheckpoint struct {
	queue  *InputQueue
	store  Store
	runID  string
	active *active
	state  workflow.RuntimeCheckpoint
}

func (c *inputCheckpoint) DrainInputs(ctx context.Context) ([]workflow.SteeringInput, error) {
	messages, err := c.store.ListPendingSteering(ctx, c.runID)
	if err != nil {
		return nil, err
	}
	for _, message := range messages {
		c.active.inputGate.start(message.ClientMessageID, message.Content)
	}
	if err := c.active.inputGate.wait(ctx); err != nil {
		c.active.mu.Lock()
		stopped := c.active.inputStop != nil
		c.active.mu.Unlock()
		if !stopped && ctx.Err() == nil {
			c.PersistenceFailed(ctx, "input_decision_write_failed")
		}
		return nil, err
	}
	out := make([]workflow.SteeringInput, 0, len(messages))
	for _, message := range messages {
		if c.active.inputGate != nil {
			result := c.active.inputGate.result(message.ClientMessageID)
			if reply := result.Reply(); reply != "" {
				if err := c.active.bus.Emit(ctx, model.EventMessageReply, model.MessageReplyPayload{PublicEventBase: model.NewPublicEventBase(c.runID), MessageID: "input_" + message.ClientMessageID, InputID: message.ClientMessageID, Text: reply}); err != nil {
					return nil, err
				}
				// Consume the durable inbox entry. Its classifier result and reply
				// record distinguish application handling from model injection.
				if err := c.store.MarkSteering(ctx, c.runID, []string{message.ClientMessageID}, model.SteeringInjected, time.Now().UnixNano(), ""); err != nil {
					return nil, err
				}
				continue
			}
		}
		out = append(out, workflow.SteeringInput{
			ID: message.ClientMessageID, Content: message.Content,
			Attachments: message.Attachments, DOMSelections: message.DOMSelections,
			ReferenceOrder: message.ReferenceOrder, Scope: message.Scope, ProjectID: c.active.run.ProjectID,
		})
	}
	return out, nil
}

// The snapshot linearizes the start of a model call against input acceptance.
// If new inputs arrived during preparation, rebuild the outgoing request first.
func (c *inputCheckpoint) AwaitModelBoundary(ctx context.Context) (bool, error) {
	c.active.mu.Lock()
	messages, err := c.store.ListPendingSteering(ctx, c.runID)
	if err == nil {
		for _, message := range messages {
			c.active.inputGate.start(message.ClientMessageID, message.Content)
		}
	}
	c.active.mu.Unlock()
	if err != nil {
		return false, err
	}
	if err := c.active.inputGate.wait(ctx); err != nil {
		return false, err
	}
	return len(messages) > 0, nil
}

func (c *inputCheckpoint) MarkInputsInjected(ctx context.Context, ids []string) error {
	return c.store.MarkSteering(ctx, c.runID, ids, model.SteeringInjected, time.Now().UnixNano(), "")
}

func (c *inputCheckpoint) PhaseChanged(phase workflow.RunPhase) {
	c.active.mu.Lock()
	c.active.phase = phase
	c.active.mu.Unlock()
	if phase == workflow.PhaseWaitingInput && c.active.inputGate != nil {
		go c.respondWhileWaiting()
	}
}

func (c *inputCheckpoint) respondWhileWaiting() {
	// Publish benign replies or stop on rejection without consuming an ask_user
	// answer. Real task inputs stay in the inbox for the next authoring turn.
	ctx := c.active.inputGate.ctx
	if _, err := c.DrainInputs(ctx); err != nil {
		c.active.mu.Lock()
		stopped := c.active.inputStop != nil
		waiting := c.active.phase == workflow.PhaseWaitingInput
		c.active.mu.Unlock()
		if stopped {
			if waiting {
				c.active.cancel()
			}
		} else if ctx.Err() == nil {
			c.PersistenceFailed(ctx, "input_reply_write_failed")
		}
	}
}

func (c *inputCheckpoint) SaveCheckpoint(ctx context.Context, state workflow.RuntimeCheckpoint) error {
	c.state = state
	if store, ok := c.store.(interface {
		SaveCheckpoint(context.Context, workflow.RuntimeCheckpoint) error
	}); ok {
		err := store.SaveCheckpoint(ctx, state)
		if err != nil {
			c.PersistenceFailed(ctx, "checkpoint_write_failed")
		}
		return err
	}
	return nil
}

// PersistenceFailed keeps the last durable checkpoint recoverable and stops the
// worker before it can consume another answer or perform a new side effect.
func (c *inputCheckpoint) PersistenceFailed(ctx context.Context, reason string) {
	c.active.mu.Lock()
	c.active.pauseRequested = true
	c.active.mu.Unlock()
	pauseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if lifecycle, ok := c.store.(LifecycleStore); ok {
		_, _ = lifecycle.PauseRun(pauseCtx, c.runID, c.active.run.OwnerInstanceID, reason, time.Now().Unix())
	}
	c.active.cancel()
}

// Execution is the outer scheduler contract for one canonical ReAct Runtime execution.
type Execution interface {
	Run(ctx context.Context, em workflow.EventEmitter, cp Checkpointer, prompter Prompter) workflow.StructuredOutcome
}

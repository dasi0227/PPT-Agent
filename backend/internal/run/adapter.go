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

func (e *workflowEmitter) PreparePublicUpdate(evt model.EventType, payload any) (any, bool) {
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
			return payload, false
		}
		if inputStopped && (evt.Terminal() || evt == model.EventMessageFinal) {
			if terminal, ok := payload.(model.RunTerminalPayload); ok {
				e.active.mu.Lock()
				e.active.inputStopTargets = terminal.AffectedTargets
				e.active.mu.Unlock()
			}
			return payload, false
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
	return payload, true
}

func (e *workflowEmitter) Emit(evt model.EventType, payload any) {
	var allowed bool
	payload, allowed = e.PreparePublicUpdate(evt, payload)
	if !allowed {
		return
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
		if _, err := lifecycle.PauseRun(ctx, e.active.run.ID, e.active.run.OwnerInstanceID, reason, time.Now().UnixMilli()); err != nil {
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
	return "session_commit_failed"
}

type Checkpointer interface {
	InitialInputIsTask() bool
	workflow.SteeringSource
	workflow.LifecycleObserver
	workflow.CheckpointSink
}

type inputCheckpoint struct {
	store  Store
	runID  string
	active *active
	err    error
}

func (c *inputCheckpoint) InitialInputIsTask() bool {
	gate := c.active.inputGate
	return gate == nil || gate.result(initialInputKey(c.active.run.ClientRequestID)).Disposition == model.InputTask
}

func (c *inputCheckpoint) Err() error {
	if c.active == nil {
		return nil
	}
	c.active.mu.Lock()
	defer c.active.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	if c.active.pauseRequested {
		return ErrRunNotRunning
	}
	return nil
}

func (c *inputCheckpoint) DrainInputs(ctx context.Context) ([]workflow.SteeringInput, error) {
	if err := c.Err(); err != nil {
		return nil, err
	}
	messages, err := c.store.ListPendingSteering(ctx, c.runID)
	if err != nil {
		return nil, err
	}
	for _, message := range messages {
		c.active.inputGate.start(message.ClientMessageID)
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
		if c.active.inputGate != nil && c.active.inputGate.result(message.ClientMessageID).Ignored() {
			continue
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
			c.active.inputGate.start(message.ClientMessageID)
		}
	}
	c.active.mu.Unlock()
	if err != nil {
		return false, err
	}
	if err := c.active.inputGate.wait(ctx); err != nil {
		return false, err
	}
	for _, message := range messages {
		if c.active.inputGate == nil || !c.active.inputGate.result(message.ClientMessageID).Ignored() {
			return true, nil
		}
	}
	return false, nil
}

func (c *inputCheckpoint) PhaseChanged(phase workflow.RunPhase) {
	c.active.mu.Lock()
	c.active.phase = phase
	c.active.mu.Unlock()
}

func (c *inputCheckpoint) SaveCheckpoint(ctx context.Context, state workflow.RuntimeCheckpoint) error {
	if err := c.Err(); err != nil {
		return err
	}
	if store, ok := c.store.(interface {
		SaveCheckpoint(context.Context, workflow.RuntimeCheckpoint) error
	}); ok {
		err := store.SaveCheckpoint(ctx, state)
		if err != nil {
			c.active.mu.Lock()
			if c.err == nil {
				c.err = err
			}
			c.active.mu.Unlock()
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
	if c.active.pauseRequested || (c.active.inputStop != nil && ctx.Err() != nil) {
		c.active.mu.Unlock()
		return
	}
	c.active.pauseRequested = true
	c.active.mu.Unlock()
	pauseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if lifecycle, ok := c.store.(LifecycleStore); ok {
		_, _ = lifecycle.PauseRun(pauseCtx, c.runID, c.active.run.OwnerInstanceID, reason, time.Now().UnixMilli())
	}
	c.active.cancel()
}

// Execution is the outer scheduler contract for one canonical ReAct Runtime execution.
type Execution interface {
	Run(ctx context.Context, em workflow.EventEmitter, cp Checkpointer, prompter Prompter) workflow.StructuredOutcome
}

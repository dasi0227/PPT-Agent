package run

import (
	"context"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type workflowEmitter struct {
	ctx    context.Context
	bus    *Bus
	cancel context.CancelFunc
	active *active
	mu     sync.Mutex
	err    error
}

func (e *workflowEmitter) Emit(evt model.EventType, payload any) {
	if e.active != nil {
		e.active.mu.Lock()
		paused := e.active.pauseRequested
		e.active.mu.Unlock()
		if paused {
			return
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
		if e.active != nil {
			e.active.mu.Lock()
			e.active.pauseRequested = true
			e.active.mu.Unlock()
			if lifecycle, ok := e.bus.store.(LifecycleStore); ok {
				pauseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				_, _ = lifecycle.PauseRun(pauseCtx, e.active.run.ID, e.active.run.OwnerInstanceID, "journal_write_failed", time.Now().Unix())
				cancel()
			}
		}
		if e.cancel != nil {
			e.cancel()
		}
	}
}

func (e *workflowEmitter) Err() error { e.mu.Lock(); defer e.mu.Unlock(); return e.err }

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
	out := make([]workflow.SteeringInput, 0, len(messages))
	for _, message := range messages {
		out = append(out, workflow.SteeringInput{
			ID: message.ClientMessageID, Content: message.Content,
			Attachments: message.Attachments, DOMSelections: message.DOMSelections,
			ReferenceOrder: message.ReferenceOrder, Scope: message.Scope, ProjectID: c.active.run.ProjectID,
		})
	}
	return out, nil
}

func (c *inputCheckpoint) MarkInputsInjected(ctx context.Context, ids []string) error {
	return c.store.MarkSteering(ctx, c.runID, ids, model.SteeringInjected, time.Now().UnixNano(), "")
}

func (c *inputCheckpoint) PhaseChanged(phase workflow.RunPhase) {
	c.active.mu.Lock()
	c.active.phase = phase
	c.active.mu.Unlock()
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

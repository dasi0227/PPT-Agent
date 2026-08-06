package run

import (
	"context"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type workflowEmitter struct {
	ctx context.Context
	bus *Bus
}

func (e *workflowEmitter) Emit(evt model.EventType, payload any) {
	_ = e.bus.Emit(e.ctx, evt, payload)
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
	out := make([]workflow.SteeringInput, 0, len(messages))
	for _, message := range messages {
		out = append(out, workflow.SteeringInput{ID: message.ClientMessageID, Content: message.Content})
	}
	return out, nil
}

func (c *inputCheckpoint) MarkInputsInjected(ctx context.Context, ids []string) error {
	return c.store.MarkSteering(ctx, c.runID, ids, model.SteeringInjected, time.Now().UnixNano(), "")
}

func (c *inputCheckpoint) PhaseChanged(phase workflow.RuntimePhase) {
	c.active.mu.Lock()
	c.active.phase = phase
	c.active.mu.Unlock()
}

func (c *inputCheckpoint) SaveCheckpoint(ctx context.Context, state workflow.RuntimeCheckpoint) error {
	c.state = state
	if store, ok := c.store.(interface {
		SaveCheckpoint(context.Context, workflow.RuntimeCheckpoint) error
	}); ok {
		return store.SaveCheckpoint(ctx, state)
	}
	return nil
}

// Execution is the outer scheduler contract for one canonical ReAct Runtime execution.
type Execution interface {
	Run(ctx context.Context, em workflow.EventEmitter, cp Checkpointer, prompter Prompter) workflow.StructuredOutcome
}

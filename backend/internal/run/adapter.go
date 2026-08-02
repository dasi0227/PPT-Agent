package run

import (
	"context"

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
	DrainInputs() []string
	workflow.CheckpointSink
}

type inputCheckpoint struct {
	queue *InputQueue
	state workflow.RuntimeCheckpoint
}

func (c *inputCheckpoint) DrainInputs() []string {
	return c.queue.Drain()
}

func (c *inputCheckpoint) SaveCheckpoint(_ context.Context, state workflow.RuntimeCheckpoint) error {
	c.state = state
	return nil
}

// Execution is the outer scheduler contract for one canonical ReAct Runtime execution.
type Execution interface {
	Run(ctx context.Context, em workflow.EventEmitter, cp Checkpointer, prompter Prompter) workflow.StructuredOutcome
}

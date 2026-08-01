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
}

type inputCheckpoint struct {
	queue *InputQueue
}

func (c *inputCheckpoint) DrainInputs() []string {
	return c.queue.Drain()
}

// Execution is the outer scheduler contract for one canonical Adaptive Runtime execution.
type Execution interface {
	Run(ctx context.Context, em workflow.EventEmitter, cp Checkpointer, prompter Prompter) workflow.StructuredOutcome
}

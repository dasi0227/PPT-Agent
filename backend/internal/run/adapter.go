package run

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// harnessEmitter 把 harness 领域事件桥接到 Bus（转 SSE）。
type harnessEmitter struct {
	ctx context.Context
	bus *Bus
}

func (e *harnessEmitter) Emit(evt model.EventType, payload any) {
	_ = e.bus.Emit(e.ctx, evt, payload)
}

// harnessCheckpoint 把 harness checkpoint 桥接到控制输入队列（HITL 排空）。
type harnessCheckpoint struct {
	queue *InputQueue
}

func (c *harnessCheckpoint) DrainInputs() []string {
	return c.queue.Drain()
}

// Runner 抽象「一次 harness 执行」。engine 只认识这个接口，
// 具体如何构造 harness.Loop（含工具/prompt）由 agent 层通过 HarnessFactory 提供。
// Prompter 供需要被动应答（needs_input）的 runner 使用；不需要可忽略。
type Runner interface {
	Run(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, prompter Prompter) harness.Outcome
}

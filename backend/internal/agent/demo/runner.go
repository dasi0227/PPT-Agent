// Package demo 提供一个最小可运行的 agent runner，用于 M1 端到端打通
// 「一次工具化 ReAct 循环 + SSE + HITL」（AC-GLOBAL-001/002），不含真实业务逻辑。
// 真实的 outline/generate/edit agent 在 M2+ 落地。
package demo

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

// Runner 用给定 LLM client 跑一次 harness 循环。满足 run.Runner。
type Runner struct {
	client llm.Client
	scope  model.Scope
	mode   model.Mode
	runID  string
	instr  string
	extra  []tools.Tool
}

// New 构造 demo runner。extra 可注入额外工具（如测试用 demo 工具）；默认仅 finish。
func New(client llm.Client, runID string, scope model.Scope, mode model.Mode, instruction string, extra ...tools.Tool) *Runner {
	return &Runner{client: client, runID: runID, scope: scope, mode: mode, instr: instruction, extra: extra}
}

func (r *Runner) Run(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, _ run.Prompter) harness.Outcome {
	all := append([]tools.Tool{tools.NewFinishTool()}, r.extra...)
	loop := harness.New(r.client, harness.Config{
		RunID:        r.runID,
		Kind:         model.KindCommand,
		Scope:        r.scope,
		Mode:         r.mode,
		SystemPrompt: "You are a PPT agent. Use tools to accomplish the task, then call finish.",
		Instruction:  r.instr,
		Tools:        all,
	})
	return loop.Run(ctx, em, cp)
}

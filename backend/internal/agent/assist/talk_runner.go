package assist

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

// TalkRunner 实现 /talk（mode=talk）：只输出分析/思考，MUST NOT 产 artifact/改文件
// （SPEC-CMD-TALK-001/003）。复用 harness 主循环，但 Mode=talk 使 Gate 剥离所有写工具，
// LLM 只能输出文本（→ info 事件）或 finish。满足 run.Runner。
type TalkRunner struct {
	client      llm.Client
	runID       string
	instruction string
}

func NewTalkRunner(client llm.Client, runID, instruction string) *TalkRunner {
	return &TalkRunner{client: client, runID: runID, instruction: instruction}
}

func (r *TalkRunner) Run(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, _ run.Prompter) harness.Outcome {
	// 即使传入编辑工具，Mode=talk 的 Gate 只保留 ClassControl（finish），机制级保证零产物。
	loop := harness.New(r.client, harness.Config{
		RunID:        r.runID,
		Kind:         model.KindCommand,
		Scope:        model.ScopeCurrent,
		Mode:         model.ModeTalk,
		SystemPrompt: prompt.TalkSystem(),
		Instruction:  r.instruction,
		Tools:        []tools.Tool{tools.NewFinishTool()},
	})
	return loop.Run(ctx, em, cp)
}

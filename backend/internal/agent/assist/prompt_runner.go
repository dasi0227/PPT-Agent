// Package assist 提供命令/模式类运行器：/prompt（改写输入）、/recap（只读汇总）、
// /talk（只说不做）、/ask（必要时提问）。前三者零产物；/ask 复用 Prompter 发 needs_input。
// 均满足 run.Runner，由 service 工厂按 command/mode 选择。
package assist

import (
	"context"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

// PromptRunner 实现 /prompt：让 LLM 改写/优化用户输入，只发 info 事件，零产物、不执行。
// （SPEC-CMD-PROMPT-001/004）满足 run.Runner。
type PromptRunner struct {
	client      llm.Client
	runID       string
	instruction string
}

func NewPromptRunner(client llm.Client, runID, instruction string) *PromptRunner {
	return &PromptRunner{client: client, runID: runID, instruction: instruction}
}

func (r *PromptRunner) Run(ctx context.Context, em harness.Emitter, _ harness.Checkpointer, _ run.Prompter) harness.Outcome {
	em.Emit(model.EventRunStarted, harness.RunStartedPayload{
		RunID: r.runID, Kind: string(model.KindCommand), Scope: string(model.ScopeCurrent), Mode: string(model.ModeNormal),
		UserInput: r.instruction,
	})

	// 仅做文本补全（不给任何工具）：改写后的指令文本。MUST NOT 执行改写结果（ARCH-CMD PROMPT-004）。
	resp, err := r.client.Chat(ctx, llm.ChatRequest{Messages: []llm.Message{
		{Role: llm.RoleSystem, Content: prompt.PromptRewriteSystem()},
		{Role: llm.RoleUser, Content: strings.TrimSpace(r.instruction)},
	}})
	if err != nil {
		if ctx.Err() != nil {
			return harness.Outcome{Status: harness.OutcomeCanceled, Code: harness.CodeCanceled, Message: "run canceled"}
		}
		em.Emit(model.EventError, harness.ErrorPayload{Code: harness.CodeLLMBadCall, Message: err.Error()})
		return harness.Outcome{Status: harness.OutcomeLLMError, Code: harness.CodeLLMBadCall, Message: err.Error()}
	}

	// 只发 info（改写后的文本）；不产 artifact、不落文件。
	em.Emit(model.EventInfo, harness.InfoPayload{Text: resp.Content})
	return harness.Outcome{Status: harness.OutcomeFinished, Summary: resp.Content}
}

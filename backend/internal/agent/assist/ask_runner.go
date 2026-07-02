package assist

import (
	"context"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/edit"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

// AskParams 是 /ask（mode=ask）执行所需输入。scope 归一为 current/page（单页编辑）。
type AskParams struct {
	RunID       string
	ProjectID   string
	WorkDir     string
	Scope       model.Scope
	PageIndex   int
	Instruction string
}

// EditStore 是 /ask 委派给 edit.Runner 执行时所需的落库能力。
type EditStore = edit.Store

// AskRunner 实现 /ask（mode=ask）：先判定是否存在不确定点，有则 needs_input 提问并 waiting
// （SPEC-CMD-ASK-001）；无歧义则直接执行（SPEC-CMD-ASK-002）。收到应答后并入指令继续执行。
// 执行阶段委派给 M4 的 edit.Runner（单页锚定编辑），不重复造轮子。满足 run.Runner。
type AskRunner struct {
	client llm.Client
	store  EditStore
	params AskParams
	clock  func() int64
	newID  func() string
}

func NewAskRunner(client llm.Client, store EditStore, p AskParams, clock func() int64, newID func() string) *AskRunner {
	return &AskRunner{client: client, store: store, params: p, clock: clock, newID: newID}
}

func (r *AskRunner) Run(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, prompter run.Prompter) harness.Outcome {
	instruction := r.params.Instruction

	// 澄清判定：让 LLM 就本次任务判断是否有影响产出的关键不确定点。
	// 用 CallTool 提供两个控制工具：ask_user（有疑问）/ proceed（无疑问）。
	decision, err := r.client.CallTool(ctx, llm.ToolCallRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: prompt.AskDecisionSystem()},
			{Role: llm.RoleUser, Content: strings.TrimSpace(instruction)},
		},
		Tools: []llm.ToolSchema{askUserSchema(), proceedSchema()},
	})
	if err != nil {
		if ctx.Err() != nil {
			return harness.Outcome{Status: harness.OutcomeCanceled, Code: harness.CodeCanceled, Message: "run canceled"}
		}
		em.Emit(model.EventError, harness.ErrorPayload{Code: harness.CodeLLMBadCall, Message: err.Error()})
		return harness.Outcome{Status: harness.OutcomeLLMError, Code: harness.CodeLLMBadCall, Message: err.Error()}
	}

	// 有不确定点：发 needs_input 并阻塞等待（SPEC-CMD-ASK-001）。
	if decision.ToolCall != nil && decision.ToolCall.Name == "ask_user" && prompter != nil {
		question, _ := decision.ToolCall.Args["question"].(string)
		choices := toStrings(decision.ToolCall.Args["choices"])
		answer, err := prompter.NeedsInput(ctx, r.params.RunID+"-ask", question, choices)
		if err != nil {
			if ctx.Err() != nil {
				return harness.Outcome{Status: harness.OutcomeCanceled, Code: harness.CodeCanceled, Message: "run canceled"}
			}
			em.Emit(model.EventError, harness.ErrorPayload{Code: harness.CodeLLMBadCall, Message: err.Error()})
			return harness.Outcome{Status: harness.OutcomeLLMError, Code: harness.CodeLLMBadCall, Message: err.Error()}
		}
		// 应答并入指令继续（SPEC-CMD-ASK-004：据此继续，不再就同一点重复提问）。
		instruction = instruction + "\n\n[用户澄清] " + strings.TrimSpace(answer)
	}

	// 执行阶段委派 M4 edit.Runner（单页锚定编辑）。Mode 只作用本次 Run（AGENT-MODE-005）。
	editRunner := edit.NewRunner(r.client, r.store, edit.Params{
		RunID:       r.params.RunID,
		ProjectID:   r.params.ProjectID,
		WorkDir:     r.params.WorkDir,
		Scope:       r.params.Scope,
		PageIndex:   r.params.PageIndex,
		Instruction: instruction,
	}, r.clock, r.newID)
	return editRunner.Run(ctx, em, cp, prompter)
}

func askUserSchema() llm.ToolSchema {
	return llm.ToolSchema{
		Name:        "ask_user",
		Description: "当存在影响产出的关键不确定点时，向用户提问澄清（会暂停等待用户回答）。",
		Parameters: map[string]any{
			"type":     "object",
			"required": []any{"question"},
			"properties": map[string]any{
				"question": map[string]any{"type": "string", "description": "具体、可回答的问题"},
				"choices":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "候选项（可选）"},
			},
		},
	}
}

func proceedSchema() llm.ToolSchema {
	return llm.ToolSchema{
		Name:        "proceed",
		Description: "当没有任何影响产出的不确定点时，直接开始执行，不提问。",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
	}
}

func toStrings(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

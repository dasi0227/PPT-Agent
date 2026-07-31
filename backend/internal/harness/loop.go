package harness

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Loop 是 ReAct 主循环引擎。业务无关：thought→tool_call→observation→…→finish。
type Loop struct {
	client llm.Client
	cfg    Config
	gated  []tools.Tool
	byName map[string]tools.Tool
}

// New 构造 Loop：在此处一次性完成动态工具门控（ARCH-HARNESS-001）。
func New(client llm.Client, cfg Config) *Loop {
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = defaultMaxTurns
	}
	gated := Gate(cfg.Tools, cfg.Scope, cfg.Mode)
	byName := make(map[string]tools.Tool, len(gated))
	for _, t := range gated {
		byName[t.Name()] = t
	}
	return &Loop{client: client, cfg: cfg, gated: gated, byName: byName}
}

// GatedTools 暴露门控后的工具集（供测试断言 scope 隔离 AC-HARNESS-001）。
func (l *Loop) GatedTools() []tools.Tool { return l.gated }

// schemas 投影门控后工具为 function schema（只有这些能被 LLM 看到/调用）。
func (l *Loop) schemas() []llm.ToolSchema {
	out := make([]llm.ToolSchema, 0, len(l.gated))
	for _, t := range l.gated {
		out = append(out, tools.Schema(t))
	}
	return out
}

// Run 执行 ReAct 循环，经 emitter 投影事件，返回 Outcome（外壳据此发唯一终态事件）。
func (l *Loop) Run(ctx context.Context, em Emitter, cp Checkpointer) Outcome {
	em.Emit(model.EventRunStarted, RunStartedPayload{
		RunID:     l.cfg.RunID,
		UserInput: l.cfg.Instruction,
	})

	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: l.cfg.SystemPrompt},
		{Role: llm.RoleUser, Content: l.cfg.Instruction},
	}

	consecutiveFailures := 0
	callSeq := 0

	for turn := 1; turn <= l.cfg.MaxTurns; turn++ {
		if ctx.Err() != nil {
			return Outcome{Status: OutcomeCanceled, Code: CodeCanceled, Message: "run canceled", Turns: turn - 1}
		}

		// 每轮进度：ReAct 轮次的诚实可观测投影（非业务分页）。
		em.Emit(model.EventProgress, ProgressPayload{Stage: "turn", Current: turn, Total: l.cfg.MaxTurns})

		resp, err := l.client.CallTool(ctx, llm.ToolCallRequest{Messages: msgs, Tools: l.schemas()})
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return Outcome{Status: OutcomeCanceled, Code: CodeCanceled, Message: "run canceled", Turns: turn - 1}
			}
			// LLM 整体超时（Client.Timeout / DeadlineExceeded）单独识别为 LLM_TIMEOUT，
			// 便于前端展示"可重试/调低复杂度"的友好提示，而非泛化的 LLM_BAD_REQUEST。
			if isLLMTimeout(err) {
				return Outcome{Status: OutcomeLLMError, Code: CodeLLMTimeout, Message: err.Error(), Turns: turn}
			}
			// function call 无法解析等 → 最小失败退出，不重试死循环（ARCH-LLM-FC-001）。
			return Outcome{Status: OutcomeLLMError, Code: CodeLLMBadCall, Message: err.Error(), Turns: turn}
		}

		if resp.Thought != "" {
			em.Emit(model.EventThought, ThoughtPayload{Text: resp.Thought})
		}

		// 无工具调用：LLM 返回纯文本（talk/prompt 场景），发 info 并结束。
		if resp.ToolCall == nil {
			if resp.Text != "" {
				em.Emit(model.EventInfo, InfoPayload{Text: resp.Text})
			}
			return Outcome{Status: OutcomeFinished, Summary: resp.Text, Turns: turn}
		}

		callSeq++
		tc := resp.ToolCall
		callID := tc.ID
		if callID == "" {
			callID = fmt.Sprintf("call_%d", callSeq)
		}
		tc.ID = callID
		em.Emit(model.EventToolCall, ToolCallPayload{Tool: tc.Name, Args: tc.Args, CallID: callID})
		msgs = appendToolCall(msgs, *tc, resp.Thought)

		tool, ok := l.byName[tc.Name]
		if !ok {
			// LLM 选了未注册（越权/幻觉）工具：作为一次失败 observation 回灌，不崩溃。
			obs := fmt.Sprintf("工具 %q 不可用（未在当前 scope/mode 注册）", tc.Name)
			em.Emit(model.EventToolResult, ToolResultPayload{CallID: callID, OK: false, Observation: obs})
			msgs = appendObservation(msgs, callID, obs)
			consecutiveFailures++
			if consecutiveFailures >= circuitBreakerFailures {
				return Outcome{Status: OutcomeCircuit, Code: CodeCircuit, Message: "consecutive tool failures", Turns: turn}
			}
			continue
		}

		res, execErr := tool.Execute(ctx, tc.Args)
		if execErr != nil {
			return Outcome{Status: OutcomeLLMError, Code: CodeLLMBadCall, Message: execErr.Error(), Turns: turn}
		}

		if res.IsFinish {
			return Outcome{Status: OutcomeFinished, Summary: res.Summary, Turns: turn}
		}

		em.Emit(model.EventToolResult, ToolResultPayload{CallID: callID, OK: res.OK, Observation: res.Observation})
		if res.Artifact != nil {
			em.Emit(model.EventArtifact, ArtifactPayload{ArtifactType: res.Artifact.Type, Ref: res.Artifact.Ref})
		}

		if res.OK {
			consecutiveFailures = 0
		} else {
			consecutiveFailures++
			if consecutiveFailures >= circuitBreakerFailures {
				return Outcome{Status: OutcomeCircuit, Code: CodeCircuit, Message: "consecutive tool failures", Turns: turn}
			}
		}

		msgs = appendObservation(msgs, callID, res.Observation)

		// checkpoint：排空控制输入队列，纳入后续上下文（不打断已完成的本轮调用）。
		if cp != nil {
			for _, in := range cp.DrainInputs() {
				msgs = append(msgs, llm.Message{Role: llm.RoleUser, Content: in})
			}
		}

	}

	return Outcome{Status: OutcomeMaxTurns, Code: CodeMaxTurns, Message: "max turns exceeded", Turns: l.cfg.MaxTurns}
}

func appendToolCall(msgs []llm.Message, tc llm.ToolCall, thought string) []llm.Message {
	return append(msgs, llm.Message{
		Role:      llm.RoleAssistant,
		Content:   thought,
		ToolCalls: []llm.ToolCall{tc},
	})
}

func appendObservation(msgs []llm.Message, callID, obs string) []llm.Message {
	return append(msgs, llm.Message{Role: llm.RoleTool, Content: obs, ToolCallID: callID})
}

// isLLMTimeout 识别 LLM 客户端的整体超时错误。net/http 触发 Client.Timeout 时
// 未必 wrap DeadlineExceeded，字面量兜底以匹配现场（"Client.Timeout" / "deadline exceeded"）。
// ctx.Canceled 已在调用点提前 return，不会走到这里。
func isLLMTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := err.Error()
	if strings.Contains(msg, "Client.Timeout") {
		return true
	}
	if strings.Contains(msg, "deadline exceeded") {
		return true
	}
	return false
}

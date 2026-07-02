package harness

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Emitter 是 harness 向外投影可观测事件的出口（由 Run 外壳实现，转为 SSE）。
// harness 不认识 SSE/HTTP，只发领域事件（ARCH-HARNESS-003）。
type Emitter interface {
	Emit(evt model.EventType, payload any)
}

// Checkpointer 在 ReAct 每轮结束提供 HITL 挂钩（ARCH-RUN-002）：
// DrainInputs 排空控制输入队列并返回已入队的用户消息（纳入后续上下文，不打断进行中的调用）。
type Checkpointer interface {
	DrainInputs() []string
}

// Config 是构造一次 harness 执行的配置（由 agent 层按 scope/mode 装配）。
type Config struct {
	RunID    string
	Kind     model.Kind
	Scope    model.Scope
	Mode     model.Mode
	MaxTurns int
	// SystemPrompt 承载系统级约束（system 消息，用户输入不得覆盖 ARCH-LLM-005）。
	SystemPrompt string
	// Instruction 是用户指令（user 消息）。
	Instruction string
	// Tools 是本次 Run 可用工具全集，门控前的候选（gate 按 scope/mode 再裁剪）。
	Tools []tools.Tool
}

const defaultMaxTurns = 12

// 连续工具失败熔断阈值（ARCH-HARNESS-STOP-003）。
const circuitBreakerFailures = 3

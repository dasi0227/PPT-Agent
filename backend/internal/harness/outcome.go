package harness

// OutcomeStatus 是 harness 循环的退出原因（Run 外壳据此发唯一终态事件）。
type OutcomeStatus string

const (
	OutcomeFinished OutcomeStatus = "finished"  // finish 工具显式退出（ARCH-HARNESS-STOP-002）
	OutcomeMaxTurns OutcomeStatus = "max_turns" // 达到 MAX_TURNS（ARCH-HARNESS-STOP-001）
	OutcomeCircuit  OutcomeStatus = "circuit"   // 连续失败熔断（ARCH-HARNESS-STOP-003）
	OutcomeCanceled OutcomeStatus = "canceled"  // ctx 取消（ARCH-HARNESS-STOP-004）
	OutcomeLLMError OutcomeStatus = "llm_error" // function call 无法解析等（ARCH-LLM-FC-001）
)

// 停止条件对应的错误码（映射到 SSE error 事件 code）。
const (
	CodeMaxTurns   = "MAX_TURNS_EXCEEDED"
	CodeCircuit    = "TOOL_FAILURES_EXCEEDED"
	CodeLLMBadCall = "LLM_BAD_REQUEST"
	CodeCanceled   = "CANCELED"
)

// Outcome 是循环结束的结果。
type Outcome struct {
	Status  OutcomeStatus
	Summary string // finish 的总结
	Code    string // 非正常退出的错误码
	Message string // 非正常退出的可读消息
	Turns   int    // 实际执行轮数
	// Result 是结构化交付载体（V2-M5）：非空时 done 事件用它作 result；空则回退 {summary}。
	Result map[string]any
}

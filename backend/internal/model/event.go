package model

// EventType 是 SSE 事件类型（API-SSE）。
type EventType string

const (
	EventRunStarted EventType = "run.started"
	EventThought    EventType = "thought"
	EventToolCall   EventType = "tool_call"
	EventToolResult EventType = "tool_result"
	EventProgress   EventType = "progress"
	EventToken      EventType = "token"
	EventArtifact   EventType = "artifact"
	EventPlan       EventType = "plan"
	EventPlanUpdate EventType = "plan.update"
	EventNeedsInput EventType = "needs_input"
	EventInfo       EventType = "info"
	EventDone       EventType = "done"
	EventError      EventType = "error"
)

// Terminal 报告事件是否为终态事件（done/error，API-SSE-002）。
func (t EventType) Terminal() bool {
	return t == EventDone || t == EventError
}

// Event 是一条持久化的 Run 事件。payload 为 JSON 文本。
// seq 单调递增、连续，用于 Last-Event-ID 续传（API-SSE-001/003）。
type Event struct {
	RunID     string
	Seq       int64
	Type      EventType
	Payload   string
	CreatedAt int64
}

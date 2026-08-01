package model

// EventType 是 SSE 事件类型（API-SSE）。
type EventType string

const (
	EventRunStarted            EventType = "run.started"
	EventContextAssembled      EventType = "context.assembled"
	EventStrategySelected      EventType = "strategy.selected"
	EventPlanCreated           EventType = "plan.created"
	EventStageStarted          EventType = "stage.started"
	EventStageCompleted        EventType = "stage.completed"
	EventStepStarted           EventType = "step.started"
	EventStepCompleted         EventType = "step.completed"
	EventStepFailed            EventType = "step.failed"
	EventToolCalled            EventType = "tool.called"
	EventToolCompleted         EventType = "tool.completed"
	EventVerificationCompleted EventType = "verification.completed"
	EventRepairStarted         EventType = "repair.started"
	EventRepairCompleted       EventType = "repair.completed"
	EventArtifactStaged        EventType = "artifact.staged"
	EventArtifactCommitted     EventType = "artifact.committed"
	EventStatusSummary         EventType = "status.summary"
	EventRunCompleted          EventType = "run.completed"
	EventRunFailed             EventType = "run.failed"
	EventRunCanceled           EventType = "run.canceled"
	EventNeedsInput            EventType = "needs_input"
)

// Terminal 报告事件是否为终态事件（done/error，API-SSE-002）。
func (t EventType) Terminal() bool {
	return t == EventRunCompleted || t == EventRunFailed || t == EventRunCanceled
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

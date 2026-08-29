package model

// EventType 是 SSE 事件类型（API-SSE）。
type EventType string

const (
	EventRunStarted                 EventType = "run.started"
	EventRunProgress                EventType = "run.progress"
	EventRunCompleted               EventType = "run.completed"
	EventRunFailed                  EventType = "run.failed"
	EventRunError                   EventType = "run.error"
	EventRunCanceled                EventType = "run.canceled"
	EventRunResumed                 EventType = "run.resumed"
	EventPlanUpdated                EventType = "plan.updated"
	EventPlanApprovalRequested      EventType = "plan.approval_requested"
	EventPlanApprovalAnswered       EventType = "plan.approval_answered"
	EventCommandPermissionRequested EventType = "command.permission_requested"
	EventCommandPermissionAnswered  EventType = "command.permission_answered"
	EventRunModeChanged             EventType = "run.mode_changed"
	EventMessageReasoning           EventType = "message.reasoning"
	EventMessageMilestone           EventType = "message.milestone"
	EventMessageFinal               EventType = "message.final"
	EventToolStarted                EventType = "tool.started"
	EventToolCompleted              EventType = "tool.completed"
	EventQuestionAsked              EventType = "question.asked"
	EventQuestionAnswered           EventType = "question.answered"
)

// Terminal 报告事件是否为终态事件（done/error，API-SSE-002）。
func (t EventType) Terminal() bool {
	return t == EventRunCompleted || t == EventRunFailed || t == EventRunError || t == EventRunCanceled
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

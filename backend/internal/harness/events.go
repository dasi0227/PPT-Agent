package harness

// SSE 事件负载类型（对应 sse-events.md 的 data 字段）。

type RunStartedPayload struct {
	RunID string `json:"run_id"`
	Kind  string `json:"kind"`
	Scope string `json:"scope"`
	Mode  string `json:"mode"`
}

type ThoughtPayload struct {
	Text string `json:"text"`
}

type ToolCallPayload struct {
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args"`
	CallID string         `json:"call_id"`
}

type ToolResultPayload struct {
	CallID      string `json:"call_id"`
	OK          bool   `json:"ok"`
	Observation string `json:"observation"`
}

type ProgressPayload struct {
	Stage   string `json:"stage"`
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Message string `json:"message,omitempty"`
}

type ArtifactPayload struct {
	ArtifactType string `json:"artifact_type"`
	Ref          string `json:"ref"`
	PageIndex    *int   `json:"page_index,omitempty"`
}

// PlanStepPayload 是 plan 事件里的单个步骤（V2-CONTRACTS §2）。
// status 枚举：pending | in_progress | completed | failed | skipped。
type PlanStepPayload struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// PlanPayload 在 PLAN 阶段一次性发出完整步骤清单（V2-PLAN-001，非终态）。
type PlanPayload struct {
	ID    string            `json:"id"`
	Title string            `json:"title"`
	Steps []PlanStepPayload `json:"steps"`
}

// PlanUpdatePayload 增量更新单个 step 状态（V2-PLAN-002：step_id 必须命中已发 plan）。
type PlanUpdatePayload struct {
	ID     string `json:"id"`
	StepID string `json:"step_id"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type InfoPayload struct {
	Text string `json:"text"`
}

type DonePayload struct {
	Result map[string]any `json:"result"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

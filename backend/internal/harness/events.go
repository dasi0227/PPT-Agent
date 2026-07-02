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

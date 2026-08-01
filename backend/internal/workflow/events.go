package workflow

import "time"

type EventEnvelope struct {
	RunID      string `json:"run_id"`
	WorkflowID string `json:"workflow_id"`
	Stage      Stage  `json:"stage,omitempty"`
	StepID     string `json:"step_id,omitempty"`
	Attempt    int    `json:"attempt,omitempty"`
	TS         int64  `json:"ts"`
}

func envelope(state WorkflowState) EventEnvelope {
	return EventEnvelope{
		RunID: state.RunID, WorkflowID: state.WorkflowID, Stage: state.Stage,
		StepID: state.CurrentStepID, Attempt: state.Attempt, TS: time.Now().Unix(),
	}
}

type StageEvent struct {
	EventEnvelope
	DurationMS int64 `json:"duration_ms,omitempty"`
}

type StepEvent struct {
	EventEnvelope
	Kind       StepKind   `json:"kind"`
	Title      string     `json:"title"`
	Status     StepStatus `json:"status"`
	DurationMS int64      `json:"duration_ms,omitempty"`
	Summary    string     `json:"summary,omitempty"`
	Code       string     `json:"code,omitempty"`
}

type PlanCreatedEvent struct {
	EventEnvelope
	Plan WorkflowPlan `json:"plan"`
}

type ToolEvent struct {
	EventEnvelope
	CallID    string         `json:"call_id"`
	Tool      string         `json:"tool"`
	Args      map[string]any `json:"args,omitempty"`
	OK        bool           `json:"ok,omitempty"`
	Summary   string         `json:"summary,omitempty"`
	Artifacts []ArtifactRef  `json:"artifacts,omitempty"`
	Issues    []Issue        `json:"issues,omitempty"`
}

type VerificationEvent struct {
	EventEnvelope
	Verifier string       `json:"verifier"`
	Result   VerifyResult `json:"result"`
}

type RepairEvent struct {
	EventEnvelope
	Round    int         `json:"round"`
	Artifact ArtifactRef `json:"artifact"`
	Improved bool        `json:"improved,omitempty"`
	Summary  string      `json:"summary,omitempty"`
}

type ArtifactEvent struct {
	EventEnvelope
	Artifact ArtifactRef `json:"artifact"`
	Change   string      `json:"change,omitempty"`
}

type StatusSummaryEvent struct {
	EventEnvelope
	Summary string `json:"summary"`
}

type TerminalEvent struct {
	EventEnvelope
	Outcome StructuredOutcome `json:"outcome"`
}

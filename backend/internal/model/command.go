package model

import "encoding/json"

type CommandRequest struct {
	Source        string          `json:"-"`
	RequestKey    string          `json:"request_key"`
	Kind          string          `json:"kind"`
	Input         json.RawMessage `json:"input"`
	CommandID     string          `json:"command_id,omitempty"`
	BaseAttemptID string          `json:"base_attempt_id,omitempty"`
	Feedback      string          `json:"feedback,omitempty"`
}
type CommandExecution struct {
	PreviousTitle     string            `json:"previous_title,omitempty"`
	OwnerInstanceID   string            `json:"-"`
	ExecutionRevision int64             `json:"-"`
	SceneRevision     int64             `json:"-"`
	CommandID         string            `json:"command_id"`
	AttemptID         string            `json:"attempt_id"`
	AttemptNo         int               `json:"attempt_no"`
	ThreadID          string            `json:"thread_id"`
	ProjectID         string            `json:"project_id"`
	Kind              string            `json:"kind"`
	Source            string            `json:"source"`
	Status            string            `json:"status"`
	Phase             int               `json:"phase"`
	Input             json.RawMessage   `json:"input"`
	BaseAttemptID     string            `json:"base_attempt_id,omitempty"`
	Feedback          string            `json:"feedback,omitempty"`
	Result            json.RawMessage   `json:"result,omitempty"`
	Error             json.RawMessage   `json:"error,omitempty"`
	LatestSuccess     *CommandExecution `json:"latest_success,omitempty"`
	CreatedAt         int64             `json:"created_at"`
	UpdatedAt         int64             `json:"updated_at"`
}

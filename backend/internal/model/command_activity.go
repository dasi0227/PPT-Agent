package model

import "encoding/json"

// CommandActivity is the durable timeline state of one user-invoked command.
// Retries replace the same activity; AttemptID prevents late attempts overwriting it.
type CommandActivity struct {
	ID            string          `json:"id"`
	AttemptID     string          `json:"-"`
	ThreadID      string          `json:"thread_id"`
	ProjectID     string          `json:"project_id"`
	Kind          string          `json:"kind"`
	Method        string          `json:"method"`
	Status        string          `json:"status"`
	Phase         int             `json:"phase"`
	PreviousTitle string          `json:"previous_title"`
	Request       json.RawMessage `json:"request"`
	Result        json.RawMessage `json:"result"`
	CreatedAt     int64           `json:"created_at"` // milliseconds
	UpdatedAt     int64           `json:"updated_at"`
}

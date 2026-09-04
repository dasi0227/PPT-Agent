package model

type ContextCompactionTrigger string

const (
	ContextCompactionAuto   ContextCompactionTrigger = "auto"
	ContextCompactionManual ContextCompactionTrigger = "manual"
)

type ContextCompaction struct {
	ID           string                   `json:"id"`
	ThreadID     string                   `json:"thread_id"`
	ProjectID    string                   `json:"project_id"`
	RunID        string                   `json:"run_id,omitempty"`
	Trigger      ContextCompactionTrigger `json:"trigger"`
	Summary      string                   `json:"summary"`
	BeforeTokens int                      `json:"before_tokens"`
	AfterTokens  int                      `json:"after_tokens"`
	MaxTokens    int                      `json:"max_tokens"`
	Reclaimed    int                      `json:"reclaimed_tokens"`
	DurationMS   int64                    `json:"duration_ms"`
	CreatedAt    int64                    `json:"created_at"`
}

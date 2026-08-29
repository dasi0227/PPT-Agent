package model

import "time"

const GitCommitEventSchemaVersion = 1

type GitCommitStatus string

const (
	GitCommitAccepted  GitCommitStatus = "accepted"
	GitCommitRunning   GitCommitStatus = "running"
	GitCommitEmpty     GitCommitStatus = "empty"
	GitCommitCompleted GitCommitStatus = "completed"
	GitCommitFailed    GitCommitStatus = "failed"
)

func (s GitCommitStatus) Terminal() bool {
	return s == GitCommitEmpty || s == GitCommitCompleted || s == GitCommitFailed
}

type GitCommitPhase string

const (
	GitCommitStaging    GitCommitPhase = "staging"
	GitCommitAnalyzing  GitCommitPhase = "analyzing"
	GitCommitCommitting GitCommitPhase = "committing"
)

type GitCommitEventType string

const (
	EventGitCommitProgress  GitCommitEventType = "git.commit.progress"
	EventGitCommitEmpty     GitCommitEventType = "git.commit.empty"
	EventGitCommitCompleted GitCommitEventType = "git.commit.completed"
	EventGitCommitFailed    GitCommitEventType = "git.commit.failed"
)

func (t GitCommitEventType) Terminal() bool {
	return t == EventGitCommitEmpty || t == EventGitCommitCompleted || t == EventGitCommitFailed
}

type GitCommitOperation struct {
	ID              string
	ProjectID       string
	ThreadID        string
	ClientRequestID string
	ModelProfile    string
	Status          GitCommitStatus
	Phase           GitCommitPhase
	ResultJSON      string
	ErrorJSON       string
	CreatedAt       int64
	UpdatedAt       int64
}

type GitCommitEvent struct {
	OperationID string
	Seq         int64
	Type        GitCommitEventType
	Payload     string
	CreatedAt   int64
}

type GitCommitEventBase struct {
	SchemaVersion int    `json:"schema_version"`
	OperationID   string `json:"operation_id"`
	ProjectID     string `json:"project_id"`
	ThreadID      string `json:"thread_id"`
	OccurredAt    string `json:"occurred_at"`
}

func NewGitCommitEventBase(operationID, projectID, threadID string) GitCommitEventBase {
	return GitCommitEventBase{
		SchemaVersion: GitCommitEventSchemaVersion,
		OperationID:   operationID,
		ProjectID:     projectID,
		ThreadID:      threadID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}
}

type GitCommitProgressPayload struct {
	GitCommitEventBase
	Phase GitCommitPhase `json:"phase"`
}

type GitCommitResult struct {
	Title        string   `json:"title"`
	Items        []string `json:"items"`
	Branch       string   `json:"branch"`
	Hash         string   `json:"hash"`
	FilesChanged int      `json:"files_changed"`
	Insertions   int      `json:"insertions"`
	Deletions    int      `json:"deletions"`
	CommittedAt  string   `json:"committed_at"`
}

type GitCommitCompletedPayload struct {
	GitCommitEventBase
	Commit GitCommitResult `json:"commit"`
}

type GitCommitEmptyPayload struct {
	GitCommitEventBase
}

type GitCommitPublicError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type GitCommitFailedPayload struct {
	GitCommitEventBase
	Error GitCommitPublicError `json:"error"`
}

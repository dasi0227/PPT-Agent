package model

import "errors"

// ErrRunNotFound is shared by run storage and workflow receipt recovery.
var ErrRunNotFound = errors.New("run: not found")

// Run 是一次 Agent 执行的领域表示（生命周期外壳的元数据）。
type ActiveModelSelection struct {
	Profile      string `json:"profile"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	FallbackUsed bool   `json:"fallback_used"`
}

type Run struct {
	FinalMessageID     string
	SceneRevision      int64
	ContextRevision    int64
	InputID            string
	AcceptedSeq        int64
	EntityRevision     int64
	UpdatedSeq         int64
	CanContinue        bool
	RequestHash        string
	ExecutionRevision  int64
	CheckpointRevision int64
	ExecutionModel     *ActiveModelSelection
	ActiveDurationMS   *int64
	ID                 string
	ThreadID           string
	ProjectID          string
	ClientRequestID    string
	Model              ModelSelection
	Command            RunCommand
	Status             RunStatus
	CancelRequestedAt  int64
	CancelRequestID    string
	CancelReason       RunCancelReason
	OwnerInstanceID    string
	PauseReason        string
	PausedAt           int64
	CreatedAt          int64
	UpdatedAt          int64
}

// ModelSelection is the non-sensitive profile snapshot pinned at Run creation.
// Historical runs have the zero value and are projected as model: null.
type ModelSelection struct {
	ProfileName string
	Provider    string
	Model       string
	URL         string
}

// CreateRunParams 是发起一次 Run 的入参（来自 API 层，已解析）。
type CreateRunParams struct {
	SceneRevision      int64                `json:"scene_revision,string"`
	ClientRequestID    string               `json:"client_request_id"`
	Model              string               `json:"model"`
	SkillIDs           []string             `json:"skill_ids"`
	ComponentNames     []string             `json:"component_names"`
	MentionedSlideIDs  []string             `json:"mentioned_slide_ids"`
	ThreadID           string               `json:"thread_id"`
	ProjectID          string               `json:"project_id"`
	Instruction        string               `json:"instruction"`
	AttachmentIDs      []string             `json:"attachment_ids"`
	DOMSelections      []DOMSelection       `json:"dom_selections"`
	ReferenceOrder     []ReferenceOrderItem `json:"reference_order"`
	Command            RunCommand           `json:"command"`
	ScopeInput         *CreateRunScopeInput `json:"scope_input"`
	RestoredCheckpoint bool                 `json:"restored_checkpoint"`
}

type IdempotencyRecord struct {
	Scope       string
	OwnerID     string
	Key         string
	RequestHash string
	Status      string
	ResultJSON  string
}

type SteeringStatus string

const (
	SteeringAccepted SteeringStatus = "accepted"
	SteeringInjected SteeringStatus = "injected"
	SteeringRejected SteeringStatus = "rejected"
)

type SteeringMessage struct {
	InputID         string
	AcceptedSeq     int64
	RunID           string
	ThreadID        string
	ClientMessageID string
	RequestHash     string
	Content         string
	Attachments     []AttachmentReference
	DOMSelections   []DOMSelection
	ReferenceOrder  []ReferenceOrderItem
	Scope           RunScope
	Status          SteeringStatus
	RejectionCode   string
}

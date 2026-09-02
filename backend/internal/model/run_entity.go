package model

// Run 是一次 Agent 执行的领域表示（生命周期外壳的元数据）。
type Run struct {
	ID                string
	ThreadID          string
	ProjectID         string
	ClientRequestID   string
	Model             ModelSelection
	Command           RunCommand
	Status            RunStatus
	CancelRequestedAt int64
	OwnerInstanceID   string
	PauseReason       string
	PausedAt          int64
	CreatedAt         int64
	UpdatedAt         int64
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
	ClientRequestID string
	Model           string
	SkillIDs        []string
	ComponentNames  []string
	ThreadID        string
	ProjectID       string
	PageIndex       *int
	Instruction     string
	// Spec/PPT runner implementation options.
	Brief      string
	SlideCount int
	Language   string
	// Theme falls back to project.Theme when omitted.
	Theme string
	// Internal deck runner page count.
	PageCount int
	Command   RunCommand
}

type IdempotencyRecord struct {
	Scope       string
	OwnerID     string
	Key         string
	RequestHash string
	Status      string
	ResultJSON  string
	CreatedAt   int64
	UpdatedAt   int64
}

type SteeringStatus string

const (
	SteeringAccepted SteeringStatus = "accepted"
	SteeringInjected SteeringStatus = "injected"
	SteeringRejected SteeringStatus = "rejected"
)

type SteeringMessage struct {
	RunID           string
	ThreadID        string
	ClientMessageID string
	RequestHash     string
	Content         string
	Status          SteeringStatus
	AcceptedAt      int64
	InjectedAt      int64
	RejectionCode   string
}

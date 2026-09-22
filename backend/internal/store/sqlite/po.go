package sqlite

import (
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// 持久化对象（PO）：GORM tag 仅出现在本包（ARCH-BACKEND-006）。PO↔model 在 store 边界互转。

type projectPO struct {
	ID            string `gorm:"column:id;primaryKey"`
	Title         string `gorm:"column:title"`
	WorkDir       string `gorm:"column:work_dir"`
	Theme         string `gorm:"column:theme"`
	Status        string `gorm:"column:status"`
	LayoutVersion int    `gorm:"column:layout_version"`
	CreatedAt     int64  `gorm:"column:created_at"`
	UpdatedAt     int64  `gorm:"column:updated_at"`
}

func (projectPO) TableName() string { return "projects" }

func (p projectPO) toModel() model.Project {
	return model.Project{
		ID: p.ID, Title: p.Title, WorkDir: p.WorkDir, Theme: p.Theme,
		Status:        p.Status,
		LayoutVersion: p.LayoutVersion,
		CreatedAt:     p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func projectToPO(m model.Project) projectPO {
	if m.LayoutVersion == 0 {
		m.LayoutVersion = currentProjectLayoutVersion
	}
	return projectPO{
		ID: m.ID, Title: m.Title, WorkDir: m.WorkDir, Theme: m.Theme,
		Status:        m.Status,
		LayoutVersion: m.LayoutVersion,
		CreatedAt:     m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

type threadPO struct {
	ID                     string `gorm:"column:id;primaryKey"`
	ProjectID              string `gorm:"column:project_id"`
	Title                  string `gorm:"column:title"`
	HistoryPath            string `gorm:"column:history_path"`
	Status                 string `gorm:"column:status"`
	AutoRenameEnabled      int    `gorm:"column:auto_rename_enabled"`
	NamingRevision         int64  `gorm:"column:naming_revision"`
	RenameOperationVersion int64  `gorm:"column:rename_operation_version"`
	RenameInputCount       int    `gorm:"column:rename_input_count"`
	RenameFirstInputSeen   int    `gorm:"column:rename_first_input_seen"`
	CreatedAt              int64  `gorm:"column:created_at"`
	UpdatedAt              int64  `gorm:"column:updated_at"`
}

func (threadPO) TableName() string { return "threads" }

func (t threadPO) toModel() model.Thread {
	return model.Thread{
		ID: t.ID, ProjectID: t.ProjectID, Title: t.Title, HistoryPath: t.HistoryPath,
		Status: t.Status, AutoRenameEnabled: t.AutoRenameEnabled != 0,
		NamingRevision: t.NamingRevision, RenameOperationVersion: t.RenameOperationVersion,
		RenameInputCount: t.RenameInputCount, RenameFirstInputSeen: t.RenameFirstInputSeen != 0,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func threadToPO(m model.Thread) threadPO {
	if m.NamingRevision == 0 {
		m.AutoRenameEnabled = true
		m.NamingRevision = 1
		m.RenameOperationVersion = 1
	}
	return threadPO{
		ID: m.ID, ProjectID: m.ProjectID, Title: m.Title, HistoryPath: m.HistoryPath,
		Status: m.Status, AutoRenameEnabled: boolInt(m.AutoRenameEnabled),
		NamingRevision: m.NamingRevision, RenameOperationVersion: m.RenameOperationVersion,
		RenameInputCount: m.RenameInputCount, RenameFirstInputSeen: boolInt(m.RenameFirstInputSeen),
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

type threadNamingInputPO struct {
	ThreadID   string `gorm:"column:thread_id;primaryKey"`
	InputID    string `gorm:"column:input_id;primaryKey"`
	Content    string `gorm:"column:content"`
	AcceptedAt int64  `gorm:"column:accepted_at"`
}

func (threadNamingInputPO) TableName() string { return "thread_naming_inputs" }
func (p threadNamingInputPO) toModel() model.ThreadNamingInput {
	return model.ThreadNamingInput{ThreadID: p.ThreadID, InputID: p.InputID, Content: p.Content, AcceptedAt: p.AcceptedAt}
}

type threadNamingOperationPO struct {
	ThreadID    string `gorm:"column:thread_id;primaryKey"`
	OperationID string `gorm:"column:operation_id;primaryKey"`
	RequestHash string `gorm:"column:request_hash"`
	Action      string `gorm:"column:action"`
	Status      string `gorm:"column:status"`
	RequestID   string `gorm:"column:request_id"`
	ResultJSON  string `gorm:"column:result_json"`
	CreatedAt   int64  `gorm:"column:created_at"`
	UpdatedAt   int64  `gorm:"column:updated_at"`
}

func (threadNamingOperationPO) TableName() string { return "thread_naming_operations" }
func (p threadNamingOperationPO) toModel() model.ThreadNamingOperation {
	return model.ThreadNamingOperation{ThreadID: p.ThreadID, OperationID: p.OperationID, RequestHash: p.RequestHash, Action: p.Action, Status: p.Status, RequestID: p.RequestID, ResultJSON: p.ResultJSON, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

type runPO struct {
	ID                           string `gorm:"column:id;primaryKey"`
	ThreadID                     string `gorm:"column:thread_id"`
	ProjectID                    string `gorm:"column:project_id"`
	ScopeObject                  string `gorm:"column:scope_object"`
	ScopeSlideIDsJSON            string `gorm:"column:scope_slide_ids_json"`
	ScopeSourceJSON              string `gorm:"column:scope_source_json"`
	ScopeIncludeRunCreatedSlides int    `gorm:"column:scope_include_run_created_slides"`
	ScopeRevision                int64  `gorm:"column:scope_revision"`
	Mode                         string `gorm:"column:mode"`
	RunCommandJSON               string `gorm:"column:run_command_json"`
	ClientRequestID              string `gorm:"column:client_request_id"`
	ProjectHistoryRevision       int64  `gorm:"column:project_history_revision"`
	ModelProfileName             string `gorm:"column:model_profile_name"`
	ModelProvider                string `gorm:"column:model_provider"`
	ModelName                    string `gorm:"column:model_name"`
	ModelURL                     string `gorm:"column:model_url"`
	CancelRequestedAt            *int64 `gorm:"column:cancel_requested_at"`
	OwnerInstanceID              string `gorm:"column:owner_instance_id"`
	PauseReason                  string `gorm:"column:pause_reason"`
	PausedAt                     *int64 `gorm:"column:paused_at"`
	Status                       string `gorm:"column:status"`
	CreatedAt                    int64  `gorm:"column:created_at"`
	UpdatedAt                    int64  `gorm:"column:updated_at"`
}

func (runPO) TableName() string { return "runs" }

func (r runPO) toModel() model.Run {
	var command model.RunCommand
	_ = json.Unmarshal([]byte(r.RunCommandJSON), &command)
	return model.Run{
		ID: r.ID, ThreadID: r.ThreadID, ProjectID: r.ProjectID,
		ClientRequestID: r.ClientRequestID, ProjectHistoryRevision: r.ProjectHistoryRevision,
		Command: command, Status: model.RunStatus(r.Status),
		Model: model.ModelSelection{
			ProfileName: r.ModelProfileName, Provider: r.ModelProvider,
			Model: r.ModelName, URL: r.ModelURL,
		},
		CancelRequestedAt: valueOrZero(r.CancelRequestedAt),
		OwnerInstanceID:   r.OwnerInstanceID,
		PauseReason:       r.PauseReason,
		PausedAt:          valueOrZero(r.PausedAt),
		CreatedAt:         r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func runToPO(m model.Run) runPO {
	if m.ProjectHistoryRevision < 1 {
		m.ProjectHistoryRevision = 1
	}
	raw, _ := json.Marshal(m.Command)
	slideIDs, _ := json.Marshal(m.Command.Scope.SlideIDs)
	source, _ := json.Marshal(m.Command.Scope.Source)
	return runPO{
		ID: m.ID, ThreadID: m.ThreadID, ProjectID: m.ProjectID,
		ScopeObject: string(m.Command.Scope.Object), ScopeSlideIDsJSON: string(slideIDs),
		ScopeSourceJSON: string(source), ScopeIncludeRunCreatedSlides: boolInt(m.Command.Scope.IncludeRunCreatedSlides),
		ScopeRevision:          m.Command.Scope.Revision,
		Mode:                   string(m.Command.Mode),
		RunCommandJSON:         string(raw),
		ClientRequestID:        m.ClientRequestID,
		ProjectHistoryRevision: m.ProjectHistoryRevision,
		ModelProfileName:       m.Model.ProfileName,
		ModelProvider:          m.Model.Provider,
		ModelName:              m.Model.Model,
		ModelURL:               m.Model.URL,
		CancelRequestedAt:      int64PtrOrNil(m.CancelRequestedAt),
		OwnerInstanceID:        m.OwnerInstanceID,
		PauseReason:            m.PauseReason,
		PausedAt:               int64PtrOrNil(m.PausedAt),
		Status:                 string(m.Status), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func int64PtrOrNil(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}

func valueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

type idempotencyPO struct {
	Scope       string `gorm:"column:scope;primaryKey"`
	OwnerID     string `gorm:"column:owner_id;primaryKey"`
	Key         string `gorm:"column:key;primaryKey"`
	RequestHash string `gorm:"column:request_hash"`
	Status      string `gorm:"column:status"`
	ResultJSON  string `gorm:"column:result_json"`
	CreatedAt   int64  `gorm:"column:created_at"`
	UpdatedAt   int64  `gorm:"column:updated_at"`
}

func (idempotencyPO) TableName() string { return "idempotency_records" }

func (p idempotencyPO) toModel() model.IdempotencyRecord {
	return model.IdempotencyRecord{
		Scope: p.Scope, OwnerID: p.OwnerID, Key: p.Key, RequestHash: p.RequestHash,
		Status: p.Status, ResultJSON: p.ResultJSON, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

type steeringPO struct {
	RunID           string `gorm:"column:run_id"`
	ThreadID        string `gorm:"column:thread_id;primaryKey"`
	ClientMessageID string `gorm:"column:client_message_id;primaryKey"`
	RequestHash     string `gorm:"column:request_hash"`
	Content         string `gorm:"column:content"`
	ReferencesJSON  string `gorm:"column:references_json"`
	Status          string `gorm:"column:status"`
	AcceptedAt      int64  `gorm:"column:accepted_at"`
	InjectedAt      *int64 `gorm:"column:injected_at"`
	RejectionCode   string `gorm:"column:rejection_code"`
}

func (steeringPO) TableName() string { return "steering_inbox" }

func (p steeringPO) toModel() model.SteeringMessage {
	var refs struct {
		Attachments    []model.AttachmentReference `json:"attachments"`
		DOMSelections  []model.DOMSelection        `json:"dom_selections"`
		ReferenceOrder []model.ReferenceOrderItem  `json:"reference_order"`
		Scope          model.RunScope              `json:"scope"`
	}
	_ = json.Unmarshal([]byte(p.ReferencesJSON), &refs)
	return model.SteeringMessage{
		RunID: p.RunID, ThreadID: p.ThreadID, ClientMessageID: p.ClientMessageID,
		RequestHash: p.RequestHash, Content: p.Content, Attachments: refs.Attachments,
		DOMSelections: refs.DOMSelections, ReferenceOrder: refs.ReferenceOrder, Scope: refs.Scope, Status: model.SteeringStatus(p.Status),
		AcceptedAt: p.AcceptedAt, InjectedAt: valueOrZero(p.InjectedAt), RejectionCode: p.RejectionCode,
	}
}

type runEventPO struct {
	RunID     string `gorm:"column:run_id;primaryKey"`
	Seq       int64  `gorm:"column:seq;primaryKey"`
	Type      string `gorm:"column:type"`
	Payload   string `gorm:"column:payload"`
	CreatedAt int64  `gorm:"column:created_at"`
}

func (runEventPO) TableName() string { return "run_events" }

func (e runEventPO) toModel() model.Event {
	return model.Event{
		RunID: e.RunID, Seq: e.Seq, Type: model.EventType(e.Type),
		Payload: e.Payload, CreatedAt: e.CreatedAt,
	}
}

func eventToPO(m model.Event) runEventPO {
	return runEventPO{
		RunID: m.RunID, Seq: m.Seq, Type: string(m.Type),
		Payload: m.Payload, CreatedAt: m.CreatedAt,
	}
}

type gitCommitOperationPO struct {
	ID              string `gorm:"column:id;primaryKey"`
	ProjectID       string `gorm:"column:project_id"`
	ThreadID        string `gorm:"column:thread_id"`
	ClientRequestID string `gorm:"column:client_request_id"`
	ModelProfile    string `gorm:"column:model_profile"`
	Status          string `gorm:"column:status"`
	Phase           string `gorm:"column:phase"`
	ResultJSON      string `gorm:"column:result_json"`
	ErrorJSON       string `gorm:"column:error_json"`
	CreatedAt       int64  `gorm:"column:created_at"`
	UpdatedAt       int64  `gorm:"column:updated_at"`
}

func (gitCommitOperationPO) TableName() string { return "git_commit_operations" }

func (p gitCommitOperationPO) toModel() model.GitCommitOperation {
	return model.GitCommitOperation{
		ID: p.ID, ProjectID: p.ProjectID, ThreadID: p.ThreadID,
		ClientRequestID: p.ClientRequestID, ModelProfile: p.ModelProfile,
		Status: model.GitCommitStatus(p.Status), Phase: model.GitCommitPhase(p.Phase),
		ResultJSON: p.ResultJSON, ErrorJSON: p.ErrorJSON,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func gitCommitOperationToPO(m model.GitCommitOperation) gitCommitOperationPO {
	return gitCommitOperationPO{
		ID: m.ID, ProjectID: m.ProjectID, ThreadID: m.ThreadID,
		ClientRequestID: m.ClientRequestID, ModelProfile: m.ModelProfile,
		Status: string(m.Status), Phase: string(m.Phase),
		ResultJSON: m.ResultJSON, ErrorJSON: m.ErrorJSON,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

type gitCommitEventPO struct {
	OperationID string `gorm:"column:operation_id;primaryKey"`
	Seq         int64  `gorm:"column:seq;primaryKey"`
	Type        string `gorm:"column:type"`
	Payload     string `gorm:"column:payload"`
	CreatedAt   int64  `gorm:"column:created_at"`
}

func (gitCommitEventPO) TableName() string { return "git_commit_events" }

func (p gitCommitEventPO) toModel() model.GitCommitEvent {
	return model.GitCommitEvent{
		OperationID: p.OperationID, Seq: p.Seq,
		Type: model.GitCommitEventType(p.Type), Payload: p.Payload, CreatedAt: p.CreatedAt,
	}
}

func gitCommitEventToPO(m model.GitCommitEvent) gitCommitEventPO {
	return gitCommitEventPO{
		OperationID: m.OperationID, Seq: m.Seq,
		Type: string(m.Type), Payload: m.Payload, CreatedAt: m.CreatedAt,
	}
}

type briefingVersionPO struct {
	BriefingID string `gorm:"column:briefing_id;primaryKey"`
	ThreadID   string `gorm:"column:thread_id"`
	ProjectID  string `gorm:"column:project_id"`
	Kind       string `gorm:"column:kind"`
	VersionNo  int    `gorm:"column:version_no;primaryKey"`
	Title      string `gorm:"column:title"`
	Content    string `gorm:"column:content"`
	Feedback   string `gorm:"column:feedback"`
	CreatedAt  int64  `gorm:"column:created_at"`
}

func (briefingVersionPO) TableName() string { return "briefing_versions" }

func (p briefingVersionPO) toModel() model.BriefingVersion {
	return model.BriefingVersion{
		BriefingID: p.BriefingID, ThreadID: p.ThreadID, ProjectID: p.ProjectID,
		Kind: model.BriefingKind(p.Kind), VersionNo: p.VersionNo,
		Title: p.Title, Content: p.Content, Feedback: p.Feedback, CreatedAt: p.CreatedAt,
	}
}

func briefingVersionToPO(m model.BriefingVersion) briefingVersionPO {
	return briefingVersionPO{
		BriefingID: m.BriefingID, ThreadID: m.ThreadID, ProjectID: m.ProjectID,
		Kind: string(m.Kind), VersionNo: m.VersionNo,
		Title: m.Title, Content: m.Content, Feedback: m.Feedback, CreatedAt: m.CreatedAt,
	}
}

type contextCompactionPO struct {
	ID           string `gorm:"column:id;primaryKey"`
	ThreadID     string `gorm:"column:thread_id"`
	ProjectID    string `gorm:"column:project_id"`
	RunID        string `gorm:"column:run_id"`
	Trigger      string `gorm:"column:trigger"`
	Title        string `gorm:"column:title"`
	Content      string `gorm:"column:content"`
	BeforeTokens int    `gorm:"column:before_tokens"`
	AfterTokens  int    `gorm:"column:after_tokens"`
	MaxTokens    int    `gorm:"column:max_tokens"`
	Reclaimed    int    `gorm:"column:reclaimed_tokens"`
	DurationMS   int64  `gorm:"column:duration_ms"`
	CreatedAt    int64  `gorm:"column:created_at"`
}

func (contextCompactionPO) TableName() string { return "context_compactions" }

func (p contextCompactionPO) toModel() model.ContextCompaction {
	return model.ContextCompaction{
		ID: p.ID, ThreadID: p.ThreadID, ProjectID: p.ProjectID, RunID: p.RunID,
		Trigger: model.ContextCompactionTrigger(p.Trigger), Title: p.Title, Content: p.Content,
		BeforeTokens: p.BeforeTokens, AfterTokens: p.AfterTokens, MaxTokens: p.MaxTokens,
		Reclaimed: p.Reclaimed, DurationMS: p.DurationMS, CreatedAt: p.CreatedAt,
	}
}

func contextCompactionToPO(m model.ContextCompaction) contextCompactionPO {
	return contextCompactionPO{
		ID: m.ID, ThreadID: m.ThreadID, ProjectID: m.ProjectID, RunID: m.RunID,
		Trigger: string(m.Trigger), Title: m.Title, Content: m.Content,
		BeforeTokens: m.BeforeTokens, AfterTokens: m.AfterTokens, MaxTokens: m.MaxTokens,
		Reclaimed: m.Reclaimed, DurationMS: m.DurationMS, CreatedAt: m.CreatedAt,
	}
}

type runContextPO struct {
	RunID           string `gorm:"column:run_id;primaryKey"`
	ContextID       string `gorm:"column:context_id;uniqueIndex"`
	Profile         string `gorm:"column:profile"`
	PackHash        string `gorm:"column:pack_hash"`
	EstimatedTokens int    `gorm:"column:estimated_tokens"`
	BudgetTokens    int    `gorm:"column:budget_tokens"`
	ManifestJSON    string `gorm:"column:manifest_json"`
	CreatedAt       int64  `gorm:"column:created_at"`
}

func (runContextPO) TableName() string { return "run_contexts" }

func contextToPO(m model.RunContext) runContextPO {
	return runContextPO{RunID: m.RunID, ContextID: m.ContextID, Profile: m.Profile, PackHash: m.PackHash,
		EstimatedTokens: m.EstimatedTokens, BudgetTokens: m.BudgetTokens, ManifestJSON: m.ManifestJSON, CreatedAt: m.CreatedAt}
}

func (p runContextPO) toModel() model.RunContext {
	return model.RunContext{RunID: p.RunID, ContextID: p.ContextID, Profile: p.Profile, PackHash: p.PackHash,
		EstimatedTokens: p.EstimatedTokens, BudgetTokens: p.BudgetTokens, ManifestJSON: p.ManifestJSON, CreatedAt: p.CreatedAt}
}

type runCheckpointPO struct {
	ID             string `gorm:"column:id;primaryKey"`
	RunID          string `gorm:"column:run_id"`
	LoopID         string `gorm:"column:loop_id"`
	Seq            int64  `gorm:"column:seq"`
	Phase          string `gorm:"column:phase"`
	CheckpointJSON string `gorm:"column:checkpoint_json"`
	CreatedAt      int64  `gorm:"column:created_at"`
}

func (runCheckpointPO) TableName() string { return "run_checkpoints" }

type contextIndexSnapshotPO struct {
	ID        string `gorm:"column:id;primaryKey"`
	RunID     string `gorm:"column:run_id"`
	PackHash  string `gorm:"column:pack_hash"`
	IndexJSON string `gorm:"column:index_json"`
	CreatedAt int64  `gorm:"column:created_at"`
}

func (contextIndexSnapshotPO) TableName() string { return "context_index_snapshots" }

type semanticReviewPO struct {
	ID                 string  `gorm:"column:id;primaryKey"`
	RunID              string  `gorm:"column:run_id"`
	FinishCallID       string  `gorm:"column:finish_call_id"`
	Accepted           int     `gorm:"column:accepted"`
	Confidence         float64 `gorm:"column:confidence"`
	InputHash          string  `gorm:"column:input_hash"`
	OutputJSON         string  `gorm:"column:output_json"`
	PromptManifestJSON string  `gorm:"column:prompt_manifest_json"`
	CreatedAt          int64   `gorm:"column:created_at"`
}

func (semanticReviewPO) TableName() string { return "semantic_reviews" }

type slidePO struct {
	ID           string `gorm:"column:id;primaryKey"`
	ProjectID    string `gorm:"column:project_id"`
	LastExportAt *int64 `gorm:"column:last_export_at"`
}

func (slidePO) TableName() string { return "slides" }

// toModel exposes runtime identity and current HTML metadata only.
func (s slidePO) toModel() model.Slide {
	return model.Slide{
		ID: s.ID, ProjectID: s.ProjectID,
		LastExportAt: s.LastExportAt,
	}
}

func slideToPO(m model.Slide) slidePO {
	return slidePO{
		ID: m.ID, ProjectID: m.ProjectID,
		LastExportAt: m.LastExportAt,
	}
}

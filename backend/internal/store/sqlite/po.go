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
	ID          string `gorm:"column:id;primaryKey"`
	ProjectID   string `gorm:"column:project_id"`
	Title       string `gorm:"column:title"`
	HistoryPath string `gorm:"column:history_path"`
	Status      string `gorm:"column:status"`
	CreatedAt   int64  `gorm:"column:created_at"`
	UpdatedAt   int64  `gorm:"column:updated_at"`
}

func (threadPO) TableName() string { return "threads" }

func (t threadPO) toModel() model.Thread {
	return model.Thread{
		ID: t.ID, ProjectID: t.ProjectID, Title: t.Title, HistoryPath: t.HistoryPath,
		Status: t.Status, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func threadToPO(m model.Thread) threadPO {
	return threadPO{
		ID: m.ID, ProjectID: m.ProjectID, Title: m.Title, HistoryPath: m.HistoryPath,
		Status: m.Status, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

type runPO struct {
	ID                string `gorm:"column:id;primaryKey"`
	ThreadID          string `gorm:"column:thread_id"`
	ProjectID         string `gorm:"column:project_id"`
	ScopeArtifact     string `gorm:"column:scope_artifact"`
	ScopeLevel        string `gorm:"column:scope_level"`
	ScopeSlideID      string `gorm:"column:scope_slide_id"`
	Mode              string `gorm:"column:mode"`
	RunCommandJSON    string `gorm:"column:run_command_json"`
	ClientRequestID   string `gorm:"column:client_request_id"`
	ModelProfileName  string `gorm:"column:model_profile_name"`
	ModelProvider     string `gorm:"column:model_provider"`
	ModelName         string `gorm:"column:model_name"`
	ModelURL          string `gorm:"column:model_url"`
	CancelRequestedAt *int64 `gorm:"column:cancel_requested_at"`
	OwnerInstanceID   string `gorm:"column:owner_instance_id"`
	PauseReason       string `gorm:"column:pause_reason"`
	PausedAt          *int64 `gorm:"column:paused_at"`
	Status            string `gorm:"column:status"`
	CreatedAt         int64  `gorm:"column:created_at"`
	UpdatedAt         int64  `gorm:"column:updated_at"`
}

func (runPO) TableName() string { return "runs" }

func (r runPO) toModel() model.Run {
	var command model.RunCommand
	_ = json.Unmarshal([]byte(r.RunCommandJSON), &command)
	return model.Run{
		ID: r.ID, ThreadID: r.ThreadID, ProjectID: r.ProjectID,
		ClientRequestID: r.ClientRequestID, Command: command, Status: model.RunStatus(r.Status),
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
	raw, _ := json.Marshal(m.Command)
	return runPO{
		ID: m.ID, ThreadID: m.ThreadID, ProjectID: m.ProjectID,
		ScopeArtifact: string(m.Command.Scope.Artifact),
		ScopeLevel:    string(m.Command.Scope.Level), ScopeSlideID: m.Command.Scope.SlideID,
		Mode:              string(m.Command.Mode),
		RunCommandJSON:    string(raw),
		ClientRequestID:   m.ClientRequestID,
		ModelProfileName:  m.Model.ProfileName,
		ModelProvider:     m.Model.Provider,
		ModelName:         m.Model.Model,
		ModelURL:          m.Model.URL,
		CancelRequestedAt: int64PtrOrNil(m.CancelRequestedAt),
		OwnerInstanceID:   m.OwnerInstanceID,
		PauseReason:       m.PauseReason,
		PausedAt:          int64PtrOrNil(m.PausedAt),
		Status:            string(m.Status), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
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
	Status          string `gorm:"column:status"`
	AcceptedAt      int64  `gorm:"column:accepted_at"`
	InjectedAt      *int64 `gorm:"column:injected_at"`
	RejectionCode   string `gorm:"column:rejection_code"`
}

func (steeringPO) TableName() string { return "steering_inbox" }

func (p steeringPO) toModel() model.SteeringMessage {
	return model.SteeringMessage{
		RunID: p.RunID, ThreadID: p.ThreadID, ClientMessageID: p.ClientMessageID,
		RequestHash: p.RequestHash, Content: p.Content, Status: model.SteeringStatus(p.Status),
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
	ID             string `gorm:"column:id;primaryKey"`
	ProjectID      string `gorm:"column:project_id"`
	CurrentVersion int    `gorm:"column:current_version"`
	LastExportAt   *int64 `gorm:"column:last_export_at"`
}

func (slidePO) TableName() string { return "slides" }

// toModel exposes runtime identity and version pointers only.
func (s slidePO) toModel() model.Slide {
	return model.Slide{
		ID: s.ID, ProjectID: s.ProjectID,
		CurrentVersion: s.CurrentVersion,
		LastExportAt:   s.LastExportAt,
	}
}

func slideToPO(m model.Slide) slidePO {
	return slidePO{
		ID: m.ID, ProjectID: m.ProjectID,
		CurrentVersion: m.CurrentVersion,
		LastExportAt:   m.LastExportAt,
	}
}

type versionPO struct {
	ID           string  `gorm:"column:id;primaryKey"`
	TargetType   string  `gorm:"column:target_type"`
	TargetID     string  `gorm:"column:target_id"`
	VersionNo    int     `gorm:"column:version_no"`
	SnapshotPath string  `gorm:"column:snapshot_path"`
	RunID        *string `gorm:"column:run_id"`
	CreatedAt    int64   `gorm:"column:created_at"`
}

func (versionPO) TableName() string { return "versions" }

func (v versionPO) toModel() model.Version {
	runID := ""
	if v.RunID != nil {
		runID = *v.RunID
	}
	return model.Version{
		ID: v.ID, TargetType: v.TargetType, TargetID: v.TargetID, VersionNo: v.VersionNo,
		SnapshotPath: v.SnapshotPath, RunID: runID, CreatedAt: v.CreatedAt,
	}
}

func versionToPO(m model.Version) versionPO {
	var runID *string
	if m.RunID != "" {
		runID = &m.RunID
	}
	return versionPO{
		ID: m.ID, TargetType: m.TargetType, TargetID: m.TargetID, VersionNo: m.VersionNo,
		SnapshotPath: m.SnapshotPath, RunID: runID, CreatedAt: m.CreatedAt,
	}
}

type assetPO struct {
	ID           string `gorm:"column:id;primaryKey"`
	Name         string `gorm:"column:name"`
	Kind         string `gorm:"column:kind"`
	Version      string `gorm:"column:version"`
	Source       string `gorm:"column:source"`
	Description  string `gorm:"column:description"`
	Tags         string `gorm:"column:tags"` // JSON 数组文本
	ManifestPath string `gorm:"column:manifest_path"`
	Dir          string `gorm:"column:dir"`
	CreatedAt    int64  `gorm:"column:created_at"`
	UpdatedAt    int64  `gorm:"column:updated_at"`
}

func (assetPO) TableName() string { return "assets" }

func (a assetPO) toModel() model.Asset {
	var tags []string
	if a.Tags != "" {
		_ = json.Unmarshal([]byte(a.Tags), &tags)
	}
	return model.Asset{
		ID: a.ID, Name: a.Name, Kind: a.Kind, Version: a.Version, Source: a.Source,
		Description: a.Description, Tags: tags, ManifestPath: a.ManifestPath, Dir: a.Dir,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

func assetToPO(m model.Asset) assetPO {
	tags := "[]"
	if len(m.Tags) > 0 {
		if raw, err := json.Marshal(m.Tags); err == nil {
			tags = string(raw)
		}
	}
	return assetPO{
		ID: m.ID, Name: m.Name, Kind: m.Kind, Version: m.Version, Source: m.Source,
		Description: m.Description, Tags: tags, ManifestPath: m.ManifestPath, Dir: m.Dir,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

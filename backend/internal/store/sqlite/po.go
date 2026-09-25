package sqlite

import (
	"encoding/json"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
	"gorm.io/gorm"
)

type projectPO struct {
	ID        string `gorm:"primaryKey"`
	Title     string
	Theme     string
	CreatedAt int64
	UpdatedAt int64
	WorkDir   string `gorm:"-"`
}

func (projectPO) TableName() string { return "projects" }
func (p *projectPO) AfterFind(tx *gorm.DB) error {
	root, err := workRootFromDB(tx)
	if err != nil {
		return err
	}
	p.WorkDir = filepath.Join(root, "projects", p.ID, "artifacts")
	return nil
}
func (p projectPO) toModel() model.Project {
	return model.Project{ID: p.ID, Title: p.Title, Theme: p.Theme, WorkDir: p.WorkDir, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}
func projectToPO(p model.Project) projectPO {
	return projectPO{ID: p.ID, Title: p.Title, Theme: p.Theme, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

type threadPO struct {
	ID                     string `gorm:"primaryKey"`
	ProjectID              string
	Title                  string
	AutoRenameEnabled      int
	NamingRevision         int64
	RenameOperationVersion int64
	RenameInputCount       int
	RenameFirstInputSeen   int
	NextEventSeq           int64
	DeliveredEventSeq      int64
	CreatedAt              int64
	UpdatedAt              int64
}

func (threadPO) TableName() string { return "threads" }
func (t threadPO) toModel() model.Thread {
	return model.Thread{ID: t.ID, ProjectID: t.ProjectID, Title: t.Title, AutoRenameEnabled: t.AutoRenameEnabled != 0, NamingRevision: t.NamingRevision, RenameOperationVersion: t.RenameOperationVersion, RenameInputCount: t.RenameInputCount, RenameFirstInputSeen: t.RenameFirstInputSeen != 0, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}
func threadToPO(t model.Thread) threadPO {
	if t.NamingRevision == 0 {
		t.AutoRenameEnabled = true
		t.NamingRevision = 1
		t.RenameOperationVersion = 1
	}
	return threadPO{ID: t.ID, ProjectID: t.ProjectID, Title: t.Title, AutoRenameEnabled: boolInt(t.AutoRenameEnabled), NamingRevision: t.NamingRevision, RenameOperationVersion: t.RenameOperationVersion, RenameInputCount: t.RenameInputCount, RenameFirstInputSeen: boolInt(t.RenameFirstInputSeen), NextEventSeq: 1, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

type runPO struct {
	ID                 string `gorm:"primaryKey"`
	ThreadID           string
	ProjectID          string
	StartEventSeq      int64
	ScopeJSON          string
	ScopeRevision      int64
	Mode               string
	ModelProfileName   string
	ModelProvider      string
	ModelName          string
	ModelURL           string
	CancelRequestedAt  *int64
	OwnerInstanceID    string
	ExecutionRevision  int64
	PauseReason        string
	PausedAt           *int64
	Status             string
	CheckpointRevision int64
	CreatedAt          int64
	UpdatedAt          int64
	// Input is hydrated from its acceptance event; it is not a database column.
	Command         model.RunCommand `gorm:"-"`
	ClientRequestID string           `gorm:"-"`
}

func (runPO) TableName() string { return "runs" }
func (r *runPO) AfterFind(tx *gorm.DB) error {
	e, err := eventInTransaction(tx, r.ThreadID, r.StartEventSeq)
	if err != nil {
		return err
	}
	if e.Type != "run.accepted" || e.RunID != r.ID {
		return threadjournal.ErrCorrupt
	}
	var input runAcceptance
	if err := json.Unmarshal(e.Payload, &input); err != nil {
		return err
	}
	r.Command = input.Command
	r.ClientRequestID = input.ClientRequestID
	if err := json.Unmarshal([]byte(r.ScopeJSON), &r.Command.Scope); err != nil {
		return err
	}
	r.Command.Mode = model.RunMode(r.Mode)
	return nil
}
func (r runPO) toModel() model.Run {
	return model.Run{ID: r.ID, ThreadID: r.ThreadID, ProjectID: r.ProjectID, ClientRequestID: r.ClientRequestID, Command: r.Command, Status: model.RunStatus(r.Status), Model: model.ModelSelection{ProfileName: r.ModelProfileName, Provider: r.ModelProvider, Model: r.ModelName, URL: r.ModelURL}, CancelRequestedAt: valueOrZero(r.CancelRequestedAt), OwnerInstanceID: r.OwnerInstanceID, ExecutionRevision: r.ExecutionRevision, CheckpointRevision: r.CheckpointRevision, PauseReason: r.PauseReason, PausedAt: valueOrZero(r.PausedAt), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}
func runToPO(r model.Run) runPO {
	scope, _ := json.Marshal(r.Command.Scope)
	return runPO{ID: r.ID, ThreadID: r.ThreadID, ProjectID: r.ProjectID, ScopeJSON: string(scope), ScopeRevision: r.Command.Scope.Revision, Mode: string(r.Command.Mode), ModelProfileName: r.Model.ProfileName, ModelProvider: r.Model.Provider, ModelName: r.Model.Model, ModelURL: r.Model.URL, CancelRequestedAt: int64PtrOrNil(r.CancelRequestedAt), OwnerInstanceID: r.OwnerInstanceID, ExecutionRevision: 1, PauseReason: r.PauseReason, PausedAt: int64PtrOrNil(r.PausedAt), Status: string(r.Status), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

type idempotencyPO struct {
	Scope       string `gorm:"primaryKey"`
	OwnerID     string `gorm:"primaryKey"`
	Key         string `gorm:"primaryKey"`
	RequestHash string
	Status      string
	ResultJSON  string
}

func (idempotencyPO) TableName() string { return "idempotency_records" }
func (p idempotencyPO) toModel() model.IdempotencyRecord {
	return model.IdempotencyRecord{Scope: p.Scope, OwnerID: p.OwnerID, Key: p.Key, RequestHash: p.RequestHash, Status: p.Status, ResultJSON: p.ResultJSON}
}

type steeringPO struct {
	RunID           string
	ThreadID        string `gorm:"primaryKey"`
	ClientMessageID string `gorm:"primaryKey"`
	RequestHash     string
	InputEventSeq   int64
	ResultEventSeq  *int64
	Status          string
	Input           model.SteeringMessage `gorm:"-"`
}

func (steeringPO) TableName() string { return "steering_inbox" }
func (p *steeringPO) AfterFind(tx *gorm.DB) error {
	e, err := eventInTransaction(tx, p.ThreadID, p.InputEventSeq)
	if err != nil {
		return err
	}
	if e.Type != "steering.accepted" || e.RunID != p.RunID {
		return threadjournal.ErrCorrupt
	}
	if err := json.Unmarshal(e.Payload, &p.Input); err != nil {
		return err
	}
	p.Input.Status = model.SteeringStatus(p.Status)
	if p.ResultEventSeq != nil {
		result, err := eventInTransaction(tx, p.ThreadID, *p.ResultEventSeq)
		if err != nil {
			return err
		}
		var outcome struct {
			RejectionCode string `json:"rejection_code"`
		}
		if err := json.Unmarshal(result.Payload, &outcome); err != nil {
			return err
		}
		p.Input.RejectionCode = outcome.RejectionCode
		if p.Input.Status == model.SteeringInjected {
			p.Input.InjectedAt = result.TS * 1_000_000
		}
	}
	return nil
}
func (p steeringPO) toModel() model.SteeringMessage { return p.Input }

type slidePO struct {
	ID                   string `gorm:"primaryKey"`
	ProjectID            string
	GenerationInputsJSON *string
}

func (slidePO) TableName() string { return "slides" }
func (s slidePO) toModel() model.Slide {
	return model.Slide{ID: s.ID, ProjectID: s.ProjectID, GenerationInputsJSON: s.GenerationInputsJSON}
}
func slideToPO(s model.Slide) slidePO {
	return slidePO{ID: s.ID, ProjectID: s.ProjectID, GenerationInputsJSON: s.GenerationInputsJSON}
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func int64PtrOrNil(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}
func valueOrZero(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

// Package store 定义持久化契约（interface），具体实现（GORM/FS）可替换（ARCH-BACKEND-001）。
package store

import (
	"context"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

var (
	ErrRunActive       = errors.New("store: project has an active run")
	ErrGitCommitActive = errors.New("store: project has an active Git commit")

	ErrResourceNotFound        = errors.New("store: resource not found")
	ErrResourceConflict        = errors.New("store: resource already exists")
	ErrTagNotFound             = errors.New("store: tag not found")
	ErrNamingOperationConflict = errors.New("store: naming operation conflict")
	ErrCommandActivityConflict = errors.New("store: command activity already running or belongs to another command")
)

// Store 是持久化层对外暴露的接口。随里程碑推进逐步扩展领域方法。
type Store interface {
	Health(ctx context.Context) error

	CreateProject(ctx context.Context, p model.Project) error
	GetProject(ctx context.Context, id string) (model.Project, error)
	ListProjects(ctx context.Context) ([]model.Project, error)
	DeleteProject(ctx context.Context, id string) error
	UpdateProjectTitle(ctx context.Context, id, title string, updatedAt int64) error

	CreateThread(ctx context.Context, t model.Thread) error
	GetThread(ctx context.Context, id string) (model.Thread, error)
	ListThreads(ctx context.Context, projectID string) ([]model.Thread, error)
	DeleteThread(ctx context.Context, id string) error
	RecordThreadNamingInput(ctx context.Context, input model.ThreadNamingInput) (model.Thread, bool, error)
	BeginThreadRenameRequest(ctx context.Context, id string, expectedOperationVersion int64, resetInputCount bool, updatedAt int64) (model.Thread, error)
	StartThreadExplicitRenameRequest(ctx context.Context, id string, operationVersion int64, updatedAt int64) (model.Thread, error)
	ApplyThreadRenameResult(ctx context.Context, id, title string, operationVersion int64, updatedAt int64) (model.Thread, bool, error)
	UpdateThreadNamingState(ctx context.Context, id string, title *string, enabled *bool, resetInputCount bool, updatedAt int64) (model.Thread, error)
	GetThreadNamingOperation(ctx context.Context, threadID, operationID string) (model.ThreadNamingOperation, error)
	CreateThreadNamingOperation(ctx context.Context, operation model.ThreadNamingOperation) (bool, error)
	CompleteThreadNamingOperation(ctx context.Context, operation model.ThreadNamingOperation) error
	ListThreadNamingInputs(ctx context.Context, threadID string, limit int) ([]model.ThreadNamingInput, error)
	LoadThreadRenameContext(ctx context.Context, threadID string) (model.ThreadRenameContextSource, error)

	CreateRun(ctx context.Context, r model.Run) error
	GetRun(ctx context.Context, id string) (model.Run, error)
	SetRunStatus(ctx context.Context, id string, status model.RunStatus) error
	PauseNonTerminalRuns(ctx context.Context, reason string, pausedAt int64) ([]model.Run, error)
	PauseRun(ctx context.Context, id, ownerInstanceID, reason string, pausedAt int64) (model.Run, error)
	ClaimPausedRun(ctx context.Context, id, ownerInstanceID string) (model.Run, error)
	ReleaseRecoveringRun(ctx context.Context, id, ownerInstanceID, reason string, pausedAt int64) error
	CancelPausedRun(ctx context.Context, id string, canceledAt int64) (model.Run, error)
	UpdateRunMode(ctx context.Context, id string, mode model.RunMode) error
	RequestRunCancel(ctx context.Context, id string, requestedAt int64) (model.Run, error)
	AcquireIdempotency(ctx context.Context, record model.IdempotencyRecord) (model.IdempotencyRecord, bool, error)
	CompleteIdempotency(ctx context.Context, scope, ownerID, key, status, resultJSON string) error
	GetIdempotency(ctx context.Context, scope, ownerID, key string) (model.IdempotencyRecord, error)
	CreateSteering(ctx context.Context, message model.SteeringMessage) (model.SteeringMessage, bool, error)
	ListPendingSteering(ctx context.Context, runID string) ([]model.SteeringMessage, error)
	ListThreadSteering(ctx context.Context, threadID string) ([]model.SteeringMessage, error)
	MarkSteering(ctx context.Context, runID string, ids []string, status model.SteeringStatus, at int64, rejectionCode string) error
	HasActiveRun(ctx context.Context, projectID string) (bool, error)
	AppendEvent(ctx context.Context, e model.Event) error
	EventsSince(ctx context.Context, runID string, afterSeq int64) ([]model.Event, error)
	ListThreadEvents(ctx context.Context, threadID string) ([]model.Event, error)
	SaveRunContext(ctx context.Context, manifest model.RunContext) error
	GetRunContext(ctx context.Context, runID string) (model.RunContext, error)

	CreateGitCommitOperation(ctx context.Context, operation model.GitCommitOperation) error
	GetGitCommitOperation(ctx context.Context, id string) (model.GitCommitOperation, error)
	GetGitCommitOperationByRequest(ctx context.Context, threadID, clientRequestID string) (model.GitCommitOperation, error)
	UpdateGitCommitOperation(ctx context.Context, operation model.GitCommitOperation) error
	HasActiveGitCommit(ctx context.Context, projectID string) (bool, error)
	ListActiveGitCommits(ctx context.Context) ([]model.GitCommitOperation, error)
	AppendGitCommitEvent(ctx context.Context, event model.GitCommitEvent) error
	GitCommitEventsSince(ctx context.Context, operationID string, afterSeq int64) ([]model.GitCommitEvent, error)
	ListThreadGitCommits(ctx context.Context, threadID string) ([]model.GitCommitOperation, error)

	AppendBriefingVersion(ctx context.Context, version model.BriefingVersion) error
	ListThreadBriefings(ctx context.Context, threadID string) ([]model.Briefing, error)
	GetBriefingVersions(ctx context.Context, briefingID string, limit int) ([]model.BriefingVersion, error)
	BeginCommandActivity(ctx context.Context, activity model.CommandActivity) (model.CommandActivity, error)
	SaveCommandActivity(ctx context.Context, activity model.CommandActivity) error
	ListThreadCommandActivities(ctx context.Context, threadID string) ([]model.CommandActivity, error)

	SetProjectStatus(ctx context.Context, id, status string) error
	ReplaceSlides(ctx context.Context, projectID string, slides []model.Slide) error
	ListSlides(ctx context.Context, projectID string) ([]model.Slide, error)
	GetSlide(ctx context.Context, id string) (model.Slide, error)
	InsertSlide(ctx context.Context, sl model.Slide) error
	DeleteSlideByID(ctx context.Context, slideID string) error
	IsSlideDeleted(ctx context.Context, projectID, slideID string) (bool, error)
	CommitWorkflow(ctx context.Context, commit model.ArtifactCommit) error
	UpdateProjectTheme(ctx context.Context, id, theme string, updatedAt int64) error

	CreateResource(ctx context.Context, resource model.Resource) error
	GetResource(ctx context.Context, resourceType, id string) (model.Resource, error)
	ListResources(ctx context.Context, resourceType string) ([]model.Resource, error)
	UpdateResource(ctx context.Context, resource model.Resource) error
	DeleteResource(ctx context.Context, resourceType, id string) error
}

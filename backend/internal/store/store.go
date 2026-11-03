// Package store 定义持久化契约（interface），具体实现（GORM/FS）可替换（ARCH-BACKEND-001）。
package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/decoration"
	"github.com/dasi0227/PPT-Agent/backend/internal/gitcommit"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

var (
	ErrRunActive       = errors.New("store: project has an active run")
	ErrGitCommitActive = errors.New("store: project has an active Git commit")

	ErrResourceNotFound        = errors.New("store: resource not found")
	ErrResourceConflict        = errors.New("store: resource already exists")
	ErrTagNotFound             = errors.New("store: tag not found")
	ErrNamingOperationConflict = errors.New("store: naming operation conflict")
	ErrCommandConflict         = errors.New("store: command attempt is active, stale, or belongs to another command")
	ErrDeletionExpired         = errors.New("撤销期限已过，无法恢复")
	ErrDeletionPending         = errors.New("项目存在待删除内容，请稍后重试")
)

// Deletion receipts outlive their target rows and project history. Pending
// receipts also form the durable queue for filesystem cleanup after restart.
type Deletion struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Token     string `json:"token"`
	Title     string `json:"title"`
	UndoUntil int64  `json:"undo_until"`
	Status    string `json:"-"`
}

// Store 是持久化层对外暴露的接口。随里程碑推进逐步扩展领域方法。
type Store interface {
	sessionstate.Reader
	sessionstate.ToolExecutionReader
	Health(ctx context.Context) error
	ThreadSceneRevision(context.Context, string) (int64, error)
	LoadContext(context.Context, string) (sessionstate.ContextSnapshot, error)
	LoadContextVersion(context.Context, string, int64) (sessionstate.ContextSnapshot, error)
	LatestContextRun(context.Context, string) (*model.Run, error)
	HasBlockingCommand(context.Context, string) (bool, error)
	PendingDeletions(context.Context) ([]Deletion, error)
	GetDeletion(context.Context, string, string) (Deletion, error)
	RequestDeletionRestore(context.Context, Deletion) error
	FinishDeletionRestore(context.Context, Deletion) error
	FinalizeDeletion(context.Context, Deletion) (bool, error)
	QuiesceDeletion(context.Context, Deletion) error
	HasPendingDeletion(context.Context, string) (bool, error)
	CompleteDeletion(context.Context, Deletion) error
	PurgeDeletion(context.Context, Deletion) error
	ProjectDeleted(context.Context, string) (bool, error)
	DeletedThreads(context.Context, string) ([]string, error)

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
	UpdateThreadNamingState(ctx context.Context, id string, title *string, enabled *bool, resetInputCount bool, updatedAt int64) (model.Thread, error)
	UpdateThreadSuggestState(context.Context, string, bool) (model.Thread, error)
	CheckSuggestCommand(context.Context, model.CommandExecution) error
	LatestThreadPlan(ctx context.Context, threadID string) (model.ThreadPlanSnapshot, error)
	ClearThreadPlan(ctx context.Context, threadID string, expectedRevision int64, requestKey string) (sessionstate.Receipt, error)
	LoadThreadConversationContext(ctx context.Context, threadID string) (model.ThreadConversationContext, error)
	LoadRecentThreadConversationContext(ctx context.Context, threadID string) (model.ThreadConversationContext, error)
	LoadThreadConversationContextBeforeInput(ctx context.Context, threadID, runID, inputID string) (model.ThreadConversationContext, error)

	CreateRun(ctx context.Context, r model.Run) error
	GetRun(ctx context.Context, id string) (model.Run, error)
	Interaction(context.Context, string, string, string) (model.Interaction, error)
	ResourceDraftReceipt(context.Context, string, string, int64, json.RawMessage) (model.ResourceEditApproval, bool, error)
	UpdateResourceInteraction(context.Context, model.ResourceEditApproval, json.RawMessage) error
	SetRunStatus(ctx context.Context, id string, status model.RunStatus) error
	PauseNonTerminalRuns(ctx context.Context, reason string, pausedAt int64) ([]model.Run, error)
	PauseRun(ctx context.Context, id, ownerInstanceID, reason string, pausedAt int64) (model.Run, error)
	RunResumeReceipt(context.Context, string, sessionstate.RunAction) (sessionstate.RunMutation, bool, error)
	ClaimRunForResume(context.Context, model.Run, string, sessionstate.RunAction) (sessionstate.RunMutation, bool, error)
	RequestRunCancel(context.Context, string, model.RunCancelReason, sessionstate.RunAction) (sessionstate.RunMutation, error)
	GetIdempotency(ctx context.Context, scope, ownerID, key string) (model.IdempotencyRecord, error)
	CreateSteering(ctx context.Context, message model.SteeringMessage) (model.SteeringMessage, bool, error)
	ListPendingSteering(ctx context.Context, runID string) ([]model.SteeringMessage, error)
	MarkSteering(ctx context.Context, runID string, ids []string, status model.SteeringStatus, at int64, rejectionCode string) error
	HasActiveRun(ctx context.Context, projectID string) (bool, error)
	ApplyRunUpdate(context.Context, string, sessionstate.RunUpdate) (sessionstate.Receipt, error)

	AcceptCommand(context.Context, string, model.CommandRequest, int64) (sessionstate.CommandMutation, bool, error)
	GetCommand(context.Context, string) (model.CommandExecution, error)
	SaveCommandExecution(context.Context, model.CommandExecution) error
	CompleteCommand(context.Context, model.CommandExecution, sessionstate.CommandEffects) (model.CommandExecution, error)
	RequestCommandCancel(context.Context, string, string, string) (sessionstate.CommandMutation, error)
	ListActiveCommitCommands(context.Context) ([]model.CommandExecution, error)
	PrepareGitCommit(context.Context, model.CommandExecution, gitcommit.CommitIntent) error
	ClaimCommitRecovery(context.Context, model.CommandExecution) (model.CommandExecution, error)
	HasActiveGitCommit(ctx context.Context, projectID string) (bool, error)

	LatestBriefingVersion(ctx context.Context, threadID, briefingID string) (model.BriefingVersion, error)

	ReplaceSlides(ctx context.Context, projectID string, slides []model.Slide) error
	ListSlides(ctx context.Context, projectID string) ([]model.Slide, error)
	GetSlide(ctx context.Context, id string) (model.Slide, error)
	CommitWorkflow(ctx context.Context, commit model.ArtifactCommit) error
	UpdateProjectTheme(ctx context.Context, id, theme string, updatedAt int64) error
	UpdateProjectDecorations(ctx context.Context, id string, positions decoration.Positions, updatedAt int64) error
	UpdateProjectLogo(ctx context.Context, id string, logo *decoration.LogoMetadata, updatedAt int64) error

	CreateResource(ctx context.Context, resource model.Resource) error
	GetResource(ctx context.Context, resourceType, id string) (model.Resource, error)
	ListResources(ctx context.Context, resourceType string) ([]model.Resource, error)
	UpdateResource(ctx context.Context, resource model.Resource) error
	DeleteResource(ctx context.Context, resourceType, id string) error
}

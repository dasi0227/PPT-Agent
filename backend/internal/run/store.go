package run

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

// Store 是 run 外壳所需的持久化能力（在消费方定义接口，避免反向依赖 ARCH-BACKEND）。
// Public facts are projected into current entities; recovery reads checkpoints.
type Store interface {
	CreateRun(ctx context.Context, r model.Run) error
	GetRun(ctx context.Context, id string) (model.Run, error)
	SetRunStatus(ctx context.Context, id string, status model.RunStatus) error
	RequestRunCancel(context.Context, string, model.RunCancelReason, sessionstate.RunAction) (sessionstate.RunMutation, error)
	CreateSteering(ctx context.Context, message model.SteeringMessage) (model.SteeringMessage, bool, error)
	ListPendingSteering(ctx context.Context, runID string) ([]model.SteeringMessage, error)
	MarkSteering(ctx context.Context, runID string, ids []string, status model.SteeringStatus, at int64, rejectionCode string) error
	ApplyRunUpdate(context.Context, string, sessionstate.RunUpdate) (sessionstate.Receipt, error)
}

// InputReplyStore commits a direct reply and its input receipt together.
type InputReplyStore interface {
	CommitInputReply(context.Context, string, string, sessionstate.RunUpdate) (sessionstate.Receipt, error)
}

// LifecycleStore owns durable worker ownership and lifecycle transitions.
type LifecycleStore interface {
	PauseNonTerminalRuns(ctx context.Context, reason string, pausedAt int64) ([]model.Run, error)
	PauseRun(ctx context.Context, id, ownerInstanceID, reason string, pausedAt int64) (model.Run, error)
}

type ResumeStore interface {
	RunResumeReceipt(context.Context, string, sessionstate.RunAction) (sessionstate.RunMutation, bool, error)
	ClaimRunForResume(context.Context, model.Run, string, sessionstate.RunAction) (sessionstate.RunMutation, bool, error)
}

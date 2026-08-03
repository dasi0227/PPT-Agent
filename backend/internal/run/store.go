package run

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Store 是 run 外壳所需的持久化能力（在消费方定义接口，避免反向依赖 ARCH-BACKEND）。
// run_events 持久化用于断线重连续传（ARCH-RUN-005 / API-SSE-003）。
type Store interface {
	CreateRun(ctx context.Context, r model.Run) error
	GetRun(ctx context.Context, id string) (model.Run, error)
	SetRunStatus(ctx context.Context, id string, status model.RunStatus) error
	RequestRunCancel(ctx context.Context, id string, requestedAt int64) (model.Run, error)
	CreateSteering(ctx context.Context, message model.SteeringMessage) (model.SteeringMessage, bool, error)
	ListPendingSteering(ctx context.Context, runID string) ([]model.SteeringMessage, error)
	MarkSteering(ctx context.Context, runID string, ids []string, status model.SteeringStatus, at int64, rejectionCode string) error
	// AppendEvent 持久化一条事件（seq 由调用方分配，单调连续 API-SSE-001）。
	AppendEvent(ctx context.Context, e model.Event) error
	// EventsSince 返回 seq > afterSeq 的事件（升序），用于 Last-Event-ID 续传。
	EventsSince(ctx context.Context, runID string, afterSeq int64) ([]model.Event, error)
}

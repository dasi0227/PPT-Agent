// Package store 定义持久化契约（interface），具体实现（GORM/FS）可替换（ARCH-BACKEND-001）。
package store

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Store 是持久化层对外暴露的接口。随里程碑推进逐步扩展领域方法。
type Store interface {
	Health(ctx context.Context) error

	CreateProject(ctx context.Context, p model.Project) error
	GetProject(ctx context.Context, id string) (model.Project, error)

	CreateThread(ctx context.Context, t model.Thread) error
	GetThread(ctx context.Context, id string) (model.Thread, error)

	CreateRun(ctx context.Context, r model.Run) error
	GetRun(ctx context.Context, id string) (model.Run, error)
	SetRunStatus(ctx context.Context, id string, status model.RunStatus) error
	AppendEvent(ctx context.Context, e model.Event) error
	EventsSince(ctx context.Context, runID string, afterSeq int64) ([]model.Event, error)
}

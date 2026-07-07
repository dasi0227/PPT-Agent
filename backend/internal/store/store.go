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
	ListProjects(ctx context.Context) ([]model.Project, error)
	DeleteProject(ctx context.Context, id string) error

	CreateThread(ctx context.Context, t model.Thread) error
	GetThread(ctx context.Context, id string) (model.Thread, error)
	ListThreads(ctx context.Context, projectID string) ([]model.Thread, error)
	DeleteThread(ctx context.Context, id string) error

	CreateRun(ctx context.Context, r model.Run) error
	GetRun(ctx context.Context, id string) (model.Run, error)
	SetRunStatus(ctx context.Context, id string, status model.RunStatus) error
	HasActiveRun(ctx context.Context, projectID string) (bool, error)
	AppendEvent(ctx context.Context, e model.Event) error
	EventsSince(ctx context.Context, runID string, afterSeq int64) ([]model.Event, error)

	SetProjectStatus(ctx context.Context, id, status string) error
	ReplaceSlides(ctx context.Context, projectID string, slides []model.Slide) error
	ListSlides(ctx context.Context, projectID string) ([]model.Slide, error)
	GetSlide(ctx context.Context, id string) (model.Slide, error)
	InsertSlide(ctx context.Context, sl model.Slide) error
	DeleteSlideByID(ctx context.Context, slideID string) error
	SetSlidesOrder(ctx context.Context, projectID string, orderByID map[string]int) error
	NextVersionNo(ctx context.Context, targetType, targetID string) (int, error)
	CreateVersion(ctx context.Context, v model.Version) error
	ListVersions(ctx context.Context, targetType, targetID string) ([]model.Version, error)
	DeleteVersion(ctx context.Context, targetType, targetID string, versionNo int) error
	SetSlideVersion(ctx context.Context, slideID string, versionNo int) error
	SetOutlineDirty(ctx context.Context, slideID string, dirty bool) error
	UpdateSlideMeta(ctx context.Context, slideID, title, layout string) error

	CreateAsset(ctx context.Context, a model.Asset) error
	UpsertAsset(ctx context.Context, a model.Asset) error
	ListAssets(ctx context.Context, kind string) ([]model.Asset, error)
	CountAssets(ctx context.Context) (int, error)
	GetAsset(ctx context.Context, id string) (model.Asset, error)
	DeleteAsset(ctx context.Context, id string) error
}

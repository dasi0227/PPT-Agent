package edit

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Store 是编辑落库所需的最小持久化能力（消费方定义接口，避免反向依赖）。
type Store interface {
	ListSlides(ctx context.Context, projectID string) ([]model.Slide, error)
	ListAssets(ctx context.Context, kind string) ([]model.Asset, error)
	GetAsset(ctx context.Context, id string) (model.Asset, error)
	NextVersionNo(ctx context.Context, targetType, targetID string) (int, error)
	CreateVersion(ctx context.Context, v model.Version) error
	DeleteVersion(ctx context.Context, targetType, targetID string, versionNo int) error
	SetSlideVersion(ctx context.Context, slideID string, versionNo int) error
}

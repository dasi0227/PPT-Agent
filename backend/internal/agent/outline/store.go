package outline

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Store 是 outline 落库所需的最小持久化能力（消费方定义接口，避免反向依赖）。
type Store interface {
	ReplaceSlides(ctx context.Context, projectID string, slides []model.Slide) error
	NextVersionNo(ctx context.Context, targetType, targetID string) (int, error)
	CreateVersion(ctx context.Context, v model.Version) error
}

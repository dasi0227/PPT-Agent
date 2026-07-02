package assist

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Store 是 /recap 只读汇总所需的最小持久化能力（消费方定义接口）。
type Store interface {
	GetProject(ctx context.Context, id string) (model.Project, error)
	ListSlides(ctx context.Context, projectID string) ([]model.Slide, error)
	ListVersions(ctx context.Context, targetType, targetID string) ([]model.Version, error)
}

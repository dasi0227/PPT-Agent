package overview

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Store 是 overview 落库所需的最小持久化能力（消费方定义接口，避免反向依赖）。
// 方法集与 edit.Store 一致，使跨页子代理可复用 edit.PatchSlideTool。
type Store interface {
	NextVersionNo(ctx context.Context, targetType, targetID string) (int, error)
	CreateVersion(ctx context.Context, v model.Version) error
	SetSlideVersion(ctx context.Context, projectID string, idx, versionNo int) error
}

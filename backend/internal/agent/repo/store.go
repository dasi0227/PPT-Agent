package repo

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Store 是 /repo 资产操作所需的最小持久化能力（消费方定义接口，避免反向依赖）。
// M5 只做 read/search/patch/validate（create/delete 留 M6，见 repo.md 偏差说明）。
type Store interface {
	ListAssets(ctx context.Context, kind string) ([]model.Asset, error)
	GetAsset(ctx context.Context, id string) (model.Asset, error)
	UpsertAsset(ctx context.Context, a model.Asset) error
}

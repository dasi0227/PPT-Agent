package repo

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Store 是 /repo 资产操作所需的最小持久化能力（消费方定义接口，避免反向依赖）。
// M5 只做 read/search/patch/validate（create/delete 留 M6，见 repo.md 偏差说明）。
type Store interface {
	ListAssets(ctx context.Context, kind string) ([]model.Asset, error)
	GetAsset(ctx context.Context, id string) (model.Asset, error)
	UpsertAsset(ctx context.Context, a model.Asset) error
}

// AssetManager 是 /repo 写工具依赖的资产业务服务接口。
// 由 service 层 adapter 实现，保证 REST 与 agent 工具共享同一套校验、落盘和版本化逻辑。
type AssetManager interface {
	CreateAsset(ctx context.Context, manifest asset.Manifest, payload map[string]string) (model.Asset, error)
	PatchAsset(ctx context.Context, id, file string, edits []AssetEdit) (model.Asset, error)
	DeleteAsset(ctx context.Context, id string) error
}

type AssetEdit struct {
	OldText string
	NewText string
}

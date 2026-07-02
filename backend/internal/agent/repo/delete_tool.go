package repo

import (
	"context"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// DeleteAssetTool 删除用户资产。仅 repo scope；preset 删除由 service adapter 拒绝。
type DeleteAssetTool struct {
	assets AssetManager
}

func NewDeleteAssetTool(assets AssetManager) *DeleteAssetTool {
	return &DeleteAssetTool{assets: assets}
}

func (t *DeleteAssetTool) Name() string          { return "delete_asset" }
func (t *DeleteAssetTool) Class() tools.Class    { return tools.ClassWrite }
func (t *DeleteAssetTool) Scopes() []model.Scope { return []model.Scope{model.ScopeRepo} }

func (t *DeleteAssetTool) Description() string {
	return "删除个人仓库中的 user 资产。preset 出厂资产禁止删除；删除前会产生资产版本快照。"
}

func (t *DeleteAssetTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"asset_id"},
		"properties": map[string]any{
			"asset_id": map[string]any{"type": "string", "description": "资产 id"},
		},
	}
}

func (t *DeleteAssetTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	if t.assets == nil {
		return fail("资产服务未初始化"), nil
	}
	id, _ := args["asset_id"].(string)
	if id == "" {
		return fail("参数错误：asset_id 为空"), nil
	}
	if err := t.assets.DeleteAsset(ctx, id); err != nil {
		return fail("删除资产失败：" + err.Error()), nil
	}
	return tools.Result{
		OK:          true,
		Observation: fmt.Sprintf("资产 %s 已删除，并已保留删除前版本快照", id),
		Artifact:    &tools.Artifact{Type: "asset", Ref: id},
	}, nil
}

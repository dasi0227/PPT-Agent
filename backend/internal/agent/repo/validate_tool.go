package repo

import (
	"context"
	"fmt"
	"path"

	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ValidateAssetTool 校验某资产的 manifest 是否符合资产协议 schema（validate_asset）。仅 repo scope。
// theme 资产另校 token 全集（AC-ASSET-002）。供修改后自查；patch_asset 落盘前也隐式再校验。
type ValidateAssetTool struct {
	store   Store
	sandbox *tools.Sandbox
}

func NewValidateAssetTool(store Store, sandbox *tools.Sandbox) *ValidateAssetTool {
	return &ValidateAssetTool{store: store, sandbox: sandbox}
}

func (t *ValidateAssetTool) Name() string          { return "validate_asset" }
func (t *ValidateAssetTool) Class() tools.Class    { return tools.ClassValidate }
func (t *ValidateAssetTool) Scopes() []model.Scope { return []model.Scope{model.ScopeRepo} }

func (t *ValidateAssetTool) Description() string {
	return "校验指定资产是否符合资产协议 schema（theme 另校 token 全集），返回逐项结果（不落盘）。"
}

func (t *ValidateAssetTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"asset_id"},
		"properties": map[string]any{
			"asset_id": map[string]any{"type": "string", "description": "资产 id"},
		},
	}
}

func (t *ValidateAssetTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	id, _ := args["asset_id"].(string)
	if id == "" {
		return fail("参数错误：asset_id 为空"), nil
	}
	a, err := t.store.GetAsset(ctx, id)
	if err != nil {
		return fail("资产不存在：" + err.Error()), nil
	}
	manifestRaw, err := t.sandbox.Read(a.ManifestPath)
	if err != nil {
		return fail("读取 manifest 失败：" + err.Error()), nil
	}
	if err := asset.ValidateManifestRaw(manifestRaw); err != nil {
		return tools.Result{OK: false, Observation: "manifest schema 校验未通过：" + err.Error()}, nil
	}
	m, err := asset.Parse(manifestRaw)
	if err != nil {
		return fail("解析 manifest 失败：" + err.Error()), nil
	}
	if m.Kind == asset.KindTheme && m.Assets.Tokens != "" {
		tok, err := t.sandbox.Read(path.Join(a.Dir, m.Assets.Tokens))
		if err != nil {
			return fail("读取 tokens 失败：" + err.Error()), nil
		}
		if miss := asset.ValidateThemeTokens(tok); len(miss) != 0 {
			return tools.Result{OK: false, Observation: fmt.Sprintf("theme 缺少必需 token：%v", miss)}, nil
		}
	}
	return tools.Result{OK: true, Observation: "校验通过：资产协议合规"}, nil
}

package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// CreateAssetTool 新增个人仓库资产。仅 repo scope。
// 具体校验、source=user 强制、文件落盘和资产版本化由 AssetManager（service adapter）负责。
type CreateAssetTool struct {
	assets AssetManager
}

func NewCreateAssetTool(assets AssetManager) *CreateAssetTool {
	return &CreateAssetTool{assets: assets}
}

func (t *CreateAssetTool) Name() string          { return "create_asset" }
func (t *CreateAssetTool) Class() tools.Class    { return tools.ClassWrite }
func (t *CreateAssetTool) Scopes() []model.Scope { return []model.Scope{model.ScopeRepo} }

func (t *CreateAssetTool) Description() string {
	return "在个人仓库新增一个 layout/component/theme/fx 资产。manifest 必须符合资产协议；服务端会强制 source=user 并产资产版本。"
}

func (t *CreateAssetTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"manifest", "payload"},
		"properties": map[string]any{
			"manifest": map[string]any{"type": "object", "description": "统一资产 manifest"},
			"payload":  map[string]any{"type": "object", "description": "载荷文件名到文本内容的映射"},
		},
	}
}

func (t *CreateAssetTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	if t.assets == nil {
		return fail("资产服务未初始化"), nil
	}
	m, err := parseManifestArg(args["manifest"])
	if err != nil {
		return fail(err.Error()), nil
	}
	payload, err := parsePayloadArg(args["payload"])
	if err != nil {
		return fail(err.Error()), nil
	}
	a, err := t.assets.CreateAsset(ctx, m, payload)
	if err != nil {
		return fail("新增资产失败：" + err.Error()), nil
	}
	return tools.Result{
		OK:          true,
		Observation: fmt.Sprintf("资产已创建：id=%s name=%s kind=%s source=%s，并已产生资产版本", a.ID, a.Name, a.Kind, a.Source),
		Artifact:    &tools.Artifact{Type: "asset", Ref: a.Dir},
	}, nil
}

func parseManifestArg(v any) (asset.Manifest, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return asset.Manifest{}, fmt.Errorf("参数错误：manifest 非法：%w", err)
	}
	var m asset.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return asset.Manifest{}, fmt.Errorf("参数错误：manifest 解析失败：%w", err)
	}
	return m, nil
}

func parsePayloadArg(v any) (map[string]string, error) {
	mp, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("参数错误：payload 必须为对象")
	}
	out := make(map[string]string, len(mp))
	for k, raw := range mp {
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("参数错误：payload[%s] 必须为字符串", k)
		}
		out[k] = s
	}
	return out, nil
}

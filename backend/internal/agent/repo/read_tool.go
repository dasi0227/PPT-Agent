package repo

import (
	"context"
	"fmt"
	"path"

	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ReadAssetTool 读取某资产的 manifest 与载荷内容（read_asset，只读）。仅 repo scope。
type ReadAssetTool struct {
	store   Store
	sandbox *tools.Sandbox // 根为 work_root（_assets 所在）
}

func NewReadAssetTool(store Store, sandbox *tools.Sandbox) *ReadAssetTool {
	return &ReadAssetTool{store: store, sandbox: sandbox}
}

func (t *ReadAssetTool) Name() string          { return "read_asset" }
func (t *ReadAssetTool) Class() tools.Class    { return tools.ClassRead }
func (t *ReadAssetTool) Scopes() []model.Scope { return []model.Scope{model.ScopeRepo} }

func (t *ReadAssetTool) Description() string {
	return "读取指定资产的 manifest 与载荷（html/css/js/tokens）文本，用于确认要修改的锚点上下文。"
}

func (t *ReadAssetTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"asset_id"},
		"properties": map[string]any{
			"asset_id": map[string]any{"type": "string", "description": "资产 id（来自 search_assets）"},
		},
	}
}

func (t *ReadAssetTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
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
	m, err := asset.Parse(manifestRaw)
	if err != nil {
		return fail("解析 manifest 失败：" + err.Error()), nil
	}

	out := fmt.Sprintf("资产 %s（kind=%s）manifest:\n%s\n", a.Name, a.Kind, string(manifestRaw))
	for _, pf := range payloadFiles(m) {
		body, err := t.sandbox.Read(path.Join(a.Dir, pf))
		if err != nil {
			continue
		}
		out += fmt.Sprintf("\n载荷文件 %s:\n%s\n", pf, string(body))
	}
	return tools.Result{OK: true, Observation: out}, nil
}

// payloadFiles 返回 manifest 声明的载荷相对文件名（按 kind 取用不同字段）。
func payloadFiles(m asset.Manifest) []string {
	var fs []string
	for _, f := range []string{m.Assets.HTML, m.Assets.CSS, m.Assets.JS, m.Assets.Tokens} {
		if f != "" {
			fs = append(fs, f)
		}
	}
	return fs
}

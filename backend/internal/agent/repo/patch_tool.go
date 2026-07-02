package repo

import (
	"context"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// PatchAssetTool 对某资产的一个载荷文件做锚定文本替换（patch_asset）。仅 repo scope。
// 语义（SPEC-CMD-REPO-001/003/004, ARCH-TOOLS-003/006）：
//   - 只改 _assets/<asset.Dir> 下的载荷（路径边界=work_root），MUST NOT 触碰任何 PPT 页/公共层；
//   - old_text 必须在目标载荷文件内**唯一**出现，否则整体失败；
//   - 修改后重新校验资产 manifest schema（theme 另校 token 全集），不合规拒绝落盘。
//
// M5 最小实现：patch_asset 足以验证「圆角调大→仅该资产变」的隔离（AC-CMD-REPO-001）。
// 资产版本化（SPEC-CMD-REPO-005，SHOULD）与 create/delete 留 M6。
type PatchAssetTool struct {
	assets  AssetManager
	patched bool
}

func NewPatchAssetTool(assets AssetManager) *PatchAssetTool {
	return &PatchAssetTool{assets: assets}
}

func (t *PatchAssetTool) Name() string          { return "patch_asset" }
func (t *PatchAssetTool) Class() tools.Class    { return tools.ClassWrite }
func (t *PatchAssetTool) Scopes() []model.Scope { return []model.Scope{model.ScopeRepo} }
func (t *PatchAssetTool) Patched() bool         { return t.patched }

func (t *PatchAssetTool) Description() string {
	return "对指定资产的某个载荷文件（html/css/js/tokens）做锚定文本替换。" +
		"old_text 必须在该文件内唯一出现，否则整体失败。修改后系统会重新校验资产协议，不合规会拒绝落盘。"
}

func (t *PatchAssetTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"asset_id", "file", "edits"},
		"properties": map[string]any{
			"asset_id": map[string]any{"type": "string", "description": "资产 id"},
			"file":     map[string]any{"type": "string", "enum": []any{"html", "css", "js", "tokens"}, "description": "要修改的载荷文件类别"},
			"edits": map[string]any{
				"type":     "array",
				"minItems": 1,
				"items": map[string]any{
					"type":     "object",
					"required": []any{"old_text", "new_text"},
					"properties": map[string]any{
						"old_text": map[string]any{"type": "string", "description": "锚点：该载荷文件内唯一出现的原文片段"},
						"new_text": map[string]any{"type": "string", "description": "替换后的新文本"},
					},
				},
			},
		},
	}
}

func (t *PatchAssetTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	if t.assets == nil {
		return fail("资产服务未初始化"), nil
	}
	id, _ := args["asset_id"].(string)
	fileKind, _ := args["file"].(string)
	if id == "" {
		return fail("参数错误：asset_id 为空"), nil
	}
	edits, err := parseEdits(args["edits"])
	if err != nil {
		return fail(err.Error()), nil
	}

	serviceEdits := make([]AssetEdit, len(edits))
	for i, e := range edits {
		serviceEdits[i] = AssetEdit{OldText: e.oldText, NewText: e.newText}
	}
	a, err := t.assets.PatchAsset(ctx, id, fileKind, serviceEdits)
	if err != nil {
		return fail("修改资产失败：" + err.Error()), nil
	}

	t.patched = true
	return tools.Result{
		OK:          true,
		Observation: fmt.Sprintf("资产 %s 的 %s 载荷已应用 %d 处替换并产生版本（未触碰任何 PPT 页/公共层）", a.Name, fileKind, len(edits)),
		Artifact:    &tools.Artifact{Type: "asset", Ref: a.Dir},
	}, nil
}

// ── 共享小工具 ──

type assetEdit struct {
	oldText string
	newText string
}

func parseEdits(v any) ([]assetEdit, error) {
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf("参数错误：edits 必须为非空数组")
	}
	out := make([]assetEdit, 0, len(list))
	for i, item := range list {
		mp, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("参数错误：edit #%d 非对象", i+1)
		}
		ot, _ := mp["old_text"].(string)
		nt, _ := mp["new_text"].(string)
		if ot == "" {
			return nil, fmt.Errorf("参数错误：edit #%d 缺少 old_text", i+1)
		}
		out = append(out, assetEdit{oldText: ot, newText: nt})
	}
	return out, nil
}

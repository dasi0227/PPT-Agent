package repo

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
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
	store   Store
	sandbox *tools.Sandbox // 根为 work_root
	clock   func() int64
	patched bool
}

func NewPatchAssetTool(store Store, sandbox *tools.Sandbox, clock func() int64) *PatchAssetTool {
	return &PatchAssetTool{store: store, sandbox: sandbox, clock: clock}
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
	id, _ := args["asset_id"].(string)
	fileKind, _ := args["file"].(string)
	if id == "" {
		return fail("参数错误：asset_id 为空"), nil
	}
	edits, err := parseEdits(args["edits"])
	if err != nil {
		return fail(err.Error()), nil
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

	payloadRel, err := payloadRelForKind(m, fileKind)
	if err != nil {
		return fail(err.Error()), nil
	}
	fileRel := path.Join(a.Dir, payloadRel)

	raw, err := t.sandbox.Read(fileRel)
	if err != nil {
		return fail(fmt.Sprintf("读取载荷 %s 失败：%v", payloadRel, err)), nil
	}
	content := string(raw)

	// 锚定替换：每个 old_text 必须唯一（ARCH-TOOLS-003）。
	for i, e := range edits {
		n := strings.Count(content, e.oldText)
		if n == 0 {
			return fail(fmt.Sprintf("锚点不存在（edit #%d），请补充上下文", i+1)), nil
		}
		if n > 1 {
			return fail(fmt.Sprintf("锚点不唯一（edit #%d 出现 %d 次），请补充上下文", i+1, n)), nil
		}
		content = strings.Replace(content, e.oldText, e.newText, 1)
	}

	// theme 的 tokens 载荷：改后重新校验 token 全集（AC-ASSET-002 / SPEC-CMD-REPO-003）。
	if m.Kind == asset.KindTheme && fileKind == "tokens" {
		if miss := asset.ValidateThemeTokens([]byte(content)); len(miss) != 0 {
			return fail(fmt.Sprintf("校验失败，拒绝落盘：theme 缺少必需 token %v", miss)), nil
		}
	}

	// 路径边界（ARCH-TOOLS-006）：Sandbox.Write 限定 work_root，越界拒绝。
	if err := t.sandbox.Write(fileRel, []byte(content)); err != nil {
		return fail(fmt.Sprintf("写入载荷失败：%v", err)), nil
	}

	// 触碰 updated_at（元数据变更）；资产版本化留 M6。
	a.UpdatedAt = t.clock()
	if err := t.store.UpsertAsset(ctx, a); err != nil {
		return tools.Result{}, err
	}

	t.patched = true
	return tools.Result{
		OK:          true,
		Observation: fmt.Sprintf("资产 %s 的 %s 载荷已应用 %d 处替换（仅该资产变更，未触碰任何 PPT 页/公共层）", a.Name, fileKind, len(edits)),
		Artifact:    &tools.Artifact{Type: "asset", Ref: fileRel},
	}, nil
}

// payloadRelForKind 按 file 类别返回 manifest 声明的载荷相对文件名。
func payloadRelForKind(m asset.Manifest, fileKind string) (string, error) {
	var rel string
	switch fileKind {
	case "html":
		rel = m.Assets.HTML
	case "css":
		rel = m.Assets.CSS
	case "js":
		rel = m.Assets.JS
	case "tokens":
		rel = m.Assets.Tokens
	default:
		return "", fmt.Errorf("参数错误：file 必须为 html/css/js/tokens 之一")
	}
	if rel == "" {
		return "", fmt.Errorf("该资产未声明 %s 载荷", fileKind)
	}
	return rel, nil
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

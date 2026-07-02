package assetops

import (
	"context"
	"fmt"
	"path"

	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const designRel = "common/tokens.css"

// ApplyThemeTool 将 theme 资产的 token 全集写入公共层。仅 overview scope。
type ApplyThemeTool struct {
	store       Store
	projectRoot string
	workRoot    string
	projectID   string
	runID       string
	clock       func() int64
	newID       func() string
	applied     bool
}

func NewApplyThemeTool(store Store, projectRoot, workRoot, projectID, runID string, clock func() int64, newID func() string) *ApplyThemeTool {
	return &ApplyThemeTool{store: store, projectRoot: projectRoot, workRoot: workRoot, projectID: projectID, runID: runID, clock: clock, newID: newID}
}

func (t *ApplyThemeTool) Name() string          { return "apply_theme" }
func (t *ApplyThemeTool) Class() tools.Class    { return tools.ClassWrite }
func (t *ApplyThemeTool) Scopes() []model.Scope { return []model.Scope{model.ScopeOverview} }
func (t *ApplyThemeTool) Applied() bool         { return t.applied }

func (t *ApplyThemeTool) Description() string {
	return "将仓库中某 theme 资产的 token 全集写入公共样式层，一次性全局换肤；写入前校验 token 全集。"
}

func (t *ApplyThemeTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"theme_asset_id"},
		"properties": map[string]any{
			"theme_asset_id": map[string]any{"type": "string"},
		},
	}
}

func (t *ApplyThemeTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	id, _ := args["theme_asset_id"].(string)
	if id == "" {
		return fail("参数错误：theme_asset_id 为空"), nil
	}
	projectSandbox, err := tools.NewSandbox(t.projectRoot)
	if err != nil {
		return tools.Result{}, err
	}
	assetSandbox, err := tools.NewSandbox(t.workRoot)
	if err != nil {
		return tools.Result{}, err
	}
	a, err := t.store.GetAsset(ctx, id)
	if err != nil {
		return fail("资产不存在：" + err.Error()), nil
	}
	manifestRaw, err := assetSandbox.Read(a.ManifestPath)
	if err != nil {
		return fail("读取 manifest 失败：" + err.Error()), nil
	}
	if err := asset.ValidateManifestRaw(manifestRaw); err != nil {
		return fail("manifest schema 校验未通过：" + err.Error()), nil
	}
	m, err := asset.Parse(manifestRaw)
	if err != nil {
		return fail("解析 manifest 失败：" + err.Error()), nil
	}
	if m.Kind != asset.KindTheme {
		return fail("apply_theme 只能应用 theme 资产"), nil
	}
	tokens, err := assetSandbox.Read(path.Join(a.Dir, m.Assets.Tokens))
	if err != nil {
		return fail("读取 tokens 失败：" + err.Error()), nil
	}
	if miss := asset.ValidateThemeTokens(tokens); len(miss) != 0 {
		return fail(fmt.Sprintf("theme 缺少必需 token，拒绝应用：%v", miss)), nil
	}
	previous, readErr := projectSandbox.Read(designRel)
	if err := projectSandbox.Write(designRel, tokens); err != nil {
		return fail("写入公共层失败：" + err.Error()), nil
	}
	versionNo, err := t.snapshotDesign(ctx, projectSandbox, string(tokens))
	if err != nil {
		if readErr == nil {
			_ = projectSandbox.Write(designRel, previous)
		} else {
			_ = projectSandbox.Delete(designRel)
		}
		return tools.Result{}, err
	}
	t.applied = true
	return tools.Result{
		OK:          true,
		Observation: fmt.Sprintf("主题 %s 已应用到公共层，design 版本 v%d", a.Name, versionNo),
		Artifact:    &tools.Artifact{Type: "design", Ref: designRel},
	}, nil
}

func (t *ApplyThemeTool) snapshotDesign(ctx context.Context, sandbox *tools.Sandbox, css string) (int, error) {
	target := model.DesignVersionTarget(t.projectID)
	no, err := t.store.NextVersionNo(ctx, "design", target)
	if err != nil {
		return 0, err
	}
	snap := fmt.Sprintf("versions/design/v%d.css", no)
	if err := sandbox.Write(snap, []byte(css)); err != nil {
		return 0, err
	}
	if err := t.store.CreateVersion(ctx, model.Version{
		ID: t.newID(), TargetType: "design", TargetID: target, VersionNo: no,
		SnapshotPath: snap, RunID: t.runID, CreatedAt: t.clock(),
	}); err != nil {
		return 0, err
	}
	return no, nil
}

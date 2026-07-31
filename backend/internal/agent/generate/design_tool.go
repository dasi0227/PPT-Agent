package generate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// SubmitDesignSpecTool 接收设计总监提交的结构化设计语言，校验后落盘并产 design 版本。
// 落盘 design/design-spec.json；emit artifact{artifact_type:"design_spec"}（V2-AGENT-PIPELINE §3.3）。
type SubmitDesignSpecTool struct {
	store     Store
	sandbox   *tools.Sandbox
	projectID string
	runID     string
	clock     func() int64
	newID     func() string

	submitted *DesignSpec // 最近一次成功提交（供 runner 读回）
}

// NewSubmitDesignSpecTool 构造 submit_design_spec。
func NewSubmitDesignSpecTool(store Store, sandbox *tools.Sandbox, projectID, runID string, clock func() int64, newID func() string) *SubmitDesignSpecTool {
	return &SubmitDesignSpecTool{
		store: store, sandbox: sandbox, projectID: projectID, runID: runID, clock: clock, newID: newID,
	}
}

func (t *SubmitDesignSpecTool) Name() string          { return "submit_design_spec" }
func (t *SubmitDesignSpecTool) Class() tools.Class    { return tools.ClassWrite }
func (t *SubmitDesignSpecTool) Scopes() []model.Scope { return nil }

// Spec 返回最近一次成功提交的 design_spec（未提交返回 nil）。
func (t *SubmitDesignSpecTool) Spec() *DesignSpec { return t.submitted }

func (t *SubmitDesignSpecTool) Description() string {
	return "提交整份演示文稿的设计语言（palette/type/layout/signature/motion）。系统会校验字段完整性、" +
		"落盘 design/design-spec.json 并创建 design 版本。校验失败会返回可操作错误，请修正后重新提交。"
}

func (t *SubmitDesignSpecTool) Parameters() map[string]any {
	colorItem := map[string]any{
		"type":     "object",
		"required": []any{"name", "hex", "role"},
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
			"hex":  map[string]any{"type": "string", "description": "#RGB 或 #RRGGBB"},
			"role": map[string]any{"type": "string", "description": "语义角色，如 背景/正文/主强调"},
		},
	}
	fontObj := map[string]any{
		"type":     "object",
		"required": []any{"family", "weights", "usage"},
		"properties": map[string]any{
			"family":  map[string]any{"type": "string"},
			"weights": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
			"usage":   map[string]any{"type": "string"},
		},
	}
	return map[string]any{
		"type":     "object",
		"required": []any{"subject", "palette", "type", "signature"},
		"properties": map[string]any{
			"subject": map[string]any{
				"type":       "object",
				"properties": map[string]any{"topic": map[string]any{"type": "string"}, "audience": map[string]any{"type": "string"}, "job": map[string]any{"type": "string"}},
			},
			"palette": map[string]any{"type": "array", "minItems": 3, "items": colorItem},
			"type": map[string]any{
				"type":       "object",
				"required":   []any{"display", "body"},
				"properties": map[string]any{"display": fontObj, "body": fontObj, "utility": fontObj},
			},
			"layout": map[string]any{
				"type":       "object",
				"properties": map[string]any{"grid": map[string]any{"type": "string"}, "rhythm": map[string]any{"type": "string"}, "concept": map[string]any{"type": "string"}},
			},
			"signature": map[string]any{"type": "string", "description": "一句具体、可实现为 HTML/CSS/SVG 的独特元素描述"},
			"motion": map[string]any{
				"type":       "object",
				"properties": map[string]any{"policy": map[string]any{"type": "string"}},
			},
		},
	}
}

func (t *SubmitDesignSpecTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	spec, err := parseDesignSpec(args)
	if err != nil {
		return fail(err.Error()), nil
	}
	if err := spec.Validate(); err != nil {
		return fail("design_spec 校验失败：" + err.Error()), nil
	}

	const rel = "design/design-spec.json"
	old, readErr := t.sandbox.Read(rel)
	revision := 1
	if readErr == nil {
		var current blueprint.DesignSpec
		if json.Unmarshal(old, &current) == nil && current.SchemaVersion == blueprint.SchemaVersion {
			revision = current.Revision + 1
		}
	}
	persisted := blueprintFromDirector(spec, revision)
	raw, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		return tools.Result{}, err
	}
	if err := t.sandbox.Write(rel, raw); err != nil {
		return fail(fmt.Sprintf("写入失败：%v", err)), nil
	}

	versionNo, err := t.snapshotVersion(ctx, raw)
	if err != nil {
		if readErr == nil {
			_ = t.sandbox.Write(rel, old)
		} else {
			_ = t.sandbox.Delete(rel)
		}
		return tools.Result{}, err
	}

	t.submitted = &spec
	return tools.Result{
		OK:          true,
		Observation: fmt.Sprintf("design_spec 已落盘（%d 个色板色，signature=%q），design 版本 v%d", len(spec.Palette), spec.Signature, versionNo),
		Artifact:    &tools.Artifact{Type: "design_spec", Ref: rel},
	}, nil
}

func blueprintFromDirector(spec DesignSpec, revision int) blueprint.DesignSpec {
	palette := make([]string, 0, len(spec.Palette))
	for _, color := range spec.Palette {
		palette = append(palette, color.Hex)
	}
	typography := map[string]any{}
	if raw, err := json.Marshal(spec.Type); err == nil {
		_ = json.Unmarshal(raw, &typography)
	}
	layout := map[string]any{}
	if raw, err := json.Marshal(spec.Layout); err == nil {
		_ = json.Unmarshal(raw, &layout)
	}
	if len(layout) == 0 {
		layout["grid"] = "12-col"
	}
	motion := map[string]any{}
	if raw, err := json.Marshal(spec.Motion); err == nil {
		_ = json.Unmarshal(raw, &motion)
	}
	return blueprint.DesignSpec{
		SchemaVersion: blueprint.SchemaVersion, Revision: revision, Canvas: map[string]any{"ratio": "16:9"},
		Palette: palette, Typography: typography, Spacing: map[string]any{}, Radius: map[string]any{},
		Shadows: map[string]any{}, LayoutSystem: layout, Signature: spec.Signature, Motion: motion,
	}
}

func (t *SubmitDesignSpecTool) snapshotVersion(ctx context.Context, raw []byte) (int, error) {
	target := model.DesignVersionTarget(t.projectID)
	no, err := t.store.NextVersionNo(ctx, "design", target)
	if err != nil {
		return 0, err
	}
	snap := fmt.Sprintf("versions/design/v%d.json", no)
	if err := t.sandbox.Write(snap, raw); err != nil {
		return 0, err
	}
	v := model.Version{
		ID: t.newID(), TargetType: "design", TargetID: target, VersionNo: no,
		SnapshotPath: snap, RunID: t.runID, CreatedAt: t.clock(),
	}
	if err := t.store.CreateVersion(ctx, v); err != nil {
		return 0, err
	}
	return no, nil
}

func parseDesignSpec(args map[string]any) (DesignSpec, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return DesignSpec{}, fmt.Errorf("参数错误：design_spec 无法序列化")
	}
	var spec DesignSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return DesignSpec{}, fmt.Errorf("参数错误：design_spec 结构非法")
	}
	return spec, nil
}

package overview

import (
	"context"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// PatchDesignTool 对公共样式层 common/tokens.css 做锚定文本替换（patch_design）。
// 语义（ARCH-TOOLS-003/006, SPEC-CMD-OVERVIEW-004）：
//   - 每个 old_text 必须在 tokens.css 内**唯一**出现，否则整体失败回可操作 observation；
//   - 路径边界限定 work_dir；成功后落盘并产 **design 版本**（target_type=design）。
//
// 这是「改动面最小化」的首选：能用公共层 token 表达的全局调整优先走这里，不逐页改
// （SPEC-CMD-OVERVIEW-002）。仅 overview scope 可用。
type PatchDesignTool struct {
	store     Store
	sandbox   *tools.Sandbox
	projectID string
	runID     string
	clock     func() int64
	newID     func() string
	patched   bool
}

// NewPatchDesignTool 构造 patch_design。
func NewPatchDesignTool(store Store, sandbox *tools.Sandbox, projectID, runID string, clock func() int64, newID func() string) *PatchDesignTool {
	return &PatchDesignTool{
		store: store, sandbox: sandbox, projectID: projectID, runID: runID,
		clock: clock, newID: newID,
	}
}

func (t *PatchDesignTool) Name() string          { return "patch_design" }
func (t *PatchDesignTool) Class() tools.Class    { return tools.ClassWrite }
func (t *PatchDesignTool) Scopes() []model.Scope { return []model.Scope{model.ScopeOverview} }
func (t *PatchDesignTool) Patched() bool         { return t.patched }

func (t *PatchDesignTool) Description() string {
	return "锚定替换公共样式层 common/tokens.css（用于全局微调单个/多个 token，一次影响所有页）。" +
		"每个 old_text 必须在 tokens.css 内唯一出现，否则整体失败并要求补充上下文。成功后产生 design 版本。"
}

func (t *PatchDesignTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"edits"},
		"properties": map[string]any{
			"edits": map[string]any{
				"type":        "array",
				"minItems":    1,
				"description": "锚定替换列表，按顺序应用到 common/tokens.css",
				"items": map[string]any{
					"type":     "object",
					"required": []any{"old_text", "new_text"},
					"properties": map[string]any{
						"old_text": map[string]any{"type": "string", "description": "锚点：tokens.css 内唯一出现的原文片段"},
						"new_text": map[string]any{"type": "string", "description": "替换后的新文本"},
					},
				},
			},
		},
	}
}

const designRel = "common/tokens.css"

func (t *PatchDesignTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	edits, err := parseEdits(args["edits"])
	if err != nil {
		return fail(err.Error()), nil
	}

	raw, err := t.sandbox.Read(designRel)
	if err != nil {
		return fail(fmt.Sprintf("读取公共层失败：%v", err)), nil
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

	// 路径边界（ARCH-TOOLS-006）：Sandbox.Write 内部规范化 + 前缀校验。
	if err := t.sandbox.Write(designRel, []byte(content)); err != nil {
		return fail(fmt.Sprintf("写入公共层失败：%v", err)), nil
	}

	versionNo, err := t.snapshotVersion(ctx, content)
	if err != nil {
		_ = t.sandbox.Write(designRel, raw)
		return tools.Result{}, err
	}

	t.patched = true
	return tools.Result{
		OK:          true,
		Observation: fmt.Sprintf("公共层已应用 %d 处替换，design 版本 v%d（全局生效，未逐页改）", len(edits), versionNo),
		Artifact:    &tools.Artifact{Type: "design", Ref: designRel},
	}, nil
}

// snapshotVersion 快照 tokens.css 并登记 design 版本（SPEC-CMD-OVERVIEW-004）。
func (t *PatchDesignTool) snapshotVersion(ctx context.Context, css string) (int, error) {
	target := model.DesignVersionTarget(t.projectID)
	no, err := t.store.NextVersionNo(ctx, "design", target)
	if err != nil {
		return 0, err
	}
	snap := fmt.Sprintf("versions/design/v%d.css", no)
	if err := t.sandbox.Write(snap, []byte(css)); err != nil {
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

// ── 共享小工具（与 edit 包同构，但保持包内自足，避免跨 agent 包耦合）──

type tokenEdit struct {
	oldText string
	newText string
}

func parseEdits(v any) ([]tokenEdit, error) {
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf("参数错误：edits 必须为非空数组")
	}
	out := make([]tokenEdit, 0, len(list))
	for i, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("参数错误：edit #%d 非对象", i+1)
		}
		ot, _ := m["old_text"].(string)
		nt, _ := m["new_text"].(string)
		if ot == "" {
			return nil, fmt.Errorf("参数错误：edit #%d 缺少 old_text", i+1)
		}
		out = append(out, tokenEdit{oldText: ot, newText: nt})
	}
	return out, nil
}

func fail(reason string) tools.Result {
	return tools.Result{OK: false, Observation: reason}
}

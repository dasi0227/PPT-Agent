package edit

import (
	"context"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// PatchSlideTool 对锁定页的 html 做锚定文本替换（patch_slide，编辑核心）。
// 语义（ARCH-TOOLS-003/004/006, DATA-VERSION-001）：
//   - 每个 old_text 必须在该页 html 内**唯一**出现，否则整体失败回可操作 observation；
//   - 全部替换后隐式跑 lint-slide（html-output-spec），不合规拒绝落盘；
//   - 路径边界限定 work_dir；成功后落盘并产**新版本**、更新 slides.current_version。
//
// 锁定 slideIdx：编辑工具只能改目标页（scope=current|page 隔离，AC-CMD-PAGE-001）。
// 与 M3 generate.write_slide 的区分：patch=局部锚定深编辑；write=整页覆盖（仅生成阶段）。
type PatchSlideTool struct {
	store     Store
	sandbox   *tools.Sandbox
	projectID string
	runID     string
	slideIdx  int
	clock     func() int64
	newID     func() string
	patched   bool
}

// NewPatchSlideTool 构造 patch_slide，锁定到 slideIdx。
func NewPatchSlideTool(store Store, sandbox *tools.Sandbox, projectID, runID string, slideIdx int, clock func() int64, newID func() string) *PatchSlideTool {
	return &PatchSlideTool{
		store: store, sandbox: sandbox, projectID: projectID, runID: runID,
		slideIdx: slideIdx, clock: clock, newID: newID,
	}
}

func (t *PatchSlideTool) Name() string       { return "patch_slide" }
func (t *PatchSlideTool) Class() tools.Class { return tools.ClassWrite }
func (t *PatchSlideTool) Scopes() []model.Scope {
	// current/page 主循环编辑；overview 跨页 patch 时由子代理（Scope=overview）复用本工具，
	// 每子代理仍锁定单页（slideIdx）。与 tools.md 工具总表 patch_slide 可用 scope 一致。
	return []model.Scope{model.ScopeCurrent, model.ScopePage, model.ScopeOverview}
}
func (t *PatchSlideTool) Patched() bool { return t.patched }

func (t *PatchSlideTool) Description() string {
	return "对当前页 html 做锚定文本替换。每个 old_text 必须在该页内唯一出现，否则整体失败并要求补充上下文。" +
		"替换后系统会隐式校验 html-output-spec，不合规会拒绝落盘。成功后自动产生新版本。"
}

func (t *PatchSlideTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"slide_idx", "edits"},
		"properties": map[string]any{
			"slide_idx": map[string]any{"type": "integer", "minimum": 0, "description": fmt.Sprintf("页序，必须为 %d", t.slideIdx)},
			"edits": map[string]any{
				"type":        "array",
				"minItems":    1,
				"description": "锚定替换列表，按顺序应用",
				"items": map[string]any{
					"type":     "object",
					"required": []any{"old_text", "new_text"},
					"properties": map[string]any{
						"old_text": map[string]any{"type": "string", "description": "锚点：该页内唯一出现的原文片段"},
						"new_text": map[string]any{"type": "string", "description": "替换后的新文本"},
					},
				},
			},
		},
	}
}

func (t *PatchSlideTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	idx, ok := toInt(args["slide_idx"])
	if !ok {
		return fail("参数错误：slide_idx 必须为整数"), nil
	}
	if idx != t.slideIdx {
		return fail(fmt.Sprintf("越权：本次编辑仅能改第 %d 页，收到 slide_idx=%d", t.slideIdx, idx)), nil
	}
	edits, err := parseEdits(args["edits"])
	if err != nil {
		return fail(err.Error()), nil
	}

	rel := fmt.Sprintf("slides/%03d/index.html", idx)
	raw, err := t.sandbox.Read(rel)
	if err != nil {
		return fail(fmt.Sprintf("读取第 %d 页失败：%v", idx, err)), nil
	}
	content := string(raw)

	// 锚定替换：每个 old_text 必须页内唯一（ARCH-TOOLS-003）。
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

	// 落盘前隐式校验 html-output-spec（ARCH-TOOLS-004）。
	if okv, reason := designsystem.LintSlideResult([]byte(content)); !okv {
		return fail("校验失败，拒绝落盘：" + reason), nil
	}

	// 路径边界（ARCH-TOOLS-006）：Sandbox.Write 内部规范化 + 前缀校验。
	if err := t.sandbox.Write(rel, []byte(content)); err != nil {
		return fail(fmt.Sprintf("写入失败：%v", err)), nil
	}

	versionNo, err := t.snapshotVersion(ctx, idx, content)
	if err != nil {
		_ = t.sandbox.Write(rel, raw)
		return tools.Result{}, err
	}

	t.patched = true
	return tools.Result{
		OK:          true,
		Observation: fmt.Sprintf("第 %d 页已应用 %d 处替换并通过校验，版本 v%d", idx, len(edits), versionNo),
		Artifact:    &tools.Artifact{Type: "slide_html", Ref: rel},
	}, nil
}

// snapshotVersion 快照当前页 html 并登记新版本（DATA-VERSION-001），更新 current_version。
func (t *PatchSlideTool) snapshotVersion(ctx context.Context, idx int, html string) (int, error) {
	target := model.SlideVersionTarget(t.projectID, idx)
	no, err := t.store.NextVersionNo(ctx, "slide", target)
	if err != nil {
		return 0, err
	}
	snap := fmt.Sprintf("versions/slide-%03d/v%d.html", idx, no)
	if err := t.sandbox.Write(snap, []byte(html)); err != nil {
		return 0, err
	}
	v := model.Version{
		ID: t.newID(), TargetType: "slide", TargetID: target, VersionNo: no,
		SnapshotPath: snap, RunID: t.runID, CreatedAt: t.clock(),
	}
	if err := t.store.CreateVersion(ctx, v); err != nil {
		return 0, err
	}
	if err := t.store.SetSlideVersion(ctx, t.projectID, idx, no); err != nil {
		_ = t.store.DeleteVersion(ctx, "slide", target, no)
		_ = t.sandbox.Delete(snap)
		return 0, err
	}
	return no, nil
}

type edit struct {
	oldText string
	newText string
}

func parseEdits(v any) ([]edit, error) {
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf("参数错误：edits 必须为非空数组")
	}
	out := make([]edit, 0, len(list))
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
		out = append(out, edit{oldText: ot, newText: nt})
	}
	return out, nil
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func fail(reason string) tools.Result {
	return tools.Result{OK: false, Observation: reason}
}

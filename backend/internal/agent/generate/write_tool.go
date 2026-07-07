package generate

import (
	"context"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// WriteSlideTool 整页写入 slide html（生成阶段/子代理专用，write_slide）。
// 语义（ARCH-TOOLS-004/006/002）：落盘前隐式跑 lint-slide 校验；越界路径整体失败；成功后落版本。
// 锁定 slideIdx：子代理只能写自己那一页（隔离，AC-GEN-009）。磁盘/版本路径按稳定 slideID 定位。
type WriteSlideTool struct {
	store     Store
	sandbox   *tools.Sandbox
	projectID string
	runID     string
	slideIdx  int    // 本子代理锁定的页序（-1 表示不锁定，仅整套编排内部使用）
	slideID   string // 锁定页的稳定身份，用于拼磁盘/版本路径
	clock     func() int64
	newID     func() string
	written   bool // 标记本页是否已成功写入（供 runner 判定）
}

// NewWriteSlideTool 构造 write_slide，锁定到 slideIdx（页序供 LLM 定位）与 slideID（路径定位）。
func NewWriteSlideTool(store Store, sandbox *tools.Sandbox, projectID, runID string, slideIdx int, slideID string, clock func() int64, newID func() string) *WriteSlideTool {
	return &WriteSlideTool{
		store: store, sandbox: sandbox, projectID: projectID, runID: runID,
		slideIdx: slideIdx, slideID: slideID, clock: clock, newID: newID,
	}
}

func (t *WriteSlideTool) Name() string          { return "write_slide" }
func (t *WriteSlideTool) Class() tools.Class    { return tools.ClassWrite }
func (t *WriteSlideTool) Scopes() []model.Scope { return nil }
func (t *WriteSlideTool) Written() bool         { return t.written }

func (t *WriteSlideTool) Description() string {
	return "整页写入 slide html（生成阶段）。落盘前系统会隐式校验 html-output-spec，不合规会返回错误请修正。"
}

func (t *WriteSlideTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"slide_idx", "html"},
		"properties": map[string]any{
			"slide_idx": map[string]any{"type": "integer", "minimum": 0, "description": fmt.Sprintf("页序，必须为 %d", t.slideIdx)},
			"html":      map[string]any{"type": "string", "description": "整页完整 html 文档"},
		},
	}
}

func (t *WriteSlideTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	idx, ok := toInt(args["slide_idx"])
	if !ok {
		return fail("参数错误：slide_idx 必须为整数"), nil
	}
	if t.slideIdx >= 0 && idx != t.slideIdx {
		return fail(fmt.Sprintf("越权：本子代理仅能写第 %d 页，收到 slide_idx=%d", t.slideIdx, idx)), nil
	}
	html, ok := args["html"].(string)
	if !ok || html == "" {
		return fail("参数错误：html 为空"), nil
	}

	// 落盘前隐式校验 html-output-spec（ARCH-TOOLS-004）：不合规拒绝落盘。
	if okv, reason := designsystem.LintSlideResult([]byte(html)); !okv {
		return fail("html-output-spec 校验失败，拒绝落盘：" + reason), nil
	}

	rel := model.SlideHTMLPath(t.slideID)
	old, readErr := t.sandbox.Read(rel)
	// 路径边界（ARCH-TOOLS-006）：Sandbox.Write 内部规范化 + 前缀校验，越界整体失败。
	if err := t.sandbox.Write(rel, []byte(html)); err != nil {
		return fail(fmt.Sprintf("写入失败：%v", err)), nil
	}

	// 落版本（ARCH-TOOLS-002）：快照到 versions/slide-<id>/vN.html 并登记，同步 slides.current_version。
	versionNo, err := t.snapshotVersion(ctx, html)
	if err != nil {
		if readErr == nil {
			_ = t.sandbox.Write(rel, old)
		} else {
			_ = t.sandbox.Delete(rel)
		}
		return tools.Result{}, err
	}

	t.written = true
	// 产物已与 slide.json 同步：清该页脏标记（best-effort，不阻断已成功的写入）。
	_ = t.store.SetOutlineDirty(ctx, t.slideID, false)
	return tools.Result{
		OK:          true,
		Observation: fmt.Sprintf("第 %d 页已写入并通过校验，版本 v%d", idx, versionNo),
		Artifact:    &tools.Artifact{Type: "slide_html", Ref: rel},
	}, nil
}

func (t *WriteSlideTool) snapshotVersion(ctx context.Context, html string) (int, error) {
	target := model.SlideVersionTarget(t.projectID, t.slideID)
	no, err := t.store.NextVersionNo(ctx, "slide", target)
	if err != nil {
		return 0, err
	}
	snap := model.SlideVersionSnapshot(t.slideID, no)
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
	if err := t.store.SetSlideVersion(ctx, t.slideID, no); err != nil {
		_ = t.store.DeleteVersion(ctx, "slide", target, no)
		_ = t.sandbox.Delete(snap)
		return 0, err
	}
	return no, nil
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

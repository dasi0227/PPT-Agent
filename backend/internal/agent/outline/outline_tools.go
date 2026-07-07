package outline

import (
	"context"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

// OutlinePatch 是大纲字段级更新载荷（AI 路径，与 REST 手动共用底层 service）。
type OutlinePatch struct {
	Title         *string
	Subtitle      *string
	ContentIntent *string
	Layout        *string
	Bullets       *[]string
}

// OutlineEditor 是大纲编辑工具所需的最小底层能力（消费方定义接口，避免反向依赖 service）。
// 实现方（service 层适配器）走与手动 REST 相同的 AddSlide/DeleteSlide/ReorderSlides/PatchContent 逻辑。
type OutlineEditor interface {
	ListSlides(ctx context.Context, projectID string) ([]model.Slide, error)
	PatchOutline(ctx context.Context, slideID string, p OutlinePatch) (slidejson.SlideJSON, error)
	AddOutline(ctx context.Context, projectID, afterSlideID, layout string) (model.Slide, error)
	DeleteOutline(ctx context.Context, slideID string) error
	ReorderOutline(ctx context.Context, projectID string, orderedIDs []string) error
}

// patchOutlineTool：patch_outline_slide —— 局部改某页 slide.json 字段。
type patchOutlineTool struct {
	editor OutlineEditor
}

func (t *patchOutlineTool) Name() string          { return "patch_outline_slide" }
func (t *patchOutlineTool) Class() tools.Class    { return tools.ClassWrite }
func (t *patchOutlineTool) Scopes() []model.Scope { return nil }
func (t *patchOutlineTool) Description() string {
	return "局部修改某页大纲内容（slide.json）：可改 title/subtitle/bullets/content_intent/layout。有 html 的页会被标记为待更新。"
}
func (t *patchOutlineTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"slide_id"},
		"properties": map[string]any{
			"slide_id":       map[string]any{"type": "string", "description": "目标页稳定 id"},
			"title":          map[string]any{"type": "string"},
			"subtitle":       map[string]any{"type": "string"},
			"bullets":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"content_intent": map[string]any{"type": "string"},
			"layout":         map[string]any{"type": "string", "enum": toAnySlice(slidejson.LayoutEnum())},
		},
	}
}
func (t *patchOutlineTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	slideID, _ := args["slide_id"].(string)
	if slideID == "" {
		return fail("参数错误：slide_id 必填"), nil
	}
	p := OutlinePatch{}
	if v, ok := args["title"].(string); ok {
		p.Title = &v
	}
	if v, ok := args["subtitle"].(string); ok {
		p.Subtitle = &v
	}
	if v, ok := args["content_intent"].(string); ok {
		p.ContentIntent = &v
	}
	if v, ok := args["layout"].(string); ok {
		p.Layout = &v
	}
	if bs, ok := toStringSlice(args["bullets"]); ok {
		p.Bullets = &bs
	}
	got, err := t.editor.PatchOutline(ctx, slideID, p)
	if err != nil {
		return fail(fmt.Sprintf("更新失败：%v", err)), nil
	}
	return tools.Result{OK: true, Observation: fmt.Sprintf("已更新第 %q 页：%s", slideID, got.Title)}, nil
}

// addOutlineTool：add_outline_slide —— 在锚点页后插入一张空白页。
type addOutlineTool struct {
	editor    OutlineEditor
	projectID string
}

func (t *addOutlineTool) Name() string          { return "add_outline_slide" }
func (t *addOutlineTool) Class() tools.Class    { return tools.ClassWrite }
func (t *addOutlineTool) Scopes() []model.Scope { return nil }
func (t *addOutlineTool) Description() string {
	return "在某页之后插入一张新页（after_slide_id 为空则追加到末尾）。插入后可再用 patch_outline_slide 填充内容。"
}
func (t *addOutlineTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"after_slide_id": map[string]any{"type": "string", "description": "锚点页 id；空则追加到末尾"},
			"layout":         map[string]any{"type": "string", "enum": toAnySlice(slidejson.LayoutEnum())},
		},
	}
}
func (t *addOutlineTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	after, _ := args["after_slide_id"].(string)
	layout, _ := args["layout"].(string)
	sl, err := t.editor.AddOutline(ctx, t.projectID, after, layout)
	if err != nil {
		return fail(fmt.Sprintf("加页失败：%v", err)), nil
	}
	return tools.Result{OK: true, Observation: fmt.Sprintf("已新增一页（id=%s, layout=%s）", sl.ID, sl.Layout)}, nil
}

// deleteOutlineTool：delete_outline_slide —— 删除某页（不可逆），删前经 prompter 二次确认。
type deleteOutlineTool struct {
	editor   OutlineEditor
	prompter run.Prompter
}

func (t *deleteOutlineTool) Name() string          { return "delete_outline_slide" }
func (t *deleteOutlineTool) Class() tools.Class    { return tools.ClassWrite }
func (t *deleteOutlineTool) Scopes() []model.Scope { return nil }
func (t *deleteOutlineTool) Description() string {
	return "删除某页（不可逆，无回收站）。执行前系统会请用户二次确认，仅在用户明确确认后才真正删除。"
}
func (t *deleteOutlineTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"slide_id"},
		"properties": map[string]any{
			"slide_id": map[string]any{"type": "string", "description": "目标页稳定 id"},
		},
	}
}
func (t *deleteOutlineTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	slideID, _ := args["slide_id"].(string)
	if slideID == "" {
		return fail("参数错误：slide_id 必填"), nil
	}
	if t.prompter == nil {
		return fail("当前运行不支持二次确认，删页已跳过"), nil
	}
	// HITL 二次确认：删页不可逆，须用户明确确认（回答含"确认"）。
	answer, err := t.prompter.NeedsInput(ctx, "confirm_delete_"+slideID,
		fmt.Sprintf("确认删除第 %q 页吗？此操作不可撤销。", slideID),
		[]string{"确认删除", "取消"})
	if err != nil {
		return tools.Result{}, err
	}
	if !strings.Contains(answer, "确认") {
		return tools.Result{OK: true, Observation: fmt.Sprintf("已取消删除第 %q 页", slideID)}, nil
	}
	if err := t.editor.DeleteOutline(ctx, slideID); err != nil {
		return fail(fmt.Sprintf("删页失败：%v", err)), nil
	}
	return tools.Result{OK: true, Observation: fmt.Sprintf("已删除第 %q 页", slideID)}, nil
}

// reorderOutlineTool：reorder_outline_slides —— 按给定 id 顺序重排全部页。
type reorderOutlineTool struct {
	editor    OutlineEditor
	projectID string
}

func (t *reorderOutlineTool) Name() string          { return "reorder_outline_slides" }
func (t *reorderOutlineTool) Class() tools.Class    { return tools.ClassWrite }
func (t *reorderOutlineTool) Scopes() []model.Scope { return nil }
func (t *reorderOutlineTool) Description() string {
	return "按给定的完整页 id 顺序重新排序所有页（ordered_ids 应覆盖全部现有页）。"
}
func (t *reorderOutlineTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"ordered_ids"},
		"properties": map[string]any{
			"ordered_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "按目标顺序排列的页 id 数组"},
		},
	}
}
func (t *reorderOutlineTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	ids, ok := toStringSlice(args["ordered_ids"])
	if !ok || len(ids) == 0 {
		return fail("参数错误：ordered_ids 必须为非空 id 数组"), nil
	}
	if err := t.editor.ReorderOutline(ctx, t.projectID, ids); err != nil {
		return fail(fmt.Sprintf("重排失败：%v", err)), nil
	}
	return tools.Result{OK: true, Observation: fmt.Sprintf("已按新顺序重排 %d 页", len(ids))}, nil
}

func toStringSlice(v any) ([]string, bool) {
	switch xs := v.(type) {
	case []string:
		return xs, true
	case []any:
		out := make([]string, 0, len(xs))
		for _, e := range xs {
			s, ok := e.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

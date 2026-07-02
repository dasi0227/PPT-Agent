package overview

import (
	"context"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/edit"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// FanoutPagePatchTool 把一个跨页结构性指令逐页派发给独立子代理执行（fanout_page_patch）。
// 语义（SPEC-CMD-OVERVIEW-003/006, ARCH-HARNESS-005）：token 无法表达的调整（如「每页加页脚」）
// 才走这里；对每个目标页各 spawn 一个 harness.Loop 子代理（独立上下文），子代理复用
// edit.PatchSlideTool 并锁定该页 → 每页各自版本、各自 validate；任一页失败**不影响**其它页落盘。
//
// 与 M3 generate 子代理同构（父层确定性 Go 编排 + 逐页独立 Loop），但这里是编辑而非生成。
type FanoutPagePatchTool struct {
	client    llm.Client
	store     Store
	sandbox   *tools.Sandbox
	projectID string
	runID     string
	pageCount int
	clock     func() int64
	newID     func() string
	patched   bool
}

// NewFanoutPagePatchTool 构造 fanout_page_patch。pageCount 为项目页数，用于校验/默认全页。
func NewFanoutPagePatchTool(client llm.Client, store Store, sandbox *tools.Sandbox, projectID, runID string, pageCount int, clock func() int64, newID func() string) *FanoutPagePatchTool {
	return &FanoutPagePatchTool{
		client: client, store: store, sandbox: sandbox, projectID: projectID, runID: runID,
		pageCount: pageCount, clock: clock, newID: newID,
	}
}

func (t *FanoutPagePatchTool) Name() string          { return "fanout_page_patch" }
func (t *FanoutPagePatchTool) Class() tools.Class    { return tools.ClassWrite }
func (t *FanoutPagePatchTool) Scopes() []model.Scope { return []model.Scope{model.ScopeOverview} }
func (t *FanoutPagePatchTool) Patched() bool         { return t.patched }

func (t *FanoutPagePatchTool) Description() string {
	return "当调整无法仅靠公共层 token 表达（如逐页结构性添加：每页加页脚/角标）时，逐页派发子代理执行相同的编辑指令。" +
		"每页独立处理、独立产版本；某页失败不影响其它页。优先考虑 patch_design，仅在必要时使用本工具。"
}

func (t *FanoutPagePatchTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"instruction"},
		"properties": map[string]any{
			"instruction": map[string]any{"type": "string", "description": "对每一页施加的统一编辑指令（自然语言）"},
			"pages": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "integer", "minimum": 0},
				"description": "目标页序列表（0 基）；缺省表示全部页",
			},
		},
	}
}

func (t *FanoutPagePatchTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	instruction, _ := args["instruction"].(string)
	if strings.TrimSpace(instruction) == "" {
		return fail("参数错误：instruction 为空"), nil
	}
	pages, err := t.resolvePages(args["pages"])
	if err != nil {
		return fail(err.Error()), nil
	}

	var okPages, failPages []int
	var details []string
	for _, idx := range pages {
		if ctx.Err() != nil {
			return fail("已取消"), nil
		}
		if err := t.patchOnePage(ctx, idx, instruction); err != nil {
			// 单页失败隔离（SPEC-CMD-OVERVIEW-006）：记录并继续下一页，不影响其它页落盘。
			failPages = append(failPages, idx)
			details = append(details, fmt.Sprintf("第 %d 页失败：%v", idx, err))
			continue
		}
		okPages = append(okPages, idx)
	}

	if len(okPages) > 0 {
		t.patched = true
	}
	obs := fmt.Sprintf("跨页 patch 完成：成功 %d 页 %v，失败 %d 页 %v", len(okPages), okPages, len(failPages), failPages)
	if len(details) > 0 {
		obs += "\n" + strings.Join(details, "\n")
	}
	// 只要有页成功即视为工具 OK（部分成功仍是进展）；全失败则 OK=false 供 LLM 重试。
	return tools.Result{OK: len(okPages) > 0, Observation: obs}, nil
}

// patchOnePage 为单页 spawn 一个独立子代理（harness.Loop，Scope=overview），锁定该页复用 edit 工具。
func (t *FanoutPagePatchTool) patchOnePage(ctx context.Context, idx int, instruction string) error {
	htmlRaw, err := t.sandbox.Read(fmt.Sprintf("slides/%03d/index.html", idx))
	if err != nil {
		return fmt.Errorf("读取页失败：%w", err)
	}
	jsonRaw, _ := t.sandbox.Read(fmt.Sprintf("slides/%03d/slide.json", idx))

	patch := edit.NewPatchSlideTool(t.store, t.sandbox, t.projectID, t.runID, idx, t.clock, t.newID)
	pp := prompt.EditParams{
		PageIndex:   idx,
		Instruction: instruction,
		SlideHTML:   string(htmlRaw),
		SlideJSON:   string(jsonRaw),
	}

	loop := harness.New(t.client, harness.Config{
		RunID:        t.runID,
		Kind:         model.KindEdit,
		Scope:        model.ScopeOverview, // 子代理以 overview 跑；patch/read/validate 的 Scopes 含 overview
		Mode:         model.ModeNormal,
		SystemPrompt: prompt.EditSystem(pp),
		Instruction:  prompt.EditUser(pp),
		Tools: []tools.Tool{
			edit.NewReadSlideTool(t.sandbox, idx),
			patch,
			edit.NewValidateSlideTool(),
			tools.NewFinishTool(),
		},
	})
	// 子代理独立上下文：不投影事件到父 SSE（用 nopEmitter），避免污染父进度语义。
	outcome := loop.Run(ctx, subAgentEmitter{}, nil)
	if outcome.Status != harness.OutcomeFinished {
		return fmt.Errorf("子代理未完成（%s：%s）", outcome.Code, outcome.Message)
	}
	if !patch.Patched() {
		return fmt.Errorf("子代理声称完成但未产出改动")
	}
	return nil
}

// resolvePages 解析目标页集合；缺省=全部页。校验页号在 [0,pageCount)。
func (t *FanoutPagePatchTool) resolvePages(v any) ([]int, error) {
	if v == nil {
		all := make([]int, t.pageCount)
		for i := range all {
			all[i] = i
		}
		return all, nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("参数错误：pages 必须为整数数组")
	}
	if len(list) == 0 {
		all := make([]int, t.pageCount)
		for i := range all {
			all[i] = i
		}
		return all, nil
	}
	out := make([]int, 0, len(list))
	for _, item := range list {
		n, ok := toInt(item)
		if !ok {
			return nil, fmt.Errorf("参数错误：pages 含非整数项")
		}
		if n < 0 || n >= t.pageCount {
			return nil, fmt.Errorf("页号越界：%d（共 %d 页）", n, t.pageCount)
		}
		out = append(out, n)
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

// subAgentEmitter 吞掉子代理事件（子代理是父工具的内部实现细节，不直接投影到父 SSE）。
type subAgentEmitter struct{}

func (subAgentEmitter) Emit(model.EventType, any) {}

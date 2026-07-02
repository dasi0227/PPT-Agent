package edit

import (
	"context"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ReadSlideTool 读取锁定页当前 html（read_slide，只读）。
// 上下文已注入目标页 html；本工具供 LLM 在编辑前二次确认最新内容（门控表要求 current/page 注册）。
// 锁定 slideIdx：只能读目标页，隔离一致（AGENT-CTX-001）。
type ReadSlideTool struct {
	sandbox  *tools.Sandbox
	slideIdx int
}

func NewReadSlideTool(sandbox *tools.Sandbox, slideIdx int) *ReadSlideTool {
	return &ReadSlideTool{sandbox: sandbox, slideIdx: slideIdx}
}

func (t *ReadSlideTool) Name() string       { return "read_slide" }
func (t *ReadSlideTool) Class() tools.Class { return tools.ClassRead }
func (t *ReadSlideTool) Scopes() []model.Scope {
	// current/page 主循环编辑；overview 跨页子代理（Scope=overview）也复用它读锁定页。
	return []model.Scope{model.ScopeCurrent, model.ScopePage, model.ScopeOverview}
}

func (t *ReadSlideTool) Description() string {
	return "读取当前页最新 html，用于编辑前确认锚点上下文。"
}

func (t *ReadSlideTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"slide_idx"},
		"properties": map[string]any{
			"slide_idx": map[string]any{"type": "integer", "minimum": 0, "description": fmt.Sprintf("页序，必须为 %d", t.slideIdx)},
		},
	}
}

func (t *ReadSlideTool) Execute(_ context.Context, args map[string]any) (tools.Result, error) {
	idx, ok := toInt(args["slide_idx"])
	if !ok {
		return fail("参数错误：slide_idx 必须为整数"), nil
	}
	if idx != t.slideIdx {
		return fail(fmt.Sprintf("越权：本次编辑仅能读第 %d 页", t.slideIdx)), nil
	}
	raw, err := t.sandbox.Read(fmt.Sprintf("slides/%03d/index.html", idx))
	if err != nil {
		return fail(fmt.Sprintf("读取第 %d 页失败：%v", idx, err)), nil
	}
	return tools.Result{OK: true, Observation: string(raw)}, nil
}

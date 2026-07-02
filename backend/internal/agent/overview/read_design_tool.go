package overview

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ReadDesignTool 读取公共层 tokens.css（read_design，只读）。
// 供 LLM 在 patch_design 前确认可用 token 与锚点上下文。仅 overview scope 可用。
type ReadDesignTool struct {
	sandbox *tools.Sandbox
}

func NewReadDesignTool(sandbox *tools.Sandbox) *ReadDesignTool {
	return &ReadDesignTool{sandbox: sandbox}
}

func (t *ReadDesignTool) Name() string          { return "read_design" }
func (t *ReadDesignTool) Class() tools.Class    { return tools.ClassRead }
func (t *ReadDesignTool) Scopes() []model.Scope { return []model.Scope{model.ScopeOverview} }

func (t *ReadDesignTool) Description() string {
	return "读取公共样式层 common/tokens.css 的当前内容，用于确认可改的 token 与锚点上下文。"
}

func (t *ReadDesignTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func (t *ReadDesignTool) Execute(_ context.Context, _ map[string]any) (tools.Result, error) {
	raw, err := t.sandbox.Read(designRel)
	if err != nil {
		return fail("读取公共层失败：" + err.Error()), nil
	}
	return tools.Result{OK: true, Observation: string(raw)}, nil
}

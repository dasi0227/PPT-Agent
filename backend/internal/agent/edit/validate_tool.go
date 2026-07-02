package edit

import (
	"context"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ValidateSlideTool 显式跑 html-output-spec 校验，返回逐项 observation（validate_slide）。
// 供编辑子回合自查；patch_slide 落盘前也隐式再校验（ARCH-TOOLS-004）。
type ValidateSlideTool struct{}

func NewValidateSlideTool() *ValidateSlideTool { return &ValidateSlideTool{} }

func (t *ValidateSlideTool) Name() string       { return "validate_slide" }
func (t *ValidateSlideTool) Class() tools.Class { return tools.ClassValidate }
func (t *ValidateSlideTool) Scopes() []model.Scope {
	// current/page 主循环 + overview 跨页子代理均可显式自查。
	return []model.Scope{model.ScopeCurrent, model.ScopePage, model.ScopeOverview}
}

func (t *ValidateSlideTool) Description() string {
	return "校验一段 slide html 是否满足 html-output-spec，返回逐项检查结果（不落盘）。"
}

func (t *ValidateSlideTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"html"},
		"properties": map[string]any{
			"html": map[string]any{"type": "string", "description": "待校验的整页 html"},
		},
	}
}

func (t *ValidateSlideTool) Execute(_ context.Context, args map[string]any) (tools.Result, error) {
	html, ok := args["html"].(string)
	if !ok || html == "" {
		return fail("参数错误：html 为空"), nil
	}
	checks := designsystem.LintSlide([]byte(html))
	var failed []string
	for _, c := range checks {
		if !c.OK {
			failed = append(failed, c.ID+": "+c.Reason)
		}
	}
	if len(failed) == 0 {
		return tools.Result{OK: true, Observation: "校验通过：全部 html-output-spec 检查项满足"}, nil
	}
	return tools.Result{
		OK:          false,
		Observation: fmt.Sprintf("校验未通过（%d 项）：\n- %s", len(failed), strings.Join(failed, "\n- ")),
	}, nil
}

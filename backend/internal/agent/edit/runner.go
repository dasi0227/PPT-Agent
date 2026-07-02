// Package edit 是真实的单页编辑 agent（自然语言指令 → 锚定 patch 目标页 html）。
// 复用 M1 harness（ReAct/SSE/锁/停止）与 M3 designsystem 校验/版本；本包只构造 harness 配置、
// 选 prompt、接入 patch_slide/read_slide/validate_slide。编辑走**主循环**，不走子代理（ARCH-HARNESS-005）。
package edit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

// Params 是构造一次编辑 Run 所需的输入。
type Params struct {
	RunID       string
	ProjectID   string
	WorkDir     string      // project work_dir（sandbox 根）
	Scope       model.Scope // current | page（归一为锁定某页）
	PageIndex   int         // 锁定编辑的页序（0 基；current 由上报页填充，越界校验在 service）
	Instruction string      // 用户自然语言编辑指令
}

// Runner 用 harness ReAct 主循环跑「自然语言 → 锚定 patch 目标页」，满足 run.Runner。
type Runner struct {
	client llm.Client
	store  Store
	params Params
	clock  func() int64
	newID  func() string
}

// NewRunner 构造编辑 runner。clock/newID 可注入以便测试确定性；nil 用默认。
func NewRunner(client llm.Client, store Store, p Params, clock func() int64, newID func() string) *Runner {
	if clock == nil {
		clock = func() int64 { return time.Now().Unix() }
	}
	if newID == nil {
		newID = uuid.NewString
	}
	return &Runner{client: client, store: store, params: p, clock: clock, newID: newID}
}

func (r *Runner) Run(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, _ run.Prompter) harness.Outcome {
	sandbox, err := tools.NewSandbox(r.params.WorkDir)
	if err != nil {
		return r.errOut(em, "INTERNAL", err.Error())
	}

	idx := r.params.PageIndex

	// 上下文隔离（AGENT-CTX-001）：只读目标页 html + slide-json，绝不注入别页。
	htmlRaw, err := sandbox.Read(fmt.Sprintf("slides/%03d/index.html", idx))
	if err != nil {
		return r.errOut(em, "BAD_STATE", fmt.Sprintf("第 %d 页尚无 html，无法编辑：%v", idx, err))
	}
	jsonRaw, _ := sandbox.Read(fmt.Sprintf("slides/%03d/slide.json", idx)) // 可空，容错

	pp := prompt.EditParams{
		PageIndex:   idx,
		Instruction: r.params.Instruction,
		SlideHTML:   string(htmlRaw),
		SlideJSON:   string(jsonRaw),
	}

	// 工具集：read_slide/patch_slide(锁定页)/validate_slide/finish。
	// Gate 按 scope 裁剪（AC-HARNESS-001）：patch/read/validate 的 Scopes()={current,page}，
	// 越权工具（改别页/公共层）根本不在候选集中，机制级隔离。
	patch := NewPatchSlideTool(r.store, sandbox, r.params.ProjectID, r.params.RunID, idx, r.clock, r.newID)
	toolset := []tools.Tool{
		NewReadSlideTool(sandbox, idx),
		patch,
		NewValidateSlideTool(),
		tools.NewFinishTool(),
	}

	loop := harness.New(r.client, harness.Config{
		RunID:        r.params.RunID,
		Kind:         model.KindEdit,
		Scope:        r.params.Scope,
		Mode:         model.ModeNormal,
		SystemPrompt: prompt.EditSystem(pp),
		Instruction:  prompt.EditUser(pp),
		Tools:        toolset,
	})
	return loop.Run(ctx, em, cp)
}

func (r *Runner) errOut(em harness.Emitter, code, msg string) harness.Outcome {
	em.Emit(model.EventError, harness.ErrorPayload{Code: code, Message: msg})
	return harness.Outcome{Status: harness.OutcomeLLMError, Code: code, Message: msg}
}

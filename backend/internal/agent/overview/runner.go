package overview

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/assetops"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

// Params 是构造一次 overview（跨页/全局）编辑 Run 所需的输入。
type Params struct {
	RunID       string
	ProjectID   string
	WorkDir     string // project work_dir（sandbox 根）
	WorkRoot    string // 全局 work_root（_assets 所在）
	PageCount   int    // 项目页数（fanout 默认全页、越界校验用）
	Instruction string // 用户自然语言全局调整指令
	ContextPack *contextengine.ContextPack
}

// Runner 用 harness ReAct 主循环跑「全局调整」，满足 run.Runner。
// 改动面最小化（SPEC-CMD-OVERVIEW-002）：优先 patch_design 改公共层；token 无法表达时
// 才用 fanout_page_patch 逐页走子代理（ARCH-HARNESS-005）。父身仍是主循环。
type Runner struct {
	client llm.Client
	store  Store
	params Params
	clock  func() int64
	newID  func() string
}

// NewRunner 构造 overview runner。clock/newID 可注入以便测试确定性；nil 用默认。
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

	tokensRaw, err := sandbox.Read(designRel)
	if err != nil {
		return r.errOut(em, "BAD_STATE", "公共层 tokens.css 缺失，无法全局调整（请先完成生成）："+err.Error())
	}

	pp := prompt.OverviewParams{
		Instruction: r.params.Instruction,
		TokensCSS:   string(tokensRaw),
		PageCount:   r.params.PageCount,
	}

	// 工具集（overview scope）：公共层优先；换主题走 apply_theme；结构性跨页调整走 fanout；
	// 单页资产移植可用 mount_asset，仍由 slide_idx 精确指定目标页并落页版本。
	toolset := []tools.Tool{
		assetops.NewSearchAssetsTool(r.store),
		NewReadDesignTool(sandbox),
		assetops.NewApplyThemeTool(r.store, r.params.WorkDir, r.params.WorkRoot, r.params.ProjectID, r.params.RunID, r.clock, r.newID),
		NewPatchDesignTool(r.store, sandbox, r.params.ProjectID, r.params.RunID, r.clock, r.newID),
		assetops.NewMountAssetTool(r.store, r.params.WorkDir, r.params.WorkRoot, r.params.ProjectID, r.params.RunID, nil, r.clock, r.newID),
		NewFanoutPagePatchTool(r.client, r.store, sandbox, r.params.ProjectID, r.params.RunID, r.params.PageCount, r.clock, r.newID),
		tools.NewFinishTool(),
	}
	if refTool := contextengine.RefTool(r.params.ContextPack); refTool != nil {
		toolset = append(toolset, refTool)
	}

	systemPrompt, userPrompt := contextengine.CompileForRunner(r.params.ContextPack, prompt.OverviewSystem(pp), prompt.OverviewUser(pp))
	loop := harness.New(r.client, harness.Config{
		RunID:        r.params.RunID,
		Kind:         model.KindEdit,
		Scope:        model.ScopeOverview,
		Mode:         model.ModeNormal,
		SystemPrompt: systemPrompt,
		Instruction:  userPrompt,
		Tools:        toolset,
	})
	return loop.Run(ctx, em, cp)
}

func (r *Runner) errOut(em harness.Emitter, code, msg string) harness.Outcome {
	em.Emit(model.EventError, harness.ErrorPayload{Code: code, Message: msg})
	return harness.Outcome{Status: harness.OutcomeLLMError, Code: code, Message: msg}
}

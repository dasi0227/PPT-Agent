// Package repo 是仓库资产编辑 agent（/repo scope）。核心是**工具门控隔离**：
// scope=repo 时 harness 只注册资产工具（search/read/patch/validate_asset），
// **绝不注册任何 PPT 页/公共层写工具**——机制级证明「不碰页/公共层」（SPEC-CMD-REPO-001/004）。
// M5 做 read/search/patch/validate；create/delete 与资产版本化留 M6。
package repo

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

// Params 是构造一次 /repo Run 所需的输入。
type Params struct {
	RunID       string
	WorkRoot    string // 全局 work_root（_assets 所在的根）；repo 操作与具体 project 无关
	Instruction string // 用户自然语言资产操作指令
}

// Runner 用 harness ReAct 主循环跑仓库资产编辑，满足 run.Runner。
// 编辑走主循环，不走子代理（ARCH-HARNESS-005）。
type Runner struct {
	client llm.Client
	store  Store
	params Params
	clock  func() int64
	newID  func() string
}

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
	// sandbox 根为 work_root：资产载荷在 _assets/ 下（a.Dir/a.ManifestPath 相对 work_root）。
	sandbox, err := tools.NewSandbox(r.params.WorkRoot)
	if err != nil {
		return r.errOut(em, "INTERNAL", err.Error())
	}

	pp := prompt.RepoParams{Instruction: r.params.Instruction}

	// 工具集（repo scope）：只注册资产工具。Gate 会进一步确认——但这里本就不放入任何页/公共层写工具，
	// 双重保证 SPEC-CMD-REPO-001/004：LLM 的 function schema 里根本不存在 patch_slide/patch_design。
	toolset := []tools.Tool{
		NewSearchAssetsTool(r.store),
		NewReadAssetTool(r.store, sandbox),
		NewPatchAssetTool(r.store, sandbox, r.clock),
		NewValidateAssetTool(r.store, sandbox),
		tools.NewFinishTool(),
	}

	loop := harness.New(r.client, harness.Config{
		RunID:        r.params.RunID,
		Kind:         model.KindEdit,
		Scope:        model.ScopeRepo,
		Mode:         model.ModeNormal,
		SystemPrompt: prompt.RepoSystem(pp),
		Instruction:  prompt.RepoUser(pp),
		Tools:        toolset,
	})
	return loop.Run(ctx, em, cp)
}

func (r *Runner) errOut(em harness.Emitter, code, msg string) harness.Outcome {
	em.Emit(model.EventError, harness.ErrorPayload{Code: code, Message: msg})
	return harness.Outcome{Status: harness.OutcomeLLMError, Code: code, Message: msg}
}

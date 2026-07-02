// Package outline 是真实的大纲生成 agent（主题 → slide-json[]）。
// 复用 M1 的 run+harness+llm：本包只负责构造 harness 配置、选 prompt、定义 submit_outline 工具。
package outline

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

// Params 是构造一次大纲 Run 所需的输入。
type Params struct {
	RunID      string
	ProjectID  string
	WorkDir    string // project work_dir（sandbox 根）
	Topic      string
	Brief      string
	SlideCount int
	Language   string
}

// Runner 用 harness ReAct 循环跑「主题 → slide-json[]」，满足 run.Runner。
type Runner struct {
	client llm.Client
	store  Store
	params Params
	clock  func() int64
	newID  func() string
}

// NewRunner 构造大纲 runner。clock/newID 可注入以便测试确定性；nil 用默认。
func NewRunner(client llm.Client, store Store, p Params, clock func() int64, newID func() string) *Runner {
	return &Runner{client: client, store: store, params: p, clock: clock, newID: newID}
}

func (r *Runner) Run(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, _ run.Prompter) harness.Outcome {
	sandbox, err := tools.NewSandbox(r.params.WorkDir)
	if err != nil {
		em.Emit(model.EventError, harness.ErrorPayload{Code: "INTERNAL", Message: err.Error()})
		return harness.Outcome{Status: harness.OutcomeLLMError, Code: "INTERNAL", Message: err.Error()}
	}

	submit := NewSubmitOutlineTool(r.store, sandbox, r.params.ProjectID, r.params.RunID, r.params.SlideCount, r.clock, r.newID)

	pp := prompt.OutlineParams{
		Topic:      r.params.Topic,
		Brief:      r.params.Brief,
		SlideCount: r.params.SlideCount,
		Language:   r.params.Language,
		Layouts:    slidejson.LayoutEnum(),
		MinBody:    defaultMinBody,
		MaxBody:    defaultMaxBody,
	}

	loop := harness.New(r.client, harness.Config{
		RunID:        r.params.RunID,
		Kind:         model.KindOutline,
		Scope:        model.ScopeCurrent,
		Mode:         model.ModeNormal,
		SystemPrompt: prompt.OutlineSystem(pp),
		Instruction:  prompt.OutlineUser(pp),
		Tools:        []tools.Tool{submit, tools.NewFinishTool()},
	})
	return loop.Run(ctx, em, cp)
}

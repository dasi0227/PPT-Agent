package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/assist"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/edit"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/generate"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/outline"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/overview"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

// WorkRoot 是全局工作根（_assets 所在），供 /repo 资产操作定位载荷。
// 用命名类型避免 wire 对裸 string 的注入歧义。
type WorkRoot string

// RunnerFactory 按 Run 元数据、入参与所属 project 构造一次执行的 runner。
// RunnerFactory is injectable for tests; production resolves by Artifact Target.
type RunnerFactory func(r model.Run, p model.CreateRunParams, proj model.Project) run.Runner

// RunService 编排一次 Agent 执行：解析 thread→project 归属、构造 runner、委派 engine。
type RunService struct {
	store          store.Store
	engine         *run.Engine
	factory        RunnerFactory
	contextFactory func(model.Run, model.CreateRunParams, model.Project, contextengine.ContextPack) run.Runner
	assembler      *contextengine.ContextAssembler
}

// NewRunService wires the four explicit artifact × level execution paths.
func NewRunService(s store.Store, engine *run.Engine, client llm.Client, workRoot WorkRoot) *RunService {
	registry := contextengine.NewRefRegistry()
	assembler := contextengine.NewContextAssembler(s, registry)
	buildBlueprintDeck := func(r model.Run, p model.CreateRunParams, proj model.Project, pack contextengine.ContextPack) run.Runner {
		if r.WorkSpec.Interaction.Intent == model.IntentConsult {
			return assist.NewContextTalkRunner(client, r.ID, p.Instruction, &pack)
		}
		if slides, err := s.ListSlides(context.Background(), proj.ID); err == nil && len(slides) > 0 {
			return outline.NewEditRunner(client, NewOutlineEditor(NewSlideService(s), proj.ID), outline.EditParams{
				RunID: r.ID, ProjectID: proj.ID, Instruction: p.Instruction, Language: p.Language, Mode: model.ModeNormal, ContextPack: &pack,
			})
		}
		return outline.NewRunner(client, s, outline.Params{
			RunID: r.ID, ProjectID: proj.ID, WorkDir: proj.WorkDir, Topic: p.Instruction,
			Brief: p.Brief, SlideCount: p.SlideCount, Language: p.Language,
			ContextPack: &pack,
		}, nil, nil)
	}
	buildBlueprintSlide := func(r model.Run, p model.CreateRunParams, proj model.Project, pack contextengine.ContextPack) run.Runner {
		if r.WorkSpec.Interaction.Intent == model.IntentConsult {
			return assist.NewContextTalkRunner(client, r.ID, p.Instruction, &pack)
		}
		return outline.NewEditRunner(client, NewOutlineEditor(NewSlideService(s), proj.ID), outline.EditParams{
			RunID: r.ID, ProjectID: proj.ID, Instruction: p.Instruction, Language: p.Language, Mode: model.ModeNormal, ContextPack: &pack,
		})
	}
	buildPresentationDeck := func(r model.Run, p model.CreateRunParams, proj model.Project, pack contextengine.ContextPack) run.Runner {
		if r.WorkSpec.Interaction.Intent == model.IntentConsult {
			return assist.NewContextTalkRunner(client, r.ID, p.Instruction, &pack)
		}
		slides, _ := s.ListSlides(context.Background(), proj.ID)
		hasHTML := false
		for _, slide := range slides {
			if _, err := os.Stat(filepath.Join(proj.WorkDir, filepath.FromSlash(model.SlideHTMLPath(slide.ID)))); err == nil {
				hasHTML = true
				break
			}
		}
		if hasHTML {
			p.PageCount = len(slides)
			return overview.NewRunner(client, s, overview.Params{
				RunID: r.ID, ProjectID: proj.ID, WorkDir: proj.WorkDir, WorkRoot: string(workRoot),
				PageCount: p.PageCount, Instruction: p.Instruction,
				ContextPack: &pack,
			}, nil, nil)
		}
		return generate.NewRunner(client, s, generate.Params{
			RunID: r.ID, ProjectID: proj.ID, WorkDir: proj.WorkDir, WorkRoot: string(workRoot),
			Theme: firstNonEmpty(p.Theme, proj.Theme), PageIndex: nil, Brief: p.Brief, Language: p.Language,
			ContextPack: &pack,
		}, nil, nil)
	}
	buildPresentationSlide := func(r model.Run, p model.CreateRunParams, proj model.Project, pack contextengine.ContextPack) run.Runner {
		if r.WorkSpec.Interaction.Intent == model.IntentConsult {
			return assist.NewContextTalkRunner(client, r.ID, p.Instruction, &pack)
		}
		slide, _ := s.GetSlide(context.Background(), r.WorkSpec.Target.SlideID)
		_, htmlErr := os.Stat(filepath.Join(proj.WorkDir, filepath.FromSlash(model.SlideHTMLPath(slide.ID))))
		if htmlErr != nil {
			return generate.NewRunner(client, s, generate.Params{
				RunID: r.ID, ProjectID: proj.ID, WorkDir: proj.WorkDir, WorkRoot: string(workRoot),
				Theme: firstNonEmpty(p.Theme, proj.Theme), PageIndex: p.PageIndex, Brief: p.Brief, Language: p.Language,
				ContextPack: &pack,
			}, nil, nil)
		}
		return edit.NewRunner(client, s, edit.Params{
			RunID: r.ID, ProjectID: proj.ID, WorkDir: proj.WorkDir, WorkRoot: string(workRoot),
			Scope: model.ScopePage, PageIndex: *p.PageIndex, Instruction: p.Instruction,
			ContextPack: &pack,
		}, nil, nil)
	}
	resolver := NewRunnerResolver(map[TargetKey]TargetRunnerBuilder{
		{Artifact: model.ArtifactBlueprint, Level: model.TargetDeck}:     buildBlueprintDeck,
		{Artifact: model.ArtifactBlueprint, Level: model.TargetSlide}:    buildBlueprintSlide,
		{Artifact: model.ArtifactPresentation, Level: model.TargetDeck}:  buildPresentationDeck,
		{Artifact: model.ArtifactPresentation, Level: model.TargetSlide}: buildPresentationSlide,
	})
	factory := func(r model.Run, p model.CreateRunParams, proj model.Project, pack contextengine.ContextPack) run.Runner {
		runner, _ := resolver.Resolve(r, p, proj, pack)
		if runner == nil {
			return nil
		}
		tracked := &revisionTrackingRunner{
			inner: runner, spec: r.WorkSpec, project: proj, store: s,
			blueprint: NewBlueprintService(s), runID: r.ID,
		}
		return tracked
	}
	return &RunService{store: s, engine: engine, contextFactory: factory, assembler: assembler}
}

// NewRunServiceWithFactory 允许注入自定义 runner 工厂（测试用）。
func NewRunServiceWithFactory(s store.Store, engine *run.Engine, factory RunnerFactory) *RunService {
	return &RunService{store: s, engine: engine, factory: factory}
}

type contextPackRunner struct {
	inner   run.Runner
	build   func() run.Runner
	pack    contextengine.ContextPack
	project model.Project
}

func (r *contextPackRunner) Run(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, prompter run.Prompter) harness.Outcome {
	m := r.pack.Manifest
	if r.pack.RefResolver != nil {
		defer r.pack.RefResolver.CloseRun(m.RunID)
	}
	em.Emit(model.EventContextAssembled, harness.ContextAssembledPayload{
		ContextID: m.ContextID, Profile: string(m.Profile), EstimatedTokens: m.EstimatedTokens,
		BudgetTokens: m.BudgetTokens, Segments: len(m.Segments), Refs: len(m.Refs),
		Warnings: append([]string{}, m.Warnings...), ReadOnly: m.ReadOnly,
	})
	if r.inner == nil && r.build != nil {
		r.inner = r.build()
	}
	if r.inner == nil {
		return harness.Outcome{Status: harness.OutcomeLLMError, Code: "RUN_TARGET_UNSUPPORTED", Message: "runner resolution failed"}
	}
	out := r.inner.Run(ctx, em, cp, prompter)
	if out.Status == harness.OutcomeFinished {
		store := contextengine.ThreadMemoryStore{}
		old, _, err := store.Load(r.project.WorkDir, m.ThreadID)
		if err == nil {
			next := (contextengine.ThreadMemoryUpdater{}).UpdateSuccessful(old, m.RunID, r.pack.WorkSpec.Instruction)
			_ = store.Save(r.project.WorkDir, m.ThreadID, next)
		}
	}
	return out
}

// CreateRun 发起一次 Run：查 thread 得 project 归属（加锁单元），构造 runner 交给 engine。
func (svc *RunService) CreateRun(ctx context.Context, threadID string, p model.CreateRunParams) (model.Run, error) {
	th, err := svc.store.GetThread(ctx, threadID)
	if err != nil {
		return model.Run{}, err
	}
	proj, err := svc.store.GetProject(ctx, th.ProjectID)
	if err != nil {
		return model.Run{}, err
	}

	spec := p.WorkSpec
	if spec.Interaction.Clarification == "" {
		spec.Interaction.Clarification = model.ClarifyWhenBlocked
	}
	if err := spec.Validate(); err != nil {
		return model.Run{}, err
	}
	if spec.Target.Level == model.TargetSlide {
		slides, err := svc.store.ListSlides(ctx, proj.ID)
		if err != nil {
			return model.Run{}, err
		}
		found := -1
		for i, slide := range slides {
			if slide.ID == spec.Target.SlideID {
				found = i
				break
			}
		}
		if found < 0 {
			return model.Run{}, ErrSlideTargetNotFound
		}
		p.PageIndex = &found
	}
	p.WorkSpec = spec
	p.Instruction = spec.Instruction
	p.Language = firstNonEmpty(p.Language, spec.Options.Language)
	p.Theme = firstNonEmpty(p.Theme, spec.Options.ThemeID)
	if p.SlideCount == 0 {
		p.SlideCount = spec.Options.DesiredSlideCount
	}

	r := model.Run{
		ID:        uuid.NewString(),
		ThreadID:  th.ID,
		ProjectID: th.ProjectID,
		WorkSpec:  spec,
	}

	var (
		runner     run.Runner
		runContext *model.RunContext
	)
	if svc.assembler != nil && svc.contextFactory != nil {
		pack, err := svc.assembler.Assemble(ctx, contextengine.ContextRequest{
			RunID: r.ID, ThreadID: th.ID, ProjectID: proj.ID, WorkSpec: spec, Budget: contextengine.DefaultBudget(),
		}, proj)
		if err != nil {
			return model.Run{}, err
		}
		runner = &contextPackRunner{pack: pack, project: proj, build: func() run.Runner {
			return svc.contextFactory(r, p, proj, pack)
		}}
		raw, err := json.Marshal(pack.Manifest)
		if err != nil {
			return model.Run{}, err
		}
		runContext = &model.RunContext{ContextID: pack.Manifest.ContextID, Profile: string(pack.Manifest.Profile),
			PackHash: pack.Manifest.PackHash, EstimatedTokens: pack.Manifest.EstimatedTokens,
			BudgetTokens: pack.Manifest.BudgetTokens, ManifestJSON: string(raw)}
	} else {
		runner = svc.factory(r, p, proj)
	}
	if runner == nil {
		return model.Run{}, ErrRunTargetUnsupported
	}
	return svc.engine.StartWithContext(ctx, r, runner, runContext)
}

// InjectInput 转发到 engine（HITL 控制输入）。
func (svc *RunService) InjectInput(ctx context.Context, runID, content, replyTo string) error {
	return svc.engine.InjectInput(ctx, runID, content, replyTo)
}

// Cancel 取消 Run。
func (svc *RunService) Cancel(ctx context.Context, runID string) error {
	return svc.engine.Cancel(ctx, runID)
}

// Subscribe 订阅 SSE 事件流（支持 Last-Event-ID）。
func (svc *RunService) Subscribe(ctx context.Context, runID string, afterSeq int64) (<-chan model.Event, func(), error) {
	return svc.engine.Subscribe(ctx, runID, afterSeq)
}

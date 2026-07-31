package service

import (
	"context"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/assist"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/edit"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/generate"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/outline"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/overview"
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
	store   store.Store
	engine  *run.Engine
	factory RunnerFactory
}

// NewRunService wires the four explicit artifact × level execution paths.
func NewRunService(s store.Store, engine *run.Engine, client llm.Client, workRoot WorkRoot) *RunService {
	buildBlueprintDeck := func(r model.Run, p model.CreateRunParams, proj model.Project) run.Runner {
		if r.WorkSpec.Interaction.Intent == model.IntentConsult {
			return assist.NewTalkRunner(client, r.ID, p.Instruction)
		}
		if slides, err := s.ListSlides(context.Background(), proj.ID); err == nil && len(slides) > 0 {
			return outline.NewEditRunner(client, NewOutlineEditor(NewSlideService(s), proj.ID), outline.EditParams{
				RunID: r.ID, ProjectID: proj.ID, Instruction: p.Instruction, Language: p.Language, Mode: model.ModeNormal,
			})
		}
		return outline.NewRunner(client, s, outline.Params{
			RunID: r.ID, ProjectID: proj.ID, WorkDir: proj.WorkDir, Topic: p.Instruction,
			Brief: p.Brief, SlideCount: p.SlideCount, Language: p.Language,
		}, nil, nil)
	}
	buildBlueprintSlide := func(r model.Run, p model.CreateRunParams, proj model.Project) run.Runner {
		if r.WorkSpec.Interaction.Intent == model.IntentConsult {
			return assist.NewTalkRunner(client, r.ID, p.Instruction)
		}
		return outline.NewEditRunner(client, NewOutlineEditor(NewSlideService(s), proj.ID), outline.EditParams{
			RunID: r.ID, ProjectID: proj.ID, Instruction: p.Instruction, Language: p.Language, Mode: model.ModeNormal,
		})
	}
	buildPresentationDeck := func(r model.Run, p model.CreateRunParams, proj model.Project) run.Runner {
		if r.WorkSpec.Interaction.Intent == model.IntentConsult {
			return assist.NewTalkRunner(client, r.ID, p.Instruction)
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
			}, nil, nil)
		}
		return generate.NewRunner(client, s, generate.Params{
			RunID: r.ID, ProjectID: proj.ID, WorkDir: proj.WorkDir, WorkRoot: string(workRoot),
			Theme: firstNonEmpty(p.Theme, proj.Theme), PageIndex: nil, Brief: p.Brief, Language: p.Language,
		}, nil, nil)
	}
	buildPresentationSlide := func(r model.Run, p model.CreateRunParams, proj model.Project) run.Runner {
		if r.WorkSpec.Interaction.Intent == model.IntentConsult {
			return assist.NewTalkRunner(client, r.ID, p.Instruction)
		}
		slide, _ := s.GetSlide(context.Background(), r.WorkSpec.Target.SlideID)
		_, htmlErr := os.Stat(filepath.Join(proj.WorkDir, filepath.FromSlash(model.SlideHTMLPath(slide.ID))))
		if htmlErr != nil {
			return generate.NewRunner(client, s, generate.Params{
				RunID: r.ID, ProjectID: proj.ID, WorkDir: proj.WorkDir, WorkRoot: string(workRoot),
				Theme: firstNonEmpty(p.Theme, proj.Theme), PageIndex: p.PageIndex, Brief: p.Brief, Language: p.Language,
			}, nil, nil)
		}
		return edit.NewRunner(client, s, edit.Params{
			RunID: r.ID, ProjectID: proj.ID, WorkDir: proj.WorkDir, WorkRoot: string(workRoot),
			Scope: model.ScopePage, PageIndex: *p.PageIndex, Instruction: p.Instruction,
		}, nil, nil)
	}
	resolver := NewRunnerResolver(map[TargetKey]TargetRunnerBuilder{
		{Artifact: model.ArtifactBlueprint, Level: model.TargetDeck}:     buildBlueprintDeck,
		{Artifact: model.ArtifactBlueprint, Level: model.TargetSlide}:    buildBlueprintSlide,
		{Artifact: model.ArtifactPresentation, Level: model.TargetDeck}:  buildPresentationDeck,
		{Artifact: model.ArtifactPresentation, Level: model.TargetSlide}: buildPresentationSlide,
	})
	factory := func(r model.Run, p model.CreateRunParams, proj model.Project) run.Runner {
		runner, _ := resolver.Resolve(r, p, proj)
		if runner == nil {
			return nil
		}
		return &revisionTrackingRunner{
			inner: runner, spec: r.WorkSpec, project: proj, store: s,
			blueprint: NewBlueprintService(s), runID: r.ID,
		}
	}
	return &RunService{store: s, engine: engine, factory: factory}
}

// NewRunServiceWithFactory 允许注入自定义 runner 工厂（测试用）。
func NewRunServiceWithFactory(s store.Store, engine *run.Engine, factory RunnerFactory) *RunService {
	return &RunService{store: s, engine: engine, factory: factory}
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

	runner := svc.factory(r, p, proj)
	if runner == nil {
		return model.Run{}, ErrRunTargetUnsupported
	}
	return svc.engine.Start(ctx, r, runner)
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

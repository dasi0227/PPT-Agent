package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/assist"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/edit"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/generate"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/outline"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/overview"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/repo"
	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

// WorkRoot 是全局工作根（_assets 所在），供 /repo 资产操作定位载荷。
// 用命名类型避免 wire 对裸 string 的注入歧义。
type WorkRoot string

// RunnerFactory 按 Run 元数据、入参与所属 project 构造一次执行的 runner。
// 抽出工厂便于测试注入替身，也按 kind/scope/command 选择真实 agent。
type RunnerFactory func(r model.Run, p model.CreateRunParams, proj model.Project) run.Runner

// RunService 编排一次 Agent 执行：解析 thread→project 归属、构造 runner、委派 engine。
type RunService struct {
	store   store.Store
	engine  *run.Engine
	factory RunnerFactory
	legacy  *LegacyRunAdapter
}

// NewRunService 用默认工厂装配（生产路径）：按 kind/scope/command 选择真实 agent，其余用 demo。
func NewRunService(s store.Store, engine *run.Engine, client llm.Client, workRoot WorkRoot) *RunService {
	assetSvc := NewAssetService(s, string(workRoot))
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
			if slide.CurrentVersion > 0 {
				hasHTML = true
				break
			}
		}
		if (p.Legacy && p.Kind == model.KindEdit) || hasHTML {
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
		if !(p.Legacy && p.Kind == model.KindEdit) && slide.CurrentVersion == 0 {
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
			legacyDesignOnly: p.Legacy && p.Kind == model.KindEdit && p.Scope == model.ScopeOverview,
		}
	}
	_ = assetSvc
	return &RunService{store: s, engine: engine, factory: factory, legacy: NewLegacyRunAdapter(s)}
}

// buildCommandRunner 处理 command/mode 维度（/prompt /recap /talk /ask）。返回 nil 表示不属于本类。
func buildCommandRunner(r model.Run, p model.CreateRunParams, proj model.Project, client llm.Client, s store.Store, workRoot string) run.Runner {
	switch r.Command {
	case "prompt":
		return assist.NewPromptRunner(client, r.ID, p.Instruction)
	case "recap":
		return assist.NewRecapRunner(s, r.ID, proj.ID, p.Instruction)
	}
	// talk/ask 是 mode（Command 也置为 talk/ask，但以 Mode 为准）。
	switch r.Mode {
	case model.ModeTalk:
		return assist.NewTalkRunner(client, r.ID, p.Instruction)
	case model.ModeAsk:
		idx := 0
		if p.PageIndex != nil {
			idx = *p.PageIndex
		}
		return assist.NewAskRunner(client, s, assist.AskParams{
			RunID:       r.ID,
			ProjectID:   proj.ID,
			WorkDir:     proj.WorkDir,
			WorkRoot:    workRoot,
			Scope:       r.Scope,
			PageIndex:   idx,
			Instruction: p.Instruction,
		}, nil, nil)
	}
	return nil
}

// buildEditRunner 按 scope 子路由编辑：current/page→edit（单页），overview→overview，repo→repo。
func buildEditRunner(r model.Run, p model.CreateRunParams, proj model.Project, client llm.Client, s store.Store, assets repo.AssetManager, workRoot string) run.Runner {
	switch r.Scope {
	case model.ScopeOverview:
		return overview.NewRunner(client, s, overview.Params{
			RunID:       r.ID,
			ProjectID:   proj.ID,
			WorkDir:     proj.WorkDir,
			WorkRoot:    workRoot,
			PageCount:   p.PageCount,
			Instruction: p.Instruction,
		}, nil, nil)
	case model.ScopeRepo:
		return repo.NewRunner(client, s, assets, repo.Params{
			RunID:       r.ID,
			WorkRoot:    workRoot,
			Instruction: p.Instruction,
		}, nil, nil)
	default:
		// current/page：page_index 越界/缺失已在 CreateRun 前置校验；此处必非 nil。
		idx := 0
		if p.PageIndex != nil {
			idx = *p.PageIndex
		}
		return edit.NewRunner(client, s, edit.Params{
			RunID:       r.ID,
			ProjectID:   proj.ID,
			WorkDir:     proj.WorkDir,
			WorkRoot:    workRoot,
			Scope:       r.Scope,
			PageIndex:   idx,
			Instruction: p.Instruction,
		}, nil, nil)
	}
}

type repoAssetAdapter struct {
	svc *AssetService
}

func (a repoAssetAdapter) CreateAsset(ctx context.Context, manifest asset.Manifest, payload map[string]string) (model.Asset, error) {
	return a.svc.CreateAsset(ctx, CreateAssetParams{Manifest: manifest, Payload: payload})
}

func (a repoAssetAdapter) PatchAsset(ctx context.Context, id, file string, edits []repo.AssetEdit) (model.Asset, error) {
	out := make([]AssetPatchEdit, len(edits))
	for i, e := range edits {
		out[i] = AssetPatchEdit{File: file, OldText: e.OldText, NewText: e.NewText}
	}
	return a.svc.PatchAsset(ctx, id, PatchAssetParams{Edits: out})
}

func (a repoAssetAdapter) DeleteAsset(ctx context.Context, id string) error {
	return a.svc.DeleteAsset(ctx, id)
}

// NewRunServiceWithFactory 允许注入自定义 runner 工厂（测试用）。
func NewRunServiceWithFactory(s store.Store, engine *run.Engine, factory RunnerFactory) *RunService {
	return &RunService{store: s, engine: engine, factory: factory, legacy: NewLegacyRunAdapter(s)}
}

func (svc *RunService) AdaptLegacy(ctx context.Context, threadID string, req LegacyRunRequest) (model.WorkSpec, error) {
	thread, err := svc.store.GetThread(ctx, threadID)
	if err != nil {
		return model.WorkSpec{}, err
	}
	return svc.legacy.Adapt(ctx, thread.ProjectID, req)
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
			return model.Run{}, ErrInvalidPageIndex
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

	// Only compatibility requests retain the old runner projection. New
	// callers and the persisted WorkSpec remain exclusively target-based.
	if p.Legacy {
		needsPageCheck := p.Scope == model.ScopeCurrent || p.Scope == model.ScopePage
		if (p.Kind == model.KindEdit || p.Mode == model.ModeAsk) && needsPageCheck && p.Command != "prompt" && p.Command != "recap" {
			if err := svc.validatePageIndex(ctx, proj.ID, p.PageIndex); err != nil {
				return model.Run{}, err
			}
		}
		if p.Kind == model.KindEdit && p.Scope == model.ScopeOverview {
			slides, err := svc.store.ListSlides(ctx, proj.ID)
			if err != nil {
				return model.Run{}, err
			}
			p.PageCount = len(slides)
		}
	}

	persistedKind, persistedScope, persistedMode, persistedPageIndex := legacyPersistenceProjection(spec, p.PageIndex)
	if p.Legacy {
		persistedKind, persistedScope, persistedMode = p.Kind, p.Scope, p.Mode
	}
	r := model.Run{
		ID:        uuid.NewString(),
		ThreadID:  th.ID,
		ProjectID: th.ProjectID,
		Kind:      persistedKind,
		Scope:     persistedScope,
		PageIndex: persistedPageIndex,
		Mode:      persistedMode,
		Command:   p.Command,
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

// validatePageIndex 校验 page_index 存在且在 [0, 页数) 内（AC-CMD-PAGE-003）。
func (svc *RunService) validatePageIndex(ctx context.Context, projectID string, pageIndex *int) error {
	if pageIndex == nil {
		return ErrInvalidPageIndex
	}
	slides, err := svc.store.ListSlides(ctx, projectID)
	if err != nil {
		return err
	}
	if *pageIndex < 0 || *pageIndex >= len(slides) {
		return ErrInvalidPageIndex
	}
	return nil
}

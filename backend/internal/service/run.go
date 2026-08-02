package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

var screenshotIDPattern = regexp.MustCompile(`^shot_[A-Za-z0-9-]{1,128}$`)

type WorkRoot string

type ExecutionFactory func(r model.Run, p model.CreateRunParams, proj model.Project) run.Execution

type RunService struct {
	store     store.Store
	engine    *run.Engine
	factory   ExecutionFactory
	assembler *contextengine.ContextAssembler
	runtime   *workflow.Runtime
	renderer  workflow.SlideRenderer
}

func NewRunService(s store.Store, engine *run.Engine, client llm.Client, _ WorkRoot, renderer *workflow.NodeSlideRenderer) *RunService {
	registry := contextengine.NewRefRegistry()
	return &RunService{
		store: s, engine: engine,
		assembler: contextengine.NewContextAssembler(s, registry),
		runtime:   workflow.NewRuntime(workflow.CognitiveAgent{Client: client}),
		renderer:  renderer,
	}
}

func NewRunServiceWithExecutionFactory(s store.Store, engine *run.Engine, factory ExecutionFactory) *RunService {
	return &RunService{store: s, engine: engine, factory: factory}
}

type workflowExecution struct {
	runtime  *workflow.Runtime
	pack     contextengine.ContextPack
	project  model.Project
	store    store.Store
	runID    string
	renderer workflow.SlideRenderer
}

func (r *workflowExecution) Run(ctx context.Context, emitter workflow.EventEmitter, checkpoint run.Checkpointer, prompter run.Prompter) workflow.StructuredOutcome {
	if r.pack.RefResolver != nil {
		defer r.pack.RefResolver.CloseRun(r.runID)
	}
	committer := workflowCommitter{store: r.store, project: r.project, runID: r.runID}
	outcome := r.runtime.Run(ctx, workflow.RuntimeInput{
		RunID: r.runID, ProjectDir: r.project.WorkDir, Context: r.pack,
		Emitter: emitter, Prompter: prompter, Steering: checkpoint, Checkpoint: checkpoint,
		CommitMetadata: committer.Commit,
		Trace:          workflow.ZapTraceRecorder{Logger: zap.L().Named("ppt-runtime-trace")},
		DomainTools:    workflow.DefaultDomainToolProvider{Pack: r.pack, Renderer: r.renderer},
	})
	if outcome.Status == workflow.StatusCompleted {
		memoryStore := contextengine.ThreadMemoryStore{}
		old, _, err := memoryStore.Load(r.project.WorkDir, r.pack.Manifest.ThreadID)
		if err == nil {
			next := (contextengine.ThreadMemoryUpdater{}).UpdateSuccessful(old, r.runID, r.pack.WorkSpec.Instruction)
			_ = memoryStore.Save(r.project.WorkDir, r.pack.Manifest.ThreadID, next)
		}
	}
	return outcome
}

func (svc *RunService) CreateRun(ctx context.Context, threadID string, p model.CreateRunParams) (model.Run, error) {
	thread, err := svc.store.GetThread(ctx, threadID)
	if err != nil {
		return model.Run{}, err
	}
	project, err := svc.store.GetProject(ctx, thread.ProjectID)
	if err != nil {
		return model.Run{}, err
	}
	spec := p.WorkSpec
	if err := spec.Validate(); err != nil {
		return model.Run{}, err
	}
	if spec.Target.Level == model.TargetSlide {
		slides, err := svc.store.ListSlides(ctx, project.ID)
		if err != nil {
			return model.Run{}, err
		}
		found := false
		for _, slide := range slides {
			if slide.ID == spec.Target.SlideID {
				found = true
				break
			}
		}
		if !found {
			return model.Run{}, ErrSlideTargetNotFound
		}
	}
	p.WorkSpec, p.Instruction = spec, spec.Instruction
	runModel := model.Run{
		ID: uuid.NewString(), ThreadID: thread.ID, ProjectID: project.ID, WorkSpec: spec,
	}
	if svc.assembler == nil || svc.runtime == nil {
		execution := svc.factory(runModel, p, project)
		if execution == nil {
			return model.Run{}, ErrRunTargetUnsupported
		}
		return svc.engine.Start(ctx, runModel, execution)
	}
	pack, err := svc.assembler.Assemble(ctx, contextengine.ContextRequest{
		RunID: runModel.ID, ThreadID: thread.ID, ProjectID: project.ID,
		WorkSpec: spec, Budget: contextengine.DefaultBudget(),
	}, project)
	if err != nil {
		return model.Run{}, err
	}
	raw, err := json.Marshal(pack.Manifest)
	if err != nil {
		return model.Run{}, err
	}
	runContext := &model.RunContext{
		ContextID: pack.Manifest.ContextID, Profile: string(pack.Manifest.Profile),
		PackHash: pack.Manifest.PackHash, EstimatedTokens: pack.Manifest.EstimatedTokens,
		BudgetTokens: pack.Manifest.BudgetTokens, ManifestJSON: string(raw),
	}
	execution := &workflowExecution{
		runtime: svc.runtime, pack: pack, project: project, store: svc.store, runID: runModel.ID,
		renderer: svc.renderer,
	}
	return svc.engine.StartWithContext(ctx, runModel, execution, runContext)
}

func (svc *RunService) InjectInput(ctx context.Context, runID, content, replyTo string) error {
	return svc.engine.InjectInput(ctx, runID, content, replyTo)
}

func (svc *RunService) Cancel(ctx context.Context, runID string) error {
	return svc.engine.Cancel(ctx, runID)
}

func (svc *RunService) GetRun(ctx context.Context, runID string) (model.Run, error) {
	return svc.store.GetRun(ctx, runID)
}

func (svc *RunService) GetRenderScreenshot(ctx context.Context, runID, screenshotID string) ([]byte, error) {
	if !screenshotIDPattern.MatchString(screenshotID) {
		return nil, ErrScreenshotNotFound
	}
	runModel, err := svc.store.GetRun(ctx, runID)
	if err != nil {
		return nil, ErrScreenshotNotFound
	}
	project, err := svc.store.GetProject(ctx, runModel.ProjectID)
	if err != nil {
		return nil, ErrScreenshotNotFound
	}
	path := filepath.Join(project.WorkDir, ".runtime", "renders", runID, screenshotID+".png")
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrScreenshotNotFound
	}
	return raw, err
}

func (svc *RunService) Subscribe(ctx context.Context, runID string, afterSeq int64) (<-chan model.Event, func(), error) {
	return svc.engine.Subscribe(ctx, runID, afterSeq)
}

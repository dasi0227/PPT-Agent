package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/idempotency"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

var screenshotIDPattern = regexp.MustCompile(`^shot_[A-Za-z0-9-]{1,128}$`)
var clientIdentityPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

type WorkRoot string

type ExecutionFactory func(r model.Run, p model.CreateRunParams, proj model.Project) run.Execution

type RunService struct {
	store     store.Store
	engine    *run.Engine
	factory   ExecutionFactory
	assembler *contextengine.ContextAssembler
	runtime   *workflow.Runtime
	renderer  workflow.SlideRenderer
	client    llm.Client
}

func NewRunService(s store.Store, engine *run.Engine, client llm.Client, _ WorkRoot, renderer *workflow.NodeSlideRenderer) *RunService {
	registry := contextengine.NewRefRegistry()
	return &RunService{
		store: s, engine: engine,
		assembler: contextengine.NewContextAssembler(s, registry),
		runtime:   workflow.NewRuntime(workflow.CognitiveAgent{Client: client}),
		renderer:  renderer,
		client:    client,
	}
}

func NewRunServiceWithExecutionFactory(s store.Store, engine *run.Engine, factory ExecutionFactory) *RunService {
	return &RunService{store: s, engine: engine, factory: factory}
}

type workflowExecution struct {
	runtime       *workflow.Runtime
	pack          contextengine.ContextPack
	project       model.Project
	store         store.Store
	runID         string
	renderer      workflow.SlideRenderer
	imageResolver llm.ImageRefResolver
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
		ImageResolver:  r.imageResolver,
		Lifecycle:      checkpoint,
		Idempotency:    r.store,
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
	if svc.runtime != nil && spec.Interaction.Intent == model.IntentExecute &&
		spec.Target.Artifact == model.ArtifactPresentation {
		capabilityProvider, ok := svc.client.(llm.CapabilityProvider)
		if !ok || !capabilityProvider.Capabilities().Vision {
			return model.Run{}, model.NewAgentError("VISION_CAPABILITY_REQUIRED", "create_run", nil)
		}
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
	if p.ClientRequestID == "" {
		p.ClientRequestID = "internal_" + uuid.NewString()
	}
	if !clientIdentityPattern.MatchString(p.ClientRequestID) {
		return model.Run{}, model.NewAgentError("BAD_REQUEST", "create_run", errors.New("invalid client_request_id"))
	}
	requestHash, err := idempotency.CanonicalHash(map[string]any{
		"instruction": spec.Instruction, "target": spec.Target,
		"interaction": spec.Interaction, "options": spec.Options,
	})
	if err != nil {
		return model.Run{}, err
	}
	record, created, err := svc.store.AcquireIdempotency(ctx, model.IdempotencyRecord{
		Scope: "create_run", OwnerID: thread.ID, Key: p.ClientRequestID,
		RequestHash: requestHash, Status: "in_progress",
	})
	if err != nil {
		return model.Run{}, err
	}
	if record.RequestHash != requestHash {
		return model.Run{}, model.NewAgentError("IDEMPOTENCY_KEY_REUSED", "create_run", nil)
	}
	if !created {
		return svc.replayCreateRun(ctx, record)
	}
	runModel := model.Run{
		ID: uuid.NewString(), ThreadID: thread.ID, ProjectID: project.ID,
		ClientRequestID: p.ClientRequestID, WorkSpec: spec,
	}
	if svc.assembler == nil || svc.runtime == nil {
		execution := svc.factory(runModel, p, project)
		if execution == nil {
			svc.completeCreateFailure(ctx, thread.ID, p.ClientRequestID, "RUN_TARGET_UNSUPPORTED")
			return model.Run{}, ErrRunTargetUnsupported
		}
		createdRun, startErr := svc.engine.Start(ctx, runModel, execution)
		if startErr != nil {
			svc.completeCreateFailure(ctx, thread.ID, p.ClientRequestID, "INTERNAL")
			return model.Run{}, startErr
		}
		svc.completeCreateSuccess(ctx, thread.ID, p.ClientRequestID, createdRun.ID)
		return createdRun, nil
	}
	pack, err := svc.assembler.Assemble(ctx, contextengine.ContextRequest{
		RunID: runModel.ID, ThreadID: thread.ID, ProjectID: project.ID,
		WorkSpec: spec, Budget: contextengine.DefaultBudget(),
	}, project)
	if err != nil {
		svc.completeCreateFailure(ctx, thread.ID, p.ClientRequestID, "INTERNAL")
		return model.Run{}, err
	}
	raw, err := json.Marshal(pack.Manifest)
	if err != nil {
		svc.completeCreateFailure(ctx, thread.ID, p.ClientRequestID, "INTERNAL")
		return model.Run{}, err
	}
	runContext := &model.RunContext{
		ContextID: pack.Manifest.ContextID, Profile: string(pack.Manifest.Profile),
		PackHash: pack.Manifest.PackHash, EstimatedTokens: pack.Manifest.EstimatedTokens,
		BudgetTokens: pack.Manifest.BudgetTokens, ManifestJSON: string(raw),
	}
	execution := &workflowExecution{
		runtime: svc.runtime, pack: pack, project: project, store: svc.store, runID: runModel.ID,
		renderer:      svc.renderer,
		imageResolver: runImageResolver{runID: runModel.ID, projectID: project.ID, projectDir: project.WorkDir},
	}
	createdRun, startErr := svc.engine.StartWithContext(ctx, runModel, execution, runContext)
	if startErr != nil {
		svc.completeCreateFailure(ctx, thread.ID, p.ClientRequestID, "INTERNAL")
		return model.Run{}, startErr
	}
	svc.completeCreateSuccess(ctx, thread.ID, p.ClientRequestID, createdRun.ID)
	return createdRun, nil
}

func (svc *RunService) completeCreateSuccess(ctx context.Context, threadID, key, runID string) {
	raw, _ := json.Marshal(map[string]string{"run_id": runID})
	_ = svc.store.CompleteIdempotency(ctx, "create_run", threadID, key, "completed", string(raw))
}

func (svc *RunService) completeCreateFailure(ctx context.Context, threadID, key, code string) {
	raw, _ := json.Marshal(map[string]string{"error_code": code})
	_ = svc.store.CompleteIdempotency(ctx, "create_run", threadID, key, "failed", string(raw))
}

func (svc *RunService) replayCreateRun(ctx context.Context, record model.IdempotencyRecord) (model.Run, error) {
	for record.Status == "in_progress" {
		select {
		case <-ctx.Done():
			return model.Run{}, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
		next, err := svc.store.GetIdempotency(ctx, record.Scope, record.OwnerID, record.Key)
		if err != nil {
			return model.Run{}, err
		}
		record = next
	}
	var result map[string]string
	_ = json.Unmarshal([]byte(record.ResultJSON), &result)
	if record.Status == "completed" && result["run_id"] != "" {
		return svc.store.GetRun(ctx, result["run_id"])
	}
	code := result["error_code"]
	if code == "" {
		code = "INTERNAL"
	}
	return model.Run{}, model.NewAgentError(code, "create_run", nil)
}

type runImageResolver struct {
	runID      string
	projectID  string
	projectDir string
}

func (r runImageResolver) ResolveImage(_ context.Context, ref string) (llm.ImageData, error) {
	prefix := "run:" + r.runID + "/screenshot:"
	if !strings.HasPrefix(ref, prefix) {
		return llm.ImageData{}, errors.New("image reference does not belong to the current run")
	}
	screenshotID := strings.TrimPrefix(ref, prefix)
	if !screenshotIDPattern.MatchString(screenshotID) || strings.TrimSpace(r.projectID) == "" {
		return llm.ImageData{}, errors.New("invalid runtime screenshot reference")
	}
	path := filepath.Join(r.projectDir, ".runtime", "renders", r.runID, screenshotID+".png")
	raw, err := os.ReadFile(path)
	if err != nil {
		return llm.ImageData{}, err
	}
	if len(raw) < 8 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" || len(raw) > 10*1024*1024 {
		return llm.ImageData{}, errors.New("runtime screenshot MIME or size is invalid")
	}
	return llm.ImageData{Bytes: raw, MIMEType: "image/png"}, nil
}

func (svc *RunService) InjectInput(ctx context.Context, runID, content, replyTo string) error {
	return svc.engine.InjectInput(ctx, runID, content, replyTo)
}

func (svc *RunService) Cancel(ctx context.Context, runID string) error {
	return svc.engine.Cancel(ctx, runID)
}

func (svc *RunService) RequestCancel(ctx context.Context, runID string) (model.Run, error) {
	requestHash, _ := idempotency.CanonicalHash(map[string]string{"run_id": runID, "action": "cancel"})
	record, created, err := svc.store.AcquireIdempotency(ctx, model.IdempotencyRecord{
		Scope: "cancel", OwnerID: runID, Key: "cancel", RequestHash: requestHash, Status: "in_progress",
	})
	if err != nil {
		return model.Run{}, err
	}
	if record.RequestHash != requestHash {
		return model.Run{}, model.NewAgentError("IDEMPOTENCY_KEY_REUSED", "cancel_run", nil)
	}
	if !created {
		for record.Status == "in_progress" {
			select {
			case <-ctx.Done():
				return model.Run{}, ctx.Err()
			case <-time.After(10 * time.Millisecond):
			}
			record, err = svc.store.GetIdempotency(ctx, "cancel", runID, "cancel")
			if err != nil {
				return model.Run{}, err
			}
		}
		return svc.store.GetRun(ctx, runID)
	}
	current, err := svc.engine.RequestCancel(ctx, runID)
	if err != nil {
		_ = svc.store.CompleteIdempotency(ctx, "cancel", runID, "cancel", "failed", `{"error_code":"RUN_NOT_FOUND"}`)
		return model.Run{}, err
	}
	raw, _ := json.Marshal(map[string]any{"run_id": runID, "status": current.Status, "cancel_requested_at": current.CancelRequestedAt})
	_ = svc.store.CompleteIdempotency(ctx, "cancel", runID, "cancel", "completed", string(raw))
	return current, nil
}

func (svc *RunService) Steer(ctx context.Context, runID, expectedRunID, clientMessageID, content string) (model.SteeringMessage, error) {
	content = strings.TrimSpace(content)
	if !clientIdentityPattern.MatchString(clientMessageID) || len([]rune(content)) < 1 || len([]rune(content)) > 8000 {
		return model.SteeringMessage{}, model.NewAgentError("BAD_REQUEST", "steer_run", nil)
	}
	requestHash, err := idempotency.CanonicalHash(map[string]string{
		"expected_run_id": expectedRunID, "client_message_id": clientMessageID, "content": content,
	})
	if err != nil {
		return model.SteeringMessage{}, err
	}
	return svc.engine.Steer(ctx, runID, expectedRunID, clientMessageID, requestHash, content)
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

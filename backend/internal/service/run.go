package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	renderer  workflow.SlideRenderer
	registry  *llm.Registry
}

func NewRunService(s store.Store, engine *run.Engine, registry *llm.Registry, _ WorkRoot, renderer *workflow.NodeSlideRenderer) *RunService {
	refRegistry := contextengine.NewRefRegistry()
	return &RunService{
		store: s, engine: engine,
		assembler: contextengine.NewContextAssembler(s, refRegistry),
		renderer:  renderer,
		registry:  registry,
	}
}

func NewRunServiceWithExecutionFactory(s store.Store, engine *run.Engine, factory ExecutionFactory) *RunService {
	return &RunService{store: s, engine: engine, factory: factory}
}

func NewRunServiceWithExecutionFactoryAndRegistry(
	s store.Store,
	engine *run.Engine,
	factory ExecutionFactory,
	registry *llm.Registry,
) *RunService {
	return &RunService{store: s, engine: engine, factory: factory, registry: registry}
}

type workflowExecution struct {
	runtime          *workflow.Runtime
	pack             contextengine.ContextPack
	assembler        *contextengine.ContextAssembler
	project          model.Project
	store            store.Store
	runID            string
	renderer         workflow.SlideRenderer
	imageResolver    llm.ImageRefResolver
	semanticReviewer workflow.SemanticReviewer
	resumeCheckpoint *workflow.RuntimeCheckpoint
	reconciliation   workflow.RecoverySnapshot
}

type planApprovalCommitStore interface {
	CommitPlanApproval(context.Context, string, model.RunMode, model.RunContext, workflow.RuntimeCheckpoint) error
}

func (r *workflowExecution) commitPlanApproval(
	ctx context.Context,
	mode model.RunMode,
	pack contextengine.ContextPack,
	checkpoint workflow.RuntimeCheckpoint,
) error {
	store, ok := r.store.(planApprovalCommitStore)
	if !ok {
		return errors.New("atomic plan approval store is required")
	}
	raw, err := json.Marshal(pack.Manifest)
	if err != nil {
		return err
	}
	return store.CommitPlanApproval(ctx, r.runID, mode, model.RunContext{
		RunID: r.runID, ContextID: pack.Manifest.ContextID, Profile: string(pack.Manifest.Profile),
		PackHash: pack.Manifest.PackHash, EstimatedTokens: pack.Manifest.EstimatedTokens,
		BudgetTokens: pack.Manifest.BudgetTokens, ManifestJSON: string(raw), CreatedAt: time.Now().Unix(),
	}, checkpoint)
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
		Logger:         zap.L().Named("ppt-runtime"),
		Trace:          workflow.ZapTraceRecorder{Logger: zap.L().Named("ppt-runtime-trace")},
		DomainToolsForContext: func(pack contextengine.ContextPack) workflow.DomainToolProvider {
			return workflow.DefaultDomainToolProvider{Pack: pack, Renderer: r.renderer}
		},
		ImageResolver:       r.imageResolver,
		Lifecycle:           checkpoint,
		Idempotency:         r.store,
		ContextIndexStore:   optionalContextIndexStore(r.store),
		SemanticReviews:     r.semanticReviewer,
		SemanticReviewStore: optionalSemanticReviewStore(r.store),
		ResumeCheckpoint:    r.resumeCheckpoint,
		PersistMode:         func(ctx context.Context, mode model.RunMode) error { return r.store.UpdateRunMode(ctx, r.runID, mode) },
		RefreshContext: func(ctx context.Context, mode model.RunMode) (contextengine.ContextPack, error) {
			if r.assembler == nil {
				return contextengine.ContextPack{}, errors.New("context assembler is required for mode transition")
			}
			command := r.pack.Command
			command.Mode = mode
			return r.assembler.Assemble(ctx, contextengine.ContextRequest{
				RunID: r.runID, ThreadID: r.pack.Manifest.ThreadID, ProjectID: r.project.ID,
				Command: command, Budget: contextengine.DefaultBudget(),
			}, r.project)
		},
		CommitPlanApproval: r.commitPlanApproval,
	})
	if outcome.Status == workflow.StatusCompleted {
		memoryStore := contextengine.ThreadMemoryStore{}
		old, _, err := memoryStore.Load(r.project.WorkDir, r.pack.Manifest.ThreadID)
		if err == nil {
			next := (contextengine.ThreadMemoryUpdater{}).UpdateSuccessful(old, r.runID, r.pack.Command.Instruction)
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
	command := p.Command
	if err := command.Validate(); err != nil {
		return model.Run{}, err
	}
	var selectedProfile llm.Profile
	if svc.registry != nil {
		selectedProfile, err = svc.registry.Resolve(p.Model)
		if err != nil {
			return model.Run{}, model.NewAgentError("MODEL_PROFILE_NOT_FOUND", "create_run", nil)
		}
		p.Model = selectedProfile.Name()
		capabilities := selectedProfile.Capabilities()
		if !capabilities.ToolCalls {
			agentErr := model.NewAgentError("MODEL_CAPABILITY_MISMATCH", "create_run", nil)
			agentErr.Details["required_capability"] = "tool_calls"
			agentErr.Details["next_action"] = "请选择支持工具调用的模型。"
			return model.Run{}, agentErr
		}
		if command.Mode == model.ModeExecute &&
			command.Scope.Artifact == model.ArtifactPPT &&
			!capabilities.Vision {
			agentErr := model.NewAgentError("MODEL_CAPABILITY_MISMATCH", "create_run", nil)
			agentErr.Details["required_capability"] = "vision"
			agentErr.Details["next_action"] = "请选择标记为支持页面观察的模型。"
			return model.Run{}, agentErr
		}
	}
	if command.Scope.Level == model.ScopeSlide {
		slides, err := svc.store.ListSlides(ctx, project.ID)
		if err != nil {
			return model.Run{}, err
		}
		found := false
		for _, slide := range slides {
			if slide.ID == command.Scope.SlideID {
				found = true
				break
			}
		}
		if !found {
			return model.Run{}, ErrSlideTargetNotFound
		}
	}
	p.Command, p.Instruction = command, command.Instruction
	if p.ClientRequestID == "" {
		p.ClientRequestID = "internal_" + uuid.NewString()
	}
	if !clientIdentityPattern.MatchString(p.ClientRequestID) {
		return model.Run{}, model.NewAgentError("BAD_REQUEST", "create_run", errors.New("invalid client_request_id"))
	}
	requestHash, err := idempotency.CanonicalHash(map[string]any{
		"instruction": command.Instruction, "scope": command.Scope,
		"mode": command.Mode, "options": command.Options, "model": p.Model,
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
	// Serialize runs per project: each active run owns the project's authoring
	// overlay, so a second concurrent run would race on the
	// same files. Reuse RUN_ACTIVE to reject a new run while one is in flight.
	if active, activeErr := svc.store.HasActiveRun(ctx, project.ID); activeErr != nil {
		svc.completeCreateFailure(ctx, thread.ID, p.ClientRequestID, "INTERNAL")
		return model.Run{}, activeErr
	} else if active {
		svc.completeCreateFailure(ctx, thread.ID, p.ClientRequestID, "RUN_ACTIVE")
		return model.Run{}, ErrRunActive
	}
	runModel := model.Run{
		ID: uuid.NewString(), ThreadID: thread.ID, ProjectID: project.ID,
		ClientRequestID: p.ClientRequestID, Command: command,
	}
	if selectedProfile.Adapter() != nil {
		runModel.Model = model.ModelSelection{
			ProfileName: selectedProfile.Name(), Provider: selectedProfile.ProviderName(),
			Model: selectedProfile.Model(), URL: selectedProfile.URL(),
		}
	}
	if svc.assembler == nil || selectedProfile.Adapter() == nil {
		if svc.factory == nil {
			svc.completeCreateFailure(ctx, thread.ID, p.ClientRequestID, "RUN_SCOPE_UNSUPPORTED")
			return model.Run{}, ErrRunScopeUnsupported
		}
		execution := svc.factory(runModel, p, project)
		if execution == nil {
			svc.completeCreateFailure(ctx, thread.ID, p.ClientRequestID, "RUN_SCOPE_UNSUPPORTED")
			return model.Run{}, ErrRunScopeUnsupported
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
		Command: command, Budget: contextengine.DefaultBudget(),
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
		runtime: workflow.NewRuntime(workflow.CognitiveAgent{Provider: selectedProfile.Adapter()}),
		pack:    pack, assembler: svc.assembler, project: project, store: svc.store, runID: runModel.ID,
		renderer:         svc.renderer,
		imageResolver:    runImageResolver{runID: runModel.ID, projectID: project.ID, projectDir: project.WorkDir},
		semanticReviewer: workflow.LLMSemanticReviewer{Provider: selectedProfile.Adapter()},
	}
	createdRun, startErr := svc.engine.StartWithContext(ctx, runModel, execution, runContext)
	if startErr != nil {
		svc.completeCreateFailure(ctx, thread.ID, p.ClientRequestID, "INTERNAL")
		return model.Run{}, startErr
	}
	svc.completeCreateSuccess(ctx, thread.ID, p.ClientRequestID, createdRun.ID)
	return createdRun, nil
}

func (svc *RunService) ResumeRun(ctx context.Context, runID string) (model.Run, error) {
	runModel, err := svc.store.GetRun(ctx, runID)
	if err != nil {
		return model.Run{}, err
	}
	if runModel.Status != model.RunPaused {
		return model.Run{}, run.ErrRunNotRunning
	}
	project, err := svc.store.GetProject(ctx, runModel.ProjectID)
	if err != nil {
		return model.Run{}, err
	}
	checkpointStore, ok := svc.store.(interface {
		LatestCheckpoint(context.Context, string) (workflow.RuntimeCheckpoint, error)
	})
	if !ok {
		return model.Run{}, run.ErrContextStoreUnavailable
	}
	checkpoint, checkpointErr := checkpointStore.LatestCheckpoint(ctx, runID)
	if checkpointErr != nil && !errors.Is(checkpointErr, run.ErrRunNotFound) {
		return model.Run{}, checkpointErr
	}
	hasCheckpoint := checkpointErr == nil
	pack, err := svc.assembler.Assemble(ctx, contextengine.ContextRequest{
		RunID: runModel.ID, ThreadID: runModel.ThreadID, ProjectID: runModel.ProjectID,
		Command: runModel.Command, Budget: contextengine.DefaultBudget(),
	}, project)
	if err != nil {
		return model.Run{}, err
	}
	var reconciled workflow.RecoverySnapshot
	if hasCheckpoint {
		reconciled, err = workflow.ReconcileDirectWrites(ctx, project.WorkDir, checkpoint)
		if err != nil {
			return model.Run{}, err
		}
	}
	provider, err := svc.resumeProvider(runModel)
	if err != nil {
		return model.Run{}, err
	}
	runtime := workflow.NewRuntime(workflow.CognitiveAgent{Provider: provider})
	execution := &workflowExecution{
		runtime: runtime, pack: pack, assembler: svc.assembler, project: project, store: svc.store, runID: runModel.ID,
		renderer:         svc.renderer,
		imageResolver:    runImageResolver{runID: runModel.ID, projectID: project.ID, projectDir: project.WorkDir},
		semanticReviewer: workflow.LLMSemanticReviewer{Provider: provider},
		reconciliation:   reconciled,
	}
	if hasCheckpoint {
		execution.resumeCheckpoint = &checkpoint
	}
	resumed, err := svc.engine.Resume(ctx, runModel, execution)
	if err != nil {
		return model.Run{}, err
	}
	return resumed, nil
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

func (r runImageResolver) ResolveImage(ctx context.Context, ref string) (llm.ImageData, error) {
	if err := ctx.Err(); err != nil {
		return llm.ImageData{}, err
	}
	prefix := "run:" + r.runID + "/screenshot:"
	if !strings.HasPrefix(ref, prefix) {
		return llm.ImageData{}, errors.New("image reference does not belong to the current run")
	}
	screenshotID := strings.TrimPrefix(ref, prefix)
	if !screenshotIDPattern.MatchString(screenshotID) || strings.TrimSpace(r.projectID) == "" {
		return llm.ImageData{}, errors.New("invalid runtime screenshot reference")
	}
	path := filepath.Join(r.projectDir, ".runtime", "renders", r.runID, screenshotID+".png")
	raw, err := readImageWithContext(ctx, path, 10*1024*1024)
	if err != nil {
		return llm.ImageData{}, err
	}
	if len(raw) < 8 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" {
		return llm.ImageData{}, errors.New("runtime screenshot MIME or size is invalid")
	}
	return llm.ImageData{Bytes: raw, MIMEType: "image/png"}, nil
}

func readImageWithContext(ctx context.Context, path string, maxBytes int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	raw := make([]byte, 0, min(maxBytes, 64*1024))
	buffer := make([]byte, 64*1024)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		count, readErr := file.Read(buffer)
		if count > 0 {
			if len(raw)+count > maxBytes {
				return nil, errors.New("runtime screenshot MIME or size is invalid")
			}
			raw = append(raw, buffer[:count]...)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return raw, nil
}

func (svc *RunService) InjectInput(ctx context.Context, runID, content, replyTo string) error {
	return svc.engine.InjectInput(ctx, runID, content, replyTo)
}

func (svc *RunService) SubmitPlanApproval(ctx context.Context, runID string, answer model.PlanApprovalAnswer) error {
	return svc.engine.SubmitPlanApproval(ctx, runID, answer)
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

func (svc *RunService) GetActiveRunForThread(ctx context.Context, threadID string) (model.Run, error) {
	finder, ok := svc.store.(interface {
		GetActiveRunForThread(context.Context, string) (model.Run, error)
	})
	if !ok {
		return model.Run{}, run.ErrRunNotFound
	}
	return finder.GetActiveRunForThread(ctx, threadID)
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

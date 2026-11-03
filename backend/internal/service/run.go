package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/attachment"
	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextcompact"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/idempotency"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/projecthistory"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

var screenshotIDPattern = regexp.MustCompile(`^shot_[A-Za-z0-9-]{1,128}$`)
var clientIdentityPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

type WorkRoot string

type ExecutionFactory func(r model.Run, p model.CreateRunParams, proj model.Project) run.Execution

type RunService struct {
	createMu    sync.Mutex
	snapshots   sync.Map // Run ID -> immutable model configuration while running/paused.
	history     *projecthistory.Manager
	store       store.Store
	engine      *run.Engine
	factory     ExecutionFactory
	assembler   *contextengine.ContextAssembler
	renderer    workflow.SlideRenderer
	registry    *llm.Registry
	skills      *SkillService
	components  *ComponentService
	themes      *ThemeService
	transcripts *contextengine.ContextStore
	attachments *AttachmentService
	naming      *NamingService
	commands    *commandexec.Runner
}

func (svc *RunService) domainTools() workflow.DomainToolProvider {
	return workflow.DefaultDomainToolProvider{Renderer: svc.renderer, Components: svc.components, Skills: svc.skills, Themes: svc.themes, Commands: svc.commands}
}

func (svc *RunService) WithCommandRunner(runner *commandexec.Runner) *RunService {
	svc.commands = runner
	return svc
}

func (svc *RunService) WithCompletedRunObserver(observer func(model.Run)) {
	svc.engine.WithCompletedRunObserver(observer)
}

func (svc *RunService) WithNaming(naming *NamingService) *RunService {
	svc.naming = naming
	if svc.history != nil && naming != nil {
		svc.history.SnapshotGuard = naming.BeginProjectSnapshot
	}
	return svc
}

func (svc *RunService) EnableProjectHistory(root string) (*projecthistory.Manager, error) {
	snapshotStore, ok := svc.store.(projecthistory.SnapshotStore)
	if !ok {
		return nil, errors.New("project snapshot store required")
	}
	manager := projecthistory.New(snapshotStore, svc.engine.ProjectLocks(), root)
	if err := manager.Initialize(context.Background()); err != nil {
		return nil, err
	}
	svc.history = manager
	if svc.naming != nil {
		manager.SnapshotGuard = svc.naming.BeginProjectSnapshot
	}
	return manager, nil
}

func NewRunService(
	s store.Store,
	engine *run.Engine,
	registry *llm.Registry,
	workRoot WorkRoot,
	renderer *workflow.NodeSlideRenderer,
	transcripts *contextengine.ContextStore,
) *RunService {
	components := NewComponentService(workRoot, s)
	skills := NewSkillService(workRoot, s)
	themes := NewThemeService(workRoot, s)
	if transcripts == nil {
		transcripts = contextengine.NewContextStore(s)
	}
	svc := &RunService{
		store: s, engine: engine,
		assembler: contextengine.NewContextAssembler().
			WithComponentLoader(components).
			WithSkillLoader(skills),
		renderer:    renderer,
		registry:    registry,
		skills:      skills,
		components:  components,
		themes:      themes,
		transcripts: transcripts,
		attachments: NewAttachmentService(s),
	}
	if engine != nil {
		engine.WithInputResolver(svc.resolveInput)
		engine.WithCompletedRunHandler(svc.startPendingMessages)
	}
	return svc
}

func NewRunServiceWithExecutionFactory(s store.Store, engine *run.Engine, factory ExecutionFactory) *RunService {
	svc := &RunService{store: s, engine: engine, factory: factory, attachments: NewAttachmentService(s)}
	if engine != nil {
		engine.WithCompletedRunHandler(svc.startPendingMessages)
	}
	return svc
}

func NewRunServiceWithExecutionFactoryAndRegistry(
	s store.Store,
	engine *run.Engine,
	factory ExecutionFactory,
	registry *llm.Registry,
) *RunService {
	svc := NewRunServiceWithExecutionFactory(s, engine, factory)
	svc.registry = registry
	return svc
}

func (svc *RunService) startPendingMessages(ctx context.Context, previous model.Run) error {
	messages, err := svc.store.ListPendingSteering(ctx, previous.ID)
	if err != nil || len(messages) == 0 {
		return err
	}
	first := messages[0]
	command := previous.Command
	command.Instruction = first.Content
	// All transferred messages are initial inputs of one new execution. Start
	// from their latest accepted scope with a fresh revision, so the first
	// checkpoint and queued observations use the same scope authority.
	for _, message := range messages {
		if message.Scope.Source.Kind != "" {
			command.Scope = message.Scope
		}
	}
	command.Scope.Revision = 1
	attachmentIDs := make([]string, 0, len(first.Attachments))
	for _, attachment := range first.Attachments {
		attachmentIDs = append(attachmentIDs, attachment.ID)
	}
	ctx = run.WithSteeringHandoff(ctx, previous.ID, messages)
	_, err = svc.CreateRun(ctx, previous.ThreadID, model.CreateRunParams{
		SceneRevision: previous.SceneRevision, ClientRequestID: "queued_" + previous.ID, Model: previous.Model.ProfileName,
		Command: command, AttachmentIDs: attachmentIDs,
		DOMSelections: first.DOMSelections, ReferenceOrder: first.ReferenceOrder,
	})
	return err
}

// Recover the completion-to-start gap after a process restart. Accepted inbox
// entries are consumed only by the same transaction that accepts the new Run.
func (svc *RunService) RecoverPendingMessages(ctx context.Context) error {
	if svc.engine == nil {
		return nil
	}
	if err := svc.engine.RecoverInputChecks(ctx); err != nil {
		return err
	}
	return svc.engine.RecoverPendingMessages(ctx)
}

func (svc *RunService) StopDeleted(ctx context.Context, projectID, threadID string) error {
	if svc.engine == nil {
		return nil
	}
	return svc.engine.StopDeleted(ctx, projectID, threadID)
}

type workflowExecution struct {
	releaseSnapshot  func()
	runtime          *workflow.Runtime
	pack             contextengine.ContextPack
	assembler        *contextengine.ContextAssembler
	project          model.Project
	store            store.Store
	request          contextengine.ContextRequest
	renderer         workflow.SlideRenderer
	commands         *commandexec.Runner
	components       *ComponentService
	skills           *SkillService
	themes           *ThemeService
	imageResolver    llm.ImageRefResolver
	reviewer         workflow.TaskReviewer
	transcripts      *contextengine.ContextStore
	resumeCheckpoint *workflow.RuntimeCheckpoint
}

func (r *workflowExecution) Release() {
	if r.releaseSnapshot != nil {
		r.releaseSnapshot()
	}
}

type planApprovalCommitStore interface {
	CommitPlanApproval(context.Context, string, model.RunMode, workflow.RuntimeCheckpoint) error
}

type scopeExpansionCommitStore interface {
	CommitScopeExpansion(context.Context, string, model.RunScope, workflow.RuntimeCheckpoint) error
}

func (r *workflowExecution) commitScopeExpansion(ctx context.Context, scope model.RunScope, checkpoint workflow.RuntimeCheckpoint) error {
	store, ok := r.store.(scopeExpansionCommitStore)
	if !ok {
		return errors.New("atomic scope expansion store is required")
	}
	if err := store.CommitScopeExpansion(ctx, r.request.RunID, scope, checkpoint); err != nil {
		return err
	}
	r.request.Command.Scope = scope
	return nil
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
	return store.CommitPlanApproval(ctx, r.request.RunID, mode, checkpoint)
}

func (r *workflowExecution) Run(ctx context.Context, emitter workflow.EventEmitter, checkpoint run.Checkpointer, prompter run.Prompter) workflow.StructuredOutcome {
	// Only an admitted task reaches the workflow. A resume keeps the journal's
	// latest snapshot, including a manual clear made while the run was paused.
	source, err := r.store.LatestThreadPlan(ctx, r.request.ThreadID)
	if err != nil {
		return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "CONTEXT_SOURCE_INVALID", Message: err.Error()}
	}
	plan := source.Plan
	if r.resumeCheckpoint == nil && plan != nil && source.RunID != r.request.RunID {
		if plan.Complete() {
			plan = nil
		}
		emitter.Emit(model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: model.NewPublicEventBase(r.request.RunID), Plan: plan})
		if writer, ok := emitter.(interface{ Err() error }); ok && writer.Err() != nil {
			return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: model.CodeInternal, Message: writer.Err().Error()}
		}
	}
	// A standalone reply never needs the authoring context or resource catalogs.
	if r.resumeCheckpoint == nil {
		pack, err := r.assembler.Assemble(ctx, r.request, r.project)
		if err != nil {
			return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "CONTEXT_SOURCE_INVALID", Message: err.Error()}
		}
		r.pack = pack
	}
	// A real task can arrive while a standalone reply is being prepared.
	// Keep the accepted command for validation and only expose the task inputs.
	initialIsTask := checkpoint.InitialInputIsTask()
	taskPack := func(pack contextengine.ContextPack) contextengine.ContextPack {
		if !initialIsTask {
			pack.OmitInitialInstruction = true
			pack.Command.Instruction = ""
		}
		return pack
	}
	committer := workflowCommitter{store: r.store, project: r.project, runID: r.request.RunID}
	outcome := r.runtime.Run(ctx, workflow.RuntimeInput{
		RunID: r.request.RunID, ProjectDir: r.project.WorkDir, Context: taskPack(r.pack), CurrentPlan: plan,
		Emitter: emitter, Prompter: prompter, Steering: checkpoint, Checkpoint: checkpoint,
		CommitMetadata:   committer.Commit,
		Logger:           zap.L().Named("ppt-runtime"),
		Trace:            workflow.ZapTraceRecorder{Logger: zap.L().Named("ppt-runtime-trace")},
		DomainTools:      workflow.DefaultDomainToolProvider{Renderer: r.renderer, Components: r.components, Skills: r.skills, Themes: r.themes, Commands: r.commands},
		ImageResolver:    r.imageResolver,
		Lifecycle:        checkpoint,
		Idempotency:      r.store,
		ToolExecutions:   r.store,
		Reviewer:         r.reviewer,
		ResumeCheckpoint: r.resumeCheckpoint,
		PreviewProject:   func(ctx context.Context) (model.Project, error) { return r.store.GetProject(ctx, r.project.ID) },
		ResourceApproval: func(ctx context.Context, runID, interactionID string) (model.ResourceEditApproval, error) {
			current, err := r.store.Interaction(ctx, runID, "resource", interactionID)
			if err != nil {
				return model.ResourceEditApproval{}, err
			}
			return current.ResourceEditApproval()
		},
		RefreshContext: func(ctx context.Context, mode model.RunMode) (contextengine.ContextPack, error) {
			if r.assembler == nil {
				return contextengine.ContextPack{}, errors.New("context assembler is required for mode transition")
			}
			request := r.request
			request.Command.Mode = mode
			pack, err := r.assembler.Assemble(ctx, request, r.project)
			return taskPack(pack), err
		},
		CommitPlanApproval: func(ctx context.Context, mode model.RunMode, pack contextengine.ContextPack, cp workflow.RuntimeCheckpoint) error {
			err := r.commitPlanApproval(ctx, mode, pack, cp)
			if err != nil {
				if pauser, ok := checkpoint.(interface{ PersistenceFailed(context.Context, string) }); ok {
					pauser.PersistenceFailed(ctx, "approval_commit_failed")
				}
			}
			return err
		},
		CommitScopeExpansion: func(ctx context.Context, scope model.RunScope, cp workflow.RuntimeCheckpoint) error {
			err := r.commitScopeExpansion(ctx, scope, cp)
			if err != nil {
				if pauser, ok := checkpoint.(interface{ PersistenceFailed(context.Context, string) }); ok {
					pauser.PersistenceFailed(ctx, "scope_commit_failed")
				}
			}
			return err
		},
		Transcript: r.transcripts,
	})
	return outcome
}

func (svc *RunService) CreateRun(ctx context.Context, threadID string, p model.CreateRunParams) (model.Run, error) {
	svc.createMu.Lock()
	defer svc.createMu.Unlock()
	thread, err := svc.store.GetThread(ctx, threadID)
	if err != nil {
		return model.Run{}, err
	}
	scene, err := svc.store.ThreadSceneRevision(ctx, threadID)
	if err != nil {
		return model.Run{}, err
	}
	if p.SceneRevision != scene {
		return model.Run{}, sessionstate.ErrSceneChanged
	}
	ctx = sessionstate.WithRequestScene(ctx, p.SceneRevision)
	project, err := svc.store.GetProject(ctx, thread.ProjectID)
	if err != nil {
		return model.Run{}, err
	}
	command := p.Command
	if svc.attachments == nil {
		return model.Run{}, errors.New("attachment service is unavailable")
	}
	attachments, err := svc.attachments.ResolveReferences(ctx, project, p.AttachmentIDs)
	if err != nil {
		return model.Run{}, err
	}
	command.Attachments = attachments
	command.DOMSelections = append([]model.DOMSelection{}, p.DOMSelections...)
	command.ReferenceOrder = append([]model.ReferenceOrderItem{}, p.ReferenceOrder...)
	var snapshot *spec.ProjectContentSnapshot
	loadSnapshot := func() (spec.ProjectContentSnapshot, error) {
		if snapshot != nil {
			return *snapshot, nil
		}
		value, snapshotErr := NewPPTMutationService(svc.store).Snapshot(ctx, project.ID)
		if snapshotErr == nil {
			snapshot = &value
		}
		return value, snapshotErr
	}
	if p.ScopeInput != nil {
		value, snapshotErr := loadSnapshot()
		if snapshotErr != nil {
			return model.Run{}, snapshotErr
		}
		command.Scope, err = resolveRunScope(value, *p.ScopeInput)
		if err != nil {
			return model.Run{}, err
		}
	}
	if len(command.DOMSelections) > 0 {
		value, snapshotErr := loadSnapshot()
		if snapshotErr != nil {
			return model.Run{}, snapshotErr
		}
		if err := validateSelectionProject(value, command.DOMSelections); err != nil {
			return model.Run{}, domSelectionAgentError("create_run", err)
		}
		if p.RestoredCheckpoint {
			if err := validateRestoredDOM(project, value, command.DOMSelections); err != nil {
				return model.Run{}, err
			}
		}
		reconcileSelectionHTML(value, command.DOMSelections)
		command.Scope = mergeSelectionScope(command.Scope, value, command.DOMSelections)
	}
	if len(p.SkillIDs) > 0 {
		if svc.skills == nil {
			return model.Run{}, model.NewAgentError(model.CodeInternal, "create_run", errors.New("skill repository service is unavailable"))
		}
		command.Skills, err = svc.skills.Resolve(p.SkillIDs)
		if err != nil {
			return model.Run{}, err
		}
	}
	if len(p.ComponentNames) > 0 {
		if svc.components == nil {
			return model.Run{}, model.NewAgentError(model.CodeInternal, "create_run", errors.New("component repository service is unavailable"))
		}
		command.Components, err = svc.components.ResolveByNames(p.ComponentNames)
		if err != nil {
			return model.Run{}, err
		}
	}
	if len(p.MentionedSlideIDs) > 0 {
		value, snapshotErr := loadSnapshot()
		if snapshotErr != nil {
			return model.Run{}, snapshotErr
		}
		command.MentionedPages, command.DroppedMentionedSlideIDs, err = resolveMentionedPages(value, p.MentionedSlideIDs)
		if err != nil {
			return model.Run{}, err
		}
	}
	if len(command.DroppedMentionedSlideIDs) > 0 {
		return model.Run{}, invalidRestoredReference("恢复的页面引用已失效，请移除后重新选择")
	}
	if err := command.Validate(); err != nil {
		return model.Run{}, err
	}
	var selectedProfile llm.Profile
	modelSnapshot := svc.registry.Snapshot()
	if svc.registry != nil {
		selectedProfile, err = modelSnapshot.RoutedProfile("main", p.Model)
		if err != nil {
			return model.Run{}, model.NewAgentError("MODEL_PROFILE_NOT_FOUND", "create_run", nil)
		}
		p.Model = selectedProfile.Name()
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
		"mode": command.Mode, "skills": command.Skills,
		"components": command.Components, "mentioned_pages": command.MentionedPages,
		"dropped_mentioned_slide_ids": command.DroppedMentionedSlideIDs, "attachments": command.Attachments,
		"dom_selections": command.DOMSelections, "reference_order": command.ReferenceOrder, "model": p.Model,
	})
	if err != nil {
		return model.Run{}, err
	}
	record, err := svc.store.GetIdempotency(ctx, "create_run:"+fmt.Sprint(p.SceneRevision), thread.ID, p.ClientRequestID)
	if err == nil {
		if record.RequestHash != requestHash {
			return model.Run{}, model.NewAgentError("IDEMPOTENCY_KEY_REUSED", "create_run", nil)
		}
		return svc.replayCreateRun(ctx, record)
	}
	if !errors.Is(err, run.ErrRunNotFound) {
		return model.Run{}, err
	}
	// Serialize runs per project: each active run may commit tool-level project
	// transactions, so a second concurrent run would race on the same files.
	if _, handoff := run.SteeringHandoffFromContext(ctx); !handoff && svc.engine != nil && svc.engine.HasActiveProject(project.ID) {
		return model.Run{}, ErrRunActive
	}
	if active, activeErr := svc.store.HasActiveRun(ctx, project.ID); activeErr != nil {

		return model.Run{}, activeErr
	} else if active {

		return model.Run{}, ErrRunActive
	}
	if active, activeErr := svc.store.HasActiveGitCommit(ctx, project.ID); activeErr != nil {

		return model.Run{}, activeErr
	} else if active {

		return model.Run{}, ErrGitCommitActive
	}
	if err := svc.recoverProjectMutations(ctx, project); err != nil {

		return model.Run{}, err
	}
	runModel := model.Run{
		ID: uuid.NewString(), ThreadID: thread.ID, ProjectID: project.ID, SceneRevision: p.SceneRevision,
		ClientRequestID: p.ClientRequestID, RequestHash: requestHash, Command: command,
	}
	if svc.history != nil {
		_, baselineErr := svc.history.Baseline(ctx, project, runModel.ID, thread.ID, p)
		if baselineErr != nil {

			return model.Run{}, baselineErr
		}
	}
	if selectedProfile.Adapter() != nil {
		runModel.Model = model.ModelSelection{
			ProfileName: selectedProfile.Name(), Provider: selectedProfile.ProviderName(),
			Model: selectedProfile.Model(), URL: selectedProfile.URL(),
		}
	}
	if svc.assembler == nil || selectedProfile.Adapter() == nil {
		if svc.factory == nil {

			return model.Run{}, ErrRunScopeUnsupported
		}
		execution := svc.factory(runModel, p, project)
		if execution == nil {

			return model.Run{}, ErrRunScopeUnsupported
		}
		createdRun, startErr := svc.engine.Start(ctx, runModel, execution)
		if startErr != nil {

			if errors.Is(startErr, store.ErrGitCommitActive) {
				return model.Run{}, ErrGitCommitActive
			}
			return model.Run{}, startErr
		}

		return createdRun, nil
	}
	runtime := workflow.NewRuntime(workflow.CognitiveAgent{Provider: selectedProfile.Adapter()})
	runtime.ResourceApprovalRequired = svc.resourceApprovalRequired
	runtime.HighRiskApprovalRequired = svc.highRiskApprovalRequired
	runtime.Decisions = modelSnapshot.DecisionSnapshot()
	runtime.Compactor = contextcompact.New(llm.SideProvider{Registry: modelSnapshot, Purpose: "compact"})
	svc.snapshots.Store(runModel.ID, modelSnapshot)
	execution := &workflowExecution{
		releaseSnapshot: func() { svc.snapshots.Delete(runModel.ID) },
		runtime:         runtime,
		request:         contextengine.ContextRequest{RunID: runModel.ID, ThreadID: thread.ID, ProjectID: project.ID, Command: command},
		assembler:       svc.assembler, project: project, store: svc.store,
		renderer: svc.renderer, components: svc.components, skills: svc.skills, themes: svc.themes, commands: svc.commands,
		imageResolver: runImageResolver{projectID: project.ID, projectDir: project.WorkDir},
		reviewer:      workflow.LLMTaskReviewer{Provider: selectedProfile.Adapter()},
		transcripts:   svc.transcripts,
	}
	createdRun, startErr := svc.engine.Start(ctx, runModel, execution)
	if startErr != nil {
		svc.snapshots.Delete(runModel.ID)

		if errors.Is(startErr, store.ErrGitCommitActive) {
			return model.Run{}, ErrGitCommitActive
		}
		return model.Run{}, startErr
	}

	return createdRun, nil
}

func (svc *RunService) resourceApprovalRequired() (bool, error) {
	settings, err := svc.registry.AgentSettings()
	return settings.RequireResourceApproval, err
}

func (svc *RunService) highRiskApprovalRequired() (bool, error) {
	settings, err := svc.registry.AgentSettings()
	return settings.RequireHighRiskApproval, err
}

func (svc *RunService) ResumeRun(ctx context.Context, runID string, action sessionstate.RunAction) (sessionstate.RunMutation, error) {
	svc.createMu.Lock()
	defer svc.createMu.Unlock()
	if previous, found, err := svc.store.RunResumeReceipt(ctx, runID, action); err != nil || found {
		return previous, err
	}
	runModel, err := svc.store.GetRun(ctx, runID)
	if err != nil {
		return sessionstate.RunMutation{}, err
	}
	if runModel.SceneRevision != action.SceneRevision {
		return sessionstate.RunMutation{}, sessionstate.ErrSceneChanged
	}
	if runModel.EntityRevision != action.EntityRevision {
		return sessionstate.RunMutation{}, sessionstate.ErrVersionConflict
	}
	if !runModel.CanContinue {
		return sessionstate.RunMutation{}, run.ErrRunNotContinuable
	}
	release, acquired := svc.engine.ProjectLocks().TryAcquire(runModel.ProjectID)
	if !acquired {
		return sessionstate.RunMutation{}, run.ErrRunNotContinuable
	}
	defer release()
	project, err := svc.store.GetProject(ctx, runModel.ProjectID)
	if err != nil {
		return sessionstate.RunMutation{}, err
	}
	if err := svc.recoverProjectMutations(ctx, project); err != nil {
		return sessionstate.RunMutation{}, err
	}
	checkpointStore, ok := svc.store.(interface {
		LatestCheckpoint(context.Context, string) (workflow.RuntimeCheckpoint, error)
	})
	if !ok {
		return sessionstate.RunMutation{}, run.ErrContextStoreUnavailable
	}
	checkpoint, checkpointErr := checkpointStore.LatestCheckpoint(ctx, runID)
	if checkpointErr != nil && !errors.Is(checkpointErr, run.ErrRunNotFound) {
		return sessionstate.RunMutation{}, checkpointErr
	}
	if checkpointErr != nil || (runModel.Status != model.RunPaused && !checkpoint.ContinuationAllowed) {
		return sessionstate.RunMutation{}, run.ErrRunNotContinuable
	}
	checkpoint.BudgetBaseTurns = checkpoint.Turns
	checkpoint.BudgetBaseDurationMS = checkpoint.ActiveDurationMS
	checkpoint.ContinuationAllowed = false
	checkpoint.CompletionFailures = 0
	checkpoint.ReadLoop = workflow.ReadLoopState{}
	if checkpoint.RunID != runID || checkpoint.Scope.Validate() != nil {
		return sessionstate.RunMutation{}, run.ErrRunNotContinuable
	}
	// Assemble from the latest authorization, including run-created pages.
	runModel.Command.Scope = checkpoint.Scope
	request := contextengine.ContextRequest{RunID: runModel.ID, ThreadID: runModel.ThreadID, ProjectID: runModel.ProjectID, Command: runModel.Command}
	pack, err := svc.assembler.Assemble(ctx, request, project)
	if err != nil {
		return sessionstate.RunMutation{}, err
	}
	reconciled, err := workflow.ReconcileDirectWrites(ctx, project.WorkDir, checkpoint)
	if err != nil {
		return sessionstate.RunMutation{}, err
	}
	if continuationHasArtifactConflict(checkpoint, reconciled) {
		return sessionstate.RunMutation{}, run.ErrContinuationConflict
	}
	provider, err := svc.resumeProvider(runModel, checkpoint.ModelRoute)
	if err != nil {
		return sessionstate.RunMutation{}, err
	}
	runtime := workflow.NewRuntime(workflow.CognitiveAgent{Provider: provider})
	runtime.ResourceApprovalRequired = svc.resourceApprovalRequired
	runtime.HighRiskApprovalRequired = svc.highRiskApprovalRequired
	route := provider.(*llm.RoutedProvider)
	runtime.Decisions = route.Snapshot().DecisionSnapshot()
	runtime.Compactor = contextcompact.New(llm.SideProvider{Registry: route.Snapshot(), Purpose: "compact"})
	svc.snapshots.Store(runModel.ID, route.Snapshot())
	execution := &workflowExecution{
		releaseSnapshot: func() { svc.snapshots.Delete(runModel.ID) },
		runtime:         runtime, pack: pack, request: request, assembler: svc.assembler, project: project, store: svc.store,
		renderer: svc.renderer, components: svc.components, skills: svc.skills, themes: svc.themes, commands: svc.commands,
		imageResolver: runImageResolver{projectID: project.ID, projectDir: project.WorkDir},
		reviewer:      workflow.LLMTaskReviewer{Provider: provider},
		transcripts:   svc.transcripts,
	}
	execution.resumeCheckpoint = &checkpoint
	resumed, err := svc.engine.Resume(ctx, runModel, execution, action)
	if err != nil {
		svc.snapshots.Delete(runModel.ID)
		return sessionstate.RunMutation{}, err
	}
	return resumed, nil
}

// A completed deletion is expected to be absent. A restored preimage is a
// conflict for continuation even though crash recovery can reconcile it.
func continuationHasArtifactConflict(checkpoint workflow.RuntimeCheckpoint, snapshot workflow.RecoverySnapshot) bool {
	deleted := make(map[string]bool)
	for _, change := range checkpoint.Changes.Deleted {
		deleted[change.Artifact.Key()] = true
	}
	for _, result := range snapshot.Results {
		if deleted[result.Artifact.Key()] {
			if result.Status != workflow.ReconcileMissingArtifact {
				return true
			}
			continue
		}
		if result.Status != workflow.ReconcileClean || result.CurrentHash != result.ExpectedHash {
			return true
		}
	}
	return false
}

func (svc *RunService) recoverProjectMutations(ctx context.Context, project model.Project) error {
	return workflow.RecoverMutationJournals(ctx, project.WorkDir, artifactReceiptLookup(svc.store))
}

// RecoverAllProjectMutations resolves crash journals before the HTTP server
// exposes project files to clients.
func (svc *RunService) RecoverAllProjectMutations(ctx context.Context) error {
	projects, err := svc.store.ListProjects(ctx)
	if err != nil {
		return err
	}
	for _, project := range projects {
		if err := svc.recoverProjectMutations(ctx, project); err != nil {
			return fmt.Errorf("recover project %s mutations: %w", project.ID, err)
		}
	}
	return nil
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
	projectID  string
	projectDir string
}

func (r runImageResolver) ResolveImage(ctx context.Context, ref string) (llm.ImageData, error) {
	if err := ctx.Err(); err != nil {
		return llm.ImageData{}, err
	}
	if strings.HasPrefix(ref, "project:"+r.projectID+"/render:") {
		_, raw, err := renderimage.Read(ctx, r.projectDir, r.projectID, ref)
		if err != nil {
			return llm.ImageData{}, err
		}
		return llm.ImageData{Bytes: raw, MIMEType: "image/png"}, nil
	}
	attachmentID, variant, ok := attachment.ParseImageRef(r.projectID, ref)
	if !ok {
		return llm.ImageData{}, errors.New("image reference is not a current project attachment or indexed render")
	}
	meta, raw, err := attachment.Read(ctx, r.projectDir, r.projectID, attachmentID, variant)
	if err != nil {
		return llm.ImageData{}, err
	}
	mimeType := meta.MediaType
	if variant == "thumbnail" {
		mimeType = "image/webp"
	}
	return llm.ImageData{Bytes: raw, MIMEType: mimeType}, nil
}

func (svc *RunService) InjectInput(ctx context.Context, runID, content, replyTo string) (sessionstate.Receipt, error) {
	return svc.engine.InjectInput(ctx, runID, content, replyTo)
}

func (svc *RunService) SubmitPlanApproval(ctx context.Context, runID string, answer model.PlanApprovalAnswer) (sessionstate.Receipt, error) {
	return svc.engine.SubmitPlanApproval(ctx, runID, answer)
}

func (svc *RunService) SubmitCommandPermission(ctx context.Context, runID string, answer model.CommandPermissionAnswer) (sessionstate.Receipt, error) {
	return svc.engine.SubmitCommandPermission(ctx, runID, answer)
}

func (svc *RunService) SubmitScopeExpansion(ctx context.Context, runID string, answer model.ScopeExpansionAnswer) (sessionstate.Receipt, error) {
	return svc.engine.SubmitScopeExpansion(ctx, runID, answer)
}

func (svc *RunService) GetResourceEditApproval(ctx context.Context, runID, interactionID string) (model.ResourceEditApproval, error) {
	interaction, err := svc.store.Interaction(ctx, runID, "resource", interactionID)
	if err != nil {
		return model.ResourceEditApproval{}, err
	}
	return interaction.ResourceEditApproval()
}

func (svc *RunService) resourceApprovalWriteProject(ctx context.Context, runID, interactionID string) (model.Project, error) {
	scene, ok := sessionstate.RequestScene(ctx)
	if !ok {
		return model.Project{}, sessionstate.ErrSceneChanged
	}
	revision, ok := sessionstate.InteractionRevision(ctx)
	if !ok {
		return model.Project{}, sessionstate.ErrVersionConflict
	}
	current, err := svc.store.GetRun(ctx, runID)
	if err != nil {
		return model.Project{}, err
	}
	if current.SceneRevision != scene {
		return model.Project{}, sessionstate.ErrSceneChanged
	}
	interaction, err := svc.store.Interaction(ctx, runID, "resource", interactionID)
	if err != nil {
		return model.Project{}, err
	}
	if interaction.RequestRevision != revision {
		return model.Project{}, sessionstate.ErrVersionConflict
	}
	return svc.store.GetProject(ctx, current.ProjectID)
}

func (svc *RunService) UpdateResourceEditApproval(ctx context.Context, runID, interactionID string, revision int64, draft json.RawMessage) (model.ResourceEditApproval, error) {
	if previous, found, err := svc.store.ResourceDraftReceipt(ctx, runID, interactionID, revision, draft); err != nil || found {
		return previous, err
	}
	project, err := svc.resourceApprovalWriteProject(ctx, runID, interactionID)
	if err != nil {
		return model.ResourceEditApproval{}, err
	}
	current, err := svc.GetResourceEditApproval(ctx, runID, interactionID)
	if err != nil {
		return model.ResourceEditApproval{}, err
	}
	record, err := workflow.PrepareResourceEditDraft(project.WorkDir, current, revision, draft)
	if err != nil {
		return model.ResourceEditApproval{}, err
	}
	if err := svc.store.UpdateResourceInteraction(ctx, record, draft); err != nil {
		return model.ResourceEditApproval{}, err
	}
	svc.engine.NotifyResourceEditApproval(runID, model.ResourceEditApprovalRequestedPayload{
		PublicEventBase: model.NewPublicEventBase(runID), InteractionID: record.InteractionID,
		CallID: record.CallID, Resource: record.Resource, Revision: record.Revision, Target: record.Target,
	})
	return record, nil
}

func (svc *RunService) SubmitResourceEditApproval(ctx context.Context, runID, interactionID string, answer model.ResourceEditApprovalAnswer) (sessionstate.Receipt, error) {
	project, err := svc.resourceApprovalWriteProject(ctx, runID, interactionID)
	if err != nil {
		return sessionstate.Receipt{}, err
	}
	record, err := svc.GetResourceEditApproval(ctx, runID, interactionID)
	if err != nil {
		return sessionstate.Receipt{}, err
	}
	// An accepted decision remains retryable after the artifact was committed.
	// Its canonical answer hash is checked by the same durable commit receipt.
	if record.State == "pending" && answer.Decision == "approve" && !workflow.ResourceEditApprovalBaselineMatches(project.WorkDir, record) {
		return sessionstate.Receipt{}, workflow.ErrResourceApprovalConflict
	}
	if answer.InteractionID != interactionID {
		return sessionstate.Receipt{}, run.ErrReplyMismatch
	}
	return svc.engine.SubmitResourceEditApproval(ctx, runID, answer)
}

func (svc *RunService) RequestCancel(ctx context.Context, runID string, reason model.RunCancelReason, action sessionstate.RunAction) (sessionstate.RunMutation, error) {
	return svc.engine.RequestCancelWithReason(ctx, runID, reason, action)
}

func (svc *RunService) Steer(ctx context.Context, runID, expectedRunID, clientMessageID, content string, attachmentIDs []string, domSelections []model.DOMSelection, referenceOrder []model.ReferenceOrderItem) (model.SteeringMessage, error) {
	if !clientIdentityPattern.MatchString(clientMessageID) || len([]rune(content)) > 8000 {
		return model.SteeringMessage{}, model.NewAgentError("BAD_REQUEST", "steer_run", nil)
	}
	runModel, err := svc.store.GetRun(ctx, runID)
	if err != nil {
		return model.SteeringMessage{}, err
	}
	scene, ok := sessionstate.RequestScene(ctx)
	if !ok || scene != runModel.SceneRevision {
		return model.SteeringMessage{}, sessionstate.ErrSceneChanged
	}
	project, err := svc.store.GetProject(ctx, runModel.ProjectID)
	if err != nil {
		return model.SteeringMessage{}, err
	}
	if svc.attachments == nil {
		return model.SteeringMessage{}, errors.New("attachment service is unavailable")
	}
	attachments, err := svc.attachments.ResolveReferences(ctx, project, attachmentIDs)
	if err != nil {
		return model.SteeringMessage{}, err
	}
	if err := model.ValidateDOMSelections(domSelections, attachments, referenceOrder); err != nil {
		return model.SteeringMessage{}, domSelectionAgentError("steer_run", err)
	}
	if strings.TrimSpace(content) == "" && ((len(domSelections) > 0 && !model.DOMSelectionsHaveIntent(domSelections)) || (len(domSelections) == 0 && len(attachments) == 0)) {
		return model.SteeringMessage{}, model.NewAgentError("BAD_REQUEST", "steer_run", nil)
	}
	snapshot, err := NewPPTMutationService(svc.store).Snapshot(ctx, project.ID)
	if err != nil {
		return model.SteeringMessage{}, err
	}
	if err := validateSelectionProject(snapshot, domSelections); err != nil {
		return model.SteeringMessage{}, domSelectionAgentError("steer_run", err)
	}
	reconcileSelectionHTML(snapshot, domSelections)
	requestHash, err := idempotency.CanonicalHash(map[string]any{
		"expected_run_id": expectedRunID, "client_message_id": clientMessageID, "content": content, "attachments": attachments,
		"dom_selections": domSelections, "reference_order": referenceOrder,
	})
	if err != nil {
		return model.SteeringMessage{}, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		mergedScope := mergeSelectionScope(runModel.Command.Scope, snapshot, domSelections)
		message, steerErr := svc.engine.SteerWithReferences(ctx, runID, expectedRunID, clientMessageID, requestHash, content, attachments, domSelections, referenceOrder, mergedScope)
		var agentErr *model.AgentError
		if steerErr == nil || !errors.As(steerErr, &agentErr) || agentErr.Code != "RUN_REVISION_CONFLICT" || attempt == 1 {
			return message, steerErr
		}
		// A concurrent monotonic scope update won the CAS. Re-read once and
		// apply the same union/escalation rules; a second conflict is surfaced.
		runModel, err = svc.store.GetRun(ctx, runID)
		if err != nil {
			return model.SteeringMessage{}, err
		}
		snapshot, err = NewPPTMutationService(svc.store).Snapshot(ctx, project.ID)
		if err != nil {
			return model.SteeringMessage{}, err
		}
	}
	return model.SteeringMessage{}, model.NewAgentError("RUN_REVISION_CONFLICT", "steer_run", nil)
}

func (svc *RunService) GetRun(ctx context.Context, runID string) (model.Run, error) {
	value, err := svc.store.GetRun(ctx, runID)
	if err != nil {
		return value, err
	}
	if value.ExecutionModel == nil && value.Model.ProfileName != "" {
		value.ExecutionModel = &model.ActiveModelSelection{Profile: value.Model.ProfileName, Provider: value.Model.Provider, Model: value.Model.Model}
	}

	return value, nil
}

func (svc *RunService) GetActiveRunForThread(ctx context.Context, threadID string) (model.Run, error) {
	finder, ok := svc.store.(interface {
		GetActiveRunForThread(context.Context, string) (model.Run, error)
	})
	if !ok {
		return model.Run{}, run.ErrRunNotFound
	}
	value, err := finder.GetActiveRunForThread(ctx, threadID)
	if err != nil {
		return value, err
	}
	return svc.GetRun(ctx, value.ID)
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

func invalidRestoredReference(message string) error {
	err := model.NewAgentError("BAD_REQUEST", "create_run", nil)
	err.SafeMessage = message
	return err
}

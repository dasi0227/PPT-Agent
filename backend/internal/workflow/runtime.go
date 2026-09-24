package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextcompact"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/idempotency"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type EventEmitter interface {
	Emit(model.EventType, any)
}

type Prompter interface {
	Ask(context.Context, model.QuestionAskedPayload) (model.QuestionAnswer, string, error)
}

type PlanApprovalPrompter interface {
	AskPlanApproval(context.Context, model.PlanApprovalRequestedPayload) (model.PlanApprovalAnswer, error)
}

type PlanApprovalResumer interface {
	ResumeAfterPlanApproval(context.Context)
}

type CommandPermissionPrompter interface {
	AskCommandPermission(context.Context, model.CommandPermissionRequestedPayload) (model.CommandPermissionAnswer, error)
}

type ScopeExpansionPrompter interface {
	AskScopeExpansion(context.Context, model.ScopeExpansionRequestedPayload) (model.ScopeExpansionAnswer, error)
	ResumeAfterScopeExpansion(context.Context)
}

type ScopeExpansionResumer interface {
	ResumeScopeExpansion(context.Context, model.ScopeExpansionRequestedPayload) (model.ScopeExpansionAnswer, error)
}

type SteeringSource interface {
	DrainInputs(context.Context) ([]SteeringInput, error)
	MarkInputsInjected(context.Context, []string) error
}

type SteeringInput struct {
	ID             string
	Content        string
	ProjectID      string
	Attachments    []model.AttachmentReference
	DOMSelections  []model.DOMSelection
	ReferenceOrder []model.ReferenceOrderItem
	Scope          model.RunScope
}

type LifecycleObserver interface {
	PhaseChanged(RunPhase)
}

type IdempotencyStore interface {
	AcquireIdempotency(context.Context, model.IdempotencyRecord) (model.IdempotencyRecord, bool, error)
	CompleteIdempotency(context.Context, string, string, string, string, string) error
	GetIdempotency(context.Context, string, string, string) (model.IdempotencyRecord, error)
}

type ContextCompactor interface {
	Compact(context.Context, []llm.Message) (contextcompact.Result, error)
}

type TranscriptStore interface {
	Load(workDir, threadID string) ([]llm.Message, error)
	Replace(workDir, threadID string, messages []llm.Message) error
}

type TokenCalibration interface {
	Factor(threadID string) float64
	Observe(threadID string, rawEstimate, actualInput int) float64
}

type CheckpointSink interface {
	SaveCheckpoint(context.Context, RuntimeCheckpoint) error
}

type RuntimeCheckpoint struct {
	ModelRoute            *llm.RouteState               `json:"model_route,omitempty"`
	RunID                 string                        `json:"run_id"`
	LoopID                string                        `json:"loop_id"`
	Boundary              string                        `json:"boundary,omitempty"`
	Phase                 RunPhase                      `json:"phase"`
	Mode                  model.RunMode                 `json:"mode"`
	ResumePhase           RunPhase                      `json:"resume_phase,omitempty"`
	Plan                  *Plan                         `json:"plan,omitempty"`
	Requirements          *RequirementLedger            `json:"requirements,omitempty"`
	Work                  *WorkLedger                   `json:"work_ledger,omitempty"`
	Changes               ChangeSet                     `json:"changes"`
	Evidence              []Evidence                    `json:"evidence"`
	ContextIndexRef       string                        `json:"context_index_ref,omitempty"`
	ContextBriefing       string                        `json:"context_briefing,omitempty"`
	MessageSummary        []CheckpointMessage           `json:"message_summary,omitempty"`
	LatestToolResults     []CheckpointToolResult        `json:"latest_tool_results,omitempty"`
	ActiveSkills          []model.RunSkill              `json:"active_skills,omitempty"`
	ActiveComponents      []model.RunComponent          `json:"active_components,omitempty"`
	Turns                 int                           `json:"turns"`
	ToolCalls             int                           `json:"tool_calls"`
	ActiveDurationMS      int64                         `json:"active_duration_ms"`
	WaitingDurationMS     int64                         `json:"waiting_duration_ms"`
	WaitingQuestionID     string                        `json:"waiting_question_id,omitempty"`
	PendingCommand        *PendingCommandApproval       `json:"pending_command,omitempty"`
	PendingScopeExpansion *PendingScopeExpansion        `json:"pending_scope_expansion,omitempty"`
	Scope                 model.RunScope                `json:"scope"`
	DOMSelections         []model.DOMSelection          `json:"dom_selections,omitempty"`
	Session               *RunSessionSnapshot           `json:"session,omitempty"`
	ProviderContinuation  *ProviderContinuationSnapshot `json:"provider_continuation,omitempty"`
	CompletionFailures    int                           `json:"completion_failures"`
	CreatedAt             int64                         `json:"created_at"`
}

type PendingScopeExpansion struct {
	Request     model.ScopeExpansionRequestedPayload `json:"request"`
	ResumePhase RunPhase                             `json:"resume_phase"`
}

type PendingCommandApproval struct {
	InteractionID string         `json:"interaction_id"`
	CallID        string         `json:"call_id"`
	Command       string         `json:"command"`
	Args          map[string]any `json:"args"`
	CommandHash   string         `json:"command_hash"`
	ReasonCode    string         `json:"reason_code"`
	Reason        string         `json:"reason"`
	Mutates       bool           `json:"mutates"`
	TargetPaths   []string       `json:"target_paths"`
	PreimageHash  string         `json:"preimage_hash,omitempty"`
	ResumePhase   RunPhase       `json:"resume_phase"`
}

type AgentRequest struct {
	RunID                 string
	LoopID                string
	Phase                 RunPhase
	Mode                  model.RunMode
	Context               contextengine.ContextPack
	Plan                  *Plan
	Changes               ChangeSet
	Evidence              []Evidence
	Requirements          *RequirementLedger
	Work                  *WorkLedger
	ContextBriefing       string
	RenderedImages        []RenderedImageContext
	ActiveSkills          []model.RunSkill
	LoadedComponents      []model.RunComponent
	Messages              []llm.Message
	Tools                 []ToolSchema
	ImageResolver         llm.ImageRefResolver
	Continuation          *llm.ProviderContinuation
	OnProviderRetry       func(int)
	OnContinuationReset   func(string)
	InstructionInMessages bool
}

type AgentResponse struct {
	ToolCalls    []llm.ToolCall
	Text         string
	Continuation *llm.ProviderContinuation
	Usage        llm.Usage
}

type ReActAgent interface {
	Next(context.Context, AgentRequest) (AgentResponse, error)
}

type CognitiveAgent struct {
	Provider llm.Provider
}

func (a CognitiveAgent) Next(ctx context.Context, req AgentRequest) (AgentResponse, error) {
	if a.Provider == nil {
		return AgentResponse{}, errors.New("LLM provider is required for ReAct execution")
	}
	req = prepareAgentRequest(req)
	messages := providerMessages(req)
	tools := make([]llm.ToolSchema, 0, len(req.Tools))
	for _, schema := range req.Tools {
		tools = append(tools, llm.ToolSchema{Name: schema.Name, Description: schema.Description, Parameters: schema.Parameters})
	}
	response, err := a.Provider.Generate(ctx, llm.GenerateRequest{
		Messages: messages, Tools: tools, ImageResolver: req.ImageResolver, PauseOnFallback: true,
		Continuation: req.Continuation, OnRetry: req.OnProviderRetry, OnContinuationReset: req.OnContinuationReset,
	})
	if err != nil {
		return AgentResponse{}, err
	}
	return AgentResponse{
		ToolCalls: response.ToolCalls, Text: response.Text(),
		Continuation: response.Continuation, Usage: response.Usage,
	}, nil
}

// Retained for callers that need a readable full request projection. Runtime
// sends providerMessages, never prepends this dynamic view to history.
func compiledPromptForAgentRequest(req AgentRequest) (string, string) {
	prepared := prepareAgentRequest(req)
	var parts []string
	for _, message := range prepared.Messages {
		if message.Metadata != nil && message.Metadata.Origin == "runtime" {
			parts = append(parts, message.Text())
		}
	}
	return runtimeSystemPromptForRequest(prepared), strings.Join(parts, "\n")
}

func noToolCallGuidance(mode model.RunMode) string {
	if mode == model.ModePlan {
		return "Ordinary assistant text cannot submit a plan. Use create_plan for a new complete proposal, update_plan for a complete revision, or another currently disclosed control action when more work is required."
	}
	return "Ordinary assistant text is not a completion signal. If the task is complete, call finish(message=...) with the final response. If the task is not complete, call one of the currently disclosed tools to continue."
}

func approvedPlanExecutionGuidance() string {
	return "The user approved the plan. Approval is complete and the runtime is now in execute mode. Begin executing the approved plan immediately with the currently disclosed tools; do not repeat the proposal or say that approval is still pending."
}

type RuntimeInput struct {
	RunID                 string
	ProjectDir            string
	Context               contextengine.ContextPack
	Emitter               EventEmitter
	Prompter              Prompter
	Steering              SteeringSource
	Checkpoint            CheckpointSink
	CommitMetadata        CommitMetadata
	DomainTools           DomainToolProvider
	Budget                RuntimeBudget
	Trace                 TraceRecorder
	ImageResolver         llm.ImageRefResolver
	Lifecycle             LifecycleObserver
	Idempotency           IdempotencyStore
	ContextIndexStore     ContextIndexStore
	SemanticReviews       SemanticReviewer
	SemanticReviewStore   SemanticReviewStore
	ResumeCheckpoint      *RuntimeCheckpoint
	RefreshContext        func(context.Context, model.RunMode) (contextengine.ContextPack, error)
	PersistMode           func(context.Context, model.RunMode) error
	DomainToolsForContext func(contextengine.ContextPack) DomainToolProvider
	CommitPlanApproval    func(context.Context, model.RunMode, contextengine.ContextPack, RuntimeCheckpoint) error
	CommitScopeExpansion  func(context.Context, model.RunScope, RuntimeCheckpoint) error
	Logger                *zap.Logger
	Transcript            TranscriptStore
	Calibration           TokenCalibration
	RecordCompaction      func(context.Context, contextcompact.Result, contextengine.WindowSnapshot, contextengine.WindowSnapshot, time.Duration) (model.ContextCompaction, error)
}

type Runtime struct {
	Agent               ReActAgent
	Gate                CompletionGate
	Compactor           ContextCompactor
	Embedder            EmbeddingProvider
	SemanticPolicy      SemanticReviewPolicy
	ContextWindowTokens int
	now                 func() time.Time
}

func buildDomainToolRegistry(input RuntimeInput, pack contextengine.ContextPack) (*ToolRegistry, error) {
	registry := NewToolRegistry()
	provider := input.DomainTools
	if input.DomainToolsForContext != nil {
		provider = input.DomainToolsForContext(pack)
	}
	if provider == nil {
		provider = DefaultDomainToolProvider{Pack: pack}
	}
	if err := provider.RegisterDomainTools(registry); err != nil {
		return nil, err
	}
	return registry, nil
}

func NewRuntime(agent ReActAgent) *Runtime {
	runtime := &Runtime{
		Agent: agent, Gate: NewCompletionGate(),
		Embedder: HashEmbeddingProvider{}, SemanticPolicy: DefaultSemanticReviewPolicy(),
		now: time.Now,
	}
	if cognitive, ok := agent.(CognitiveAgent); ok && cognitive.Provider != nil {
		runtime.ContextWindowTokens = cognitive.Provider.Capabilities().ContextWindowTokens
	}
	return runtime
}

type RunState struct {
	runID                   string
	loopID                  string
	phase                   RunPhase
	mode                    model.RunMode
	pack                    contextengine.ContextPack
	resumePhase             RunPhase
	plan                    *Plan
	tx                      *RunSession
	scope                   model.RunScope
	ledger                  *EvidenceLedger
	issues                  []Issue
	messages                []llm.Message
	continuation            *llm.ProviderContinuation
	turns                   int
	toolCalls               int
	tokens                  int
	toolFailures            int
	activeTools             int
	activeElapsed           time.Duration
	activeSince             time.Time
	activeRunning           bool
	waitingElapsed          time.Duration
	waitingSince            time.Time
	budget                  RuntimeBudget
	gateKey                 string
	gateCount               int
	gateEvidence            int64
	lastSummary             string
	committed               bool
	committedChanges        ChangeSet
	lastProgress            string
	lastReasoning           string
	suggestedNextInputs     []string
	trace                   TraceRecorder
	lifecycle               LifecycleObserver
	requirements            *RequirementLedger
	work                    *WorkLedger
	contextIndex            ContextIndex
	contextIndexRef         string
	retrievedContext        []RetrievedContextItem
	lastRetrievalKey        string
	contextBriefing         string
	renderedImages          []RenderedImageContext
	latestToolResults       []CheckpointToolResult
	lastCheckpointTurn      int
	lastCheckpointToolCalls int
	lastCheckpointAt        time.Time
	tools                   *ToolRegistry
	pendingCommand          *PendingCommandApproval
	pendingScopeExpansion   *PendingScopeExpansion
	projectDir              string
	activeSkills            *ActiveSkillSet
	calibrationFactor       float64
	lastWindow              contextengine.WindowSnapshot
	nextCompactionTokens    int
}

func (r *Runtime) Run(ctx context.Context, input RuntimeInput) StructuredOutcome {
	if input.Budget.MaxTurns == 0 {
		input.Budget = DefaultRuntimeBudget(r.ContextWindowTokens)
	}
	now := r.clockNow()
	state := &RunState{
		projectDir: input.ProjectDir, runID: input.RunID, loopID: "loop_" + uuid.NewString(),
		scope: input.Context.Command.Scope, mode: input.Context.Command.Mode, pack: input.Context, ledger: NewEvidenceLedger(),
		issues: []Issue{}, messages: []llm.Message{}, activeSince: now, activeRunning: true, budget: input.Budget,
		trace: input.Trace, lifecycle: input.Lifecycle, requirements: NewRequirementLedger(input.Context.Command),
		work:              NewWorkLedger(),
		committedChanges:  EmptyChangeSet(),
		activeSkills:      &ActiveSkillSet{Skills: append([]model.RunSkill{}, input.Context.Command.Skills...)},
		calibrationFactor: 1,
	}
	if cognitive, ok := r.Agent.(CognitiveAgent); ok {
		if route, ok := cognitive.Provider.(*llm.RoutedProvider); ok {
			route.OnFallback = func(switchCtx context.Context, routing llm.RouteState) error {
				if err := r.checkBudget(switchCtx, state); err != nil {
					return err
				}
				state.continuation = nil
				r.ContextWindowTokens = route.Capabilities().ContextWindowTokens
				state.budget.ContextCompactionThreshold = DefaultRuntimeBudget(r.ContextWindowTokens).ContextCompactionThreshold
				state.nextCompactionTokens = 0
				state.calibrationFactor = 1
				if err := r.saveCheckpoint(switchCtx, input, state, checkpointBoundary("model_fallback"), ""); err != nil {
					return err
				}
				if input.Emitter != nil {
					input.Emitter.Emit(model.EventRunProgress, model.RunProgressPayload{
						PublicEventBase: publicBase(state.runID), Activity: model.ActivityModelFallback,
						ModelSwitch: &model.ModelSwitch{From: routing.Initial, To: routing.Active, Purpose: routing.Purpose},
					})
				}
				return nil
			}
			defer func() { route.OnFallback = nil }()
		}
	}
	if input.ResumeCheckpoint != nil {
		r.emitProgress(input.Emitter, state, model.ActivityRunRecovering)
	} else {
		r.emitProgress(input.Emitter, state, model.ActivityRunPreparing)
	}
	if input.Calibration != nil {
		state.calibrationFactor = input.Calibration.Factor(input.Context.Manifest.ThreadID)
	}
	if input.Transcript != nil && input.Context.Manifest.ThreadID != "" {
		messages, err := input.Transcript.Load(input.ProjectDir, input.Context.Manifest.ThreadID)
		if err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		state.messages = append(state.messages, llm.NormalizeHistory(messages)...)
		instruction := input.Context.Command.Instruction
		if !containsRunInstruction(state.messages, input.RunID) {
			state.messages = append(state.messages, llm.Message{Role: llm.RoleUser, Content: referenceMessageParts(
				instruction, input.Context.Project.ID, input.Context.Command.Attachments, input.Context.Command.DOMSelections, input.Context.Command.ReferenceOrder,
			), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "instruction", RunID: input.RunID}})
		}
	}
	defer func() {
		_ = r.persistTranscript(input, state)
	}()
	defer func() {
		if state.tx != nil && !state.committed {
			state.tx.Discard()
		}
	}()
	if input.ResumeCheckpoint != nil && input.ResumeCheckpoint.RunID == input.RunID {
		recordTrace(input.Trace, state.runID, "provider.continuation_reset", map[string]any{"reason": "checkpoint_resume"})
		state.pack.Command.DOMSelections = append([]model.DOMSelection{}, input.ResumeCheckpoint.DOMSelections...)
		state.loopID = input.ResumeCheckpoint.LoopID
		state.phase = input.ResumeCheckpoint.ResumePhase
		if state.phase == "" {
			state.phase = input.ResumeCheckpoint.Phase
		}
		state.resumePhase = input.ResumeCheckpoint.ResumePhase
		state.plan = input.ResumeCheckpoint.Plan
		state.mode = input.ResumeCheckpoint.Mode
		if state.mode == "" {
			state.mode = input.Context.Command.Mode
		}
		if input.ResumeCheckpoint.Requirements != nil {
			state.requirements = input.ResumeCheckpoint.Requirements
		}
		if input.ResumeCheckpoint.Work != nil {
			state.work = input.ResumeCheckpoint.Work
		}
		state.turns = input.ResumeCheckpoint.Turns
		state.toolCalls = input.ResumeCheckpoint.ToolCalls
		if input.ResumeCheckpoint.ActiveDurationMS > 0 {
			state.activeElapsed = time.Duration(input.ResumeCheckpoint.ActiveDurationMS) * time.Millisecond
		}
		if input.ResumeCheckpoint.WaitingDurationMS > 0 {
			state.waitingElapsed = time.Duration(input.ResumeCheckpoint.WaitingDurationMS) * time.Millisecond
		}
		state.gateCount = input.ResumeCheckpoint.CompletionFailures
		state.pendingCommand = input.ResumeCheckpoint.PendingCommand
		state.pendingScopeExpansion = input.ResumeCheckpoint.PendingScopeExpansion
		state.activeSkills.Components = append([]model.RunComponent{}, input.ResumeCheckpoint.ActiveComponents...)
		if input.ResumeCheckpoint.ActiveSkills != nil {
			state.activeSkills.Skills = append([]model.RunSkill{}, input.ResumeCheckpoint.ActiveSkills...)
		}
		state.contextIndexRef = input.ResumeCheckpoint.ContextIndexRef
		state.committedChanges = input.ResumeCheckpoint.Changes
		recordTrace(input.Trace, input.RunID, "checkpoint.loaded", map[string]any{
			"loop_id": state.loopID, "phase": state.phase, "boundary": input.ResumeCheckpoint.Boundary,
		})
	}
	if r.Agent == nil {
		return r.fail(input, state, CodeAgentFailed, errors.New("ReAct agent is required"))
	}
	initialPhase := PhaseChat
	switch state.mode {
	case model.ModeChat, model.ModeGrill:
		initialPhase = PhaseChat
	case model.ModePlan:
		initialPhase = PhasePlanning
	case model.ModeExecute:
		initialPhase = PhaseExecuting
	}
	if input.ResumeCheckpoint != nil && state.phase != "" && state.phase != PhaseTerminal {
		initialPhase = state.phase
	}
	manifest := state.pack.Manifest
	recordTrace(input.Trace, input.RunID, "context.assembled", map[string]any{
		"loop_id": state.loopID, "context_id": manifest.ContextID,
		"profile": manifest.Profile, "estimated_tokens": manifest.EstimatedTokens,
		"budget_tokens": manifest.BudgetTokens, "segments": len(manifest.Segments),
		"refs": len(manifest.Refs), "warnings": manifest.Warnings, "read_only": manifest.ReadOnly,
	})
	if err := r.initializeContextIndex(ctx, input, state); err != nil {
		return r.fail(input, state, CodeAgentFailed, err)
	}
	r.changePhase(input.Emitter, state, initialPhase, "run mode initialized")
	if err := r.saveCheckpoint(ctx, input, state, checkpointRuntimeInitialized, ""); err != nil {
		return r.fail(input, state, CodeAgentFailed, err)
	}
	if state.mode == model.ModeExecute {
		// RunSession is the transaction for the currently executing tool call.
		// Every successful mutation is committed before the next call starts.
		var session *RunSession
		var err error
		if input.ResumeCheckpoint != nil && input.ResumeCheckpoint.Session != nil && len(input.ResumeCheckpoint.Session.Artifacts) == 0 {
			session, err = RestoreRunSession(input.ProjectDir, input.RunID, *input.ResumeCheckpoint.Session)
		} else {
			session, err = NewRunSession(input.ProjectDir, input.RunID)
			if err == nil && input.ResumeCheckpoint != nil && input.ResumeCheckpoint.Session != nil && len(input.ResumeCheckpoint.Session.Artifacts) > 0 {
				recordTrace(input.Trace, input.RunID, "checkpoint.uncommitted_operation_discarded", map[string]any{
					"artifacts": len(input.ResumeCheckpoint.Session.Artifacts),
				})
			}
		}
		if err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		state.tx = session
	}

	registry, err := buildDomainToolRegistry(input, state.pack)
	if err != nil {
		return r.fail(input, state, CodeAgentFailed, err)
	}
	state.tools = registry
	if state.pendingCommand != nil {
		if outcome, terminal := r.resumePendingCommand(ctx, input, state); terminal {
			return outcome
		}
	}
	if state.pendingScopeExpansion != nil {
		if outcome, terminal := r.awaitScopeExpansion(ctx, input, state, state.pendingScopeExpansion.Request, nil, ""); terminal {
			return outcome
		}
	}
	if state.mode == model.ModePlan && state.phase == PhaseWaitingInput && state.plan != nil && state.plan.Status == PlanAwaitingApproval {
		if outcome, terminal := r.awaitPlanApproval(ctx, input, state); terminal {
			return outcome
		}
	}

	for {
		if err := r.checkBudget(ctx, state); err != nil {
			code := CodeBudgetExceeded
			if errors.Is(err, context.Canceled) {
				code = CodeCanceled
			}
			return r.fail(input, state, code, err)
		}
		if err := r.appendSteering(ctx, state, input.Steering); err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		if err := r.persistTranscript(input, state); err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		if err := r.retrieveTurnContext(ctx, input, state); err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		schemas := state.tools.Disclose(state.phase, state.mode, state.scope)
		schemas = append(schemas, controlSchemas(state.phase, state.mode, state.plan)...)
		sort.Slice(schemas, func(i, j int) bool { return schemas[i].Name < schemas[j].Name })
		images := latestRenderedImages(state.pack, input.ProjectDir, state.tx)
		if hashCheckpointValue(images) != hashCheckpointValue(state.renderedImages) {
			state.continuation = nil
		}
		state.renderedImages = images
		// A provider continuation may retain pixels or an outdated runtime index.
		// Render reads use a complete prompt and must not leak into subsequent turns.
		if llm.HasRenderImages(state.messages) {
			state.continuation = nil
		}
		r.measureContextWindow(input, state, schemas)
		if err := r.compactIfNeeded(ctx, input, state, schemas); err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		if r.ContextWindowTokens > 0 && state.tokens >= r.ContextWindowTokens {
			return r.fail(input, state, CodeAgentFailed, errors.New("上下文超过当前模型窗口，压缩后仍无法发送"))
		}
		if err := r.maybePeriodicCheckpoint(ctx, input, state); err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		if state.mode == model.ModeExecute && state.plan != nil && state.plan.ApprovedContentHash != "" &&
			(state.plan.Status == PlanActive || state.plan.Status == PlanCompleted) &&
			state.plan.ApprovedContentHash != state.plan.ContentHash() {
			return r.fail(input, state, CodeAgentFailed, errors.New("approved plan content hash mismatch"))
		}
		state.turns++
		if state.mode == model.ModePlan {
			r.emitProgress(input.Emitter, state, model.ActivityPlanPreparing)
		} else {
			r.emitProgress(input.Emitter, state, model.ActivityRunAnalyzing)
		}
		r.logProviderRequest(input, state, schemas)
		request := prepareAgentRequest(agentRequestForState(input, state, schemas))
		state.messages = request.Messages
		request.OnProviderRetry = func(attempt int) { r.emitProgress(input.Emitter, state, model.ActivityRunRetrying) }
		request.OnContinuationReset = func(reason string) {
			recordTrace(input.Trace, state.runID, "provider.continuation_reset", map[string]any{"reason": reason})
		}
		if err := r.persistTranscript(input, state); err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}

		response, err := r.Agent.Next(ctx, request)
		if errors.Is(err, llm.ErrFallbackActivated) {
			state.turns-- // The failed model call did not advance the tool loop.
			continue      // Re-measure and compact for the activated model before the next call.
		}
		if err != nil {
			return r.failAgentError(input, state, classifyProviderError(ctx, err))
		}
		if err := r.checkActiveDurationBudget(state); err != nil {
			return r.fail(input, state, CodeBudgetExceeded, err)
		}
		rememberSelectedComponents(state)
		state.continuation = response.Continuation
		if llm.HasRenderImages(state.messages) {
			state.messages = llm.WithoutRenderImages(state.messages)
			state.continuation = nil
			recordTrace(input.Trace, state.runID, "provider.continuation_reset", map[string]any{"reason": "render_images_released"})
		}
		if input.Calibration != nil && response.Usage.InputTokens > 0 {
			rawEstimate := state.lastWindow.Total
			if state.calibrationFactor > 0 {
				rawEstimate = int(float64(rawEstimate) / state.calibrationFactor)
			}
			state.calibrationFactor = input.Calibration.Observe(
				input.Context.Manifest.ThreadID, rawEstimate, response.Usage.InputTokens,
			)
		}
		if len(response.ToolCalls) == 0 {
			recordTrace(input.Trace, state.runID, "raw.model.response", map[string]any{
				"turn": state.turns, "has_tool_call": false,
			})
			state.messages = append(state.messages, llm.Message{
				Role: llm.RoleAssistant, Content: llm.TextContent(response.Text),
			})
			state.messages = append(state.messages, llm.Message{Role: llm.RoleUser, Content: llm.TextContent(noToolCallGuidance(state.mode)), Metadata: runtimeControlMetadata("guidance", state.runID)})
			continue
		}
		calls := append([]llm.ToolCall{}, response.ToolCalls...)
		for index := range calls {
			if calls[index].ID == "" {
				calls[index].ID = fmt.Sprintf("%s-%d-%d", state.loopID, state.turns, index+1)
			}
		}
		r.emitReasoning(input.Emitter, state, response.Text)
		recordTrace(input.Trace, state.runID, "raw.model.response", map[string]any{
			"turn": state.turns, "has_tool_call": true,
		})
		controlCount := 0
		for _, call := range calls {
			if isControlTool(call.Name) {
				controlCount++
			}
		}
		if controlCount > 0 {
			if len(calls) != 1 {
				result := failedToolResult(CodeInvalidControlCall, "control actions must be the only call in a model response", false)
				state.messages = appendBatchObservations(state.messages, calls, response.Text, []ToolResult{result})
				continue
			}
			call := calls[0]
			if !schemasByName(schemas)[call.Name] {
				r.appendControlObservation(
					state, call, response.Text,
					failedToolResult(ErrToolNotDisclosed.Error(), "runtime control was not disclosed in this turn", false),
				)
				continue
			}
			outcome, done := r.executeControl(ctx, input, state, call, response.Text)
			if done {
				return outcome
			}
			continue
		}
		results := r.executeToolBatch(ctx, input, state, state.tools, schemasByName(schemas), calls)
		state.messages = appendBatchObservations(state.messages, calls, response.Text, results)
		recordToolFailures(state, results)
		if hasRuntimePolicyFailure(results) {
			return r.fail(input, state, ErrCapabilityDenied.Error(), errors.New("a disclosed tool was denied by Runtime policy"))
		}
		if state.requirements != nil {
			state.requirements.ObserveToolResults(results)
		}
		if state.toolFailures >= state.budget.MaxConsecutiveToolFailures {
			return r.fail(input, state, CodeConsecutiveErrors, errors.New("consecutive tool failures exhausted the runtime budget"))
		}
	}
}

func (r *Runtime) initializeContextIndex(ctx context.Context, input RuntimeInput, state *RunState) error {
	if input.ResumeCheckpoint != nil &&
		input.ResumeCheckpoint.ContextIndexRef != "" &&
		input.ContextIndexStore != nil {
		existing, err := input.ContextIndexStore.GetContextIndex(ctx, input.ResumeCheckpoint.ContextIndexRef)
		if err == nil &&
			existing.RunID == state.runID &&
			existing.PackHash == state.pack.Manifest.PackHash {
			state.contextIndex = existing
			state.contextIndexRef = existing.ID
			recordTrace(input.Trace, state.runID, "context.index_reused", map[string]any{
				"loop_id": state.loopID, "context_index_ref": existing.ID,
			})
			return nil
		}
	}

	state.contextIndex = NewContextIndexFromPack(state.pack, state.scope, r.Embedder)
	state.contextIndexRef = state.contextIndex.ID
	if input.ContextIndexStore == nil {
		return nil
	}
	id, err := input.ContextIndexStore.SaveContextIndex(ctx, state.contextIndex)
	if err != nil {
		return err
	}
	if id != "" {
		state.contextIndexRef = id
	}
	return nil
}

// CAPABILITY_DENIED after a tool was disclosed is a Runtime configuration
// invariant failure. Giving it back to the model would only invite expensive
// retries: no model action can change the Runtime policy in this loop.
func hasRuntimePolicyFailure(results []ToolResult) bool {
	for _, result := range results {
		if result.Code == ErrCapabilityDenied.Error() {
			return true
		}
	}
	return false
}

func recordToolFailures(state *RunState, results []ToolResult) {
	anySuccess := false
	anyFailure := false
	for _, result := range results {
		if result.OK {
			anySuccess = true
		} else if result.Code != CodeDependencyFailed {
			anyFailure = true
			state.issues = append(state.issues, result.Issues...)
		}
	}
	switch {
	case anySuccess:
		state.toolFailures = 0
	case anyFailure:
		// One model response is one repair round. A multi-render response may
		// legitimately report several failed pages, while fail-fast may add
		// synthetic DEPENDENCY_FAILED observations. Neither should exhaust the
		// consecutive-round budget faster merely because ToolCalls[] is used.
		state.toolFailures++
	}
}

func (r *Runtime) executeToolBatch(
	ctx context.Context,
	input RuntimeInput,
	state *RunState,
	registry *ToolRegistry,
	disclosed map[string]bool,
	calls []llm.ToolCall,
) []ToolResult {
	if state.pack.Command.Scope.Source.Kind == "" {
		state.pack = input.Context
	}
	if state.mode == "" {
		state.mode = input.Context.Command.Mode
	}
	results := make([]ToolResult, len(calls))
	started := make([]bool, len(calls))
	replayed := make([]bool, len(calls))
	emitTerminal := make([]bool, len(calls))
	decisions := make([]*ToolDecision, len(calls))
	approvals := make([]string, len(calls))
	workTargets := make([][]string, len(calls))
	projector := ToolPublicProjector{ProjectDir: input.ProjectDir, TextContext: state.publicTextContext()}
	planStepID := currentPlanStepID(state.plan)
	var lifecycleMu sync.Mutex
	state.toolCalls += len(calls)
	state.activeTools = len(calls)
	confirmIndex := -1
	for index, call := range calls {
		desc, exists := registry.Descriptor(call.Name)
		if !exists || !disclosed[call.Name] {
			continue
		}
		decision := &ToolDecision{Outcome: "allow", Mutates: !desc.ReadOnly}
		if preflight, ok := desc.Tool.(PreflightTool); ok {
			value := preflight.Preflight(ctx, DomainToolInput{
				Args: call.Args, CallID: call.ID, Context: state.pack,
				ProjectDir: input.ProjectDir, RunID: input.RunID, Session: state.tx,
				Scope: state.scope, Phase: state.phase, Mode: state.mode, ActiveSkills: state.activeSkills, Messages: state.messages,
			})
			decision = &value
		}
		decisions[index] = decision
		if call.Name == "run_command" {
			approvals[index] = "not_required"
			recordTrace(input.Trace, state.runID, "command.policy_decided", map[string]any{
				"call_id": call.ID, "command_hash": decision.CommandHash,
				"command": decision.Command, "policy_version": commandexec.PolicyVersion,
				"outcome": decision.Outcome, "reason_code": decision.ReasonCode,
				"target_paths": append([]string(nil), decision.TargetPaths...),
				"mutates":      decision.Mutates,
			})
		}
		switch decision.Outcome {
		case "deny":
			results[index] = blockedCommandResult(*decision)
			emitTerminal[index] = true
		case "confirm":
			approvals[index] = "pending"
			if confirmIndex >= 0 {
				confirmIndex = -2
			} else {
				confirmIndex = index
			}
		}
	}
	if confirmIndex >= 0 && len(calls) != 1 {
		for index := range calls {
			results[index] = failedToolResult(CodeInvalidControlCall, "an approval-bound command must be the only tool call in the model response", true)
			emitTerminal[index] = true
			if decisions[index] != nil && calls[index].Name == "run_command" {
				results[index].Command = &CommandExecution{
					Text: decisions[index].Command, Status: "failed",
					Reason: "需要授权的命令必须单独提交。",
				}
			}
		}
		confirmIndex = -2
	}
	if confirmIndex == 0 && len(calls) == 1 {
		decision := decisions[0]
		prompter, ok := input.Prompter.(CommandPermissionPrompter)
		if !ok {
			results[0] = failedToolResult(CodeAgentFailed, "command permission prompter is required", false)
			emitTerminal[0] = true
		} else {
			call := calls[0]
			resumePhase := state.phase
			interactionID := "cmdperm_" + call.ID
			state.resumePhase = resumePhase
			state.pendingCommand = &PendingCommandApproval{
				InteractionID: interactionID, CallID: call.ID,
				Command: decision.Command, Args: call.Args, CommandHash: decision.CommandHash,
				ReasonCode: decision.ReasonCode, Reason: decision.PublicReason,
				Mutates: decision.Mutates, TargetPaths: append([]string(nil), decision.TargetPaths...),
				PreimageHash: decision.PreimageHash, ResumePhase: resumePhase,
			}
			r.changePhase(input.Emitter, state, PhaseWaitingInput, "command permission required")
			if err := r.saveCheckpoint(ctx, input, state, checkpointBeforeCommandPermission, ""); err != nil {
				results[0] = failedToolResult(CodeAgentFailed, err.Error(), false)
				emitTerminal[0] = true
			} else {
				state.pauseActiveClock(r.clockNow())
				answer, err := prompter.AskCommandPermission(ctx, model.CommandPermissionRequestedPayload{
					PublicEventBase: publicBase(state.runID), InteractionID: interactionID,
					CallID: call.ID, Command: decision.Command, CommandHash: decision.CommandHash,
					ReasonCode: decision.ReasonCode, Reason: decision.PublicReason,
				})
				if err != nil {
					approvals[0] = "not_answered"
					results[0] = failedToolResult(CodeCanceled, err.Error(), false)
					emitTerminal[0] = true
				} else {
					state.resumeActiveClock(r.clockNow())
					r.changePhase(input.Emitter, state, resumePhase, "command permission answered")
					state.pendingCommand = nil
					_ = r.saveCheckpoint(ctx, input, state, checkpointAfterCommandPermission, "")
					if answer.InteractionID != interactionID || answer.CallID != call.ID ||
						answer.CommandHash != decision.CommandHash ||
						(answer.Decision != "allow_once" && answer.Decision != "deny") {
						approvals[0] = "invalid"
						results[0] = failedToolResult(CodeInvalidControlCall, "command permission answer does not match the pending command", false)
						emitTerminal[0] = true
					} else if answer.Decision == "deny" {
						approvals[0] = "deny"
						results[0] = blockedCommandResult(*decision)
						results[0].Summary = "command permission denied"
						results[0].Command.Reason = "用户拒绝了本次命令执行。"
						emitTerminal[0] = true
					} else {
						approvals[0] = "allow_once"
					}
				}
			}
		}
	}
	aggregateToolProgress := batchIsIndependentReads(calls) ||
		batchIsAllowedCommands(calls, decisions, emitTerminal) ||
		batchIsIndependentRenders(calls)
	prepare := func(index int) bool {
		if emitTerminal[index] {
			return false
		}
		call := calls[index]
		replay, shouldExecute := r.acquireToolCall(ctx, input, state, call)
		if !shouldExecute {
			results[index] = replay
			replayed[index] = replay.Code != CodeCanceled
			return false
		}
		started[index] = true
		if desc, ok := registry.Descriptor(call.Name); ok && !desc.ReadOnly {
			workTargets[index] = slideIDsFromArgs(call.Args)
			state.work.MarkRunning(workTargets[index], state.scope)
		}
		lifecycleMu.Lock()
		if !aggregateToolProgress {
			r.emitToolProgress(input.Emitter, state, call)
		}
		if input.Emitter != nil {
			if event, ok := projector.Started(state.runID, call.ID, call.Name, call.Args, planStepID, decisions[index]); ok {
				input.Emitter.Emit(model.EventToolStarted, event)
			}
		}
		calledTrace := map[string]any{
			"loop_id": state.loopID, "call_id": call.ID, "tool": call.Name,
		}
		if call.Name == "run_command" && decisions[index] != nil {
			calledTrace["command_hash"] = decisions[index].CommandHash
			calledTrace["command"] = decisions[index].Command
		} else {
			calledTrace["args"] = call.Args
		}
		recordTrace(input.Trace, state.runID, "tool.called", calledTrace)
		lifecycleMu.Unlock()
		return true
	}
	runPrepared := func(index int) {
		call := calls[index]
		results[index] = registry.Execute(ctx, disclosed, call.Name, call.Args, DomainToolInput{
			Args: call.Args, CallID: call.ID, Context: state.pack, ProjectDir: input.ProjectDir, RunID: input.RunID,
			Session: state.tx, Scope: state.scope, Phase: state.phase,
			Mode: state.mode, Decision: decisions[index], ActiveSkills: state.activeSkills, Messages: state.messages,
		})
		if ctx.Err() != nil {
			results[index] = failedToolResult(CodeCanceled, "run canceled", false)
		}
	}
	commitPrepared := func(index int) {
		if !started[index] || state.tx == nil {
			return
		}
		call := calls[index]
		desc, exists := registry.Descriptor(call.Name)
		if !exists {
			return
		}
		mutates := !desc.ReadOnly
		if decisions[index] != nil {
			mutates = mutates || decisions[index].Mutates
		}
		proofs := materializationProofs(results[index])
		if !results[index].OK {
			if mutates {
				state.tx.RollbackOperation()
			}
			return
		}
		if !mutates && len(proofs) == 0 {
			return
		}
		err := stageMaterializationRecords(state.tx, proofs)
		resultJSON := ""
		if err == nil {
			resultJSON, err = marshalPersistedToolResult(results[index])
		}
		if err == nil {
			state.tx.AcceptMaterializationProofs(proofs)
			var committed ChangeSet
			committed, err = state.tx.CommitOperation(ctx, call.ID, resultJSON, input.CommitMetadata)
			if err == nil {
				state.committedChanges = mergeChangeSets(state.committedChanges, committed)
				refreshTargets := append([]ChangedTarget{}, results[index].ChangedTargets...)
				for _, proof := range proofs {
					refreshTargets = append(refreshTargets, ChangedTarget{Type: "slide", SlideID: proof.SlideID, Part: "html"})
				}
				refreshRuntimePack(input.ProjectDir, state, refreshTargets)
				if input.DomainToolsForContext != nil {
					if next, registryErr := buildDomainToolRegistry(input, state.pack); registryErr == nil {
						registry = next
						state.tools = next
					} else {
						recordTrace(input.Trace, state.runID, "tool_registry.refresh_failed", map[string]any{
							"call_id": call.ID, "tool": call.Name, "error": registryErr.Error(),
						})
					}
				}
				for _, change := range committed.All() {
					recordTrace(input.Trace, state.runID, "target.committed", map[string]any{
						"call_id": call.ID, "target": resourceForArtifact(change.Artifact), "artifact": change.Artifact,
					})
				}
			}
		}
		if err != nil {
			state.tx.RollbackOperation()
			results[index] = failedToolResult(CodeCommitFailed, err.Error(), false)
		}
	}
	execute := func(index int) {
		if prepare(index) {
			runPrepared(index)
			commitPrepared(index)
		}
	}
	if aggregateToolProgress {
		r.emitProgress(input.Emitter, state, toolBatchActivity(calls))
	}
	switch {
	case batchIsIndependentReads(calls):
		runConcurrentBatch(ctx, len(calls), 4, execute)
	case batchIsAllowedCommands(calls, decisions, emitTerminal):
		prepared := make([]bool, len(calls))
		for index := range calls {
			prepared[index] = prepare(index)
		}
		runConcurrentBatch(ctx, len(calls), 3, func(index int) {
			if prepared[index] {
				runPrepared(index)
				commitPrepared(index)
			}
		})
	case batchIsIndependentRenders(calls):
		prepared := make([]bool, len(calls))
		for index := range calls {
			prepared[index] = prepare(index)
		}
		runConcurrentBatch(ctx, len(calls), 3, func(index int) {
			if prepared[index] {
				runPrepared(index)
			}
		})
		for index := range calls {
			if prepared[index] {
				commitPrepared(index)
			}
		}
	default:
		writeFailed := false
		for index, call := range calls {
			desc, _ := registry.Descriptor(call.Name)
			if writeFailed {
				results[index] = failedToolResult(CodeDependencyFailed, "call skipped after an earlier write failure", false)
				continue
			}
			if ctx.Err() != nil {
				results[index] = failedToolResult(CodeCanceled, "run canceled before tool start", false)
				continue
			}
			execute(index)
			if !desc.ReadOnly && !results[index].OK {
				writeFailed = true
			}
		}
	}
	state.activeTools = 0
	for index, call := range calls {
		result := results[index]
		if result.Code == "" && !result.OK {
			result = failedToolResult(CodeCanceled, "run canceled before tool start", false)
		}
		results[index] = bindToolErrorObservation(result, call)
	}
	type workOutcome struct {
		ok      bool
		message string
	}
	workOutcomes := map[string]workOutcome{}
	for index, slideIDs := range workTargets {
		for _, slideID := range slideIDs {
			outcome, exists := workOutcomes[slideID]
			if !exists {
				outcome.ok = true
			}
			outcome.ok = outcome.ok && results[index].OK
			if !results[index].OK && outcome.message == "" {
				outcome.message = results[index].Summary
			}
			workOutcomes[slideID] = outcome
		}
	}
	for slideID, outcome := range workOutcomes {
		state.work.Complete([]string{slideID}, outcome.ok, outcome.message)
	}
	for index, call := range calls {
		result := results[index]
		if started[index] {
			r.persistToolCall(context.Background(), input, state, call, result)
		}
		if (started[index] || emitTerminal[index]) && input.Emitter != nil {
			if event, ok := projector.Completed(state.runID, call.ID, call.Name, call.Args, result); ok {
				input.Emitter.Emit(model.EventToolCompleted, event)
			}
		}
		if replayed[index] {
			recordTrace(input.Trace, state.runID, "tool.replayed", map[string]any{"call_id": call.ID, "tool": call.Name})
		}
		toolTrace := map[string]any{
			"loop_id": state.loopID, "call_id": call.ID, "tool": call.Name,
			"ok": result.OK, "summary": result.Summary,
			"issues": result.Issues, "changed_targets": result.ChangedTargets,
			"retryable": result.Retryable, "code": result.Code,
		}
		if call.Name != "run_command" {
			toolTrace["data"] = result.Data
		}
		recordTrace(input.Trace, state.runID, "tool.completed", toolTrace)
		if call.Name == "run_command" && decisions[index] != nil {
			recordCommandAudit(input.Trace, state.runID, call.ID, *decisions[index], approvals[index], result)
		}
		state.latestToolResults = append(state.latestToolResults, checkpointToolResult(call, result))
		if len(state.latestToolResults) > 12 {
			state.latestToolResults = state.latestToolResults[len(state.latestToolResults)-12:]
		}
		for _, target := range uniqueTargets(append(result.InvalidatedTargets, changedResources(result.ChangedTargets)...)) {
			state.ledger.Invalidate(target)
		}
		for _, evidence := range result.Evidence {
			recorded := state.ledger.Record(evidence)
			recordTrace(input.Trace, state.runID, "evidence.recorded", map[string]any{"evidence": recorded})
		}
		for _, target := range result.ChangedTargets {
			recordTrace(input.Trace, state.runID, "target.written", map[string]any{
				"target": target.Target(), "hash": target.Hash,
				"fields": target.Fields, "tentative": false,
			})
		}
		if result.OK {
			if len(result.ChangedTargets) > 0 {
				_ = r.saveCheckpoint(context.Background(), input, state, checkpointAfterWrite, "")
			}
			if call.Name == "render_slide" {
				_ = r.saveCheckpoint(context.Background(), input, state, checkpointAfterRender, "")
			}
			if call.Name == "load_skill" {
				_ = r.saveCheckpoint(context.Background(), input, state, checkpointPeriodic, "")
			}
		}
	}
	return results
}

func slideIDsFromArgs(args map[string]any) []string {
	seen := map[string]bool{}
	var visit func(any)
	visit = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if key == "slide_id" {
					if id, ok := child.(string); ok && strings.HasPrefix(id, "sli_") {
						seen[id] = true
					}
				}
				visit(child)
			}
		case []any:
			for _, child := range typed {
				visit(child)
			}
		}
	}
	visit(args)
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

func bindToolErrorObservation(result ToolResult, call llm.ToolCall) ToolResult {
	if result.OK {
		return result
	}
	agentErr := model.NewAgentError(result.Code, "tool_call", errors.New(result.Summary))
	agentErr.CallID = call.ID
	target, targetErr := parseResource(call.Args)
	hasTarget := targetErr == nil
	if call.Name == "mutate_ppt" {
		op := stringValue(call.Args["op"])
		if strings.HasPrefix(op, "slide.") {
			part := "spec"
			if strings.HasPrefix(op, "slide.html.") {
				part = "html"
			}
			target, hasTarget = Resource{Type: "slide", SlideID: stringValue(call.Args["slide_id"]), Part: part}, true
		} else {
			part := strings.Split(op, ".")[0]
			if part == "manifest" || part == "outline" || part == "design" {
				target, hasTarget = Resource{Type: "deck", Part: part}, true
			}
		}
	}
	if call.Name == "render_slide" {
		if slideID := stringValue(call.Args["slide_id"]); slideID != "" {
			target, hasTarget = Resource{Type: "slide", SlideID: slideID, Part: "html"}, true
		}
	}
	if hasTarget {
		agentErr.Resource = &model.ErrorResource{Type: target.Type, SlideID: target.SlideID, Part: target.Part}
	}
	agentErr.Details["reason"] = result.Summary
	agentErr.Details["next_action"] = agentErr.ModelMessage
	if hasTarget && target.Part == "html" && len(result.Issues) > 0 {
		checks := make([]string, 0, len(result.Issues))
		for _, issue := range result.Issues {
			if issue.Action != "" {
				checks = append(checks, issue.Action)
			} else if issue.Summary != "" {
				checks = append(checks, issue.Summary)
			}
		}
		agentErr.Details["html_checks"] = checks
	}
	if call.Name == "render_slide" {
		for _, key := range []string{"image_path", "hash", "content_size", "overflow", "clipping", "runtime_decorations", "console_errors", "failed_resources", "font_status"} {
			if value, exists := result.Data[key]; exists {
				agentErr.Details[key] = value
			}
		}
	}
	raw, _ := json.Marshal(agentErr.ModelObservation())
	result.Observation = string(raw)
	result.ObservationParts = nil
	return result
}

type persistedToolResult struct {
	Result                ToolResult             `json:"result"`
	Evidence              []Evidence             `json:"evidence"`
	MaterializationProofs []MaterializationProof `json:"materialization_proofs,omitempty"`
	InvalidatedTargets    []Resource             `json:"invalidated_targets"`
	ObservationParts      []llm.ContentPart      `json:"observation_parts"`
	ObservationMetadata   *llm.MessageMetadata   `json:"observation_metadata,omitempty"`
	Observation           string                 `json:"observation,omitempty"`
}

func marshalPersistedToolResult(result ToolResult) (string, error) {
	raw, err := json.Marshal(persistedToolResult{
		Result: result, Evidence: result.Evidence, MaterializationProofs: materializationProofs(result),
		InvalidatedTargets: result.InvalidatedTargets, ObservationParts: result.ObservationParts, ObservationMetadata: result.ObservationMetadata, Observation: result.Observation,
	})
	return string(raw), err
}

func materializationProofs(result ToolResult) []MaterializationProof {
	proofs := make([]MaterializationProof, 0)
	seen := map[string]bool{}
	for _, evidence := range result.Evidence {
		if evidence.Materialization == nil || seen[evidence.Materialization.SlideID] {
			continue
		}
		seen[evidence.Materialization.SlideID] = true
		proofs = append(proofs, *evidence.Materialization)
	}
	return proofs
}

func stageMaterializationRecords(session *RunSession, proofs []MaterializationProof) error {
	for _, proof := range proofs {
		if !stableSlideID.MatchString(proof.SlideID) {
			return errors.New("materialization proof has an invalid slide_id")
		}
		record := spec.MaterializationRecord{
			SchemaVersion: spec.SchemaVersion,
			Artifact: spec.MaterializationArtifact{
				Hash: "sha256:" + proof.ArtifactHash,
			},
			Source: spec.MaterializationSource{
				ManifestHash:      proof.ManifestHash,
				OutlineNodeHash:   proof.OutlineNodeHash,
				SpecHash:          proof.SpecHash,
				DesignContentHash: proof.DesignContentHash,
				Hash:              proof.SourceHash,
			},
			Frame:      spec.MaterializationFrame{ContextHash: proof.FrameContextHash},
			RenderedAt: time.Now().Unix(),
		}
		if err := spec.ValidateMaterialization(record); err != nil {
			return err
		}
		raw, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return err
		}
		path := model.SlideMaterializationPath(proof.SlideID)
		if _, err := session.Write(ArtifactRef{
			Kind: ArtifactDerived, ID: proof.SlideID + ":materialization", Path: path,
		}, "render_slide", raw); err != nil {
			return err
		}
	}
	return nil
}

func refreshRuntimePack(projectDir string, state *RunState, targets []ChangedTarget) {
	touched := map[string]bool{}
	all := false
	for _, target := range targets {
		if target.Type == "deck" {
			all = true
			switch target.Part {
			case "manifest":
				var value spec.Manifest
				if raw, err := os.ReadFile(filepath.Join(projectDir, "manifest.json")); err == nil && json.Unmarshal(raw, &value) == nil {
					state.pack.PresentationManifest.Manifest = value
				}
			case "outline":
				var value spec.Outline
				if raw, err := os.ReadFile(filepath.Join(projectDir, "outline.json")); err == nil && json.Unmarshal(raw, &value) == nil {
					state.pack.Outline.Outline = value
				}
			case "design":
				var value spec.Design
				if raw, err := os.ReadFile(filepath.Join(projectDir, "design.json")); err == nil && json.Unmarshal(raw, &value) == nil {
					state.pack.Design.Design = &value
				}
			}
		} else if target.SlideID != "" {
			touched[target.SlideID] = true
		}
	}
	contextengine.RefreshPageContext(&state.pack, projectDir, touched, all)
	if state.scope.IncludeRunCreatedSlides {
		state.scope.SlideIDs = runtimeSlideOrderIDs(state.pack)
		state.pack.Command.Scope = state.scope
	}
}

func runtimeSlideOrderIDs(pack contextengine.ContextPack) []string {
	_, ordered := runtimeSlideOrder(pack)
	return ordered
}

func (r *Runtime) acquireToolCall(ctx context.Context, input RuntimeInput, state *RunState, call llm.ToolCall) (ToolResult, bool) {
	if input.Idempotency == nil {
		return ToolResult{}, true
	}
	requestHash, err := idempotency.CanonicalHash(map[string]any{
		"tool": call.Name, "args": call.Args, "scope": state.scope,
	})
	if err != nil {
		return failedToolResult("INTERNAL", "cannot hash tool request", false), false
	}
	record, created, err := input.Idempotency.AcquireIdempotency(ctx, model.IdempotencyRecord{
		Scope: "tool_call", OwnerID: state.runID, Key: call.ID,
		RequestHash: requestHash, Status: "in_progress",
	})
	if err != nil {
		return failedToolResult("INTERNAL", "cannot acquire tool call", false), false
	}
	if record.RequestHash != requestHash {
		return failedToolResult("IDEMPOTENCY_KEY_REUSED", "call_id was reused with different tool arguments", false), false
	}
	if created {
		return ToolResult{}, true
	}
	for record.Status == "in_progress" {
		select {
		case <-ctx.Done():
			return failedToolResult(CodeCanceled, "run canceled before tool start", false), false
		case <-time.After(10 * time.Millisecond):
		}
		record, err = input.Idempotency.GetIdempotency(ctx, "tool_call", state.runID, call.ID)
		if err != nil {
			return failedToolResult("INTERNAL", "cannot replay tool call", false), false
		}
	}
	var persisted persistedToolResult
	if json.Unmarshal([]byte(record.ResultJSON), &persisted) != nil {
		return failedToolResult("INTERNAL", "stored tool result is invalid", false), false
	}
	persisted.Result.Evidence = persisted.Evidence
	for proofIndex := range persisted.MaterializationProofs {
		proof := persisted.MaterializationProofs[proofIndex]
		for evidenceIndex := range persisted.Result.Evidence {
			if persisted.Result.Evidence[evidenceIndex].Target.SlideID == proof.SlideID {
				persisted.Result.Evidence[evidenceIndex].Materialization = &proof
				break
			}
		}
	}
	persisted.Result.InvalidatedTargets = persisted.InvalidatedTargets
	persisted.Result.ObservationParts = persisted.ObservationParts
	persisted.Result.ObservationMetadata = persisted.ObservationMetadata
	persisted.Result.Observation = persisted.Observation
	return persisted.Result, false
}

func (r *Runtime) persistToolCall(ctx context.Context, input RuntimeInput, state *RunState, call llm.ToolCall, result ToolResult) {
	if input.Idempotency == nil {
		return
	}
	raw, err := marshalPersistedToolResult(result)
	if err == nil {
		_ = input.Idempotency.CompleteIdempotency(ctx, "tool_call", state.runID, call.ID, "completed", raw)
	}
}

func batchIsIndependentReads(calls []llm.ToolCall) bool {
	if len(calls) < 2 {
		return false
	}
	for _, call := range calls {
		if call.Name != "read_ppt" {
			return false
		}
	}
	return true
}

func batchIsAllowedCommands(calls []llm.ToolCall, decisions []*ToolDecision, terminal []bool) bool {
	if len(calls) < 2 {
		return false
	}
	for index, call := range calls {
		if call.Name != "run_command" || index >= len(decisions) || decisions[index] == nil ||
			decisions[index].Outcome != "allow" || decisions[index].Mutates || terminal[index] {
			return false
		}
	}
	return true
}

func blockedCommandResult(decision ToolDecision) ToolResult {
	result := failedToolResult(decision.ReasonCode, decision.PublicReason, false)
	result.Command = &CommandExecution{
		Text: decision.Command, Status: "blocked", Reason: decision.PublicReason,
	}
	return result
}

func recordCommandAudit(
	recorder TraceRecorder,
	runID string,
	callID string,
	decision ToolDecision,
	approval string,
	result ToolResult,
) {
	exitCode := -1
	durationMS := int64(0)
	truncated := false
	afterHash := ""
	if result.Command != nil {
		exitCode = result.Command.ExitCode
		durationMS = result.Command.DurationMS
		truncated = result.Command.OutputTruncated
	}
	if len(result.ChangedTargets) > 0 {
		afterHash = result.ChangedTargets[0].Hash
	}
	recordTrace(recorder, runID, "command.audit", map[string]any{
		"call_id": callID, "command_hash": decision.CommandHash,
		"command": decision.Command, "policy_version": commandexec.PolicyVersion,
		"outcome": decision.Outcome, "reason_code": decision.ReasonCode,
		"target_paths": append([]string(nil), decision.TargetPaths...),
		"approval":     approval, "duration_ms": durationMS, "exit_code": exitCode,
		"timed_out": result.Code == commandexec.CodeTimeout, "truncated": truncated,
		"before_hash": decision.PreimageHash, "after_hash": afterHash,
	})
}

func batchIsIndependentRenders(calls []llm.ToolCall) bool {
	if len(calls) < 2 {
		return false
	}
	seen := map[string]bool{}
	for _, call := range calls {
		if call.Name != "render_slide" {
			return false
		}
		slideID, _ := call.Args["slide_id"].(string)
		if slideID == "" || seen[slideID] {
			return false
		}
		seen[slideID] = true
	}
	return true
}

func runConcurrentBatch(ctx context.Context, count, limit int, execute func(int)) {
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for index := 0; index < count; index++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				execute(i)
			case <-ctx.Done():
			}
		}(index)
	}
	wg.Wait()
}

func (r *Runtime) prepareAndCommitPlanApproval(
	ctx context.Context,
	input RuntimeInput,
	state *RunState,
) (*RunState, error) {
	if state == nil || state.plan == nil || state.mode != model.ModePlan || state.phase != PhaseWaitingInput || state.plan.Status != PlanAwaitingApproval {
		return nil, errors.New("plan approval transition requires an awaiting plan run")
	}
	candidate := *state
	candidatePlan := *state.plan
	candidatePlan.Status = PlanActive
	candidatePlan.ApprovedContentHash = candidatePlan.ContentHash()
	candidatePlan.UpdatedAt = time.Now().Unix()
	candidate.plan = &candidatePlan
	candidate.mode = model.ModeExecute
	candidate.phase = PhaseExecuting
	if candidate.tx == nil {
		session, err := NewRunSession(input.ProjectDir, input.RunID)
		if err != nil {
			return nil, err
		}
		candidate.tx = session
	}
	if input.RefreshContext != nil {
		refreshed, err := input.RefreshContext(ctx, model.ModeExecute)
		if err != nil {
			return nil, err
		}
		candidate.pack = refreshed
	} else {
		candidate.pack.Command.Mode = model.ModeExecute
		candidate.pack.Manifest.ReadOnly = false
	}
	if candidate.pack.Command.Mode != model.ModeExecute || candidate.pack.Manifest.ReadOnly {
		return nil, errors.New("refreshed execute context is not write-enabled")
	}
	candidate.contextIndex = NewContextIndexFromPack(candidate.pack, candidate.scope, r.Embedder)
	candidate.contextIndexRef = candidate.contextIndex.ID
	candidate.retrievedContext = nil
	candidate.lastRetrievalKey = ""
	candidate.contextBriefing = BuildContextBriefing(candidate.pack, &candidate)
	tools, err := buildDomainToolRegistry(input, candidate.pack)
	if err != nil {
		return nil, err
	}
	candidate.tools = tools
	if input.ContextIndexStore != nil {
		id, err := input.ContextIndexStore.SaveContextIndex(ctx, candidate.contextIndex)
		if err != nil {
			return nil, err
		}
		if id != "" {
			candidate.contextIndexRef = id
		}
	}
	checkpoint := r.checkpointForBoundary(&candidate, checkpointPlanUpdated, "")
	if input.CommitPlanApproval != nil {
		if err := input.CommitPlanApproval(ctx, model.ModeExecute, candidate.pack, checkpoint); err != nil {
			return nil, err
		}
		return &candidate, nil
	}
	if input.PersistMode != nil {
		if err := input.PersistMode(ctx, model.ModeExecute); err != nil {
			return nil, err
		}
	}
	if input.Checkpoint != nil {
		if err := input.Checkpoint.SaveCheckpoint(ctx, checkpoint); err != nil {
			if input.PersistMode != nil {
				_ = input.PersistMode(ctx, model.ModePlan)
			}
			return nil, err
		}
	}
	return &candidate, nil
}

func (r *Runtime) awaitPlanApproval(
	ctx context.Context,
	input RuntimeInput,
	state *RunState,
) (StructuredOutcome, bool) {
	if state == nil || state.plan == nil || state.plan.Status != PlanAwaitingApproval {
		return r.fail(input, state, CodeInvalidControlCall, errors.New("awaiting plan is required")), true
	}
	prompter, ok := input.Prompter.(PlanApprovalPrompter)
	if !ok {
		return r.fail(input, state, CodeAgentFailed, errors.New("plan approval prompter is required")), true
	}
	plan := *state.plan
	interactionID := plan.ApprovalID
	if interactionID == "" {
		return r.fail(input, state, CodeAgentFailed, errors.New("plan approval identity is missing")), true
	}
	request := model.PlanApprovalRequestedPayload{PublicEventBase: publicBase(state.runID), InteractionID: interactionID, Plan: publicPlan(plan, state.publicTextContext())}
	for {
		state.pauseActiveClock(r.clockNow())
		answer, err := prompter.AskPlanApproval(ctx, request)
		if err != nil {
			return r.fail(input, state, CodeCanceled, err), true
		}
		state.resumeActiveClock(r.clockNow())
		if answer.InteractionID != interactionID || answer.PlanID != plan.ID ||
			(answer.Decision != "approve" && answer.Decision != "revise" && answer.Decision != "cancel") ||
			(answer.Decision == "revise" && strings.TrimSpace(answer.Feedback) == "") {
			return r.fail(input, state, CodeInvalidControlCall, errors.New("invalid plan approval answer")), true
		}
		switch answer.Decision {
		case "cancel":
			state.plan.Status = PlanCanceled
			state.plan.UpdatedAt = time.Now().Unix()
			if input.Emitter != nil {
				input.Emitter.Emit(model.EventPlanApprovalAnswered, model.PlanApprovalAnsweredPayload{PublicEventBase: publicBase(state.runID), InteractionID: interactionID, PlanID: plan.ID, Decision: answer.Decision, Feedback: answer.Feedback})
				input.Emitter.Emit(model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: publicBase(state.runID), Plan: publicPlan(*state.plan, state.publicTextContext())})
			}
			r.changePhase(input.Emitter, state, PhaseTerminal, "plan canceled")
			_ = r.saveCheckpoint(ctx, input, state, checkpointTerminal, "")
			return r.outcome(state, StatusCanceled, CodeCanceled, "plan canceled"), true
		case "revise":
			if input.Emitter != nil {
				input.Emitter.Emit(model.EventPlanApprovalAnswered, model.PlanApprovalAnsweredPayload{PublicEventBase: publicBase(state.runID), InteractionID: interactionID, PlanID: plan.ID, Decision: answer.Decision, Feedback: answer.Feedback})
			}
			r.changePhase(input.Emitter, state, PhasePlanning, "plan revision requested")
			if resumer, ok := input.Prompter.(PlanApprovalResumer); ok {
				resumer.ResumeAfterPlanApproval(ctx)
			}
			state.messages = append(state.messages, llm.Message{Role: llm.RoleUser, Content: llm.TextContent("Plan revision feedback: " + answer.Feedback), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "feedback", RunID: state.runID}})
			return StructuredOutcome{}, false
		case "approve":
			candidate, transitionErr := r.prepareAndCommitPlanApproval(ctx, input, state)
			if transitionErr != nil {
				recordTrace(state.trace, state.runID, "plan.approval_transition_failed", map[string]any{
					"loop_id": state.loopID, "plan_id": plan.ID, "error": transitionErr.Error(),
				})
				continue
			}
			previous := state.mode
			*state = *candidate
			// create_plan leaves a tool result saying that approval is pending as
			// the final conversation message. Without an explicit transition
			// observation, the first execute turn can follow that stale tail and
			// repeat the proposal even though runtime_state already says active.
			state.messages = append(state.messages, llm.Message{
				Role: llm.RoleUser, Content: llm.TextContent(approvedPlanExecutionGuidance()), Metadata: runtimeControlMetadata("approval", state.runID),
			})
			if resumer, ok := input.Prompter.(PlanApprovalResumer); ok {
				resumer.ResumeAfterPlanApproval(ctx)
			}
			if state.lifecycle != nil {
				state.lifecycle.PhaseChanged(PhaseExecuting)
			}
			recordTrace(state.trace, state.runID, "phase.changed", map[string]any{
				"loop_id": state.loopID, "from": PhaseWaitingInput, "phase": PhaseExecuting, "reason": "plan approved; execute mode enabled",
			})
			if input.Emitter != nil {
				input.Emitter.Emit(model.EventPlanApprovalAnswered, model.PlanApprovalAnsweredPayload{PublicEventBase: publicBase(state.runID), InteractionID: interactionID, PlanID: plan.ID, Decision: answer.Decision, Feedback: answer.Feedback})
				input.Emitter.Emit(model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: publicBase(state.runID), Plan: publicPlan(*state.plan, state.publicTextContext())})
				input.Emitter.Emit(model.EventRunModeChanged, model.RunModeChangedPayload{PublicEventBase: publicBase(state.runID), PreviousMode: previous, Mode: state.mode})
			}
			return StructuredOutcome{}, false
		}
	}
}

func (r *Runtime) resumePendingCommand(
	ctx context.Context,
	input RuntimeInput,
	state *RunState,
) (StructuredOutcome, bool) {
	pending := state.pendingCommand
	if pending == nil {
		return StructuredOutcome{}, false
	}
	desc, ok := state.tools.Descriptor("run_command")
	if !ok {
		return r.fail(input, state, CodeAgentFailed, errors.New("run_command is unavailable during recovery")), true
	}
	preflight, ok := desc.Tool.(PreflightTool)
	if !ok {
		return r.fail(input, state, CodeAgentFailed, errors.New("run_command preflight is unavailable during recovery")), true
	}
	resumePhase := pending.ResumePhase
	if resumePhase == "" || resumePhase == PhaseWaitingInput {
		resumePhase = PhaseExecuting
	}
	state.phase = resumePhase
	decision := preflight.Preflight(ctx, DomainToolInput{
		Args: pending.Args, CallID: pending.CallID, Context: state.pack,
		ProjectDir: input.ProjectDir, RunID: input.RunID, Session: state.tx,
		Scope: state.scope, Phase: resumePhase, Mode: state.mode, ActiveSkills: state.activeSkills, Messages: state.messages,
	})
	if decision.CommandHash != pending.CommandHash || decision.PreimageHash != pending.PreimageHash ||
		decision.Outcome != "confirm" {
		return r.fail(input, state, commandexec.CodeInvariantViolation, errors.New("pending command no longer matches its approved preflight")), true
	}
	call := llm.ToolCall{ID: pending.CallID, Name: "run_command", Args: pending.Args}
	schemas := state.tools.Disclose(state.phase, state.mode, state.scope)
	results := r.executeToolBatch(ctx, input, state, state.tools, schemasByName(schemas), []llm.ToolCall{call})
	state.messages = appendBatchObservations(state.messages, []llm.ToolCall{call}, "", results)
	recordToolFailures(state, results)
	return StructuredOutcome{}, false
}

func (r *Runtime) executeControl(
	ctx context.Context,
	input RuntimeInput,
	state *RunState,
	call llm.ToolCall,
	assistantText string,
) (StructuredOutcome, bool) {
	switch call.Name {
	case "create_plan":
		if state.mode != model.ModePlan || state.phase != PhasePlanning || state.plan != nil {
			r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, "create_plan is only allowed for a new plan proposal", false))
			return StructuredOutcome{}, false
		}
		raw, _ := json.Marshal(call.Args)
		var update PlanUpdate
		if err := json.Unmarshal(raw, &update); err != nil {
			r.appendControlObservation(state, call, assistantText, failedToolResult(ErrPlanInvalid.Error(), err.Error(), true))
			return StructuredOutcome{}, false
		}
		next, _, err := ApplyPlanUpdate(nil, update, state.runID, time.Now())
		if err != nil {
			r.appendControlObservation(state, call, assistantText, failedToolResult(ErrPlanInvalid.Error(), err.Error(), true))
			return StructuredOutcome{}, false
		}
		state.plan = &next
		if err := state.work.SyncPlan(state.plan, state.scope); err != nil {
			state.plan = nil
			r.appendControlObservation(state, call, assistantText, failedToolResult(ErrPlanInvalid.Error(), err.Error(), true))
			return StructuredOutcome{}, false
		}
		if input.Emitter != nil {
			input.Emitter.Emit(model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: publicBase(state.runID), Plan: publicPlan(next, state.publicTextContext())})
		}
		r.appendControlObservation(state, call, assistantText, SuccessfulToolResult("plan proposal created; waiting for approval"))
		r.changePhase(input.Emitter, state, PhaseWaitingInput, "plan approval required")
		if err := r.saveCheckpoint(ctx, input, state, checkpointPlanUpdated, next.ApprovalID); err != nil {
			return r.fail(input, state, CodeAgentFailed, err), true
		}
		return r.awaitPlanApproval(ctx, input, state)
	case "update_plan":
		if (state.mode != model.ModePlan || state.phase != PhasePlanning) && (state.mode != model.ModeExecute || state.phase != PhaseExecuting) {
			r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, "update_plan is not allowed now", false))
			return StructuredOutcome{}, false
		}
		raw, _ := json.Marshal(call.Args)
		if state.mode == model.ModeExecute && state.plan != nil {
			var progress PlanProgressUpdate
			if err := json.Unmarshal(raw, &progress); err != nil {
				r.appendControlObservation(state, call, assistantText, failedToolResult(ErrPlanInvalid.Error(), err.Error(), true))
				return StructuredOutcome{}, false
			}
			next, err := ApplyPlanProgress(state.plan, progress, time.Now())
			if err != nil {
				r.appendControlObservation(state, call, assistantText, failedToolResult(ErrPlanInvalid.Error(), err.Error(), true))
				return StructuredOutcome{}, false
			}
			previous := state.plan
			state.plan = &next
			if err := state.work.SyncPlan(state.plan, state.scope); err != nil {
				state.plan = previous
				r.appendControlObservation(state, call, assistantText, failedToolResult(ErrPlanInvalid.Error(), err.Error(), true))
				return StructuredOutcome{}, false
			}
			if input.Emitter != nil {
				input.Emitter.Emit(model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: publicBase(state.runID), Plan: publicPlan(next, state.publicTextContext())})
				completed := completedPlanSteps(previous, next)
				if len(completed) > 0 {
					input.Emitter.Emit(model.EventMessageMilestone, model.MessageMilestonePayload{PublicEventBase: publicBase(state.runID), MessageID: newMessageID(), Text: milestoneText(next.Title, completed, state.publicTextContext()), CompletedStepIDs: planStepIDs(completed)})
				}
			}
			r.appendControlObservation(state, call, assistantText, SuccessfulToolResult("plan progress accepted"))
			_ = r.saveCheckpoint(ctx, input, state, checkpointPlanUpdated, "")
			return StructuredOutcome{}, false
		}
		var update PlanUpdate
		if err := json.Unmarshal(raw, &update); err != nil {
			r.appendControlObservation(state, call, assistantText, failedToolResult(ErrPlanInvalid.Error(), err.Error(), true))
			return StructuredOutcome{}, false
		}
		next, _, err := ApplyPlanUpdate(state.plan, update, state.runID, time.Now())
		if err != nil {
			r.appendControlObservation(state, call, assistantText, failedToolResult(ErrPlanInvalid.Error(), err.Error(), true))
			return StructuredOutcome{}, false
		}
		previous := state.plan
		if state.mode == model.ModeExecute && previous == nil {
			next.Status = PlanActive
		}
		state.plan = &next
		if err := state.work.SyncPlan(state.plan, state.scope); err != nil {
			state.plan = previous
			r.appendControlObservation(state, call, assistantText, failedToolResult(ErrPlanInvalid.Error(), err.Error(), true))
			return StructuredOutcome{}, false
		}
		if input.Emitter != nil {
			input.Emitter.Emit(model.EventPlanUpdated, model.PlanUpdatedPayload{
				PublicEventBase: publicBase(state.runID), Plan: publicPlan(next, state.publicTextContext()),
			})
			completed := completedPlanSteps(previous, next)
			if len(completed) > 0 {
				ids := make([]string, 0, len(completed))
				for _, step := range completed {
					ids = append(ids, step.ID)
				}
				input.Emitter.Emit(model.EventMessageMilestone, model.MessageMilestonePayload{
					PublicEventBase: publicBase(state.runID), MessageID: newMessageID(),
					Text: milestoneText(next.Title, completed, state.publicTextContext()), CompletedStepIDs: ids,
				})
			}
		}
		r.appendControlObservation(state, call, assistantText, SuccessfulToolResult("plan revision accepted"))
		if err := r.saveCheckpoint(ctx, input, state, checkpointPlanUpdated, ""); err != nil {
			return r.fail(input, state, CodeAgentFailed, err), true
		}
		return StructuredOutcome{}, false
	case "ask_user":
		if input.Prompter == nil {
			r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, "ask_user requires an interactive prompter", false))
			return StructuredOutcome{}, false
		}
		questions, _ := call.Args["questions"].([]any)
		if len(questions) == 0 {
			r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, "questions are required", true))
			return StructuredOutcome{}, false
		}
		questionID := call.ID
		questionEvent := publicQuestion(state.runID, questionID, call.Args, state.publicTextContext())
		state.resumePhase = state.phase
		r.changePhase(input.Emitter, state, PhaseWaitingInput, "agent requested required user input")
		if err := r.saveCheckpoint(ctx, input, state, checkpointBeforeAskUser, questionID); err != nil {
			return r.fail(input, state, CodeAgentFailed, err), true
		}
		state.pauseActiveClock(r.clockNow())
		answer, displayText, err := input.Prompter.Ask(ctx, questionEvent)
		if err != nil {
			code := CodeAgentFailed
			if errors.Is(err, context.Canceled) {
				code = CodeCanceled
			}
			return r.fail(input, state, code, err), true
		}
		state.resumeActiveClock(r.clockNow())
		r.changePhase(input.Emitter, state, state.resumePhase, "user input received")
		result := SuccessfulToolResult("user answered")
		result.Data = map[string]any{
			"answers": answer.Answers, "display_text": displayText,
		}
		r.appendControlObservation(state, call, assistantText, result)
		if err := r.saveCheckpoint(ctx, input, state, checkpointAfterUserAnswer, ""); err != nil {
			return r.fail(input, state, CodeAgentFailed, err), true
		}
		return StructuredOutcome{}, false
	case "request_privilege":
		if state.mode != model.ModeExecute || state.phase != PhaseExecuting {
			r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, "request_privilege is only available while executing", false))
			return StructuredOutcome{}, false
		}
		var request struct {
			AddSlideIDs []string `json:"add_slide_ids"`
			Reason      string   `json:"reason"`
		}
		raw, _ := json.Marshal(call.Args)
		if err := json.Unmarshal(raw, &request); err != nil || strings.TrimSpace(request.Reason) == "" || len(request.AddSlideIDs) == 0 {
			r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, "request_privilege requires a reason and at least one scope addition", true))
			return StructuredOutcome{}, false
		}
		proposed, addition, err := proposeScopeExpansion(state.scope, state.pack, request.AddSlideIDs)
		if err != nil {
			r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, err.Error(), true))
			return StructuredOutcome{}, false
		}
		if proposed.Equal(state.scope) {
			result := SuccessfulToolResult("requested privileges are already included in the active scope")
			result.Data = map[string]any{"scope": state.scope, "changed": false}
			r.appendControlObservation(state, call, assistantText, result)
			return StructuredOutcome{}, false
		}
		requestEvent := model.ScopeExpansionRequestedPayload{
			PublicEventBase: publicBase(state.runID), InteractionID: "scope_" + uuid.NewString(), CallID: call.ID,
			BaseRevision: state.scope.Revision, CurrentScope: state.scope, RequestedAddition: addition,
			ProposedScope: proposed, AffectedPageCount: len(proposed.SlideIDs), Reason: model.PublicText(request.Reason, state.publicTextContext()),
		}
		return r.awaitScopeExpansion(ctx, input, state, requestEvent, &call, assistantText)
	case "review_completion":
		mode := state.mode
		if mode != model.ModePlan && mode != model.ModeExecute {
			r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, "review_completion is not allowed now", false))
			return StructuredOutcome{}, false
		}
		candidate := stringValue(call.Args["candidate_message"])
		focus := stringValue(call.Args["focus"])
		result := r.runReviewCompletion(ctx, input, state, call.ID, candidate, focus)
		toolResult := SuccessfulToolResult("completion review completed")
		toolResult.Data = map[string]any{"checks": result.Checks}
		observation, _ := json.Marshal(toolResult.Data)
		toolResult.Observation = string(observation)
		r.appendControlObservation(state, call, assistantText, toolResult)
		if err := r.saveCheckpoint(ctx, input, state, checkpointAfterReview, ""); err != nil {
			return r.fail(input, state, CodeAgentFailed, err), true
		}
		return StructuredOutcome{}, false
	case "finish":
		message := stringValue(call.Args["message"])
		suggestions := NormalizeSuggestedNextInputs(call.Args["suggested_next_inputs"])
		return r.finishCandidate(ctx, input, state, call, assistantText, message, suggestions)
	default:
		r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, "unknown runtime control tool", false))
		return StructuredOutcome{}, false
	}
}

func (r *Runtime) awaitScopeExpansion(
	ctx context.Context,
	input RuntimeInput,
	state *RunState,
	request model.ScopeExpansionRequestedPayload,
	call *llm.ToolCall,
	assistantText string,
) (StructuredOutcome, bool) {
	prompter, ok := input.Prompter.(ScopeExpansionPrompter)
	if !ok {
		if call != nil {
			r.appendControlObservation(state, *call, assistantText, failedToolResult(CodeInvalidControlCall, "scope expansion requires an interactive prompter", false))
		}
		return StructuredOutcome{}, false
	}
	state.resumePhase = PhaseExecuting
	state.pendingScopeExpansion = &PendingScopeExpansion{Request: request, ResumePhase: PhaseExecuting}
	r.changePhase(input.Emitter, state, PhaseWaitingInput, "scope expansion approval required")
	if err := r.saveCheckpoint(ctx, input, state, checkpointBeforeScopeExpansion, request.InteractionID); err != nil {
		return r.fail(input, state, CodeAgentFailed, err), true
	}
	state.pauseActiveClock(r.clockNow())
	var answer model.ScopeExpansionAnswer
	var err error
	if call == nil {
		if resumer, ok := input.Prompter.(ScopeExpansionResumer); ok {
			answer, err = resumer.ResumeScopeExpansion(ctx, request)
		} else {
			answer, err = prompter.AskScopeExpansion(ctx, request)
		}
	} else {
		answer, err = prompter.AskScopeExpansion(ctx, request)
	}
	if err != nil {
		code := CodeAgentFailed
		if errors.Is(err, context.Canceled) {
			code = CodeCanceled
		}
		return r.fail(input, state, code, err), true
	}
	state.resumeActiveClock(r.clockNow())
	if state.scope.Revision != request.BaseRevision {
		return r.fail(input, state, CodeAgentFailed, errors.New("scope changed while expansion approval was pending")), true
	}
	var applied *model.RunScope
	if answer.Decision == "approve" {
		next := request.ProposedScope
		applied = &next
	} else if answer.Decision == "adjust" {
		if answer.AdjustedScope == nil {
			return r.fail(input, state, CodeAgentFailed, errors.New("adjusted scope is required")), true
		}
		next, resolveErr := resolveRuntimeScopeSelection(state.pack, *answer.AdjustedScope)
		if resolveErr != nil || !scopeContains(next, state.scope) {
			if resolveErr == nil {
				resolveErr = errors.New("adjusted scope cannot remove current privileges")
			}
			return r.fail(input, state, CodeAgentFailed, resolveErr), true
		}
		next.Revision = state.scope.Revision + 1
		applied = &next
	}
	if applied != nil {
		previous := state.scope
		candidate := *state
		candidate.scope = *applied
		candidate.pack.Command.Scope = *applied
		candidate.pack.Target.SlideIDs = append([]string{}, applied.SlideIDs...)
		candidate.pendingScopeExpansion = nil
		candidate.phase = PhaseExecuting
		checkpoint := r.checkpointForBoundary(&candidate, checkpointAfterScopeExpansion, "")
		if input.CommitScopeExpansion == nil {
			return r.fail(input, state, CodeAgentFailed, errors.New("atomic scope expansion store is required")), true
		}
		if err := input.CommitScopeExpansion(ctx, *applied, checkpoint); err != nil {
			return r.fail(input, state, CodeAgentFailed, err), true
		}
		state.scope = *applied
		state.pack.Command.Scope = *applied
		state.pack.Target.SlideIDs = append([]string{}, applied.SlideIDs...)
		registry, registryErr := buildDomainToolRegistry(input, state.pack)
		if registryErr != nil {
			return r.fail(input, state, CodeAgentFailed, registryErr), true
		}
		state.tools = registry
		if input.Emitter != nil {
			input.Emitter.Emit(model.EventScopeUpdated, model.ScopeUpdatedPayload{PublicEventBase: publicBase(state.runID), PreviousScope: previous, Scope: *applied, Cause: "user_approved_expansion", InteractionID: request.InteractionID})
		}
	}
	state.pendingScopeExpansion = nil
	state.resumePhase = PhaseExecuting
	r.changePhase(input.Emitter, state, PhaseExecuting, "scope expansion answered")
	prompter.ResumeAfterScopeExpansion(ctx)
	if input.Emitter != nil {
		input.Emitter.Emit(model.EventScopeExpansionAnswered, model.ScopeExpansionAnsweredPayload{PublicEventBase: publicBase(state.runID), InteractionID: request.InteractionID, CallID: request.CallID, BaseRevision: request.BaseRevision, Decision: answer.Decision, AppliedScope: applied})
	}
	if call == nil {
		call = &llm.ToolCall{ID: request.CallID, Name: "request_privilege", Args: map[string]any{}}
	}
	result := SuccessfulToolResult("scope expansion answered")
	result.Data = map[string]any{"decision": answer.Decision, "scope": state.scope}
	r.appendControlObservation(state, *call, assistantText, result)
	if applied == nil {
		if err := r.saveCheckpoint(ctx, input, state, checkpointAfterScopeExpansion, ""); err != nil {
			return r.fail(input, state, CodeAgentFailed, err), true
		}
	}
	return StructuredOutcome{}, false
}

func proposeScopeExpansion(current model.RunScope, pack contextengine.ContextPack, addSlideIDs []string) (model.RunScope, model.ScopeExpansionAddition, error) {
	known, ordered := runtimeSlideOrder(pack)
	requested := make(map[string]bool)
	for _, raw := range addSlideIDs {
		id := strings.TrimSpace(raw)
		if !known[id] {
			return model.RunScope{}, model.ScopeExpansionAddition{}, fmt.Errorf("slide %q is not present in the current outline", id)
		}
		requested[id] = true
	}
	next := current
	selected := make(map[string]bool, len(current.SlideIDs)+len(requested))
	for _, id := range current.SlideIDs {
		selected[id] = true
	}
	for id := range requested {
		selected[id] = true
	}
	if current.Source.Kind != model.ScopeAllPages && len(selected) > len(current.SlideIDs) {
		next.Source = model.ScopeSource{Kind: model.ScopeCustomPages}
		next.IncludeRunCreatedSlides = false
	}
	next.SlideIDs = orderedSelection(ordered, selected)
	next.Revision = current.Revision
	if !slices.Equal(next.SlideIDs, current.SlideIDs) || next.Source.Kind != current.Source.Kind {
		next.Revision++
	}
	return next, model.ScopeExpansionAddition{SlideIDs: orderedSelection(ordered, requested)}, next.Validate()
}

func runtimeSlideOrder(pack contextengine.ContextPack) (map[string]bool, []string) {
	known := make(map[string]bool, len(pack.Outline.Summaries))
	ordered := make([]string, 0, len(pack.Outline.Summaries))
	for _, slide := range pack.Outline.Summaries {
		known[slide.ID] = true
		ordered = append(ordered, slide.ID)
	}
	return known, ordered
}

func orderedSelection(ordered []string, selected map[string]bool) []string {
	out := make([]string, 0, len(selected))
	for _, id := range ordered {
		if selected[id] {
			out = append(out, id)
		}
	}
	return out
}

func resolveRuntimeScopeSelection(pack contextengine.ContextPack, input model.CreateRunScopeInput) (model.RunScope, error) {
	known, ordered := runtimeSlideOrder(pack)
	selected := map[string]bool{}
	source := model.ScopeSource{Kind: input.Selection.Kind}
	switch input.Selection.Kind {
	case model.ScopeCurrentPage:
		if !known[input.Selection.CurrentSlideID] {
			return model.RunScope{}, errors.New("adjusted current page is invalid")
		}
		selected[input.Selection.CurrentSlideID] = true
	case model.ScopeAllPages:
		for _, id := range ordered {
			selected[id] = true
		}
	case model.ScopeCustomPages:
		for _, id := range input.Selection.SlideIDs {
			if !known[id] {
				return model.RunScope{}, fmt.Errorf("adjusted slide %q is invalid", id)
			}
			selected[id] = true
		}
	case model.ScopeCustomSections:
		wanted := map[string]bool{}
		for _, id := range input.Selection.SectionIDs {
			wanted[id] = true
		}
		for _, section := range pack.Outline.Outline.Sections {
			if !wanted[section.ID] {
				continue
			}
			delete(wanted, section.ID)
			source.SectionIDs = append(source.SectionIDs, section.ID)
			for _, slide := range section.Slides {
				selected[slide.SlideID] = true
			}
			for _, subsection := range section.Subsections {
				for _, slide := range subsection.Slides {
					selected[slide.SlideID] = true
				}
			}
		}
		if len(wanted) > 0 {
			return model.RunScope{}, errors.New("adjusted section is invalid")
		}
	default:
		return model.RunScope{}, errors.New("adjusted scope selection is invalid")
	}
	next := model.RunScope{SlideIDs: orderedSelection(ordered, selected), Source: source, IncludeRunCreatedSlides: source.Kind == model.ScopeAllPages, Revision: 1}
	return next, next.Validate()
}

func scopeContains(candidate, current model.RunScope) bool {
	if current.IncludeRunCreatedSlides && !candidate.IncludeRunCreatedSlides {
		return false
	}
	for _, id := range current.SlideIDs {
		if !candidate.ContainsSlide(id) {
			return false
		}
	}
	return true
}

func (r *Runtime) finishCandidate(
	ctx context.Context,
	input RuntimeInput,
	state *RunState,
	call llm.ToolCall,
	assistantText string,
	message string,
	suggestedNextInputs []string,
) (StructuredOutcome, bool) {
	if violation := finishMessageViolation(message); violation != "" {
		r.appendControlObservation(state, call, assistantText, failedToolResult(CodeFinishMessageEmpty, violation, true))
		return StructuredOutcome{}, false
	}
	finishPhase := state.phase
	r.changePhase(input.Emitter, state, PhaseCompletionCheck, "finish candidate submitted")
	r.emitProgress(input.Emitter, state, model.ActivityCompletionReviewing)
	changes := state.changeSet()
	result := r.Gate.Check(CompletionContext{
		Mode: state.mode, FinishPhase: finishPhase, ActiveTools: state.activeTools,
		Issues: state.issues, Scope: state.scope, Session: state.tx, Changes: changes,
		Evidence: state.ledger, Context: state.pack, Plan: state.plan,
		Requirements: state.requirements, Work: state.work, FinishMessage: message, Canceled: ctx.Err() != nil,
	})
	recordTrace(input.Trace, state.runID, "completion.checked", map[string]any{
		"loop_id": state.loopID, "accepted": result.Accepted, "issues": result.Issues,
	})
	if !result.Accepted {
		key, evidenceVersion := result.RejectionKey(), state.ledger.Version()
		if key == state.gateKey && evidenceVersion == state.gateEvidence {
			state.gateCount++
		} else {
			state.gateKey, state.gateCount, state.gateEvidence = key, 1, evidenceVersion
		}
		if state.gateCount >= state.budget.MaxIdenticalGateRejections {
			return r.fail(input, state, CodeGateRejectedRepeated, errors.New("completion gate rejected the same unchanged state three times")), true
		}
		r.changePhase(input.Emitter, state, finishPhase, "completion rejected; continuing the same loop")
		for i := range result.Issues {
			result.Issues[i].NextAction = model.ErrorDefinitionFor(result.Issues[i].Code).ModelMessage
		}
		observation := ToolResult{
			OK: false, Summary: "completion rejected", Data: map[string]any{"issues": result.Issues},
			ChangedTargets: []ChangedTarget{}, Evidence: []Evidence{}, Issues: []Issue{},
			Retryable: false, Code: CodeCompletionGateBlocked,
		}
		state.messages = appendToolObservation(
			state.messages,
			call, assistantText, observation,
		)
		if err := r.saveCheckpoint(ctx, input, state, checkpointGateRejected, ""); err != nil {
			return r.fail(input, state, CodeAgentFailed, err), true
		}
		return StructuredOutcome{}, false
	}
	state.lastSummary = safeFinalMessage(message, state.mode, changes.Count(), state.publicTextContext())
	// The finish payload is the assistant's authoritative final reply. Persist it
	// so the next run receives the same conversation that the user saw.
	state.messages = append(state.messages, llm.Message{
		Role: llm.RoleAssistant, Content: llm.TextContent(state.lastSummary),
	})
	if state.mode == model.ModeExecute {
		// All successful tool calls are already durable. Completion only closes
		// the now-empty transaction and records the terminal audit boundary.
		r.changePhase(input.Emitter, state, PhaseCommitting, "completion accepted; finalizing durable changes")
		state.tx.Discard()
		state.committed = true
		if err := r.saveCheckpoint(ctx, input, state, checkpointAfterCommit, ""); err != nil {
			// Artifact files and metadata were committed at their tool boundaries.
			recordTrace(input.Trace, state.runID, "checkpoint.persistence_failed", map[string]any{
				"loop_id": state.loopID, "boundary": checkpointAfterCommit,
			})
		}
	}
	r.changePhase(input.Emitter, state, PhaseTerminal, "run completed")
	if err := r.saveCheckpoint(ctx, input, state, checkpointTerminal, ""); err != nil {
		if !state.committed {
			return r.fail(input, state, CodeAgentFailed, err), true
		}
		recordTrace(input.Trace, state.runID, "checkpoint.persistence_failed", map[string]any{
			"loop_id": state.loopID, "boundary": checkpointTerminal,
		})
	}
	for i := range suggestedNextInputs {
		suggestedNextInputs[i] = model.PublicText(suggestedNextInputs[i], state.publicTextContext())
	}
	state.suggestedNextInputs = append([]string{}, suggestedNextInputs...)
	outcome := r.outcome(state, StatusCompleted, "", "")
	if input.Emitter != nil {
		affected := publicAffectedTargets(input.ProjectDir, changes)
		input.Emitter.Emit(model.EventMessageFinal, model.MessageFinalPayload{
			PublicEventBase: publicBase(state.runID), MessageID: newMessageID(),
			Text:                state.lastSummary,
			AffectedTargets:     affected,
			SuggestedNextInputs: suggestedNextInputs,
		})
		input.Emitter.Emit(model.EventRunCompleted, model.NewRunTerminalPayloadFromBase(
			publicBase(state.runID),
			outcomeDurationMS(outcome),
			affected,
			nil,
		))
	}
	return outcome, true
}

func changedResources(values []ChangedTarget) []Resource {
	out := make([]Resource, 0, len(values))
	for _, value := range values {
		out = append(out, value.Target())
	}
	return out
}

func (r *Runtime) changePhase(emitter EventEmitter, state *RunState, next RunPhase, reason string) {
	previous := state.phase
	state.phase = next
	if state.lifecycle != nil {
		state.lifecycle.PhaseChanged(next)
	}
	recordTrace(state.trace, state.runID, "phase.changed", map[string]any{
		"loop_id": state.loopID, "from": previous, "phase": next, "reason": reason,
	})
}

func (r *Runtime) fail(input RuntimeInput, state *RunState, code string, err error) StructuredOutcome {
	return r.failAgentError(input, state, model.NewAgentError(code, "run", err))
}

func (r *Runtime) failAgentError(input RuntimeInput, state *RunState, agentErr *model.AgentError) StructuredOutcome {
	if agentErr == nil {
		agentErr = model.NewAgentError("INTERNAL", "run", errors.New("unknown runtime error"))
	}
	status, publicStatus := StatusFailed, "failed"
	if errors.Is(agentErr, context.Canceled) || agentErr.Code == CodeCanceled {
		status, publicStatus = StatusCanceled, "canceled"
	}
	recordTrace(input.Trace, state.runID, "error.projected", agentErr.TraceProjection())
	if input.Logger != nil {
		input.Logger.Error("workflow run failed",
			zap.String("run_id", state.runID),
			zap.String("loop_id", state.loopID),
			zap.String("mode", string(state.mode)),
			zap.String("phase", string(state.phase)),
			zap.Int("turn", state.turns),
			zap.String("code", agentErr.Code),
			zap.String("operation", agentErr.Operation),
			zap.Any("error", agentErr.TraceProjection()),
		)
	}
	r.changePhase(input.Emitter, state, PhaseTerminal, agentErr.Code)
	_ = r.saveCheckpoint(context.Background(), input, state, checkpointTerminal, "")
	outcome := r.outcome(state, status, agentErr.Code, agentErr.Error())
	if input.Emitter != nil {
		event := runTerminalEventForError(publicStatus, agentErr)
		input.Emitter.Emit(event, model.NewRunTerminalPayloadFromBase(
			publicBase(state.runID),
			outcomeDurationMS(outcome),
			publicAffectedTargets(input.ProjectDir, state.changeSet()),
			terminalPublicError(event, agentErr),
		))
	}
	return outcome
}

func (r *Runtime) logProviderRequest(input RuntimeInput, state *RunState, schemas []ToolSchema) {
	if input.Logger == nil {
		return
	}
	input.Logger.Debug("workflow provider request",
		zap.String("run_id", state.runID),
		zap.String("loop_id", state.loopID),
		zap.String("mode", string(state.mode)),
		zap.String("phase", string(state.phase)),
		zap.Int("turn", state.turns),
		zap.Int("message_count", len(state.messages)),
		zap.Int("tool_count", len(schemas)),
		zap.Strings("tools", toolSchemaNames(schemas)),
		zap.Bool("has_continuation", state.continuation != nil),
		zap.String("plan_status", planStatus(state.plan)),
		zap.String("system_hash", hashBytes([]byte(runtimeSystemPromptForRequest(agentRequestForState(input, state, schemas))))),
		zap.String("tools_hash", hashCheckpointValue(schemas)),
		zap.Int("estimated_input_tokens", state.tokens),
	)
}

func toolSchemaNames(schemas []ToolSchema) []string {
	out := make([]string, 0, len(schemas))
	for _, schema := range schemas {
		out = append(out, schema.Name)
	}
	return out
}

func planStatus(plan *Plan) string {
	if plan == nil {
		return ""
	}
	return string(plan.Status)
}

func runTerminalEventForError(publicStatus string, agentErr *model.AgentError) model.EventType {
	if publicStatus == "canceled" {
		return model.EventRunCanceled
	}
	if isRuntimeErrorCode(agentErr.Code) {
		return model.EventRunError
	}
	return model.EventRunFailed
}

func terminalPublicError(event model.EventType, agentErr *model.AgentError) *model.PublicError {
	if event == model.EventRunCompleted || event == model.EventRunCanceled {
		return nil
	}
	return agentErr.Public()
}

func isRuntimeErrorCode(code string) bool {
	switch code {
	case "INTERNAL", CodeAgentFailed, CodeCommitFailed, ErrCapabilityDenied.Error(), "RUN_FATAL_EXIST", "MODEL_PROVIDER_UNSUPPORTED", "PROVIDER_BAD_REQUEST":
		return true
	default:
		return false
	}
}

func (r *Runtime) outcome(state *RunState, status WorkflowStatus, code, message string) StructuredOutcome {
	now := r.clockNow()
	active, wall, waiting := state.durationSnapshot(now)
	durationMS := active.Milliseconds()
	recordTrace(state.trace, state.runID, "runtime.duration", map[string]any{
		"active_duration_ms":  durationMS,
		"wall_duration_ms":    wall.Milliseconds(),
		"waiting_duration_ms": waiting.Milliseconds(),
	})
	return StructuredOutcome{
		LoopID: state.loopID, Phase: state.phase, Status: status,
		Scope: state.scope, Changes: state.changeSet(), Issues: append([]Issue{}, state.issues...),
		Summary: state.lastSummary, SuggestedNextInputs: append([]string{}, state.suggestedNextInputs...),
		Code: code, Message: message, DurationMS: &durationMS,
	}
}

func outcomeDurationMS(outcome StructuredOutcome) int64 {
	if outcome.DurationMS == nil || *outcome.DurationMS < 0 {
		return 0
	}
	return *outcome.DurationMS
}

func (state *RunState) changeSet() ChangeSet {
	if state.tx == nil {
		return state.committedChanges
	}
	return mergeChangeSets(state.committedChanges, state.tx.ChangeSet())
}

func (state *RunState) checkpoint(questionID string, now time.Time) RuntimeCheckpoint {
	return RuntimeCheckpoint{
		RunID: state.runID, LoopID: state.loopID, Phase: state.phase, Mode: state.mode,
		ResumePhase: state.resumePhase, Plan: state.plan, Requirements: state.requirements, Work: state.work, Changes: state.changeSet(),
		Evidence: state.ledger.Entries(state.changeSet()), Turns: state.turns, ToolCalls: state.toolCalls,
		ActiveDurationMS:  state.activeDurationAt(now).Milliseconds(),
		WaitingDurationMS: state.waitingDurationAt(now).Milliseconds(),
		WaitingQuestionID: questionID, CompletionFailures: state.gateCount,
		PendingCommand: state.pendingCommand, PendingScopeExpansion: state.pendingScopeExpansion,
		Scope: state.scope, Session: state.tx.Snapshot(),
	}
}

func (r *Runtime) checkBudget(ctx context.Context, state *RunState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if state.turns >= state.budget.MaxTurns {
		recordTrace(state.trace, state.runID, "runtime.budget_exhausted", map[string]any{
			"budget_kind": "turns", "turns": state.turns, "max_turns": state.budget.MaxTurns,
		})
		return errors.New("runtime turn budget exhausted")
	}
	return r.checkActiveDurationBudget(state)
}

func (r *Runtime) checkActiveDurationBudget(state *RunState) error {
	active, wall, waiting := state.durationSnapshot(r.clockNow())
	if active >= state.budget.MaxDuration {
		recordTrace(state.trace, state.runID, "runtime.budget_exhausted", map[string]any{
			"budget_kind": "active_duration", "active_duration_ms": active.Milliseconds(),
			"max_duration_ms":  state.budget.MaxDuration.Milliseconds(),
			"wall_duration_ms": wall.Milliseconds(), "waiting_duration_ms": waiting.Milliseconds(),
		})
		return errors.New("runtime active duration budget exhausted")
	}
	return nil
}

func (r *Runtime) clockNow() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

func (state *RunState) pauseActiveClock(now time.Time) {
	if state == nil || !state.activeRunning {
		return
	}
	state.activeElapsed = state.activeDurationAt(now)
	state.activeSince = time.Time{}
	state.activeRunning = false
	state.waitingSince = now
}

func (state *RunState) resumeActiveClock(now time.Time) {
	if state == nil || state.activeRunning {
		return
	}
	state.waitingElapsed = state.waitingDurationAt(now)
	state.waitingSince = time.Time{}
	state.activeSince = now
	state.activeRunning = true
}

func (state *RunState) activeDurationAt(now time.Time) time.Duration {
	if state == nil {
		return 0
	}
	elapsed := state.activeElapsed
	if state.activeRunning && !state.activeSince.IsZero() && now.After(state.activeSince) {
		elapsed += now.Sub(state.activeSince)
	}
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

func (state *RunState) durationSnapshot(now time.Time) (active, wall, waiting time.Duration) {
	active = state.activeDurationAt(now)
	waiting = state.waitingDurationAt(now)
	wall = active + waiting
	return active, wall, waiting
}

func (state *RunState) waitingDurationAt(now time.Time) time.Duration {
	if state == nil {
		return 0
	}
	elapsed := state.waitingElapsed
	if !state.waitingSince.IsZero() && now.After(state.waitingSince) {
		elapsed += now.Sub(state.waitingSince)
	}
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

func (r *Runtime) appendSteering(ctx context.Context, state *RunState, steering SteeringSource) error {
	if steering == nil {
		return nil
	}
	messages, err := steering.DrainInputs(ctx)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		if strings.TrimSpace(message.Content) != "" || len(message.Attachments) > 0 || len(message.DOMSelections) > 0 {
			state.messages = append(state.messages, llm.Message{Role: llm.RoleUser, Content: referenceMessageParts(
				"User steering: "+message.Content, message.ProjectID, message.Attachments, message.DOMSelections, message.ReferenceOrder,
			), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "steering", RunID: state.runID}})
			if message.Scope.Source.Kind != "" {
				state.scope = message.Scope
				state.pack.Command.Scope = message.Scope
			}
			state.pack.Command.DOMSelections = append(state.pack.Command.DOMSelections, message.DOMSelections...)
			ids = append(ids, message.ID)
		}
	}
	return steering.MarkInputsInjected(ctx, ids)
}

func (r *Runtime) persistTranscript(input RuntimeInput, state *RunState) error {
	if input.Transcript == nil || input.Context.Manifest.ThreadID == "" {
		return nil
	}
	return input.Transcript.Replace(input.ProjectDir, input.Context.Manifest.ThreadID, llm.NormalizeHistory(state.messages))
}

func attachmentMessageParts(text, projectID string, attachments []model.AttachmentReference) []llm.ContentPart {
	return referenceMessageParts(text, projectID, attachments, nil, model.NormalizeReferenceOrder(nil, attachments, nil))
}

func referenceMessageParts(text, projectID string, attachments []model.AttachmentReference, selections []model.DOMSelection, order []model.ReferenceOrderItem) []llm.ContentPart {
	parts := llm.TextContent(text)
	attachmentByID := map[string]model.AttachmentReference{}
	for _, item := range attachments {
		attachmentByID[item.ID] = item
	}
	selectionByID := map[string]model.DOMSelection{}
	for _, item := range selections {
		selectionByID[item.SelectionID] = item
	}
	for _, ref := range order {
		if ref.Kind == "dom" {
			selection, ok := selectionByID[ref.RefID]
			if !ok {
				continue
			}
			description, _ := json.Marshal(selection)
			parts = append(parts, llm.ContentPart{Type: "text", Text: "<selected_dom>" + string(description) + "</selected_dom>"})
			continue
		}
		attachment, ok := attachmentByID[ref.RefID]
		if !ok {
			continue
		}
		description, _ := json.Marshal(map[string]any{
			"attachment_id": attachment.ID, "name": attachment.OriginalName,
			"media_type": attachment.MediaType, "width": attachment.Width, "height": attachment.Height,
			"size_bytes": attachment.SizeBytes,
		})
		parts = append(parts,
			llm.ContentPart{Type: "text", Text: "<image_attachment>" + string(description) + "</image_attachment>"},
			llm.ContentPart{Type: "image", ImageRef: "project:" + projectID + "/attachment:" + attachment.ID + "/original", MIMEType: attachment.MediaType, Detail: "high"},
		)
	}
	return parts
}

func containsMessageText(messages []llm.Message, text string) bool {
	for _, message := range messages {
		if message.Role == llm.RoleUser && message.Text() == text {
			return true
		}
	}
	return false
}

func containsRunInstruction(messages []llm.Message, runID string) bool {
	for _, message := range messages {
		if m := message.Metadata; m != nil && m.Origin == "user" && m.Kind == "instruction" && m.RunID == runID {
			return true
		}
	}
	return false
}

func (r *Runtime) emitReasoning(emitter EventEmitter, state *RunState, raw string) {
	if emitter == nil {
		return
	}
	text := model.PublicText(raw, state.publicTextContext())
	if text == "" || reasoningDuplicate(state.lastReasoning, text) {
		return
	}
	state.lastReasoning = text
	emitter.Emit(model.EventMessageReasoning, model.MessageReasoningPayload{
		PublicEventBase: publicBase(state.runID), MessageID: newMessageID(), Text: text,
	})
}

func (r *Runtime) emitProgress(
	emitter EventEmitter,
	state *RunState,
	activity model.RunActivity,
) {
	if emitter == nil {
		return
	}
	key := string(activity)
	if key == state.lastProgress {
		return
	}
	state.lastProgress = key
	payload := model.RunProgressPayload{PublicEventBase: publicBase(state.runID), Activity: activity}
	if cognitive, ok := r.Agent.(CognitiveAgent); ok {
		if route, ok := cognitive.Provider.(*llm.RoutedProvider); ok && route.State().FallbackUsed {
			selected := route.State()
			payload.ModelSwitch = &model.ModelSwitch{From: selected.Initial, To: selected.Active, Purpose: selected.Purpose}
		}
	}
	emitter.Emit(model.EventRunProgress, payload)
}

func (r *Runtime) emitToolProgress(emitter EventEmitter, state *RunState, call llm.ToolCall) {
	r.emitProgress(emitter, state, toolActivity(call))
}

func toolBatchActivity(calls []llm.ToolCall) model.RunActivity {
	activity := model.ActivityRunAnalyzing
	priority := 0
	for _, call := range calls {
		candidate := toolActivity(call)
		if value := activityPriority(candidate); value > priority {
			activity, priority = candidate, value
		}
	}
	return activity
}

func toolActivity(call llm.ToolCall) model.RunActivity {
	switch call.Name {
	case "read_ppt":
		resource, _ := call.Args["resource"].(map[string]any)
		switch stringValue(resource["kind"]) {
		case "manifest", "outline":
			return model.ActivityPresentationStructureReading
		case "design":
			return model.ActivityPresentationDesignReading
		default:
			return model.ActivitySlideContentReading
		}
	case "read_image":
		return model.ActivityReferenceInspecting
	case "mutate_ppt":
		op := stringValue(call.Args["op"])
		switch {
		case strings.HasPrefix(op, "manifest."), strings.HasPrefix(op, "outline."):
			return model.ActivityPresentationStructureUpdating
		case strings.HasPrefix(op, "design."):
			return model.ActivityPresentationDesignUpdating
		case strings.HasPrefix(op, "slide.") && strings.HasSuffix(op, ".write"):
			return model.ActivitySlideCreating
		default:
			return model.ActivitySlideUpdating
		}
	case "render_slide":
		return model.ActivitySlideLayoutChecking
	case "load_component", "load_skill":
		return model.ActivityResourcePreparing
	case "run_command":
		return model.ActivityCommandExecuting
	default:
		return model.ActivityRunAnalyzing
	}
}

func activityPriority(activity model.RunActivity) int {
	switch activity {
	case model.ActivitySlideLayoutChecking:
		return 70
	case model.ActivitySlideCreating:
		return 60
	case model.ActivitySlideUpdating:
		return 50
	case model.ActivityPresentationDesignUpdating:
		return 45
	case model.ActivityPresentationStructureUpdating:
		return 40
	case model.ActivitySlideContentReading:
		return 35
	case model.ActivityPresentationDesignReading:
		return 30
	case model.ActivityPresentationStructureReading:
		return 25
	case model.ActivityReferenceInspecting:
		return 20
	case model.ActivityResourcePreparing:
		return 15
	case model.ActivityCommandExecuting:
		return 10
	default:
		return 1
	}
}

func (r *Runtime) compactIfNeeded(ctx context.Context, input RuntimeInput, state *RunState, schemas []ToolSchema) error {
	if r.Compactor == nil || state.tokens < state.budget.ContextCompactionThreshold ||
		(state.nextCompactionTokens > 0 && state.tokens < state.nextCompactionTokens) {
		return nil
	}
	progress := &model.ContextCompactionProgress{ID: model.MustShortID("compact"), Phase: 0}
	r.emitContextWindow(input.Emitter, state, state.lastWindow, "compacting", progress)
	before := state.lastWindow
	startedAt := r.clockNow()
	// Preserve only this turn's explicitly requested visual observations across
	// compaction; the compactor and durable history operate on text references.
	pendingImages := []llm.Message{}
	for _, message := range state.messages {
		if llm.HasRenderImages([]llm.Message{message}) {
			pendingImages = append(pendingImages, llm.Message{Role: llm.RoleUser, Content: append([]llm.ContentPart{}, message.Content...)})
		}
	}
	compactionInput := llm.NormalizeHistory(state.messages)
	progress.Phase = 1
	r.emitContextWindow(input.Emitter, state, before, "compacting", progress)
	result, err := r.Compactor.Compact(ctx, compactionInput)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	progress.Phase = 2
	r.emitContextWindow(input.Emitter, state, before, "compacting", progress)
	state.messages = append(result.Messages, pendingImages...)
	state.continuation = nil
	recordTrace(input.Trace, state.runID, "provider.continuation_reset", map[string]any{"reason": "history_compacted"})
	if err := r.persistTranscript(input, state); err != nil {
		return err
	}
	r.measureContextWindow(input, state, schemas)
	if state.tokens >= state.budget.ContextCompactionThreshold {
		state.nextCompactionTokens = state.tokens + 1024
	} else {
		state.nextCompactionTokens = 0
	}
	if input.RecordCompaction != nil {
		compaction, err := input.RecordCompaction(ctx, result, before, state.lastWindow, r.clockNow().Sub(startedAt))
		if err != nil {
			return err
		}
		if input.Emitter != nil {
			input.Emitter.Emit(model.EventContextCompacted, model.ContextCompactedPayload{
				PublicEventBase: publicBase(state.runID), Compaction: compaction,
			})
		}
	}
	return nil
}

func (r *Runtime) measureContextWindow(input RuntimeInput, state *RunState, schemas []ToolSchema) {
	request := prepareAgentRequest(agentRequestForState(input, state, schemas))
	state.messages = request.Messages
	system := runtimeSystemPromptForRequest(request)
	tools := make([]llm.ToolSchema, 0, len(schemas))
	for _, schema := range schemas {
		tools = append(tools, llm.ToolSchema{
			Name: schema.Name, Description: schema.Description, Parameters: schema.Parameters,
		})
	}
	snapshot := (contextengine.PromptEstimator{}).Estimate(contextengine.PromptEstimateInput{
		System: system, Messages: request.Messages, Tools: tools,
		Max: r.ContextWindowTokens, Factor: state.calibrationFactor,
	})
	snapshot.CompactableTokens = contextcompact.CompactableTokens(state.messages)
	snapshot.CompactThresholdTokens = contextcompact.MinimumCompactableTokens
	state.tokens = snapshot.Total
	state.lastWindow = snapshot
	if snapshots, ok := input.Calibration.(interface {
		SetSnapshot(string, contextengine.WindowSnapshot)
	}); ok {
		snapshots.SetSnapshot(input.Context.Manifest.ThreadID, snapshot)
	}
	r.emitContextWindow(input.Emitter, state, snapshot, "idle")
}

func (r *Runtime) emitContextWindow(
	emitter EventEmitter,
	state *RunState,
	snapshot contextengine.WindowSnapshot,
	status string,
	progress ...*model.ContextCompactionProgress,
) {
	if emitter == nil || snapshot.Max <= 0 {
		return
	}
	buckets := make(map[string]int, len(snapshot.Buckets))
	details := make(map[string][]model.ContextWindowBucketDetail, len(snapshot.Details))
	for _, bucket := range contextengine.ContextBuckets {
		key := string(bucket)
		buckets[key] = snapshot.Buckets[bucket]
		details[key] = make([]model.ContextWindowBucketDetail, 0, len(snapshot.Details[bucket]))
		for _, detail := range snapshot.Details[bucket] {
			details[key] = append(details[key], model.ContextWindowBucketDetail{
				Name: detail.Name, Tokens: detail.Tokens,
			})
		}
	}
	var compaction *model.ContextCompactionProgress
	if len(progress) > 0 {
		value := *progress[0]
		compaction = &value
	}
	emitter.Emit(model.EventContextWindowUpdated, model.ContextWindowUpdatedPayload{
		PublicEventBase: publicBase(state.runID), Total: snapshot.Total, Max: snapshot.Max,
		Ratio: snapshot.Ratio, CompactableTokens: snapshot.CompactableTokens,
		CompactThresholdTokens: snapshot.CompactThresholdTokens,
		Status:                 status, Buckets: buckets, Details: details, Compaction: compaction,
	})
}

func (r *Runtime) appendControlObservation(
	state *RunState,
	call llm.ToolCall,
	assistantText string,
	result ToolResult,
) {
	state.messages = appendToolObservation(state.messages, call, assistantText, result)
}

func finishMessageViolation(message string) string {
	final := strings.TrimSpace(message)
	if final == "" {
		return "finish.message is required and must contain the complete final user-facing answer"
	}
	return ""
}

func appendToolObservation(
	messages []llm.Message,
	call llm.ToolCall,
	assistantText string,
	result ToolResult,
) []llm.Message {
	parts := append([]llm.ContentPart(nil), result.ObservationParts...)
	content := result.Observation
	if len(parts) == 0 && content == "" {
		content = modelToolObservation(result)
	}
	if len(parts) == 0 {
		parts = llm.TextContent(content)
	}
	return append(messages,
		llm.Message{
			Role: llm.RoleAssistant, Content: llm.TextContent(assistantText),
			ToolCalls: []llm.ToolCall{call},
		},
		llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, Content: parts, Metadata: result.ObservationMetadata},
	)
}

func appendBatchObservations(
	messages []llm.Message,
	calls []llm.ToolCall,
	assistantText string,
	results []ToolResult,
) []llm.Message {
	messages = append(messages, llm.Message{
		Role: llm.RoleAssistant, Content: llm.TextContent(assistantText),
		ToolCalls: calls,
	})
	for index, call := range calls {
		result := failedToolResult(CodeInvalidControlCall, "tool call was not executed", false)
		if index < len(results) {
			result = results[index]
		}
		parts := append([]llm.ContentPart(nil), result.ObservationParts...)
		content := result.Observation
		if len(parts) == 0 && content == "" {
			content = modelToolObservation(result)
		}
		if len(parts) == 0 {
			parts = llm.TextContent(content)
		}
		messages = append(messages, llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, Content: parts, Metadata: result.ObservationMetadata})
	}
	return messages
}

func approximateTokens(value string) int {
	if value == "" {
		return 0
	}
	return len([]rune(value))/4 + 1
}

func approximateMessageTokens(messages []llm.Message) int {
	total := 0
	for _, message := range messages {
		for _, part := range message.Content {
			if part.Type == "text" {
				total += approximateTokens(part.Text)
			}
		}
	}
	return total
}

func isControlTool(name string) bool {
	return name == "create_plan" || name == "update_plan" || name == "ask_user" || name == "request_privilege" || name == "review_completion" || name == "finish"
}

func controlSchemas(phase RunPhase, mode model.RunMode, plan *Plan) []ToolSchema {
	out := []ToolSchema{}
	if mode == model.ModePlan && phase == PhasePlanning {
		out = append(out, ToolSchema{Name: "create_plan", Description: "Persist the complete Markdown plan and wait for explicit user approval. Do not call finish.", Parameters: objectSchema([]string{"title", "content", "steps"}, map[string]any{
			"title": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"},
			"steps": map[string]any{"type": "array", "minItems": 1, "items": objectSchema([]string{"title"}, map[string]any{"title": map[string]any{"type": "string"}, "target_slide_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}})},
		})})
	}
	if mode == model.ModePlan && phase == PhasePlanning {
		out = append(out, ToolSchema{Name: "update_plan", Description: "Replace the complete proposed plan after user feedback. Runtime owns IDs and approval state.", Parameters: objectSchema([]string{"title", "content", "steps"}, map[string]any{
			"title": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"},
			"steps": map[string]any{"type": "array", "minItems": 1, "items": objectSchema([]string{"title"}, map[string]any{"title": map[string]any{"type": "string"}, "target_slide_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}})},
		})})
	}
	if mode == model.ModeExecute && phase == PhaseExecuting {
		out = append(out, ToolSchema{
			Name: "request_privilege", Description: "Request a user-approved expansion of the editable page set. Provide only additional slide IDs and explain why in page terms. Global resources and both Spec/HTML of authorized pages are already writable. This call must be the only call in the response.",
			Parameters: objectSchema([]string{"add_slide_ids", "reason"}, map[string]any{
				"add_slide_ids": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string", "pattern": "^sli_[A-Za-z0-9_-]+$"}},
				"reason":        map[string]any{"type": "string"},
			}),
		})
		proposal := objectSchema([]string{"title", "content", "steps"}, map[string]any{
			"title": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"},
			"steps": map[string]any{"type": "array", "minItems": 1, "items": objectSchema([]string{"title"}, map[string]any{"title": map[string]any{"type": "string"}, "target_slide_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}})},
		})
		progress := objectSchema([]string{"updates"}, map[string]any{"updates": map[string]any{"type": "array", "minItems": 1, "items": objectSchema([]string{"step_id", "status"}, map[string]any{"step_id": map[string]any{"type": "string"}, "status": map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed", "failed"}}})}})
		out = append(out, ToolSchema{Name: "update_plan", Description: "Without a current plan, create a lightweight checklist using title/content/steps. Once a plan exists, use updates for step statuses only; its content is locked.", Parameters: map[string]any{"type": "object", "oneOf": []any{proposal, progress}}})
	}
	allowAsk := mode == model.ModeGrill || mode == model.ModePlan || (mode == model.ModeExecute && phase != PhaseCompletionCheck)
	if allowAsk && phase != PhaseWaitingInput && phase != PhaseCommitting && phase != PhaseTerminal {
		out = append(out, ToolSchema{
			Name: "ask_user", Description: "Ask one blocking group of atomic user questions and pause this same loop until the user answers. Each item is either single-choice with 1-3 options, optionally allow_custom=true, or fill-in with no options. Do not merge multiple choices into one free-text question.",
			Parameters: objectSchema([]string{"questions"}, map[string]any{
				"questions": map[string]any{"type": "array", "minItems": 1, "items": objectSchema([]string{"id", "title"}, map[string]any{
					"id":          map[string]any{"type": "string"},
					"title":       map[string]any{"type": "string"},
					"description": map[string]any{"type": "string"},
					"options": map[string]any{"type": "array", "maxItems": 3, "items": objectSchema([]string{"id", "label"}, map[string]any{
						"id": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"},
					})},
					"allow_custom": map[string]any{"type": "boolean"},
				})},
			}),
		})
	}
	if (mode == model.ModePlan && phase == PhasePlanning) ||
		(mode == model.ModeExecute && phase == PhaseExecuting) {
		reviewDescription := "Ask the semantic reviewer for a second pass on the current execution result or candidate final message. Returns checks[] only; the main agent decides the next ReAct step."
		candidateDescription := "Optional draft message intended for finish(message). Provide it when reviewing final delivery wording."
		focusValues := []string{"execution", "final", "all"}
		if mode == model.ModePlan {
			reviewDescription = "Ask the semantic reviewer for a second pass on the current plan proposal. Returns checks[] only; the main agent decides whether to revise or submit the plan."
			candidateDescription = "Optional draft plan text intended for create_plan or update_plan."
			focusValues = []string{"plan", "all"}
		}
		out = append(out, ToolSchema{
			Name: "review_completion", Description: reviewDescription,
			Parameters: objectSchema([]string{}, map[string]any{
				"candidate_message": map[string]any{
					"type":        "string",
					"description": candidateDescription,
				},
				"focus": map[string]any{
					"type": "string", "enum": focusValues,
					"description": "Review focus. Use all when unsure.",
				},
			}),
		})
	}
	if (phase == PhaseChat && (mode == model.ModeChat || mode == model.ModeGrill)) ||
		(phase == PhaseExecuting && mode == model.ModeExecute) {
		out = append(out, ToolSchema{
			Name: "finish", Description: "Submit the complete final user-facing response and optional next-input suggestions for the current chat, grill, or execute run. Ordinary assistant text is not a completion signal. Runtime checks the requested outcome and required evidence before completion.",
			Parameters: objectSchema([]string{"message"}, map[string]any{
				"message": map[string]any{"type": "string"},
				"suggested_next_inputs": map[string]any{
					"type": "array", "maxItems": 3,
					"items": map[string]any{"type": "string", "maxLength": 80},
				},
			}),
		})
	}
	return out
}

func (state *RunState) publicTextContext() model.PublicTextContext {
	display := contextengine.PublicTextContext(state.pack)
	display.HiddenValues = append(display.HiddenValues, state.projectDir, state.runID, state.loopID)
	display.SourceText += "\n" + contextengine.PublicSourceText(state.messages)
	return display
}

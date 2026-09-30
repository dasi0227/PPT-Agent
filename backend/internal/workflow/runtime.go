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
	"github.com/dasi0227/PPT-Agent/backend/internal/decision"
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
	SeenVersions           map[string]string         `json:"seen_versions,omitempty"`
	ContentAssessmentIDs   []string                  `json:"content_assessment_ids,omitempty"`
	PendingContent         *PendingContentBatch      `json:"pending_content,omitempty"`
	ToolDecision           *RunToolDecision          `json:"tool_decision,omitempty"`
	DecisionIdentity       string                    `json:"decision_identity"`
	ContinuationAllowed    bool                      `json:"continuation_allowed"`
	BudgetBaseTurns        int                       `json:"budget_base_turns"`
	BudgetBaseDurationMS   int64                     `json:"budget_base_duration_ms"`
	PendingPlanCall        *PendingPlan              `json:"pending_plan_call,omitempty"`
	PendingPlanPublication *model.PlanApprovalAnswer `json:"pending_plan_publication,omitempty"`
	PendingQuestion        *PendingQuestion          `json:"pending_question,omitempty"`
	OwnerInstanceID        string                    `json:"owner_instance_id"`
	ExecutionRevision      int64                     `json:"execution_revision"`
	CheckpointRevision     int64                     `json:"checkpoint_revision"`
	ModelRoute             *llm.RouteState           `json:"model_route,omitempty"`
	RunID                  string                    `json:"run_id"`
	LoopID                 string                    `json:"loop_id"`
	Boundary               string                    `json:"boundary,omitempty"`
	Phase                  RunPhase                  `json:"phase"`
	Mode                   model.RunMode             `json:"mode"`
	ResumePhase            RunPhase                  `json:"resume_phase,omitempty"`
	Plan                   *Plan                     `json:"plan,omitempty"`
	Requirements           *RequirementLedger        `json:"requirements,omitempty"`
	Work                   *WorkLedger               `json:"work_ledger,omitempty"`
	Changes                ChangeSet                 `json:"changes"`
	Evidence               []Evidence                `json:"evidence"`
	ActiveSkills           []model.RunSkill          `json:"active_skills,omitempty"`
	ActiveComponents       []model.RunComponent      `json:"active_components,omitempty"`
	Turns                  int                       `json:"turns"`
	ToolCalls              int                       `json:"tool_calls"`
	ActiveDurationMS       int64                     `json:"active_duration_ms"`
	WaitingDurationMS      int64                     `json:"waiting_duration_ms"`
	PendingCommand         *PendingCommandApproval   `json:"pending_command,omitempty"`
	PendingScopeExpansion  *PendingScopeExpansion    `json:"pending_scope_expansion,omitempty"`
	Scope                  model.RunScope            `json:"scope"`
	DOMSelections          []model.DOMSelection      `json:"dom_selections,omitempty"`
	CompletionFailures     int                       `json:"completion_failures"`
	ReadLoop               ReadLoopState             `json:"read_loop"`
	PendingReview          *PendingReview            `json:"pending_review,omitempty"`
	ReviewInstructions     []ReviewInstruction       `json:"review_instructions"`
	ReviewBaselineError    string                    `json:"review_baseline_error,omitempty"`
	ReadImages             []RunReadImage            `json:"read_images,omitempty"`
	CreatedAt              int64                     `json:"created_at"`
}

type PendingReview struct {
	Result        *ToolResult  `json:"result,omitempty"`
	Call          llm.ToolCall `json:"call"`
	AssistantText string       `json:"assistant_text"`
}

type PendingPlan struct {
	Call          llm.ToolCall  `json:"call"`
	AssistantText string        `json:"assistant_text"`
	OriginMode    model.RunMode `json:"origin_mode"`
}

type PendingQuestion struct {
	Call          llm.ToolCall               `json:"call"`
	AssistantText string                     `json:"assistant_text"`
	Question      model.QuestionAskedPayload `json:"question"`
}

type PendingScopeExpansion struct {
	Call          *llm.ToolCall                        `json:"call"`
	AssistantText string                               `json:"assistant_text"`
	Applied       bool                                 `json:"applied,omitempty"`
	Request       model.ScopeExpansionRequestedPayload `json:"request"`
	ResumePhase   RunPhase                             `json:"resume_phase"`
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
	OutlineExists         bool
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
	ReadImages            []RunReadImage
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

// Main authoring turns need room for complete HTML and tool-call arguments.
const authoringMaxOutputTokens = 16384

func (a CognitiveAgent) Next(ctx context.Context, req AgentRequest) (AgentResponse, error) {
	if a.Provider == nil {
		return AgentResponse{}, errors.New("LLM provider is required for ReAct execution")
	}
	req = prepareAgentRequest(req)
	messages := providerMessages(req)
	tools := make([]llm.ToolSchema, 0, len(req.Tools))
	for _, schema := range req.Tools {
		tools = append(tools, llm.ToolSchema{Name: schema.Name, Description: schema.Description, Parameters: schema.Parameters, OutputSchema: schema.OutputSchema})
	}
	response, err := a.Provider.Generate(ctx, llm.GenerateRequest{
		Messages: messages, Tools: tools, ImageResolver: req.ImageResolver, PauseOnFallback: true,
		MaxOutputTokens: authoringMaxOutputTokens,
		Continuation:    req.Continuation, OnRetry: req.OnProviderRetry, OnContinuationReset: req.OnContinuationReset,
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
		return "Ordinary assistant text cannot submit a plan. Use create_plan for a new complete proposal or a complete unapproved revision. After refusal, honor the decision and finish_task using the currently disclosed tools."
	}
	return "Ordinary assistant text is not a completion signal. If your reply already answers the current request, submit that reply directly through finish_task(message=...) without expanding it, adding a project recap, or inventing follow-up work. Greetings and identity questions can finish immediately. If the requested task is not complete, call one of the currently disclosed tools to continue."
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
	Reviewer              TaskReviewer
	ResumeCheckpoint      *RuntimeCheckpoint
	RefreshContext        func(context.Context, model.RunMode) (contextengine.ContextPack, error)
	PersistMode           func(context.Context, model.RunMode) error
	DomainToolsForContext func(contextengine.ContextPack) DomainToolProvider
	CommitPlanApproval    func(context.Context, model.RunMode, contextengine.ContextPack, RuntimeCheckpoint) error
	CommitScopeExpansion  func(context.Context, model.RunScope, RuntimeCheckpoint) error
	Logger                *zap.Logger
	Transcript            TranscriptStore
	Calibration           TokenCalibration
	RecordCompaction      func(context.Context, string, contextcompact.Result, contextengine.WindowSnapshot, contextengine.WindowSnapshot, time.Duration) (model.ContextCompaction, error)
}

type Runtime struct {
	Decisions           decision.Snapshot
	Agent               ReActAgent
	Gate                CompletionGate
	Compactor           ContextCompactor
	Embedder            EmbeddingProvider
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
		Embedder: HashEmbeddingProvider{},
		now:      time.Now,
	}
	if cognitive, ok := agent.(CognitiveAgent); ok && cognitive.Provider != nil {
		runtime.ContextWindowTokens = cognitive.Provider.Capabilities().ContextWindowTokens
	}
	return runtime
}

type RunState struct {
	seenVersions            map[string]string
	contentAssessmentIDs    []string
	pendingContent          *PendingContentBatch
	toolDecision            *RunToolDecision
	decisionIdentity        string
	continuationAllowed     bool
	budgetBaseTurns         int
	budgetBaseDurationMS    int64
	pendingPlanCall         *PendingPlan
	pendingPlanPublication  *model.PlanApprovalAnswer
	persistenceCtx          context.Context
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
	readLoop                ReadLoopState
	pendingReview           *PendingReview
	reviewInstructions      []ReviewInstruction
	reviewBaselineError     string
	readImages              []RunReadImage
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
	lastCheckpointTurn      int
	lastCheckpointToolCalls int
	lastCheckpointAt        time.Time
	tools                   *ToolRegistry
	pendingQuestion         *PendingQuestion
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
		decisionIdentity: r.Decisions.Identity,
		persistenceCtx:   ctx,
		projectDir:       input.ProjectDir, runID: input.RunID, loopID: "loop_" + uuid.NewString(),
		scope: input.Context.Command.Scope, mode: input.Context.Command.Mode, pack: input.Context, ledger: NewEvidenceLedger(),
		issues: []Issue{}, messages: []llm.Message{}, activeSince: now, activeRunning: true, budget: input.Budget,
		trace: input.Trace, lifecycle: input.Lifecycle, requirements: NewRequirementLedger(input.Context.Command),
		reviewInstructions: []ReviewInstruction{{Text: input.Context.Command.Instruction, Attachments: input.Context.Command.Attachments, DOMSelections: input.Context.Command.DOMSelections}},
		work:               NewWorkLedger(),
		committedChanges:   EmptyChangeSet(),
		activeSkills:       &ActiveSkillSet{Skills: append([]model.RunSkill{}, input.Context.Command.Skills...)},
		calibrationFactor:  1,
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
				if err := r.saveCheckpoint(switchCtx, input, state, checkpointBoundary("model_fallback")); err != nil {
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
		_ = r.persistTranscript(context.WithoutCancel(ctx), input, state)
	}()
	defer func() {
		if state.tx != nil && !state.committed {
			state.tx.Discard()
		}
	}()
	if input.ResumeCheckpoint != nil && input.ResumeCheckpoint.RunID == input.RunID {
		if err := input.ResumeCheckpoint.Scope.Validate(); err != nil {
			return r.fail(input, state, CodeAgentFailed, fmt.Errorf("invalid checkpoint scope: %w", err))
		}
		state.contentAssessmentIDs = append([]string(nil), input.ResumeCheckpoint.ContentAssessmentIDs...)
		state.pendingContent = input.ResumeCheckpoint.PendingContent
		state.toolDecision = input.ResumeCheckpoint.ToolDecision
		state.decisionIdentity = input.ResumeCheckpoint.DecisionIdentity
		state.scope = input.ResumeCheckpoint.Scope
		state.pack.Command.Scope = state.scope
		state.pack.Target.SlideIDs = append([]string{}, state.scope.SlideIDs...)
		recordTrace(input.Trace, state.runID, "provider.continuation_reset", map[string]any{"reason": "checkpoint_resume"})
		state.pack.Command.DOMSelections = append([]model.DOMSelection{}, input.ResumeCheckpoint.DOMSelections...)
		state.loopID = input.ResumeCheckpoint.LoopID
		state.phase = input.ResumeCheckpoint.ResumePhase
		if state.phase == "" {
			state.phase = input.ResumeCheckpoint.Phase
		}
		state.resumePhase = input.ResumeCheckpoint.ResumePhase
		state.plan = input.ResumeCheckpoint.Plan
		state.seenVersions = input.ResumeCheckpoint.SeenVersions
		state.pendingPlanCall = input.ResumeCheckpoint.PendingPlanCall
		state.pendingPlanPublication = input.ResumeCheckpoint.PendingPlanPublication
		// A previous question can leave ResumePhase at planning. The pending
		// approval itself remains authoritative until its answer is consumed.
		if input.ResumeCheckpoint.Phase == PhaseWaitingInput && state.plan != nil && state.plan.Status == PlanAwaitingApproval {
			state.phase = PhaseWaitingInput
		}
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
		state.budgetBaseTurns = input.ResumeCheckpoint.BudgetBaseTurns
		state.budgetBaseDurationMS = input.ResumeCheckpoint.BudgetBaseDurationMS
		state.turns = input.ResumeCheckpoint.Turns
		state.toolCalls = input.ResumeCheckpoint.ToolCalls
		if input.ResumeCheckpoint.ActiveDurationMS > 0 {
			state.activeElapsed = time.Duration(input.ResumeCheckpoint.ActiveDurationMS) * time.Millisecond
		}
		if input.ResumeCheckpoint.WaitingDurationMS > 0 {
			state.waitingElapsed = time.Duration(input.ResumeCheckpoint.WaitingDurationMS) * time.Millisecond
		}
		state.gateCount = input.ResumeCheckpoint.CompletionFailures
		state.readLoop = input.ResumeCheckpoint.ReadLoop
		state.readLoop.Seen = append([]string(nil), state.readLoop.Seen...)
		state.readImages = append([]RunReadImage(nil), input.ResumeCheckpoint.ReadImages...)
		state.reviewInstructions = append([]ReviewInstruction(nil), input.ResumeCheckpoint.ReviewInstructions...)
		state.reviewBaselineError = input.ResumeCheckpoint.ReviewBaselineError
		state.pendingReview = input.ResumeCheckpoint.PendingReview
		state.pendingQuestion = input.ResumeCheckpoint.PendingQuestion
		state.pendingCommand = input.ResumeCheckpoint.PendingCommand
		state.pendingScopeExpansion = input.ResumeCheckpoint.PendingScopeExpansion
		state.activeSkills.Components = append([]model.RunComponent{}, input.ResumeCheckpoint.ActiveComponents...)
		if input.ResumeCheckpoint.ActiveSkills != nil {
			state.activeSkills.Skills = append([]model.RunSkill{}, input.ResumeCheckpoint.ActiveSkills...)
		}
		for _, evidence := range input.ResumeCheckpoint.Evidence {
			fresh := false
			if evidence.Render != nil {
				proof, err := currentRenderProof(state.pack, input.ProjectDir, nil, evidence.Render.SlideID, evidence.Render.ArtifactHash)
				fresh = err == nil && proof == *evidence.Render
			}
			evidence.Fresh = evidence.Fresh && fresh
			state.ledger.restore(evidence)
		}
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
	if state.reviewBaselineError == "" {
		if err := ensureReviewBaseline(ctx, input.ProjectDir, state.runID, input.ResumeCheckpoint != nil); err != nil {
			state.reviewBaselineError = err.Error()
		}
	}
	if err := r.initializeContextIndex(ctx, input, state); err != nil {
		return r.fail(input, state, CodeAgentFailed, err)
	}
	r.changePhase(input.Emitter, state, initialPhase, "run mode initialized")
	if err := r.saveCheckpoint(ctx, input, state, checkpointRuntimeInitialized); err != nil {
		return r.fail(input, state, CodeAgentFailed, err)
	}
	if state.mode == model.ModeExecute {
		// RunSession is the transaction for the currently executing tool call.
		// Every successful mutation is committed before the next call starts.
		var session *RunSession
		var err error
		session, err = NewRunSession(input.ProjectDir, input.RunID)
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
	if state.toolDecision != nil {
		state.tools.Limit(state.toolDecision.AllowedTools)
	}
	if err := r.resumeContentBatch(ctx, input, state); err != nil {
		return r.fail(input, state, CodeAgentFailed, err)
	}
	if state.pendingReview != nil {
		pending := state.pendingReview
		result := failedToolResult(CodeReviewFailed, "Artifact review was interrupted; no assessment was submitted. Call review_task again if needed.", false)
		if pending.Result != nil {
			result = *pending.Result
		}
		projector := ToolPublicProjector{ProjectDir: input.ProjectDir, TextContext: state.publicTextContext()}
		if input.Emitter != nil {
			if event, ok := projector.Completed(state.runID, pending.Call.ID, pending.Call.Name, pending.Call.Args, result); ok {
				input.Emitter.Emit(model.EventToolCompleted, event)
			}
		}
		observed := false
		for _, message := range state.messages {
			if message.Role == llm.RoleTool && message.ToolCallID == pending.Call.ID {
				observed = true
				break
			}
		}
		if !observed {
			r.appendControlObservation(state, pending.Call, pending.AssistantText, result)
		}
		state.pendingReview = nil
		if err := r.saveCheckpoint(ctx, input, state, checkpointAfterReview); err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
	}
	if state.pendingPlanPublication != nil {
		if err := r.publishPlanApproval(ctx, input, state); err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
	}
	if state.pendingQuestion != nil {
		pending := state.pendingQuestion
		answered := false
		for _, message := range state.messages {
			if message.Role == llm.RoleTool && message.ToolCallID == pending.Call.ID {
				answered = true
				break
			}
		}
		if answered {
			state.pendingQuestion = nil
		} else if outcome, terminal := r.executeControl(ctx, input, state, pending.Call, pending.AssistantText); terminal {
			return outcome
		}
	}
	if state.pendingCommand != nil {
		// Explicit stop may have already recorded a canceled tool result. Let
		// the model request a fresh call/authorization instead of replaying it.
		for _, message := range state.messages {
			if message.Role == llm.RoleTool && message.ToolCallID == state.pendingCommand.CallID {
				state.pendingCommand = nil
				break
			}
		}
	}
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
	if state.pendingPlanCall != nil && state.phase == PhaseWaitingInput && state.plan != nil && state.plan.Status == PlanAwaitingApproval {
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
		if err := r.appendSteering(ctx, input, state, input.Steering); err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		if err := r.persistTranscript(context.WithoutCancel(ctx), input, state); err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		r.invalidateContentPrechecks(input, state)
		if err := r.retrieveTurnContext(ctx, input, state); err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		if err := r.decideTools(ctx, input, state); err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		if err := r.checkBudget(ctx, state); err != nil {
			code := CodeBudgetExceeded
			if errors.Is(err, context.Canceled) {
				code = CodeCanceled
			}
			return r.fail(input, state, code, err)
		}
		schemas := state.tools.Disclose(state.phase, state.mode, state.scope)
		schemas = append(schemas, controlSchemas(state.phase, state.mode, state.plan)...)
		sort.Slice(schemas, func(i, j int) bool { return schemas[i].Name < schemas[j].Name })
		images := latestRenderedImages(state.pack, input.ProjectDir, state.tx)
		if hashCheckpointValue(images) != hashCheckpointValue(state.renderedImages) {
			state.continuation = nil
		}
		state.renderedImages = images
		state.readLoop.syncProgress(state.readProgressHash())
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
		before := state.messages
		request := prepareAgentRequest(agentRequestForState(input, state, schemas))
		state.messages = request.Messages
		state.rememberPreparedResourceVersions(before)
		request.OnProviderRetry = func(attempt int) { r.emitProgress(input.Emitter, state, model.ActivityRunRetrying) }
		request.OnContinuationReset = func(reason string) {
			recordTrace(input.Trace, state.runID, "provider.continuation_reset", map[string]any{"reason": reason})
		}
		if err := r.persistTranscript(context.WithoutCancel(ctx), input, state); err != nil {
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

		if state.mode == model.ModeExecute && contentBatchMayWrite(state.tools, calls) {
			state.pendingContent = &PendingContentBatch{Calls: calls, AssistantText: response.Text, Before: captureContentBefore(input.ProjectDir)}
			if err := r.saveCheckpoint(ctx, input, state, checkpointBoundary("tool_batch_pending")); err != nil {
				return r.fail(input, state, CodeAgentFailed, err)
			}
		}
		results := r.executeToolBatch(ctx, input, state, state.tools, schemasByName(schemas), calls)
		imagesAdded := state.rememberReadImages(calls, results)
		beforeResults := len(state.messages)
		state.messages = appendBatchObservations(state.messages, calls, response.Text, results)
		state.rememberResourceVersions(beforeResults)
		state.pendingContent = nil
		if err := r.saveCheckpoint(context.WithoutCancel(ctx), input, state, checkpointBoundary("tool_batch_observed")); err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		state.messages = withoutReadImageParts(state.messages, state.readImages)
		if imagesAdded {
			state.continuation = nil
			if err := r.saveCheckpoint(context.WithoutCancel(ctx), input, state, checkpointAfterImageRead); err != nil {
				return r.fail(input, state, CodeAgentFailed, err)
			}
		}
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
		state.readLoop.syncProgress(state.readProgressHash())
		if state.readLoop.observe(state.tools, calls, results) {
			recordTrace(input.Trace, state.runID, "runtime.read_loop_detected", map[string]any{
				"turn": state.turns, "repeat_rounds": state.readLoop.RepeatRounds,
				"max_repeat_rounds": maxRepeatedReadRounds,
			})
			state.messages = append(state.messages, llm.Message{Role: llm.RoleUser,
				Content:  llm.TextContent("Runtime stopped repeated reads of unchanged results. If the user continues, use the retained Run images and observations to make progress or finish; do not repeat the same inspection cycle."),
				Metadata: runtimeControlMetadata("guidance", state.runID)})
			return r.fail(input, state, CodeReadLoop, errors.New("successful reads repeated without new information or task progress"))
		}
	}
}

func (r *Runtime) initializeContextIndex(_ context.Context, _ RuntimeInput, state *RunState) error {
	state.contextIndex = NewContextIndexFromPack(state.pack, state.scope, r.Embedder)
	state.contextIndexRef = state.contextIndex.ID
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
	changedHTML := map[string]int{}
	var changedMu sync.Mutex
	started := make([]bool, len(calls))
	replayed := make([]bool, len(calls))
	emitTerminal := make([]bool, len(calls))
	committedReceipts := make([]bool, len(calls))
	decisions := make([]*ToolDecision, len(calls))
	approvals := make([]string, len(calls))
	workTargets := make([][]string, len(calls))
	projector := ToolPublicProjector{ProjectDir: input.ProjectDir, TextContext: state.publicTextContext()}
	planStepID := currentPlanStepID(state.plan)
	var lifecycleMu sync.Mutex
	state.toolCalls += len(calls)
	state.activeTools = len(calls)
	confirmCount := 0
	for index, call := range calls {
		desc, exists := registry.Descriptor(call.Name)
		if !exists || !disclosed[call.Name] {
			continue
		}
		if schema, available := scopeToolSchema(desc.Tool.Schema(), state.scope, desc.ReadOnly); available {
			args, converted, err := prepareToolArguments(schema, call.Args)
			if err != nil {
				results[index] = argumentFailure(err)
				emitTerminal[index] = true
				continue
			}
			call.Args, calls[index].Args = args, args
			traceArgumentConversion(input.Trace, state.runID, call, converted)
		}
		decision := &ToolDecision{Outcome: "allow", Mutates: !desc.ReadOnly}
		if preflight, ok := desc.Tool.(PreflightTool); ok {
			value := preflight.Preflight(ctx, DomainToolInput{
				Args: call.Args, CallID: call.ID, Context: state.pack,
				ProjectDir: input.ProjectDir, RunID: input.RunID, Session: state.tx,
				Scope: state.scope, Phase: state.phase, Mode: state.mode, ActiveSkills: state.activeSkills, Messages: state.messages, SeenVersions: state.seenVersions,
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
			confirmCount++
		}
	}
	if confirmCount > 0 && len(calls) != 1 {
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
	}
	if confirmCount == 1 && len(calls) == 1 {
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
			if err := r.saveCheckpoint(ctx, input, state, checkpointBeforeCommandPermission); err != nil {
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
					_ = r.saveCheckpoint(ctx, input, state, checkpointAfterCommandPermission)
					if answer.InteractionID != interactionID || answer.CallID != call.ID ||
						answer.CommandHash != decision.CommandHash ||
						(answer.Decision != "allow_once" && answer.Decision != "deny") {
						approvals[0] = "invalid"
						results[0] = failedToolResult(CodeInvalidControlCall, "command permission answer does not match the pending command", false)
						emitTerminal[0] = true
					} else if answer.Decision == "deny" {
						approvals[0] = "deny"
						denied := *decision
						denied.ReasonCode = "COMMAND_PERMISSION_DENIED"
						denied.PublicReason = "command permission denied"
						results[0] = blockedCommandResult(denied)
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
			Mode: state.mode, Decision: decisions[index], ActiveSkills: state.activeSkills, Messages: state.messages, SeenVersions: state.seenVersions,
		})
		if ctx.Err() != nil && !(call.Name == "git_commit" && results[index].OK) {
			results[index] = failedToolResult(CodeCanceled, "run canceled", false)
		}
	}
	// A response keeps its own authoring view across sequential tool commits.
	generationPack := state.pack
	commitPrepared := func(index int) {
		if !started[index] || state.tx == nil {
			return
		}
		call := calls[index]
		// Git has its own durable intent and receipt. It commits the already
		// persisted source tree, without creating a second artifact transaction.
		if call.Name == "git_commit" {
			return
		}
		desc, exists := registry.Descriptor(call.Name)
		if !exists {
			return
		}
		mutates := !desc.ReadOnly
		if decisions[index] != nil {
			mutates = mutates || decisions[index].Mutates
		}
		if !results[index].OK {
			if mutates {
				state.tx.RollbackOperation()
			}
			return
		}
		if !mutates {
			return
		}
		candidate := state.tx.generationContext(generationPack)
		candidate.Command.Scope = state.scope
		inputs, err := state.tx.StageGenerationInputs(candidate)
		resultJSON := ""
		if err == nil {
			if call.Name == "run_command" {
				results[index].Evidence = append(results[index].Evidence, commandDomainEvidence(state.tx)...)
			}
			resultJSON, err = marshalPersistedToolResult(results[index])
		}
		if err == nil {
			var committed ChangeSet
			committed, err = state.tx.CommitOperation(ctx, call.ID, resultJSON, input.CommitMetadata)
			if err == nil {
				generationPack = candidate
				committedReceipts[index] = true
				changedMu.Lock()
				for _, change := range committed.All() {
					target := resourceForArtifact(change.Artifact)
					id := changedHTMLTarget(ChangedTarget{Type: target.Type, Part: target.Part, SlideID: target.SlideID, Path: target.Path})
					if id != "" && change.BeforeHash != change.AfterHash {
						changedHTML[id] = index
						if state.pendingContent != nil {
							if state.pendingContent.ExpectedHTML == nil {
								state.pendingContent.ExpectedHTML = map[string]string{}
							}
							state.pendingContent.ExpectedHTML[id] = change.AfterHash
						}
					}
				}
				changedMu.Unlock()
				state.committedChanges = mergeChangeSets(state.committedChanges, committed)
				refreshTargets := append([]ChangedTarget{}, results[index].ChangedTargets...)
				contextengine.AcceptGenerationInputs(&state.pack, inputs)
				for _, change := range committed.All() {
					target := resourceForArtifact(change.Artifact)
					refreshTargets = append(refreshTargets, ChangedTarget{Type: target.Type, SlideID: target.SlideID, Part: target.Part})
				}
				refreshRuntimePack(input.ProjectDir, state, refreshTargets)
				if input.DomainToolsForContext != nil {
					if next, registryErr := buildDomainToolRegistry(input, state.pack); registryErr == nil {
						if state.toolDecision != nil {
							next.Limit(state.toolDecision.AllowedTools)
						}
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
		if started[index] && !committedReceipts[index] {
			r.persistToolCall(context.WithoutCancel(ctx), input, state, call, results[index])
		}
	}
	for index, call := range calls {
		result := results[index]
		if (started[index] || emitTerminal[index]) && input.Emitter != nil {
			event, ok := projector.Completed(state.runID, call.ID, call.Name, call.Args, result)
			if !started[index] {
				event, ok = projector.Blocked(state.runID, call.ID, call.Name, call.Args, result)
			}
			if ok {
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
		for _, target := range result.InvalidatedTargets {
			state.ledger.InvalidateRender(target)
		}
		for _, target := range uniqueTargets(changedResources(result.ChangedTargets)) {
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
				_ = r.saveCheckpoint(context.WithoutCancel(ctx), input, state, checkpointAfterWrite)
			}
			if call.Name == "render_slide" {
				_ = r.saveCheckpoint(context.WithoutCancel(ctx), input, state, checkpointAfterRender)
			}
			if call.Name == "load_skill" {
				_ = r.saveCheckpoint(context.WithoutCancel(ctx), input, state, checkpointPeriodic)
			}
		}
	}
	r.invalidateContentPrechecks(input, state)
	r.contentPrecheck(ctx, input, state, calls, results, changedHTML)
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
	if result.OK || (call.Name == "render_slide" && len(result.ObservationParts) > 0) {
		return result
	}
	agentErr := toolResultError(result)
	agentErr.CallID = call.ID
	target, targetErr := parseResource(call.Args)
	hasTarget := targetErr == nil
	if isResourceEditTool(call.Name) {
		target = resourceForTool(call.Name, stringValue(call.Args["slide_id"]))
		hasTarget = true
	}

	if call.Name == "render_slide" {
		hasTarget = false
	}
	if hasTarget {
		agentErr.Resource = &model.ErrorResource{Type: target.Type, SlideID: target.SlideID, Part: target.Part}
	}
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
	raw, _ := json.Marshal(contextengine.ModelValue(agentErr.ModelObservation()))
	result.Retryable = agentErr.ShouldAutoRetry()
	result.Observation = string(raw)
	result.ObservationParts = nil
	return result
}

type persistedToolResult struct {
	Result              ToolResult           `json:"result"`
	Evidence            []Evidence           `json:"evidence"`
	RenderProofs        []RenderProof        `json:"render_proofs,omitempty"`
	InvalidatedTargets  []Resource           `json:"invalidated_targets"`
	ObservationParts    []llm.ContentPart    `json:"observation_parts"`
	ObservationMetadata *llm.MessageMetadata `json:"observation_metadata,omitempty"`
	Observation         string               `json:"observation,omitempty"`
}

func marshalPersistedToolResult(result ToolResult) (string, error) {
	raw, err := json.Marshal(persistedToolResult{
		Result: result, Evidence: result.Evidence, RenderProofs: renderProofs(result),
		InvalidatedTargets: result.InvalidatedTargets, ObservationParts: result.ObservationParts, ObservationMetadata: result.ObservationMetadata, Observation: result.Observation,
	})
	return string(raw), err
}

func renderProofs(result ToolResult) []RenderProof {
	proofs := make([]RenderProof, 0)
	seen := map[string]bool{}
	for _, evidence := range result.Evidence {
		if evidence.Render == nil || seen[evidence.Render.SlideID] {
			continue
		}
		seen[evidence.Render.SlideID] = true
		proofs = append(proofs, *evidence.Render)
	}
	return proofs
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
				if raw, err := os.ReadFile(filepath.Join(projectDir, ".manifest.json")); err == nil && json.Unmarshal(raw, &value) == nil {
					state.pack.PresentationManifest.Manifest = value
				}
			case "outline":
				var value spec.Outline
				if raw, err := os.ReadFile(filepath.Join(projectDir, ".outline.json")); err == nil && json.Unmarshal(raw, &value) == nil {
					state.pack.Outline.Outline = value
				}
			case "design":
				var value spec.Design
				if raw, err := os.ReadFile(filepath.Join(projectDir, ".design.json")); err == nil && json.Unmarshal(raw, &value) == nil {
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
		state.pack.Target.SlideIDs = append([]string{}, state.scope.SlideIDs...)
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
	for proofIndex := range persisted.RenderProofs {
		proof := persisted.RenderProofs[proofIndex]
		for evidenceIndex := range persisted.Result.Evidence {
			if persisted.Result.Evidence[evidenceIndex].Target.SlideID == proof.SlideID {
				persisted.Result.Evidence[evidenceIndex].Render = &proof
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
		if call.Name != "read_resource" {
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
	answer model.PlanApprovalAnswer,
) (*RunState, error) {
	if state == nil || state.plan == nil || (state.mode != model.ModePlan && state.mode != model.ModeExecute) || state.phase != PhaseWaitingInput || state.plan.Status != PlanAwaitingApproval {
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
	candidate.resumePhase = PhaseExecuting
	candidate.pendingPlanPublication = &answer
	candidate.work = cloneWorkLedger(state.work)
	if err := candidate.work.SyncPlan(candidate.plan, candidate.scope); err != nil {
		return nil, err
	}
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
	candidate.toolDecision = nil
	decisionInput := input
	decisionInput.Checkpoint = nil
	if err := r.decideTools(ctx, decisionInput, &candidate); err != nil {
		return nil, err
	}
	checkpoint := r.checkpointForBoundary(&candidate, checkpointPlanUpdated)
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
				_ = input.PersistMode(ctx, state.mode)
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
		err := errors.New("plan approval prompter is required")
		r.failPendingPlanTool(state, err)
		return r.fail(input, state, CodeAgentFailed, err), true
	}
	plan := *state.plan
	interactionID := plan.ApprovalID
	if interactionID == "" {
		return r.fail(input, state, CodeAgentFailed, errors.New("plan approval identity is missing")), true
	}
	request := model.PlanApprovalRequestedPayload{PublicEventBase: publicBase(state.runID), InteractionID: interactionID, Plan: publicPlan(plan, state.publicTextContext())}
	for attempts := 0; ; attempts++ {
		state.pauseActiveClock(r.clockNow())
		answer, err := prompter.AskPlanApproval(ctx, request)
		if err != nil {
			r.failPendingPlanTool(state, err)
			code := CodeAgentFailed
			if errors.Is(err, context.Canceled) {
				code = CodeCanceled
			}
			return r.fail(input, state, code, err), true
		}
		state.resumeActiveClock(r.clockNow())
		if answer.InteractionID != interactionID || answer.PlanID != plan.ID ||
			(answer.Decision != "approve" && answer.Decision != "revise" && answer.Decision != "refuse") {
			return r.fail(input, state, CodeInvalidControlCall, errors.New("invalid plan approval answer")), true
		}
		switch answer.Decision {
		case "refuse", "revise":
			if answer.Decision == "refuse" {
				state.plan.Status = PlanCanceled
			}
			state.plan.UpdatedAt = time.Now().Unix()
			if input.Emitter != nil {
				input.Emitter.Emit(model.EventPlanApprovalAnswered, model.PlanApprovalAnsweredPayload{PublicEventBase: publicBase(state.runID), InteractionID: interactionID, PlanID: plan.ID, Decision: answer.Decision, Feedback: answer.Feedback})
				input.Emitter.Emit(model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: publicBase(state.runID), Plan: publicPlan(*state.plan, state.publicTextContext())})
			}
			phase := PhasePlanning
			r.changePhase(input.Emitter, state, phase, "plan approval answered")
			state.resumePhase = phase
			r.completePlanTool(state, answer)
			if err := r.saveCheckpoint(ctx, input, state, checkpointAfterUserAnswer); err != nil {
				return r.fail(input, state, CodeAgentFailed, err), true
			}
			if resumer, ok := input.Prompter.(PlanApprovalResumer); ok {
				resumer.ResumeAfterPlanApproval(ctx)
			}
			return StructuredOutcome{}, false
		case "approve":
			candidate, transitionErr := r.prepareAndCommitPlanApproval(ctx, input, state, answer)
			if transitionErr != nil {
				recordTrace(state.trace, state.runID, "plan.approval_transition_failed", map[string]any{
					"loop_id": state.loopID, "plan_id": plan.ID, "error": transitionErr.Error(),
				})
				if attempts >= 2 {
					r.failPendingPlanTool(state, transitionErr)
					return r.fail(input, state, CodeAgentFailed, transitionErr), true
				}
				continue
			}
			*state = *candidate
			if err := r.publishPlanApproval(ctx, input, state); err != nil {
				return r.fail(input, state, CodeAgentFailed, err), true
			}
			return StructuredOutcome{}, false
		}
	}
}

// Approval may commit before its original tool result is saved.
// This checkpoint marker completes that boundary without requesting approval again.
func (r *Runtime) publishPlanApproval(ctx context.Context, input RuntimeInput, state *RunState) error {
	answer := state.pendingPlanPublication
	if answer == nil || state.plan == nil {
		return nil
	}
	originMode := state.mode
	if state.pendingPlanCall != nil {
		originMode = state.pendingPlanCall.OriginMode
	}
	r.completePlanTool(state, *answer)
	state.phase, state.resumePhase = PhaseExecuting, PhaseExecuting
	if resumer, ok := input.Prompter.(PlanApprovalResumer); ok {
		resumer.ResumeAfterPlanApproval(ctx)
	}
	if state.lifecycle != nil {
		state.lifecycle.PhaseChanged(PhaseExecuting)
	}
	if input.Emitter != nil {
		input.Emitter.Emit(model.EventPlanApprovalAnswered, model.PlanApprovalAnsweredPayload{PublicEventBase: publicBase(state.runID), InteractionID: answer.InteractionID, PlanID: answer.PlanID, Decision: answer.Decision, Feedback: answer.Feedback})
		input.Emitter.Emit(model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: publicBase(state.runID), Plan: publicPlan(*state.plan, state.publicTextContext()), InteractionID: answer.InteractionID})
		if originMode != model.ModeExecute {
			input.Emitter.Emit(model.EventRunModeChanged, model.RunModeChangedPayload{PublicEventBase: publicBase(state.runID), PreviousMode: originMode, Mode: model.ModeExecute})
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	state.pendingPlanPublication = nil
	return r.saveCheckpoint(ctx, input, state, checkpointPlanUpdated)
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
		Scope: state.scope, Phase: resumePhase, Mode: state.mode, ActiveSkills: state.activeSkills, Messages: state.messages, SeenVersions: state.seenVersions,
	})
	if decision.CommandHash != pending.CommandHash || decision.PreimageHash != pending.PreimageHash ||
		decision.Outcome != "confirm" {
		return r.fail(input, state, commandexec.CodeInvariantViolation, errors.New("pending command no longer matches its approved preflight")), true
	}
	call := llm.ToolCall{ID: pending.CallID, Name: "run_command", Args: pending.Args}
	schemas := state.tools.Disclose(state.phase, state.mode, state.scope)

	results := r.executeToolBatch(ctx, input, state, state.tools, schemasByName(schemas), []llm.ToolCall{call})
	state.messages = appendBatchObservations(state.messages, []llm.ToolCall{call}, "", results)
	state.pendingContent = nil
	if err := r.saveCheckpoint(ctx, input, state, checkpointBoundary("tool_batch_observed")); err != nil {
		return r.fail(input, state, CodeAgentFailed, err), true
	}
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
	var schema *ToolSchema
	for _, candidate := range controlSchemas(state.phase, state.mode, state.plan) {
		if candidate.Name == call.Name {
			schema = &candidate
			break
		}
	}
	if schema == nil {
		r.appendControlObservation(state, call, assistantText, failedToolResult(ErrToolNotDisclosed.Error(), "control tool is unavailable in the current plan state", false))
		return StructuredOutcome{}, false
	}
	args, converted, err := prepareToolArguments(*schema, call.Args)
	if err != nil {
		r.appendControlObservation(state, call, assistantText, argumentFailure(err))
		return StructuredOutcome{}, false
	}
	call.Args = args
	traceArgumentConversion(input.Trace, state.runID, call, converted)
	switch call.Name {
	case "create_plan":
		raw, _ := json.Marshal(call.Args)
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
		// Validate draft targets without adding unapproved work to the execution ledger.
		if err := NewWorkLedger().SyncPlan(&next, state.scope); err != nil {
			r.appendControlObservation(state, call, assistantText, failedToolResult(ErrPlanInvalid.Error(), err.Error(), true))
			return StructuredOutcome{}, false
		}
		state.plan = &next
		if input.Emitter != nil {
			input.Emitter.Emit(model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: publicBase(state.runID), Plan: publicPlan(next, state.publicTextContext())})
		}
		state.pendingPlanCall = &PendingPlan{Call: call, AssistantText: assistantText, OriginMode: state.mode}
		r.changePhase(input.Emitter, state, PhaseWaitingInput, "plan approval required")
		if err := r.saveCheckpoint(ctx, input, state, checkpointPlanUpdated); err != nil {
			r.failPendingPlanTool(state, err)
			return r.fail(input, state, CodeAgentFailed, err), true
		}
		return r.awaitPlanApproval(ctx, input, state)
	case "update_plan":
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
			candidate := *state
			candidate.plan = &next
			candidate.work = cloneWorkLedger(state.work)
			candidate.messages = append([]llm.Message(nil), state.messages...)
			if err := candidate.work.SyncPlan(candidate.plan, candidate.scope); err != nil {
				r.appendControlObservation(state, call, assistantText, failedToolResult(ErrPlanInvalid.Error(), err.Error(), true))
				return StructuredOutcome{}, false
			}
			r.appendControlObservation(&candidate, call, assistantText, SuccessfulToolResult("plan progress accepted"))
			if err := r.saveCheckpoint(ctx, input, &candidate, checkpointPlanUpdated); err != nil {
				r.appendControlObservation(state, call, assistantText, failedToolResult(CodeAgentFailed, err.Error(), false))
				return r.fail(input, state, CodeAgentFailed, err), true
			}
			*state = candidate
			if input.Emitter != nil {
				input.Emitter.Emit(model.EventPlanUpdated, model.PlanUpdatedPayload{PublicEventBase: publicBase(state.runID), Plan: publicPlan(next, state.publicTextContext())})
				completed := completedPlanSteps(previous, next)
				if len(completed) > 0 {
					input.Emitter.Emit(model.EventMessageMilestone, model.MessageMilestonePayload{PublicEventBase: publicBase(state.runID), MessageID: newMessageID(), Text: milestoneText(next.Title, completed, state.publicTextContext()), CompletedStepIDs: planStepIDs(completed)})
				}
			}
			return StructuredOutcome{}, false
		}
		r.appendControlObservation(state, call, assistantText, failedToolResult(ErrPlanInvalid.Error(), "update_plan requires an approved active plan", false))
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
		if state.pendingQuestion != nil {
			questionEvent = state.pendingQuestion.Question
		} else {
			state.pendingQuestion = &PendingQuestion{Call: call, AssistantText: assistantText, Question: questionEvent}
		}
		state.resumePhase = state.phase
		r.changePhase(input.Emitter, state, PhaseWaitingInput, "agent requested required user input")
		if err := r.saveCheckpoint(ctx, input, state, checkpointBeforeAskUser); err != nil {
			return r.fail(input, state, CodeAgentFailed, err), true
		}
		state.pauseActiveClock(r.clockNow())
		answer, _, err := input.Prompter.Ask(ctx, questionEvent)
		if err != nil {
			code := CodeAgentFailed
			if errors.Is(err, context.Canceled) {
				code = CodeCanceled
			}
			return r.fail(input, state, code, err), true
		}
		state.resumeActiveClock(r.clockNow())
		r.changePhase(input.Emitter, state, state.resumePhase, "user input received")
		modelAnswers := modelQuestionAnswers(call.Args, questionEvent, answer)
		answerJSON, _ := json.Marshal(modelAnswers)
		state.reviewInstructions = append(state.reviewInstructions, ReviewInstruction{Text: "User answers: " + string(answerJSON)})
		result := SuccessfulToolResult("user answered")
		state.readLoop = ReadLoopState{}
		result.Data = map[string]any{
			"answers": modelAnswers,
		}
		state.pendingQuestion = nil
		r.appendControlObservation(state, call, assistantText, result)
		if err := r.saveCheckpoint(ctx, input, state, checkpointAfterUserAnswer); err != nil {
			return r.fail(input, state, CodeAgentFailed, err), true
		}
		return StructuredOutcome{}, false
	case "request_privilege":
		if state.mode != model.ModeExecute || state.phase != PhaseExecuting {
			r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, "request_privilege is only available while executing", false))
			return StructuredOutcome{}, false
		}
		var request struct {
			SlideIDs []string `json:"slide_ids"`
			Reason   string   `json:"reason"`
		}
		raw, _ := json.Marshal(call.Args)
		if err := json.Unmarshal(raw, &request); err != nil || strings.TrimSpace(request.Reason) == "" || len(request.SlideIDs) == 0 {
			r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, "request_privilege requires a reason and at least one scope addition", true))
			return StructuredOutcome{}, false
		}
		proposed, addition, err := proposeScopeExpansion(state.scope, state.pack, request.SlideIDs)
		if err != nil {
			r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, err.Error(), true))
			return StructuredOutcome{}, false
		}
		if proposed.Equal(state.scope) {
			result := SuccessfulToolResult("requested privileges are already included in the active scope")
			result.Data = map[string]any{"decision": "approve", "summary": "所请求页面已具备编辑权限，无需再次审批。"}
			r.appendControlObservation(state, call, assistantText, result)
			return StructuredOutcome{}, false
		}
		requestEvent := model.ScopeExpansionRequestedPayload{
			PublicEventBase: publicBase(state.runID), InteractionID: "scope_" + uuid.NewString(), CallID: call.ID,
			BaseRevision: state.scope.Revision, CurrentScope: state.scope, RequestedAddition: addition,
			ProposedScope: proposed, AffectedPageCount: len(proposed.SlideIDs), Reason: model.PublicText(request.Reason, state.publicTextContext()),
		}
		return r.awaitScopeExpansion(ctx, input, state, requestEvent, &call, assistantText)
	case "review_task":
		if state.mode != model.ModeExecute || state.phase != PhaseExecuting || strings.TrimSpace(stringValue(call.Args["demand"])) == "" {
			r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, "review_task requires a non-empty demand and an active execution", false))
			return StructuredOutcome{}, false
		}
		state.pendingReview = &PendingReview{Call: call, AssistantText: assistantText}
		if err := r.saveCheckpoint(ctx, input, state, checkpointBeforeReview); err != nil {
			return r.fail(input, state, CodeAgentFailed, err), true
		}
		result := r.runReviewTask(ctx, input, state, call)
		state.pendingReview.Result = &result
		// Persist the assessment before publication so recovery cannot turn an
		// already published successful result into an interrupted review.
		if err := r.saveCheckpoint(context.WithoutCancel(ctx), input, state, checkpointAfterReview); err != nil {
			return r.fail(input, state, CodeAgentFailed, err), true
		}
		projector := ToolPublicProjector{ProjectDir: input.ProjectDir, TextContext: state.publicTextContext()}
		if input.Emitter != nil {
			if event, ok := projector.Completed(state.runID, call.ID, call.Name, call.Args, result); ok {
				input.Emitter.Emit(model.EventToolCompleted, event)
			}
		}
		state.pendingReview = nil
		r.appendControlObservation(state, call, assistantText, result)
		if err := r.saveCheckpoint(context.WithoutCancel(ctx), input, state, checkpointAfterReview); err != nil {
			return r.fail(input, state, CodeAgentFailed, err), true
		}
		return StructuredOutcome{}, false
	case "finish_task":
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
	resuming := call == nil
	if call == nil && state.pendingScopeExpansion != nil {
		call = state.pendingScopeExpansion.Call
		assistantText = state.pendingScopeExpansion.AssistantText
	}
	alreadyApplied := state.pendingScopeExpansion != nil && state.pendingScopeExpansion.Applied
	state.pendingScopeExpansion = &PendingScopeExpansion{Call: call, AssistantText: assistantText, Request: request, ResumePhase: PhaseExecuting, Applied: alreadyApplied}
	r.changePhase(input.Emitter, state, PhaseWaitingInput, "scope expansion approval required")
	if err := r.saveCheckpoint(ctx, input, state, checkpointBeforeScopeExpansion); err != nil {
		return r.fail(input, state, CodeAgentFailed, err), true
	}
	state.pauseActiveClock(r.clockNow())
	var answer model.ScopeExpansionAnswer
	var err error
	if resuming {
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
	if answer.InteractionID != request.InteractionID || answer.CallID != request.CallID || answer.BaseRevision != request.BaseRevision ||
		(answer.Decision != "approve" && answer.Decision != "revise" && answer.Decision != "refuse") {
		return r.fail(input, state, CodeAgentFailed, errors.New("invalid scope approval answer")), true
	}
	if (!alreadyApplied && state.scope.Revision != request.BaseRevision) || (alreadyApplied && state.scope.Revision != request.BaseRevision+1) {
		return r.fail(input, state, CodeAgentFailed, errors.New("scope changed while expansion approval was pending")), true
	}
	var applied *model.RunScope
	if alreadyApplied {
		next := state.scope
		applied = &next
	} else if answer.Decision == "approve" {
		next := request.ProposedScope
		applied = &next
	} else if answer.Decision == "revise" {
		next, resolveErr := resolveRuntimeScopeSelection(state.pack, model.CreateRunScopeInput{Selection: model.ScopeSelectionInput{Kind: "all_pages"}})
		if resolveErr != nil || !scopeContains(next, state.scope) {
			if resolveErr == nil {
				resolveErr = errors.New("adjusted scope cannot remove current privileges")
			}
			return r.fail(input, state, CodeAgentFailed, resolveErr), true
		}
		next.Revision = state.scope.Revision + 1
		applied = &next
	}
	if applied != nil && !alreadyApplied {
		previous := state.scope
		candidate := *state
		candidate.scope = *applied
		candidate.pack.Command.Scope = *applied
		candidate.pack.Target.SlideIDs = append([]string{}, applied.SlideIDs...)
		candidate.pendingScopeExpansion = &PendingScopeExpansion{Call: call, AssistantText: assistantText, Request: request, ResumePhase: PhaseExecuting, Applied: true}
		candidate.phase = PhaseExecuting
		checkpoint := r.checkpointForBoundary(&candidate, checkpointAfterScopeExpansion)
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
	if applied != nil && alreadyApplied && input.Emitter != nil {
		input.Emitter.Emit(model.EventScopeUpdated, model.ScopeUpdatedPayload{PublicEventBase: publicBase(state.runID), PreviousScope: request.CurrentScope, Scope: *applied, Cause: "user_approved_expansion", InteractionID: request.InteractionID})
	}
	state.pendingScopeExpansion = nil
	state.resumePhase = PhaseExecuting
	r.changePhase(input.Emitter, state, PhaseExecuting, "scope expansion answered")
	prompter.ResumeAfterScopeExpansion(ctx)
	if input.Emitter != nil {
		input.Emitter.Emit(model.EventScopeExpansionAnswered, model.ScopeExpansionAnsweredPayload{PublicEventBase: publicBase(state.runID), InteractionID: request.InteractionID, CallID: request.CallID, BaseRevision: request.BaseRevision, Decision: answer.Decision, AppliedScope: applied})
	}
	if call == nil {
		return r.fail(input, state, CodeAgentFailed, errors.New("pending scope expansion is missing its original tool call")), true
	}
	result := SuccessfulToolResult("scope expansion answered")
	summaries := map[string]string{"approve": "用户已批准所请求页面的编辑权限。", "revise": "用户已将编辑范围扩大至所有页面，包括本次运行中新建的页面，可以继续执行。", "refuse": "用户已拒绝扩展权限，请在原有授权范围内继续。"}
	result.Data = map[string]any{"decision": answer.Decision, "summary": summaries[answer.Decision]}
	observed := false
	for _, message := range state.messages {
		if message.Role == llm.RoleTool && message.ToolCallID == call.ID {
			observed = true
			break
		}
	}
	if !observed {
		r.appendControlObservation(state, *call, assistantText, result)
	}
	if err := r.saveCheckpoint(ctx, input, state, checkpointAfterScopeExpansion); err != nil {
		return r.fail(input, state, CodeAgentFailed, err), true
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
		if !current.ContainsSlide(id) {
			requested[id] = true
		}
	}
	// Authorization is a set: outline reordering or deletion is not an expansion.
	if len(requested) == 0 {
		return current, model.ScopeExpansionAddition{SlideIDs: []string{}}, nil
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
	// Preserve previously authorized IDs even if those pages have been removed.
	for _, id := range current.SlideIDs {
		if !known[id] {
			next.SlideIDs = append(next.SlideIDs, id)
		}
	}
	next.Revision = current.Revision + 1
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
		r.changePhase(input.Emitter, state, finishPhase, "completion rejected; continuing the same loop")
		for i := range result.Issues {
			if result.Issues[i].NextAction == "" {
				result.Issues[i].NextAction = model.ErrorDefinitionFor(result.Issues[i].Code).ModelMessage
			}
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
		if state.gateCount >= state.budget.MaxIdenticalGateRejections {
			return r.fail(input, state, CodeGateRejectedRepeated, errors.New("completion gate rejected the same unchanged state three times")), true
		}
		if err := r.saveCheckpoint(ctx, input, state, checkpointGateRejected); err != nil {
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
		if err := r.saveCheckpoint(ctx, input, state, checkpointAfterCommit); err != nil {
			// Artifact files and metadata were committed at their tool boundaries.
			recordTrace(input.Trace, state.runID, "checkpoint.persistence_failed", map[string]any{
				"loop_id": state.loopID, "boundary": checkpointAfterCommit,
			})
		}
	}
	r.changePhase(input.Emitter, state, PhaseTerminal, "run completed")
	if err := r.saveCheckpoint(ctx, input, state, checkpointTerminal); err != nil {
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
	if state.tx != nil {
		state.tx.RollbackOperation()
	}
	// Keep the last executable phase, not a terminal cursor, in the checkpoint.
	// Rule-based stops and explicit interruption can continue; unexpected errors cannot.
	state.continuationAllowed = !isRuntimeErrorCode(agentErr.Code) || publicStatus == "canceled"
	if state.phase != PhaseWaitingInput {
		state.resumePhase = state.phase
	}
	if state.resumePhase == PhaseCompletionCheck || state.resumePhase == PhaseCommitting {
		switch state.mode {
		case model.ModeExecute:
			state.resumePhase = PhaseExecuting
		case model.ModePlan:
			state.resumePhase = PhasePlanning
		default:
			state.resumePhase = PhaseChat
		}
	}
	writeCtx := state.persistenceCtx
	if writeCtx == nil {
		writeCtx = context.Background()
	}
	if err := r.saveCheckpoint(context.WithoutCancel(writeCtx), input, state, checkpointTerminal); err != nil {
		state.continuationAllowed = false
	}
	r.changePhase(input.Emitter, state, PhaseTerminal, agentErr.Code)
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
	case CodeBudgetExceeded, CodeConsecutiveErrors, CodeGateRejectedRepeated, CodeReadLoop, "LOCK_TIMEOUT", "MODEL_TOOL_CALL_INVALID":
		return false
	default:
		return true
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

func (state *RunState) checkpoint(now time.Time) RuntimeCheckpoint {
	return RuntimeCheckpoint{
		ContentAssessmentIDs: append([]string(nil), state.contentAssessmentIDs...), ToolDecision: state.toolDecision, DecisionIdentity: state.decisionIdentity, PendingContent: state.pendingContent, ContinuationAllowed: state.continuationAllowed, BudgetBaseTurns: state.budgetBaseTurns, BudgetBaseDurationMS: state.budgetBaseDurationMS,
		RunID: state.runID, LoopID: state.loopID, Phase: state.phase, Mode: state.mode,
		ResumePhase: state.resumePhase, Plan: state.plan, Requirements: state.requirements, Work: state.work, Changes: state.changeSet(),
		Evidence: state.ledger.Entries(state.changeSet()), Turns: state.turns, ToolCalls: state.toolCalls,
		ActiveDurationMS:    state.activeDurationAt(now).Milliseconds(),
		WaitingDurationMS:   state.waitingDurationAt(now).Milliseconds(),
		CompletionFailures:  state.gateCount,
		ReadLoop:            ReadLoopState{ProgressHash: state.readLoop.ProgressHash, Seen: append([]string(nil), state.readLoop.Seen...), RepeatRounds: state.readLoop.RepeatRounds},
		ReadImages:          append([]RunReadImage(nil), state.readImages...),
		PendingReview:       state.pendingReview,
		ReviewInstructions:  append([]ReviewInstruction(nil), state.reviewInstructions...),
		ReviewBaselineError: state.reviewBaselineError,
		SeenVersions:        state.seenVersions, PendingPlanCall: state.pendingPlanCall, PendingPlanPublication: state.pendingPlanPublication, PendingQuestion: state.pendingQuestion, PendingCommand: state.pendingCommand, PendingScopeExpansion: state.pendingScopeExpansion,
		Scope: state.scope,
	}
}

func (r *Runtime) checkBudget(ctx context.Context, state *RunState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if state.turns-state.budgetBaseTurns >= state.budget.MaxTurns {
		recordTrace(state.trace, state.runID, "runtime.budget_exhausted", map[string]any{
			"budget_kind": "turns", "turns": state.turns, "max_turns": state.budget.MaxTurns,
		})
		return errors.New("runtime turn budget exhausted")
	}
	return r.checkActiveDurationBudget(state)
}

func (r *Runtime) checkActiveDurationBudget(state *RunState) error {
	active, wall, waiting := state.durationSnapshot(r.clockNow())
	if active-time.Duration(state.budgetBaseDurationMS)*time.Millisecond >= state.budget.MaxDuration {
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

func (r *Runtime) appendSteering(ctx context.Context, input RuntimeInput, state *RunState, steering SteeringSource) error {
	if steering == nil {
		return nil
	}
	messages, err := steering.DrainInputs(ctx)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		duplicate := false
		for _, existing := range state.messages {
			if m := existing.Metadata; m != nil && m.Origin == "user" && m.Kind == "steering" && m.RunID == state.runID && m.Key == message.ID {
				duplicate = true
				break
			}
		}
		if duplicate {
			ids = append(ids, message.ID)
			continue
		}
		if strings.TrimSpace(message.Content) != "" || len(message.Attachments) > 0 || len(message.DOMSelections) > 0 {
			// Preserve acceptance order even before the first model request.
			state.messages = appendRunInstruction(state.messages, state.pack.Command, state.pack.Project.ID, state.runID)
			state.readLoop = ReadLoopState{}
			state.reviewInstructions = append(state.reviewInstructions, ReviewInstruction{Text: message.Content, Attachments: message.Attachments, DOMSelections: message.DOMSelections})
			state.messages = append(state.messages, llm.Message{Role: llm.RoleUser, Content: referenceMessageParts(
				"User steering: "+message.Content, message.ProjectID, message.Attachments, message.DOMSelections, message.ReferenceOrder,
			), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "steering", RunID: state.runID, Key: message.ID}})
			if message.Scope.Source.Kind != "" {
				state.scope = message.Scope
				state.pack.Command.Scope = message.Scope
			}
			state.pack.Command.DOMSelections = append(state.pack.Command.DOMSelections, message.DOMSelections...)
			ids = append(ids, message.ID)
		}
	}
	if len(ids) > 0 {
		if err := r.persistTranscript(ctx, input, state); err != nil {
			return err
		}
	}
	return steering.MarkInputsInjected(ctx, ids)
}

func (r *Runtime) persistTranscript(ctx context.Context, input RuntimeInput, state *RunState) error {
	if input.Transcript == nil || input.Context.Manifest.ThreadID == "" {
		return nil
	}
	if journal, ok := input.Transcript.(interface {
		ReplaceForRun(context.Context, string, string, string, []llm.Message) error
	}); ok {
		return journal.ReplaceForRun(ctx, input.RunID, input.ProjectDir, input.Context.Manifest.ThreadID, llm.NormalizeHistory(state.messages))
	}
	return input.Transcript.Replace(input.ProjectDir, input.Context.Manifest.ThreadID, llm.NormalizeHistory(state.messages))
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
			"size_bytes":    attachment.SizeBytes,
			"original_path": "attachments/" + attachment.ID + "." + attachment.Extension,
		})
		parts = append(parts,
			llm.ContentPart{Type: "text", Text: "<image_attachment>" + string(description) + "</image_attachment>"},
			llm.ContentPart{Type: "image", ImageRef: "project:" + projectID + "/attachment:" + attachment.ID + "/original", MIMEType: attachment.MediaType, Detail: "high"},
		)
	}
	return parts
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
	case "read_resource":
		switch stringValue(call.Args["resource"]) {
		case "manifest", "outline":
			return model.ActivityPresentationStructureReading
		case "design":
			return model.ActivityPresentationDesignReading
		default:
			return model.ActivitySlideContentReading
		}
	case "read_image":
		return model.ActivityReferenceInspecting
	case "edit_manifest", "edit_outline":
		return model.ActivityPresentationStructureUpdating
	case "edit_design":
		return model.ActivityPresentationDesignUpdating
	case "edit_spec", "edit_html":
		return model.ActivitySlideUpdating

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
	progress := &model.ContextCompactionProgress{ID: model.MustShortID("cmp"), Phase: 0}
	r.emitContextWindow(input.Emitter, state, state.lastWindow, "compacting", progress)
	before := state.lastWindow
	startedAt := r.clockNow()
	// Compact text only. Run images are reconstructed by prepareAgentRequest
	// after compaction, including images read before previous compactions.
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
	state.messages = result.Messages
	state.continuation = nil
	recordTrace(input.Trace, state.runID, "provider.continuation_reset", map[string]any{"reason": "history_compacted"})
	if err := r.persistTranscript(context.WithoutCancel(ctx), input, state); err != nil {
		return err
	}
	r.measureContextWindow(input, state, schemas)
	if state.tokens >= state.budget.ContextCompactionThreshold {
		state.nextCompactionTokens = state.tokens + 1024
	} else {
		state.nextCompactionTokens = 0
	}
	if input.RecordCompaction != nil {
		compaction, err := input.RecordCompaction(ctx, progress.ID, result, before, state.lastWindow, r.clockNow().Sub(startedAt))
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
	before := state.messages
	request := prepareAgentRequest(agentRequestForState(input, state, schemas))
	state.messages = request.Messages
	state.rememberPreparedResourceVersions(before)
	system := runtimeSystemPromptForRequest(request)
	tools := make([]llm.ToolSchema, 0, len(schemas))
	for _, schema := range schemas {
		tools = append(tools, llm.ToolSchema{
			Name: schema.Name, Description: schema.Description, Parameters: schema.Parameters, OutputSchema: schema.OutputSchema,
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
	result = bindToolErrorObservation(result, call)
	state.messages = appendToolObservation(state.messages, call, assistantText, result)
	if !result.OK {
		recordTrace(state.trace, state.runID, "control.rejected", map[string]any{
			"tool": call.Name, "call_id": call.ID, "code": result.Code,
			"reason": result.Summary, "data": result.Data,
		})
	}
}

func finishMessageViolation(message string) string {
	final := strings.TrimSpace(message)
	if final == "" {
		return "finish_task.message is required and must contain the complete final user-facing answer"
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

func isControlTool(name string) bool {
	return name == "create_plan" || name == "update_plan" || name == "ask_user" || name == "request_privilege" || name == "review_task" || name == "finish_task"
}

func controlSchemas(phase RunPhase, mode model.RunMode, plan *Plan) []ToolSchema {
	out := []ToolSchema{}
	if (mode == model.ModePlan && phase == PhasePlanning) || (mode == model.ModeExecute && (phase == PhaseExecuting || phase == PhasePlanning)) {
		if plan == nil || plan.Status == PlanAwaitingApproval {
			out = append(out, ToolSchema{Name: "create_plan", OutputSchema: toolOutputSchema("create_plan"), Description: "Create or fully replace an unapproved plan draft and wait for user approval.", Parameters: planProposalParameters()})
		} else if mode == model.ModeExecute && plan.Status == PlanActive {
			out = append(out, ToolSchema{Name: "update_plan", OutputSchema: toolOutputSchema("update_plan"), Description: "Update existing step statuses using the updates array. Use exact step IDs from the current plan. Approved content and step structure are locked. At most one step may be processing; completed steps cannot regress.", Parameters: planProgressParameters()})
		}
	}
	if mode == model.ModeExecute && phase == PhaseExecuting {
		out = append(out, ToolSchema{
			Name: "request_privilege", OutputSchema: toolOutputSchema("request_privilege"), Description: "Request a user-approved expansion of the editable page set. Provide only additional slide IDs and explain why in page terms. Global resources and both Spec/HTML of authorized pages are already writable. This call must be the only call in the response.",
			Parameters: objectSchema([]string{"slide_ids", "reason"}, map[string]any{
				"slide_ids": map[string]any{"type": "array", "minItems": 1, "description": "Existing pages to add to the editable scope, not a replacement for the current scope. Use IDs from the current outline.", "items": map[string]any{"type": "string", "pattern": "^sli_[A-Za-z0-9_-]+$", "description": "Stable ID of a page whose Spec or HTML needs editing beyond the current authorization."}},
				"reason":    map[string]any{"type": "string", "description": "User-facing explanation of which additional pages need changes and why those changes are needed for the requested task."},
			}),
		})
	}
	allowAsk := mode == model.ModeGrill || mode == model.ModePlan || (mode == model.ModeExecute && phase != PhaseCompletionCheck)
	if allowAsk && phase != PhaseWaitingInput && phase != PhaseCommitting && phase != PhaseTerminal {
		out = append(out, ToolSchema{
			Name: "ask_user", OutputSchema: toolOutputSchema("ask_user"), Description: "Ask one blocking group of atomic user questions needed for the user's actual task and pause this same loop until the user answers. Do not use for greetings, identity questions, or to solicit a task the user has not requested. Each item is either single-choice with 1-3 options, optionally allow_custom=true, or fill-in with no options. Do not merge multiple choices into one free-text question.",
			Parameters: objectSchema([]string{"questions"}, map[string]any{
				"questions": map[string]any{"type": "array", "minItems": 1, "description": "Questions to present together in order. Each question should ask for one decision or missing fact needed to continue the user's task.", "items": objectSchema([]string{"title", "reason"}, map[string]any{
					"title":  map[string]any{"type": "string", "minLength": 1, "pattern": `\S`, "description": "The complete, clear question shown to the user, with enough context to answer it; not a short category label."},
					"reason": map[string]any{"type": "string", "minLength": 1, "pattern": `\S`, "description": "Briefly explain to the user why their answer is needed and which subsequent work or decision it will affect. Do not repeat the question or provide internal reasoning."},
					"options": map[string]any{"type": "array", "maxItems": 3, "description": "One to three preset answers for a single-choice question. Omit or use [] for a free-text question.", "items": objectSchema([]string{"label", "description"}, map[string]any{
						"label": map[string]any{"type": "string", "minLength": 1, "pattern": `\S`, "description": "Concise, distinct answer text displayed as the option and returned as the answer when selected."}, "description": map[string]any{"type": "string", "minLength": 1, "pattern": `\S`, "description": "Explain this option's concrete meaning, effect or tradeoff so the user can choose; do not merely repeat its label."},
					})},
					"allow_custom": map[string]any{"type": "boolean", "description": "Whether to allow a free-text answer alongside preset options; defaults to false when options exist. With no options, free-text input is always enabled regardless of this value."},
				})},
			}),
		})
	}
	if mode == model.ModeExecute && phase == PhaseExecuting {
		out = append(out, ToolSchema{
			Name: "review_task", OutputSchema: toolOutputSchema("review_task"), Description: "Ask the independent artifact reviewer to inspect current PPT artifacts and cumulative Run changes. Describe the target pages and review focus in demand. It can read resources and images and render slides. It does not review plans or final replies, edit artifacts, or finish the task. Use its reasons to decide the next action.",
			Parameters: objectSchema([]string{"demand"}, map[string]any{
				"demand": map[string]any{"type": "string", "minLength": 1, "pattern": `\S`, "description": "What artifacts should be reviewed, what to check, and any task-specific acceptance requirements. User requirements remain authoritative."},
			}),
		})
	}
	if (phase == PhaseChat && (mode == model.ModeChat || mode == model.ModeGrill)) ||
		(phase == PhaseExecuting && mode == model.ModeExecute) || (phase == PhasePlanning && plan != nil && plan.Status == PlanCanceled) {
		out = append(out, ToolSchema{
			Name: "finish_task", OutputSchema: toolOutputSchema("finish_task"), Description: "Submit the complete final user-facing response for the current chat, grill, or execute run. For greetings, identity questions, acknowledgments, or questions answerable from available context, call this tool directly in the first response with a brief answer, without preceding assistant text or other tools. No project work is required for ordinary conversation. Use this call alone. Ordinary assistant text is not a completion signal. Runtime checks the requested outcome and any required evidence before completion.",
			Parameters: objectSchema([]string{"message"}, map[string]any{
				"message": map[string]any{"type": "string", "description": "The complete answer to the current request. A greeting or identity question usually needs only one sentence; do not add project summaries, capability lists or next steps unless requested or relevant. For actual work, report the result and relevant checks or limitations."},
				"suggested_next_inputs": map[string]any{
					"type": "array", "maxItems": 3,
					"description": "Optional follow-up messages the user can select to fill the input composer, then edit or send. Omit or use an empty array for ordinary conversation; do not invent tasks to fill this field.",
					"items":       map[string]any{"type": "string", "maxLength": 80, "description": "One concise, ready-to-send user request grounded in the current conversation; not an assistant promise or a status label."},
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

func (r *Runtime) completePlanTool(state *RunState, answer model.PlanApprovalAnswer) {
	pending := state.pendingPlanCall
	if pending == nil {
		return
	}
	summary := "用户已批准计划，可以开始执行。"
	if answer.Decision == "refuse" {
		summary = "用户已拒绝计划，不得执行该计划。"
	}
	if answer.Decision == "revise" {
		summary = answer.Feedback
		if strings.TrimSpace(summary) == "" {
			summary = "用户要求修改计划，但未提供具体建议。"
		}
	}
	result := SuccessfulToolResult(summary)
	result.Data = map[string]any{"decision": answer.Decision, "summary": summary}
	for _, message := range state.messages {
		if message.Role == llm.RoleTool && message.ToolCallID == pending.Call.ID {
			state.pendingPlanCall = nil
			return
		}
	}
	r.appendControlObservation(state, pending.Call, pending.AssistantText, result)
	state.pendingPlanCall = nil
}

const skippedQuestionAnswer = "用户跳过了此问题，请结合已有信息自行判断；如有推荐选项，可优先采用，但不要将其视为用户明确选择。"

func modelQuestionAnswers(args map[string]any, question model.QuestionAskedPayload, answer model.QuestionAnswer) []map[string]string {
	byID := map[string]model.QuestionFieldAnswer{}
	for _, item := range answer.Answers {
		byID[item.QuestionID] = item
	}
	original, _ := args["questions"].([]any)
	out := []map[string]string{}
	for i, field := range question.Questions {
		reply := byID[field.ID]
		source, _ := original[i].(map[string]any)
		value := reply.CustomText
		if reply.SelectedOptionID != "" {
			options, _ := source["options"].([]any)
			for j, option := range field.Options {
				if option.ID == reply.SelectedOptionID {
					raw, _ := options[j].(map[string]any)
					value = stringValue(raw["label"])
				}
			}
		}
		if reply.Skipped {
			value = skippedQuestionAnswer
		}
		out = append(out, map[string]string{"question": stringValue(source["title"]), "answer": value})
	}
	return out
}

func (r *Runtime) failPendingPlanTool(state *RunState, err error) {
	if pending := state.pendingPlanCall; pending != nil {
		r.appendControlObservation(state, pending.Call, pending.AssistantText, failedToolResult(CodeAgentFailed, err.Error(), false))
		state.pendingPlanCall = nil
	}
}

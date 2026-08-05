package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/idempotency"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
)

type EventEmitter interface {
	Emit(model.EventType, any)
}

type Prompter interface {
	Ask(context.Context, model.QuestionAskedPayload) (model.QuestionAnswer, string, error)
}

type SteeringSource interface {
	DrainInputs(context.Context) ([]SteeringInput, error)
	MarkInputsInjected(context.Context, []string) error
}

type SteeringInput struct {
	ID      string
	Content string
}

type LifecycleObserver interface {
	PhaseChanged(RuntimePhase)
}

type IdempotencyStore interface {
	AcquireIdempotency(context.Context, model.IdempotencyRecord) (model.IdempotencyRecord, bool, error)
	CompleteIdempotency(context.Context, string, string, string, string, string) error
	GetIdempotency(context.Context, string, string, string) (model.IdempotencyRecord, error)
}

type ContextCompactor interface {
	Compact(context.Context, []llm.Message) ([]llm.Message, error)
}

type CheckpointSink interface {
	SaveCheckpoint(context.Context, RuntimeCheckpoint) error
}

type RuntimeCheckpoint struct {
	RunID              string            `json:"run_id"`
	LoopID             string            `json:"loop_id"`
	Strategy           ExecutionStrategy `json:"strategy"`
	ExecuteMode        ExecuteMode       `json:"execute_mode,omitempty"`
	Phase              RuntimePhase      `json:"phase"`
	ResumePhase        RuntimePhase      `json:"resume_phase,omitempty"`
	Plan               *Plan             `json:"plan,omitempty"`
	Changes            ChangeSet         `json:"changes"`
	Evidence           []Evidence        `json:"evidence"`
	Turns              int               `json:"turns"`
	ToolCalls          int               `json:"tool_calls"`
	WaitingQuestionID  string            `json:"waiting_question_id,omitempty"`
	CompletionFailures int               `json:"completion_failures"`
}

type AgentRequest struct {
	RunID           string
	LoopID          string
	Strategy        ExecutionStrategy
	ExecuteMode     ExecuteMode
	Phase           RuntimePhase
	Context         contextengine.ContextPack
	Plan            *Plan
	Changes         ChangeSet
	Evidence        []Evidence
	Messages        []llm.Message
	Tools           []ToolSchema
	ImageResolver   llm.ImageRefResolver
	Continuation    *llm.ProviderContinuation
	OnProviderRetry func(int)
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
	runtimeState, _ := json.Marshal(map[string]any{
		"strategy": req.Strategy, "execute_mode": req.ExecuteMode, "phase": req.Phase, "plan": req.Plan,
		"changes": req.Changes, "evidence": req.Evidence,
	})
	system, user := contextengine.CompileForRunner(&req.Context,
		runtimeSystemPrompt(req.Phase, req.Strategy, string(runtimeState)),
		req.Context.WorkSpec.Instruction)
	messages := append([]llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent(system)},
		{Role: llm.RoleUser, Content: llm.TextContent(user)},
	}, req.Messages...)
	tools := make([]llm.ToolSchema, 0, len(req.Tools))
	for _, schema := range req.Tools {
		tools = append(tools, llm.ToolSchema{Name: schema.Name, Description: schema.Description, Parameters: schema.Parameters})
	}
	response, err := a.Provider.Generate(ctx, llm.GenerateRequest{
		Messages: messages, Tools: tools, ImageResolver: req.ImageResolver,
		Continuation: req.Continuation, OnRetry: req.OnProviderRetry,
	})
	if err != nil {
		return AgentResponse{}, err
	}
	return AgentResponse{
		ToolCalls: response.ToolCalls, Text: response.Text(),
		Continuation: response.Continuation, Usage: response.Usage,
	}, nil
}

func runtimeSystemPrompt(phase RuntimePhase, strategy ExecutionStrategy, state string) string {
	contracts := map[string]any{}
	for _, name := range []string{pptschema.OutlineName, pptschema.DesignName, pptschema.SlideSpecName} {
		if contract, err := pptschema.AgentContract(name); err == nil {
			contracts[name] = contract
		}
	}
	contractJSON, _ := json.Marshal(contracts)
	return fmt.Sprintf(`<runtime_policy>
You are the single continuous ReAct agent for this HTML PPT run. Strategy=%s Phase=%s.
Use only disclosed tools. talk, ask and plan are read-only; execute writes through the active run session.
Re-check resource disclosure, interaction, target scope, strategy and phase on every call.
The only business tools are read_ppt, write_ppt, edit_ppt, search_refs and render_slide.
The only control actions are update_plan, ask_user and finish. ask_user and finish must each be the sole action in a response.
StrategyPlan must create a concise checklist with update_plan before finish, then explain the complete plan in finish(message).
StrategyExecute may run direct or planned. If coordination is needed, call update_plan first; Runtime then treats execution as planned.
Plan steps in update_plan must be short UI checklist items. Put full rationale and detailed execution notes in finish(message).
A dynamic checklist is not a workflow DAG or a separate verification stage.
All normal successful exits require finish(message=...). Ordinary assistant text never completes a run.
</runtime_policy>

<ppt_business_policy>
Outline owns deck goal, audience, narrative sections and stable slide order.
Design owns the deck-wide 16:9 visual system: 1600x900 canvas, palette, typography, spacing, grid, density and motion.
Each Slide Spec owns one page's semantic role, title, key message, content hierarchy, visual intent and speaker notes.
Each Slide HTML is the final page implementation. It must use a 1600x900 .slide-stage, accessible semantic HTML, useful alt text, CJK-safe fonts and project-local/data/blob resources only.
Every Slide HTML must link ../../common/tokens.css and ../../common/base.css. Runtime derives tokens.css from Design. Available shared tokens are --color-bg, --color-fg, --color-primary, --color-accent, --color-muted, --font-sans, --font-serif, --font-mono, --text-title, --text-h1, --text-body, --text-caption, --space-1, --space-2, --space-3, --space-4, --space-6, --space-8, --radius-sm, --radius-md, --radius-lg, --shadow-card, --shadow-pop, --stage-w and --stage-h.
For an empty complete Presentation, establish Outline, then Design, then every Slide Spec in outline order, then every Slide HTML. Maintain a coherent cross-slide narrative, controlled information density, strong hierarchy and visual consistency.
Use read_ppt when exact saved text is needed. Use edit_ppt only for small uniquely anchored replacements. Use write_ppt for full creation or broad reconstruction.
After every latest HTML or design-affecting change, render affected pages, observe diagnostics, repair failures in the same ReAct loop, and render again.
Independent reads/searches and renders for different pages may be returned together. Writes must respect resource dependency order.
</ppt_business_policy>

<current_resource_contracts>
Resources are exactly deck:outline, deck:design, slide:&lt;slide_id&gt;:spec and slide:&lt;slide_id&gt;:html.
Never pass disk paths, project paths, runtime paths, database IDs or storage artifact kinds.
read_ppt(resource) returns the complete saved JSON or HTML string.
write_ppt(resource, content) always receives content as a string. Runtime owns schema_version, revision, project_id, slide_id, source revisions and timestamps.
edit_ppt(resource, edits) uses ordered objects with old_text and new_text. Each old_text must match exactly once.
Domain contracts generated from the authoritative schemas: %s
</current_resource_contracts>

<current_runtime_state>
%s
</current_runtime_state>`, strategy, phase, contractJSON, state)
}

const noToolCallGuidance = "Ordinary assistant text is not a completion signal. If the task is complete, call finish(message=...) with the final response. If the task is not complete, call one of the currently disclosed tools to continue."

type RuntimeInput struct {
	RunID          string
	ProjectDir     string
	Context        contextengine.ContextPack
	Emitter        EventEmitter
	Prompter       Prompter
	Steering       SteeringSource
	Checkpoint     CheckpointSink
	CommitMetadata CommitMetadata
	DomainTools    DomainToolProvider
	Budget         RuntimeBudget
	Trace          TraceRecorder
	ImageResolver  llm.ImageRefResolver
	Lifecycle      LifecycleObserver
	Idempotency    IdempotencyStore
}

type Runtime struct {
	Router    StrategyRouter
	Agent     ReActAgent
	Gate      CompletionGate
	Compactor ContextCompactor
}

func NewRuntime(agent ReActAgent) *Runtime {
	return &Runtime{Router: StrategyRouter{}, Agent: agent, Gate: NewCompletionGate()}
}

type runtimeState struct {
	runID                 string
	loopID                string
	strategy              ExecutionStrategy
	executeMode           ExecuteMode
	phase                 RuntimePhase
	resumePhase           RuntimePhase
	decision              StrategyDecision
	plan                  *Plan
	tx                    *RunSession
	scope                 Scope
	ledger                *EvidenceLedger
	issues                []Issue
	messages              []llm.Message
	continuation          *llm.ProviderContinuation
	turns                 int
	toolCalls             int
	tokens                int
	toolFailures          int
	activeTools           int
	started               time.Time
	budget                RuntimeBudget
	gateKey               string
	gateCount             int
	gateEvidence          int64
	lastSummary           string
	committed             bool
	lastProgress          string
	lastReasoning         string
	lastMilestoneRevision int
	trace                 TraceRecorder
	lifecycle             LifecycleObserver
}

func (r *Runtime) Run(ctx context.Context, input RuntimeInput) StructuredOutcome {
	if input.Budget.MaxTurns == 0 {
		input.Budget = DefaultRuntimeBudget()
	}
	decision := r.Router.Decide(input.Context)
	executeMode := decision.ExecuteMode
	if decision.Strategy == StrategyExecute && executeMode == ExecuteModeNone {
		executeMode = ExecuteModeDirect
	}
	decision.ExecuteMode = executeMode
	state := &runtimeState{
		runID: input.RunID, loopID: "loop_" + uuid.NewString(), strategy: decision.Strategy,
		executeMode: executeMode,
		decision:    decision, scope: ScopeFromSpec(input.Context.WorkSpec), ledger: NewEvidenceLedger(),
		issues: []Issue{}, messages: []llm.Message{}, started: time.Now(), budget: input.Budget,
		trace: input.Trace, lifecycle: input.Lifecycle,
	}
	if r.Agent == nil {
		return r.fail(input, state, CodeAgentFailed, errors.New("ReAct agent is required"))
	}
	initialPhase := PhaseChat
	switch state.strategy {
	case StrategyTalk, StrategyAsk:
		initialPhase = PhaseChat
	case StrategyPlan:
		initialPhase = PhasePlanning
	case StrategyExecute:
		if state.executeMode == ExecuteModePlanned {
			initialPhase = PhasePlanning
		} else {
			initialPhase = PhaseExecuting
		}
	}
	manifest := input.Context.Manifest
	recordTrace(input.Trace, input.RunID, "context.assembled", map[string]any{
		"loop_id": state.loopID, "context_id": manifest.ContextID,
		"profile": manifest.Profile, "estimated_tokens": manifest.EstimatedTokens,
		"budget_tokens": manifest.BudgetTokens, "segments": len(manifest.Segments),
		"refs": len(manifest.Refs), "warnings": manifest.Warnings, "read_only": manifest.ReadOnly,
	})
	r.emitStrategy(state, false)
	r.changePhase(input.Emitter, state, initialPhase, "strategy initialized")
	if initialPhase == PhasePlanning {
		r.emitProgress(input.Emitter, state, "planning", "正在整理执行计划", nil)
	} else {
		r.emitProgress(input.Emitter, state, "thinking", "正在分析任务与当前内容", nil)
	}

	if state.strategy == StrategyExecute {
		// Direct-write session: typed tools write artifacts straight to the
		// project directory. A failed or canceled run keeps its partial
		// products on disk instead of discarding a private write sandbox.
		session, err := NewRunSession(input.ProjectDir, input.RunID)
		if err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		state.tx = session
	}

	registry := NewToolRegistry()
	provider := input.DomainTools
	if provider == nil {
		provider = DefaultDomainToolProvider{Pack: input.Context}
	}
	if err := provider.RegisterDomainTools(registry); err != nil {
		return r.fail(input, state, CodeAgentFailed, err)
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
		if err := r.compactIfNeeded(ctx, state); err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		schemas := registry.Disclose(state.strategy, state.phase, input.Context.WorkSpec.Interaction.Intent)
		schemas = append(schemas, controlSchemas(state.strategy, state.phase, input.Context.WorkSpec.Interaction.Intent)...)
		state.turns++
		response, err := r.Agent.Next(ctx, AgentRequest{
			RunID: state.runID, LoopID: state.loopID, Strategy: state.strategy, ExecuteMode: state.executeMode, Phase: state.phase,
			Context: input.Context, Plan: state.plan, Changes: state.changeSet(),
			Evidence: state.ledger.Entries(state.changeSet()), Messages: append([]llm.Message{}, state.messages...),
			Tools:         schemas,
			ImageResolver: input.ImageResolver,
			Continuation:  state.continuation,
			OnProviderRetry: func(attempt int) {
				r.emitProgress(input.Emitter, state, "thinking", fmt.Sprintf("模型服务暂时不可用，正在自动重试（%d）", attempt), nil)
			},
		})
		if err != nil {
			return r.failAgentError(input, state, classifyProviderError(ctx, err))
		}
		state.continuation = response.Continuation
		if response.Usage.TotalTokens > 0 {
			state.tokens += response.Usage.TotalTokens
		} else {
			state.tokens += approximateTokens(response.Text)
		}
		if len(response.ToolCalls) == 0 {
			recordTrace(input.Trace, state.runID, "raw.model.response", map[string]any{
				"turn": state.turns, "has_tool_call": false,
			})
			state.messages = append(state.messages, llm.Message{
				Role: llm.RoleAssistant, Content: llm.TextContent(response.Text),
			})
			state.messages = append(state.messages, llm.Message{Role: llm.RoleUser, Content: llm.TextContent(noToolCallGuidance)})
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
		results := r.executeToolBatch(ctx, input, state, registry, schemasByName(schemas), calls)
		state.messages = appendBatchObservations(state.messages, calls, response.Text, results)
		upgrade := false
		recordToolFailures(state, results)
		for _, result := range results {
			upgrade = upgrade || result.Code == CodeScopeExpansion
		}
		if state.strategy == StrategyExecute && state.executeMode == ExecuteModeDirect && (upgrade || requiresComplexCoordination(state.changeSet())) {
			r.upgradeToPlanned(input.Emitter, state, "runtime scope expansion requires coordinated planning")
			continue
		}
		if state.strategy == StrategyExecute && state.executeMode == ExecuteModeDirect && state.toolCalls >= state.budget.SimpleUpgradeToolRoundTrips {
			r.upgradeToPlanned(input.Emitter, state, "direct execution exceeded six tool round trips")
			continue
		}
		if state.toolFailures >= state.budget.MaxConsecutiveToolFailures {
			return r.fail(input, state, CodeConsecutiveErrors, errors.New("consecutive tool failures exhausted the runtime budget"))
		}
	}
}

func recordToolFailures(state *runtimeState, results []ToolResult) {
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
	state *runtimeState,
	registry *ToolRegistry,
	disclosed map[string]bool,
	calls []llm.ToolCall,
) []ToolResult {
	results := make([]ToolResult, len(calls))
	started := make([]bool, len(calls))
	replayed := make([]bool, len(calls))
	projector := ToolPublicProjector{}
	planStepID := currentPlanStepID(state.plan)
	var lifecycleMu sync.Mutex
	state.toolCalls += len(calls)
	state.activeTools = len(calls)
	execute := func(index int) {
		call := calls[index]
		replay, shouldExecute := r.acquireToolCall(ctx, input, state, call)
		if !shouldExecute {
			results[index] = replay
			replayed[index] = replay.Code != CodeCanceled
			return
		}
		started[index] = true
		lifecycleMu.Lock()
		r.emitToolProgress(input.Emitter, state, call)
		if input.Emitter != nil {
			if event, ok := projector.Started(state.runID, call.ID, call.Name, call.Args, planStepID); ok {
				input.Emitter.Emit(model.EventToolStarted, event)
			}
		}
		recordTrace(input.Trace, state.runID, "tool.called", map[string]any{
			"loop_id": state.loopID, "call_id": call.ID, "tool": call.Name, "args": call.Args,
		})
		lifecycleMu.Unlock()
		results[index] = registry.Execute(ctx, disclosed, call.Name, call.Args, DomainToolInput{
			Args: call.Args, CallID: call.ID, Context: input.Context, ProjectDir: input.ProjectDir, RunID: input.RunID,
			Session: state.tx, Scope: state.scope, Strategy: state.strategy, Phase: state.phase,
			Interaction: input.Context.WorkSpec.Interaction.Intent, Risk: state.decision.Risk,
		})
		if ctx.Err() != nil {
			results[index] = failedToolResult(CodeCanceled, "run canceled", false)
		}
	}
	switch {
	case batchIsIndependentReads(calls):
		runConcurrentBatch(ctx, len(calls), 4, execute)
	case batchIsIndependentRenders(calls):
		runConcurrentBatch(ctx, len(calls), 3, execute)
	default:
		writeFailed := false
		for index, call := range calls {
			desc, _ := registry.Descriptor(call.Name)
			if writeFailed {
				started[index] = true
				if input.Emitter != nil {
					if event, ok := projector.Started(state.runID, call.ID, call.Name, call.Args, planStepID); ok {
						input.Emitter.Emit(model.EventToolStarted, event)
					}
				}
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
		result = bindToolErrorObservation(result, call, input.Context)
		results[index] = result
		if started[index] {
			r.persistToolCall(context.Background(), input, state, call, result)
		}
		if started[index] && input.Emitter != nil {
			if event, ok := projector.Completed(state.runID, call.ID, call.Name, call.Args, result); ok {
				input.Emitter.Emit(model.EventToolCompleted, event)
			}
		}
		if replayed[index] {
			recordTrace(input.Trace, state.runID, "tool.replayed", map[string]any{"call_id": call.ID, "tool": call.Name})
		}
		recordTrace(input.Trace, state.runID, "tool.completed", map[string]any{
			"loop_id": state.loopID, "call_id": call.ID, "tool": call.Name,
			"ok": result.OK, "summary": result.Summary, "data": result.Data,
			"issues": result.Issues, "changed_targets": result.ChangedTargets,
			"retryable": result.Retryable, "code": result.Code,
		})
		for _, target := range uniqueTargets(append(result.InvalidatedTargets, changedResources(result.ChangedTargets)...)) {
			state.ledger.Invalidate(target)
		}
		for _, evidence := range result.Evidence {
			recorded := state.ledger.Record(evidence)
			recordTrace(input.Trace, state.runID, "evidence.recorded", map[string]any{"evidence": recorded})
		}
		for _, target := range result.ChangedTargets {
			recordTrace(input.Trace, state.runID, "target.written", map[string]any{
				"target": target.Target(), "revision": target.Revision, "hash": target.Hash,
				"fields": target.Fields, "tentative": false,
			})
		}
	}
	return results
}

func bindToolErrorObservation(result ToolResult, call llm.ToolCall, pack contextengine.ContextPack) ToolResult {
	if result.OK || len(result.ObservationParts) > 0 {
		return result
	}
	agentErr := model.NewAgentError(result.Code, "tool_call", errors.New(result.Summary))
	agentErr.CallID = call.ID
	target, ok := declaredTarget(call.Args)
	if call.Name == "render_slide" {
		if slideID := stringValue(call.Args["slide_id"]); slideID != "" {
			target, ok = Resource{Type: "slide", SlideID: slideID, Part: "html"}, true
		}
	}
	if ok {
		agentErr.Resource = &model.ErrorResource{Type: target.Type, SlideID: target.SlideID, Part: target.Part}
		switch {
		case target.Type == "deck" && target.Part == "outline":
			agentErr.Details["current_revision"] = pack.Revisions.Outline
		case target.Type == "deck" && target.Part == "design":
			agentErr.Details["current_revision"] = pack.Revisions.Design
		case target.Type == "slide" && target.Part == "spec":
			agentErr.Details["current_revision"] = pack.Revisions.SlideSpecs[target.SlideID]
		case target.Type == "slide" && target.Part == "html":
			agentErr.Details["current_revision"] = pack.Revisions.SlideHTML[target.SlideID]
		}
	}
	agentErr.Details["reason"] = result.Summary
	agentErr.Details["next_action"] = agentErr.ModelMessage
	if ok && target.Part == "html" && len(result.Issues) > 0 {
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
	raw, _ := json.Marshal(agentErr.ModelObservation())
	result.Observation = string(raw)
	return result
}

type persistedToolResult struct {
	Result             ToolResult        `json:"result"`
	Evidence           []Evidence        `json:"evidence"`
	InvalidatedTargets []Resource        `json:"invalidated_targets"`
	ObservationParts   []llm.ContentPart `json:"observation_parts"`
}

func (r *Runtime) acquireToolCall(ctx context.Context, input RuntimeInput, state *runtimeState, call llm.ToolCall) (ToolResult, bool) {
	if input.Idempotency == nil {
		return ToolResult{}, true
	}
	requestHash, err := idempotency.CanonicalHash(map[string]any{
		"tool": call.Name, "args": call.Args, "scope": state.scope.Target,
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
	persisted.Result.InvalidatedTargets = persisted.InvalidatedTargets
	persisted.Result.ObservationParts = persisted.ObservationParts
	return persisted.Result, false
}

func (r *Runtime) persistToolCall(ctx context.Context, input RuntimeInput, state *runtimeState, call llm.ToolCall, result ToolResult) {
	if input.Idempotency == nil {
		return
	}
	raw, err := json.Marshal(persistedToolResult{
		Result: result, Evidence: result.Evidence,
		InvalidatedTargets: result.InvalidatedTargets, ObservationParts: result.ObservationParts,
	})
	if err == nil {
		_ = input.Idempotency.CompleteIdempotency(ctx, "tool_call", state.runID, call.ID, "completed", string(raw))
	}
}

func batchIsIndependentReads(calls []llm.ToolCall) bool {
	if len(calls) < 2 {
		return false
	}
	for _, call := range calls {
		if call.Name != "read_ppt" && call.Name != "search_refs" {
			return false
		}
	}
	return true
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

func (r *Runtime) executeControl(
	ctx context.Context,
	input RuntimeInput,
	state *runtimeState,
	call llm.ToolCall,
	assistantText string,
) (StructuredOutcome, bool) {
	switch call.Name {
	case "update_plan":
		if (state.strategy != StrategyPlan && state.strategy != StrategyExecute) ||
			(state.phase != PhasePlanning && state.phase != PhaseExecuting) {
			r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, "update_plan is not allowed now", false))
			return StructuredOutcome{}, false
		}
		raw, _ := json.Marshal(call.Args)
		var update PlanUpdate
		if err := json.Unmarshal(raw, &update); err != nil {
			r.appendControlObservation(state, call, assistantText, failedToolResult(ErrPlanInvalid.Error(), err.Error(), true))
			return StructuredOutcome{}, false
		}
		next, created, err := ApplyPlanUpdate(state.plan, update, state.runID, time.Now())
		if err != nil {
			r.appendControlObservation(state, call, assistantText, failedToolResult(ErrPlanInvalid.Error(), err.Error(), true))
			return StructuredOutcome{}, false
		}
		previous := state.plan
		state.plan = &next
		if input.Emitter != nil {
			input.Emitter.Emit(model.EventPlanUpdated, model.PlanUpdatedPayload{
				PublicEventBase: publicBase(state.runID), Plan: publicPlan(next),
			})
			completed := completedPlanSteps(previous, next)
			if len(completed) > 0 && state.lastMilestoneRevision != next.Revision {
				ids := make([]string, 0, len(completed))
				for _, step := range completed {
					ids = append(ids, step.ID)
				}
				input.Emitter.Emit(model.EventMessageMilestone, model.MessageMilestonePayload{
					PublicEventBase: publicBase(state.runID), MessageID: newMessageID(),
					Text: milestoneText(next.Explanation, completed), CompletedStepIDs: ids,
				})
				state.lastMilestoneRevision = next.Revision
			}
		}
		r.appendControlObservation(state, call, assistantText, SuccessfulToolResult("plan revision accepted"))
		if state.strategy == StrategyExecute {
			state.executeMode = ExecuteModePlanned
			state.decision.ExecuteMode = ExecuteModePlanned
		}
		if state.strategy == StrategyExecute && created && state.phase == PhasePlanning {
			r.changePhase(input.Emitter, state, PhaseExecuting, "first valid plan created")
		}
		return StructuredOutcome{}, false
	case "ask_user":
		if input.Prompter == nil {
			r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, "ask_user requires an interactive prompter", false))
			return StructuredOutcome{}, false
		}
		question := stringValue(call.Args["question"])
		if question == "" {
			r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, "question is required", true))
			return StructuredOutcome{}, false
		}
		questionID := call.ID
		questionEvent := publicQuestion(state.runID, questionID, call.Args)
		state.resumePhase = state.phase
		r.changePhase(input.Emitter, state, PhaseWaitingInput, "agent requested required user input")
		if input.Checkpoint != nil {
			if err := input.Checkpoint.SaveCheckpoint(ctx, state.checkpoint(questionID)); err != nil {
				return r.fail(input, state, CodeAgentFailed, err), true
			}
		}
		answer, displayText, err := input.Prompter.Ask(ctx, questionEvent)
		if err != nil {
			code := CodeAgentFailed
			if errors.Is(err, context.Canceled) {
				code = CodeCanceled
			}
			return r.fail(input, state, code, err), true
		}
		r.changePhase(input.Emitter, state, state.resumePhase, "user input received")
		result := SuccessfulToolResult("user answered")
		result.Data = map[string]any{
			"selected_option_ids": answer.SelectedOptionIDs,
			"custom_text":         answer.CustomText, "display_text": displayText,
		}
		r.appendControlObservation(state, call, assistantText, result)
		return StructuredOutcome{}, false
	case "finish":
		message := stringValue(call.Args["message"])
		return r.finishCandidate(ctx, input, state, call, assistantText, message)
	default:
		r.appendControlObservation(state, call, assistantText, failedToolResult(CodeInvalidControlCall, "unknown runtime control tool", false))
		return StructuredOutcome{}, false
	}
}

func (r *Runtime) finishCandidate(
	ctx context.Context,
	input RuntimeInput,
	state *runtimeState,
	call llm.ToolCall,
	assistantText string,
	message string,
) (StructuredOutcome, bool) {
	finishPhase := state.phase
	r.changePhase(input.Emitter, state, PhaseCompletionCheck, "finish candidate submitted")
	r.emitProgress(input.Emitter, state, "finalizing", "正在完成最终检查", nil)
	changes := state.changeSet()
	result := r.Gate.Check(CompletionContext{
		Strategy: state.strategy, ExecuteMode: state.executeMode, FinishPhase: finishPhase, ActiveTools: state.activeTools,
		Issues: state.issues, WorkScope: state.scope, Session: state.tx, Changes: changes,
		Evidence: state.ledger, Context: input.Context, Plan: state.plan, Canceled: ctx.Err() != nil,
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
		if state.strategy == StrategyExecute && state.executeMode == ExecuteModeDirect && state.gateCount >= 2 && gateNeedsCoordination(result) {
			r.upgradeToPlanned(input.Emitter, state, "completion issues require multi-step coordination")
		} else {
			r.changePhase(input.Emitter, state, finishPhase, "completion rejected; continuing the same loop")
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
		return StructuredOutcome{}, false
	}
	state.lastSummary = strings.TrimSpace(message)
	if state.lastSummary == "" {
		state.lastSummary = "Run completed"
	}
	if state.strategy == StrategyExecute {
		r.changePhase(input.Emitter, state, PhaseCommitting, "completion accepted")
		state.tx.AcceptMaterializationProofs(result.MaterializationProofs)
		commitHash, hashErr := idempotency.CanonicalHash(map[string]any{
			"changes": changes, "proofs": result.MaterializationProofs,
		})
		if hashErr != nil {
			return r.fail(input, state, CodeCommitFailed, hashErr), true
		}
		shouldCommit := true
		if input.Idempotency != nil {
			record, created, acquireErr := input.Idempotency.AcquireIdempotency(ctx, model.IdempotencyRecord{
				Scope: "commit", OwnerID: state.runID, Key: "commit",
				RequestHash: commitHash, Status: "in_progress",
			})
			if acquireErr != nil {
				return r.fail(input, state, CodeCommitFailed, acquireErr), true
			}
			if record.RequestHash != commitHash {
				return r.fail(input, state, "IDEMPOTENCY_KEY_REUSED", errors.New("commit key reused with different changes")), true
			}
			if !created {
				for record.Status == "in_progress" {
					select {
					case <-ctx.Done():
						return r.fail(input, state, CodeCanceled, ctx.Err()), true
					case <-time.After(10 * time.Millisecond):
					}
					record, acquireErr = input.Idempotency.GetIdempotency(ctx, "commit", state.runID, "commit")
					if acquireErr != nil {
						return r.fail(input, state, CodeCommitFailed, acquireErr), true
					}
				}
				if record.Status != "completed" {
					return r.fail(input, state, CodeCommitFailed, errors.New("previous commit result was not confirmed")), true
				}
				shouldCommit = false
			}
		}
		if shouldCommit {
			if err := state.tx.Commit(ctx, input.CommitMetadata); err != nil {
				if input.Idempotency != nil {
					_ = input.Idempotency.CompleteIdempotency(context.Background(), "commit", state.runID, "commit", "failed", `{"code":"COMMIT_FAILED"}`)
				}
				if ctx.Err() != nil {
					return r.fail(input, state, CodeCanceled, ctx.Err()), true
				}
				return r.fail(input, state, CodeCommitFailed, err), true
			}
			if input.Idempotency != nil {
				_ = input.Idempotency.CompleteIdempotency(context.Background(), "commit", state.runID, "commit", "completed", `{"status":"completed"}`)
			}
		}
		state.committed = true
		for _, change := range changes.All() {
			recordTrace(input.Trace, state.runID, "target.committed", map[string]any{
				"target": resourceForArtifact(change.Artifact), "artifact": change.Artifact,
			})
		}
	}
	r.changePhase(input.Emitter, state, PhaseTerminal, "run completed")
	outcome := state.outcome(StatusCompleted, "", "")
	if input.Emitter != nil {
		affected := publicAffectedTargets(changes)
		input.Emitter.Emit(model.EventMessageFinal, model.MessageFinalPayload{
			PublicEventBase: publicBase(state.runID), MessageID: newMessageID(),
			Text:            safeFinalMessage(state.lastSummary, state.strategy, len(affected)),
			AffectedTargets: affected,
		})
		input.Emitter.Emit(model.EventRunFinished, model.RunFinishedPayload{
			PublicEventBase: publicBase(state.runID), Status: "completed",
			AffectedTargets: affected, DurationMS: time.Since(state.started).Milliseconds(),
		})
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

func (r *Runtime) upgradeToPlanned(emitter EventEmitter, state *runtimeState, reason string) {
	if state.strategy != StrategyExecute || state.executeMode == ExecuteModePlanned {
		return
	}
	state.executeMode = ExecuteModePlanned
	state.decision = StrategyDecision{
		Strategy: StrategyExecute, ExecuteMode: ExecuteModePlanned, Reason: reason, Confidence: 1, Risk: RiskMedium,
		Signals: append(state.decision.Signals, DecisionSignal{Name: "runtime_upgrade", Value: "true"}),
	}
	if state.tx != nil {
		state.tx.MarkTentative()
		for _, change := range state.tx.ChangeSet().All() {
			recordTrace(state.trace, state.runID, "target.written", map[string]any{
				"target": resourceForArtifact(change.Artifact), "artifact": change.Artifact, "tentative": true,
			})
		}
	}
	r.emitStrategy(state, true)
	r.changePhase(emitter, state, PhasePlanning, reason)
	r.emitProgress(emitter, state, "planning", "任务范围扩大，正在更新执行计划", nil)
}

func (r *Runtime) changePhase(emitter EventEmitter, state *runtimeState, next RuntimePhase, reason string) {
	previous := state.phase
	state.phase = next
	if state.lifecycle != nil {
		state.lifecycle.PhaseChanged(next)
	}
	recordTrace(state.trace, state.runID, "phase.changed", map[string]any{
		"loop_id": state.loopID, "from": previous, "phase": next, "reason": reason,
	})
}

func (r *Runtime) emitStrategy(state *runtimeState, upgraded bool) {
	recordTrace(state.trace, state.runID, "strategy.selected", map[string]any{
		"loop_id":  state.loopID,
		"strategy": state.decision.Strategy, "execute_mode": state.decision.ExecuteMode, "reason": state.decision.Reason,
		"confidence": state.decision.Confidence, "risk": state.decision.Risk,
		"signals": state.decision.Signals, "upgraded": upgraded,
	})
}

func (r *Runtime) fail(input RuntimeInput, state *runtimeState, code string, err error) StructuredOutcome {
	return r.failAgentError(input, state, model.NewAgentError(code, "run", err))
}

func (r *Runtime) failAgentError(input RuntimeInput, state *runtimeState, agentErr *model.AgentError) StructuredOutcome {
	if agentErr == nil {
		agentErr = model.NewAgentError("INTERNAL", "run", errors.New("unknown runtime error"))
	}
	status, publicStatus := StatusFailed, "failed"
	if errors.Is(agentErr, context.Canceled) || agentErr.Code == CodeCanceled {
		status, publicStatus = StatusCanceled, "canceled"
	}
	recordTrace(input.Trace, state.runID, "error.projected", agentErr.TraceProjection())
	r.changePhase(input.Emitter, state, PhaseTerminal, agentErr.Code)
	outcome := state.outcome(status, agentErr.Code, agentErr.Error())
	if input.Emitter != nil {
		payload := model.RunFinishedPayload{
			PublicEventBase: publicBase(state.runID), Status: publicStatus,
			DurationMS: time.Since(state.started).Milliseconds(),
		}
		if publicStatus == "failed" {
			payload.Error = agentErr.Public()
		}
		input.Emitter.Emit(model.EventRunFinished, payload)
	}
	return outcome
}

func (state *runtimeState) outcome(status WorkflowStatus, code, message string) StructuredOutcome {
	return StructuredOutcome{
		LoopID: state.loopID, Strategy: state.strategy, Phase: state.phase, Status: status,
		Target: state.scope.Target, Changes: state.changeSet(), Issues: append([]Issue{}, state.issues...),
		Summary: state.lastSummary, Code: code, Message: message,
	}
}

func (state *runtimeState) changeSet() ChangeSet {
	if state.tx == nil {
		return EmptyChangeSet()
	}
	return state.tx.ChangeSet()
}

func (state *runtimeState) checkpoint(questionID string) RuntimeCheckpoint {
	return RuntimeCheckpoint{
		RunID: state.runID, LoopID: state.loopID, Strategy: state.strategy, Phase: state.phase,
		ExecuteMode: state.executeMode, ResumePhase: state.resumePhase, Plan: state.plan, Changes: state.changeSet(),
		Evidence: state.ledger.Entries(state.changeSet()), Turns: state.turns, ToolCalls: state.toolCalls,
		WaitingQuestionID: questionID, CompletionFailures: state.gateCount,
	}
}

func (r *Runtime) checkBudget(ctx context.Context, state *runtimeState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.budgetExhausted(state) {
		return errors.New("runtime budget exhausted")
	}
	return nil
}

func (r *Runtime) budgetExhausted(state *runtimeState) bool {
	return state.turns >= state.budget.MaxTurns ||
		time.Since(state.started) >= state.budget.MaxDuration
}

func (r *Runtime) appendSteering(ctx context.Context, state *runtimeState, steering SteeringSource) error {
	if steering == nil {
		return nil
	}
	messages, err := steering.DrainInputs(ctx)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		if strings.TrimSpace(message.Content) != "" {
			state.messages = append(state.messages, llm.Message{Role: llm.RoleUser, Content: llm.TextContent("User steering: " + message.Content)})
			state.tokens += approximateTokens(message.Content)
			ids = append(ids, message.ID)
		}
	}
	return steering.MarkInputsInjected(ctx, ids)
}

func (r *Runtime) emitReasoning(emitter EventEmitter, state *runtimeState, raw string) {
	if emitter == nil {
		return
	}
	text := sanitizePublicReasoning(raw)
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
	state *runtimeState,
	stage string,
	text string,
	target *model.PublicTarget,
) {
	if emitter == nil {
		return
	}
	key := stage + "|" + text
	if target != nil {
		key += "|" + target.Type + "|" + target.SlideID + "|" + target.Part
	}
	if key == state.lastProgress {
		return
	}
	state.lastProgress = key
	emitter.Emit(model.EventRunProgress, model.RunProgressPayload{
		PublicEventBase: publicBase(state.runID), Stage: stage,
		Text: sanitizePublicText(text, 120), Target: target,
	})
}

func (r *Runtime) emitToolProgress(emitter EventEmitter, state *runtimeState, call llm.ToolCall) {
	stage, text := "thinking", "正在继续处理任务"
	target := publicToolTarget(call.Name, call.Args)
	switch call.Name {
	case "read_ppt":
		stage, text = "reading", "正在读取 PPT 内容"
	case "search_refs":
		stage, text = "reading", "正在查找相关参考"
	case "write_ppt":
		stage, text = "writing", "正在生成 PPT 内容"
	case "edit_ppt":
		stage, text = "writing", "正在更新 PPT 内容"
	case "render_slide":
		stage, text = "rendering", "正在检查页面布局"
	}
	if target != nil && target.Type == "slide" {
		switch call.Name {
		case "read_ppt":
			text = "正在读取" + slideDisplayName(target.SlideID)
		case "write_ppt":
			text = "正在生成" + slideDisplayName(target.SlideID)
		case "edit_ppt":
			text = "正在更新" + slideDisplayName(target.SlideID)
		case "render_slide":
			text = "正在检查" + slideDisplayName(target.SlideID) + "布局"
		}
	}
	r.emitProgress(emitter, state, stage, text, target)
}

func (r *Runtime) compactIfNeeded(ctx context.Context, state *runtimeState) error {
	if r.Compactor == nil || state.tokens < state.budget.ContextCompactionThreshold {
		return nil
	}
	compactionInput := pruneSupersededRenderImages(append([]llm.Message{}, state.messages...))
	messages, err := r.Compactor.Compact(ctx, compactionInput)
	if err != nil {
		return err
	}
	state.messages = messages
	state.tokens = approximateMessageTokens(messages)
	return nil
}

func pruneSupersededRenderImages(messages []llm.Message) []llm.Message {
	seenSlides := map[string]bool{}
	for index := len(messages) - 1; index >= 0; index-- {
		message := &messages[index]
		slideID := ""
		for _, part := range message.Content {
			if part.Type != "text" {
				continue
			}
			var payload map[string]any
			if json.Unmarshal([]byte(part.Text), &payload) == nil {
				slideID = stringValue(payload["slide_id"])
			}
		}
		if slideID == "" {
			continue
		}
		keepImage := !seenSlides[slideID]
		seenSlides[slideID] = true
		parts := make([]llm.ContentPart, 0, len(message.Content))
		for _, part := range message.Content {
			if part.Type != "image" || keepImage {
				parts = append(parts, part)
			}
		}
		message.Content = parts
	}
	return messages
}

func (r *Runtime) appendControlObservation(
	state *runtimeState,
	call llm.ToolCall,
	assistantText string,
	result ToolResult,
) {
	state.messages = appendToolObservation(state.messages, call, assistantText, result)
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
		raw, _ := json.Marshal(result)
		content = string(raw)
	}
	if len(parts) == 0 {
		parts = llm.TextContent(content)
	}
	return append(messages,
		llm.Message{
			Role: llm.RoleAssistant, Content: llm.TextContent(assistantText),
			ToolCalls: []llm.ToolCall{call},
		},
		llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, Content: parts},
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
			raw, _ := json.Marshal(result)
			content = string(raw)
		}
		if len(parts) == 0 {
			parts = llm.TextContent(content)
		}
		messages = append(messages, llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, Content: parts})
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

func requiresComplexCoordination(changes ChangeSet) bool {
	owners := map[string]bool{}
	for _, change := range changes.All() {
		target := resourceForArtifact(change.Artifact)
		owner := target.Type
		if target.Type == "slide" {
			// Slide Spec and Slide HTML are two representations of the same
			// page target. Updating both is the normal single-slide path and
			// must not force a mid-run Complex upgrade.
			owner += ":" + target.SlideID
		} else {
			owner += ":" + target.Part
		}
		owners[owner] = true
	}
	return len(owners) > 1
}

func gateNeedsCoordination(result CompletionResult) bool {
	if len(result.Issues) > 1 {
		return true
	}
	for _, issue := range result.Issues {
		if issue.Code == "PLAN_INCOMPLETE" || issue.Code == "TARGET_OUT_OF_SCOPE" {
			return true
		}
	}
	return false
}

func isControlTool(name string) bool {
	return name == "update_plan" || name == "ask_user" || name == "finish"
}

func controlSchemas(strategy ExecutionStrategy, phase RuntimePhase, interaction model.InteractionIntent) []ToolSchema {
	out := []ToolSchema{}
	if (strategy == StrategyPlan || strategy == StrategyExecute) && (phase == PhasePlanning || phase == PhaseExecuting) {
		out = append(out, ToolSchema{
			Name: "update_plan", Description: "Create or replace the current lightweight plan snapshot.",
			Parameters: objectSchema([]string{"steps"}, map[string]any{
				"explanation": map[string]any{"type": "string"},
				"steps": map[string]any{
					"type": "array", "items": objectSchema([]string{"id", "title", "status"}, map[string]any{
						"id": map[string]any{"type": "string"}, "title": map[string]any{"type": "string"},
						"status": map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed", "failed"}},
					}),
				},
			}),
		})
	}
	allowAsk := interaction == model.IntentAsk || interaction == model.IntentPlan || (interaction == model.IntentExecute && phase != PhaseCompletionCheck)
	if allowAsk && phase != PhaseWaitingInput && phase != PhaseCommitting && phase != PhaseTerminal {
		out = append(out, ToolSchema{
			Name: "ask_user", Description: "Ask one blocking question and pause this same loop until the user answers.",
			Parameters: objectSchema([]string{"question"}, map[string]any{
				"question": map[string]any{"type": "string"},
				"options":  map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
				"multiple": map[string]any{"type": "boolean"}, "allow_custom": map[string]any{"type": "boolean"},
			}),
		})
	}
	if phase == PhaseChat || phase == PhaseExecuting || (strategy == StrategyPlan && phase == PhasePlanning) {
		out = append(out, ToolSchema{
			Name: "finish", Description: "所有策略的最终答复都必须通过 finish(message=...) 提交。普通 assistant 文本不是结束信号。The message is checked by the Completion Gate before the run may complete.",
			Parameters: objectSchema([]string{"message"}, map[string]any{
				"message": map[string]any{"type": "string"},
			}),
		})
	}
	return out
}

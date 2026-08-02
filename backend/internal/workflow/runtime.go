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
	DrainInputs() []string
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
	RunID    string
	LoopID   string
	Strategy ExecutionStrategy
	Phase    RuntimePhase
	Context  contextengine.ContextPack
	Plan     *Plan
	Changes  ChangeSet
	Evidence []Evidence
	Messages []llm.Message
	Tools    []ToolSchema
}

type AgentResponse struct {
	ToolCalls         []llm.ToolCall
	Text              string
	ProviderReasoning string
}

type ReActAgent interface {
	Next(context.Context, AgentRequest) (AgentResponse, error)
}

type CognitiveAgent struct {
	Client llm.Client
}

func (a CognitiveAgent) Next(ctx context.Context, req AgentRequest) (AgentResponse, error) {
	if a.Client == nil {
		return AgentResponse{}, errors.New("LLM client is required for ReAct execution")
	}
	runtimeState, _ := json.Marshal(map[string]any{
		"strategy": req.Strategy, "phase": req.Phase, "plan": req.Plan,
		"changes": req.Changes, "evidence": req.Evidence,
	})
	system, user := contextengine.CompileForRunner(&req.Context,
		runtimeSystemPrompt(req.Phase, req.Strategy, string(runtimeState)),
		req.Context.WorkSpec.Instruction)
	messages := append([]llm.Message{{Role: llm.RoleSystem, Content: system}, {Role: llm.RoleUser, Content: user}}, req.Messages...)
	tools := make([]llm.ToolSchema, 0, len(req.Tools))
	for _, schema := range req.Tools {
		tools = append(tools, llm.ToolSchema{Name: schema.Name, Description: schema.Description, Parameters: schema.Parameters})
	}
	response, err := a.Client.CallTool(ctx, llm.ToolCallRequest{Messages: messages, Tools: tools})
	if err != nil {
		return AgentResponse{}, err
	}
	return AgentResponse{
		ToolCalls: response.ToolCalls, Text: response.Text,
		ProviderReasoning: response.ReasoningContent,
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
Use only disclosed tools. talk and ask are read-only; execute writes only to run staging.
Re-check resource disclosure, interaction, target scope, strategy and phase on every call.
The only business tools are read_ppt, write_ppt, edit_ppt, search_refs and render_slide.
The only control actions are update_plan, ask_user and finish. ask_user and finish must each be the sole action in a response.
Complex strategy may maintain a dynamic checklist, but it is not a workflow DAG or a separate verification stage.
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
Never pass disk paths, project paths, staging paths, database IDs or storage artifact kinds.
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
	phase                 RuntimePhase
	resumePhase           RuntimePhase
	decision              StrategyDecision
	plan                  *Plan
	tx                    *Transaction
	scope                 Scope
	ledger                *EvidenceLedger
	issues                []Issue
	messages              []llm.Message
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
}

func (r *Runtime) Run(ctx context.Context, input RuntimeInput) StructuredOutcome {
	if input.Budget.MaxTurns == 0 {
		input.Budget = DefaultRuntimeBudget()
	}
	decision := r.Router.Decide(input.Context)
	state := &runtimeState{
		runID: input.RunID, loopID: "loop_" + uuid.NewString(), strategy: decision.Strategy,
		decision: decision, scope: ScopeFromSpec(input.Context.WorkSpec), ledger: NewEvidenceLedger(),
		issues: []Issue{}, messages: []llm.Message{}, started: time.Now(), budget: input.Budget,
		trace: input.Trace,
	}
	if r.Agent == nil {
		return r.fail(input, state, CodeAgentFailed, errors.New("ReAct agent is required"))
	}
	initialPhase := PhaseChat
	switch state.strategy {
	case StrategyChat:
		initialPhase = PhaseChat
	case StrategySimple:
		initialPhase = PhaseExecuting
	case StrategyComplex:
		initialPhase = PhasePlanning
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

	if state.strategy != StrategyChat {
		tx, err := NewTransaction(input.ProjectDir, input.RunID)
		if err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		state.tx = tx
		defer func() {
			if !state.committed {
				_ = tx.Cleanup()
			}
		}()
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
		r.appendSteering(state, input.Steering)
		if err := r.compactIfNeeded(ctx, state); err != nil {
			return r.fail(input, state, CodeAgentFailed, err)
		}
		schemas := registry.Disclose(state.strategy, state.phase, input.Context.WorkSpec.Interaction.Intent)
		schemas = append(schemas, controlSchemas(state.strategy, state.phase, input.Context.WorkSpec.Interaction.Intent)...)
		state.turns++
		response, err := r.Agent.Next(ctx, AgentRequest{
			RunID: state.runID, LoopID: state.loopID, Strategy: state.strategy, Phase: state.phase,
			Context: input.Context, Plan: state.plan, Changes: state.changeSet(),
			Evidence: state.ledger.Entries(state.changeSet()), Messages: append([]llm.Message{}, state.messages...),
			Tools: schemas,
		})
		if err != nil {
			if ctx.Err() != nil {
				return r.fail(input, state, CodeCanceled, ctx.Err())
			}
			return r.fail(input, state, CodeAgentFailed, err)
		}
		state.tokens += approximateTokens(response.Text)
		if len(response.ToolCalls) == 0 {
			recordTrace(input.Trace, state.runID, "raw.model.response", map[string]any{
				"turn": state.turns, "has_tool_call": false,
				"provider_reasoning_content": response.ProviderReasoning,
			})
			state.messages = append(state.messages, llm.Message{
				Role: llm.RoleAssistant, Content: response.Text,
				ReasoningContent: response.ProviderReasoning,
			})
			state.messages = append(state.messages, llm.Message{Role: llm.RoleUser, Content: noToolCallGuidance})
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
			"provider_reasoning_content": response.ProviderReasoning,
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
				state.messages = appendBatchObservations(state.messages, calls, response.Text, response.ProviderReasoning, []ToolResult{result})
				continue
			}
			call := calls[0]
			if !schemasByName(schemas)[call.Name] {
				r.appendControlObservation(
					state, call, response.Text, response.ProviderReasoning,
					failedToolResult(ErrToolNotDisclosed.Error(), "runtime control was not disclosed in this turn", false),
				)
				continue
			}
			outcome, done := r.executeControl(ctx, input, state, call, response.Text, response.ProviderReasoning)
			if done {
				return outcome
			}
			continue
		}
		results := r.executeToolBatch(ctx, input, state, registry, schemasByName(schemas), calls)
		state.messages = appendBatchObservations(state.messages, calls, response.Text, response.ProviderReasoning, results)
		upgrade := false
		recordToolFailures(state, results)
		for _, result := range results {
			upgrade = upgrade || result.Code == CodeScopeExpansion
		}
		if state.strategy == StrategySimple && (upgrade || requiresComplexCoordination(state.changeSet())) {
			r.upgradeToComplex(input.Emitter, state, "runtime scope expansion requires coordinated planning")
			continue
		}
		if state.strategy == StrategySimple && state.toolCalls >= state.budget.SimpleUpgradeToolRoundTrips {
			r.upgradeToComplex(input.Emitter, state, "simple execution exceeded six tool round trips")
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
	projector := ToolPublicProjector{}
	planStepID := currentPlanStepID(state.plan)
	for _, call := range calls {
		r.emitToolProgress(input.Emitter, state, call)
		if input.Emitter != nil {
			if event, ok := projector.Started(state.runID, call.ID, call.Name, call.Args, planStepID); ok {
				input.Emitter.Emit(model.EventToolStarted, event)
			}
		}
		recordTrace(input.Trace, state.runID, "tool.called", map[string]any{
			"loop_id": state.loopID, "call_id": call.ID, "tool": call.Name, "args": call.Args,
		})
	}
	state.toolCalls += len(calls)
	state.activeTools += len(calls)
	execute := func(index int) {
		call := calls[index]
		results[index] = registry.Execute(ctx, disclosed, call.Name, call.Args, DomainToolInput{
			Args: call.Args, Context: input.Context, ProjectDir: input.ProjectDir, RunID: input.RunID,
			Transaction: state.tx, Scope: state.scope, Strategy: state.strategy, Phase: state.phase,
			Interaction: input.Context.WorkSpec.Interaction.Intent, Risk: state.decision.Risk,
		})
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
				results[index] = failedToolResult(CodeDependencyFailed, "call skipped after an earlier write failure", false)
				continue
			}
			execute(index)
			if !desc.ReadOnly && !results[index].OK {
				writeFailed = true
			}
		}
	}
	state.activeTools -= len(calls)
	for index, call := range calls {
		result := results[index]
		if input.Emitter != nil {
			if event, ok := projector.Completed(state.runID, call.ID, call.Name, call.Args, result); ok {
				input.Emitter.Emit(model.EventToolCompleted, event)
			}
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
			recordTrace(input.Trace, state.runID, "target.staged", map[string]any{
				"target": target.Target(), "revision": target.Revision, "hash": target.Hash,
				"fields": target.Fields, "tentative": false,
			})
		}
	}
	return results
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
	providerReasoning string,
) (StructuredOutcome, bool) {
	switch call.Name {
	case "update_plan":
		if state.strategy != StrategyComplex || (state.phase != PhasePlanning && state.phase != PhaseExecuting) {
			r.appendControlObservation(state, call, assistantText, providerReasoning, failedToolResult(CodeInvalidControlCall, "update_plan is not allowed now", false))
			return StructuredOutcome{}, false
		}
		raw, _ := json.Marshal(call.Args)
		var update PlanUpdate
		if err := json.Unmarshal(raw, &update); err != nil {
			r.appendControlObservation(state, call, assistantText, providerReasoning, failedToolResult(ErrPlanInvalid.Error(), err.Error(), true))
			return StructuredOutcome{}, false
		}
		next, created, err := ApplyPlanUpdate(state.plan, update, state.runID, time.Now())
		if err != nil {
			r.appendControlObservation(state, call, assistantText, providerReasoning, failedToolResult(ErrPlanInvalid.Error(), err.Error(), true))
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
		r.appendControlObservation(state, call, assistantText, providerReasoning, SuccessfulToolResult("plan revision accepted"))
		if created && state.phase == PhasePlanning {
			r.changePhase(input.Emitter, state, PhaseExecuting, "first valid plan created")
		}
		return StructuredOutcome{}, false
	case "ask_user":
		if input.Prompter == nil {
			r.appendControlObservation(state, call, assistantText, providerReasoning, failedToolResult(CodeInvalidControlCall, "ask_user requires an interactive prompter", false))
			return StructuredOutcome{}, false
		}
		question := stringValue(call.Args["question"])
		if question == "" {
			r.appendControlObservation(state, call, assistantText, providerReasoning, failedToolResult(CodeInvalidControlCall, "question is required", true))
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
		r.appendControlObservation(state, call, assistantText, providerReasoning, result)
		return StructuredOutcome{}, false
	case "finish":
		message := stringValue(call.Args["message"])
		return r.finishCandidate(ctx, input, state, call, assistantText, providerReasoning, message)
	default:
		r.appendControlObservation(state, call, assistantText, providerReasoning, failedToolResult(CodeInvalidControlCall, "unknown runtime control tool", false))
		return StructuredOutcome{}, false
	}
}

func (r *Runtime) finishCandidate(
	ctx context.Context,
	input RuntimeInput,
	state *runtimeState,
	call llm.ToolCall,
	assistantText string,
	providerReasoning string,
	message string,
) (StructuredOutcome, bool) {
	finishPhase := state.phase
	r.changePhase(input.Emitter, state, PhaseCompletionCheck, "finish candidate submitted")
	r.emitProgress(input.Emitter, state, "finalizing", "正在完成最终检查", nil)
	changes := state.changeSet()
	result := r.Gate.Check(CompletionContext{
		Strategy: state.strategy, FinishPhase: finishPhase, ActiveTools: state.activeTools,
		Issues: state.issues, WorkScope: state.scope, Transaction: state.tx, Changes: changes,
		Evidence: state.ledger, Context: input.Context, Plan: state.plan, Canceled: ctx.Err() != nil,
		BudgetExhausted: r.budgetExhausted(state),
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
		if state.strategy == StrategySimple && state.gateCount >= 2 && gateNeedsCoordination(result) {
			r.upgradeToComplex(input.Emitter, state, "completion issues require multi-step coordination")
		} else {
			r.changePhase(input.Emitter, state, finishPhase, "completion rejected; continuing the same loop")
		}
		observation := ToolResult{
			OK: false, Summary: "completion rejected", Data: map[string]any{"issues": result.Issues},
			ChangedTargets: []ChangedTarget{}, Evidence: []Evidence{}, Issues: []Issue{},
			Retryable: true, Code: CodeCompletionGateBlocked,
		}
		state.messages = appendToolObservation(
			state.messages,
			call, assistantText, providerReasoning, observation,
		)
		return StructuredOutcome{}, false
	}
	state.lastSummary = strings.TrimSpace(message)
	if state.lastSummary == "" {
		state.lastSummary = "Run completed"
	}
	if state.strategy != StrategyChat {
		r.changePhase(input.Emitter, state, PhaseCommitting, "completion accepted")
		state.tx.AcceptMaterializationProofs(result.MaterializationProofs)
		if err := state.tx.Commit(ctx, input.CommitMetadata); err != nil {
			return r.fail(input, state, CodeCommitFailed, err), true
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

func (r *Runtime) upgradeToComplex(emitter EventEmitter, state *runtimeState, reason string) {
	if state.strategy != StrategySimple {
		return
	}
	state.strategy = StrategyComplex
	state.decision = StrategyDecision{
		Strategy: StrategyComplex, Reason: reason, Confidence: 1, Risk: RiskMedium,
		Signals: append(state.decision.Signals, DecisionSignal{Name: "runtime_upgrade", Value: "true"}),
	}
	if state.tx != nil {
		state.tx.MarkTentative()
		for _, change := range state.tx.ChangeSet().All() {
			recordTrace(state.trace, state.runID, "target.staged", map[string]any{
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
	recordTrace(state.trace, state.runID, "phase.changed", map[string]any{
		"loop_id": state.loopID, "from": previous, "phase": next, "reason": reason,
	})
}

func (r *Runtime) emitStrategy(state *runtimeState, upgraded bool) {
	recordTrace(state.trace, state.runID, "strategy.selected", map[string]any{
		"loop_id":  state.loopID,
		"strategy": state.decision.Strategy, "reason": state.decision.Reason,
		"confidence": state.decision.Confidence, "risk": state.decision.Risk,
		"signals": state.decision.Signals, "upgraded": upgraded,
	})
}

func (r *Runtime) fail(input RuntimeInput, state *runtimeState, code string, err error) StructuredOutcome {
	status, publicStatus := StatusFailed, "failed"
	if errors.Is(err, context.Canceled) || code == CodeCanceled {
		status, publicStatus = StatusCanceled, "canceled"
	}
	r.changePhase(input.Emitter, state, PhaseTerminal, code)
	outcome := state.outcome(status, code, err.Error())
	if input.Emitter != nil {
		payload := model.RunFinishedPayload{
			PublicEventBase: publicBase(state.runID), Status: publicStatus,
			DurationMS: time.Since(state.started).Milliseconds(),
		}
		if publicStatus == "failed" {
			payload.Error = &model.PublicError{
				Code: code, Message: safeRunError(code, err), Retryable: retryableRunError(code),
			}
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
		ResumePhase: state.resumePhase, Plan: state.plan, Changes: state.changeSet(),
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
		state.toolCalls >= state.budget.MaxToolCalls ||
		state.tokens >= state.budget.MaxTokens ||
		time.Since(state.started) >= state.budget.MaxDuration
}

func (r *Runtime) appendSteering(state *runtimeState, steering SteeringSource) {
	if steering == nil {
		return
	}
	for _, message := range steering.DrainInputs() {
		if strings.TrimSpace(message) != "" {
			state.messages = append(state.messages, llm.Message{Role: llm.RoleUser, Content: "User steering: " + message})
			state.tokens += approximateTokens(message)
		}
	}
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
	messages, err := r.Compactor.Compact(ctx, append([]llm.Message{}, state.messages...))
	if err != nil {
		return err
	}
	state.messages = messages
	state.tokens = approximateMessageTokens(messages)
	return nil
}

func (r *Runtime) appendControlObservation(
	state *runtimeState,
	call llm.ToolCall,
	assistantText string,
	providerReasoning string,
	result ToolResult,
) {
	state.messages = appendToolObservation(state.messages, call, assistantText, providerReasoning, result)
}

func appendToolObservation(
	messages []llm.Message,
	call llm.ToolCall,
	assistantText string,
	providerReasoning string,
	result ToolResult,
) []llm.Message {
	content := result.Observation
	if content == "" {
		raw, _ := json.Marshal(result)
		content = string(raw)
	}
	return append(messages,
		llm.Message{
			Role: llm.RoleAssistant, Content: assistantText,
			ReasoningContent: providerReasoning, ToolCalls: []llm.ToolCall{call},
		},
		llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, Content: content},
	)
}

func appendBatchObservations(
	messages []llm.Message,
	calls []llm.ToolCall,
	assistantText string,
	providerReasoning string,
	results []ToolResult,
) []llm.Message {
	messages = append(messages, llm.Message{
		Role: llm.RoleAssistant, Content: assistantText,
		ReasoningContent: providerReasoning, ToolCalls: calls,
	})
	for index, call := range calls {
		result := failedToolResult(CodeInvalidControlCall, "tool call was not executed", false)
		if index < len(results) {
			result = results[index]
		}
		content := result.Observation
		if content == "" {
			raw, _ := json.Marshal(result)
			content = string(raw)
		}
		messages = append(messages, llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, Content: content})
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
		total += approximateTokens(message.Content)
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
	if strategy == StrategyComplex && (phase == PhasePlanning || phase == PhaseExecuting) {
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
	allowAsk := interaction == model.IntentAsk || (interaction == model.IntentExecute && phase != PhaseCompletionCheck)
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
	if phase == PhaseChat || phase == PhaseExecuting {
		out = append(out, ToolSchema{
			Name: "finish", Description: "所有策略的最终答复都必须通过 finish(message=...) 提交。普通 assistant 文本不是结束信号。The message is checked by the Completion Gate before the run may complete.",
			Parameters: objectSchema([]string{"message"}, map[string]any{
				"message": map[string]any{"type": "string"},
			}),
		})
	}
	return out
}

package workflow

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

var ErrDirectActionUpgrade = errors.New("SCOPE_EXPANSION_REQUIRED")

type AdaptiveRuntime struct {
	Router    ExecutionStrategyRouter
	Workflow  *Engine
	Executor  StepExecutor
	Verifiers *VerifierRegistry
	Risk      RiskPolicy
}

func NewAdaptiveRuntime(executor StepExecutor) *AdaptiveRuntime {
	engine := NewEngine(executor)
	return &AdaptiveRuntime{
		Router: ExecutionStrategyRouter{}, Workflow: engine, Executor: executor,
		Verifiers: engine.Verifiers,
	}
}

func (r *AdaptiveRuntime) Run(ctx context.Context, input RuntimeInput) StructuredOutcome {
	workflowID := input.WorkflowID
	if workflowID == "" {
		workflowID = "workflow_" + uuid.NewString()
	}
	input.WorkflowID = workflowID
	decision := r.Router.Decide(input.Context)
	input.Decision = decision
	if input.Emitter != nil {
		input.Emitter.Emit(model.EventRunStarted, map[string]any{
			"run_id": input.RunID, "workflow_id": workflowID,
			"target": input.Context.WorkSpec.Target, "interaction": input.Context.WorkSpec.Interaction,
			"user_input": input.Context.WorkSpec.Instruction, "ts": time.Now().Unix(),
		})
		manifest := input.Context.Manifest
		input.Emitter.Emit(model.EventContextAssembled, map[string]any{
			"run_id": input.RunID, "workflow_id": workflowID,
			"context_id": manifest.ContextID, "profile": manifest.Profile,
			"estimated_tokens": manifest.EstimatedTokens, "budget_tokens": manifest.BudgetTokens,
			"segments": len(manifest.Segments), "refs": len(manifest.Refs),
			"warnings": manifest.Warnings, "read_only": manifest.ReadOnly, "ts": time.Now().Unix(),
		})
		input.Emitter.Emit(model.EventStrategySelected, map[string]any{
			"run_id": input.RunID, "workflow_id": workflowID,
			"strategy": decision.Strategy, "reason": decision.Reason,
			"risk": decision.Risk, "complexity": decision.Complexity,
			"signals": decision.Signals, "ts": time.Now().Unix(),
		})
	}
	switch decision.Strategy {
	case StrategyRespond:
		return r.respond(ctx, input)
	case StrategyDirectAction:
		outcome, upgrade := r.direct(ctx, input)
		if !upgrade {
			return outcome
		}
		input.Decision = StrategyDecision{
			Strategy: StrategyCompactWorkflow,
			Reason:   "direct action discovered a wider scope and abandoned staging",
			Risk:     RiskLevelMedium, Complexity: ComplexityMedium,
			Signals: append(decision.Signals, DecisionSignal{Name: "runtime_upgrade", Value: "scope_expanded"}),
		}
		if input.Emitter != nil {
			input.Emitter.Emit(model.EventStrategySelected, map[string]any{
				"run_id": input.RunID, "workflow_id": workflowID,
				"strategy": input.Decision.Strategy, "reason": input.Decision.Reason,
				"risk": input.Decision.Risk, "complexity": input.Decision.Complexity,
				"signals": input.Decision.Signals, "ts": time.Now().Unix(),
			})
		}
		input.PreludeEmitted = true
		return r.Workflow.Run(ctx, input)
	case StrategyCompactWorkflow, StrategyFullPEV:
		input.PreludeEmitted = true
		return r.Workflow.Run(ctx, input)
	default:
		return r.terminal(input, WorkflowState{
			WorkflowID: workflowID, RunID: input.RunID, WorkSpec: input.Context.WorkSpec,
			ContextID: input.Context.Manifest.ContextID, Strategy: decision.Strategy,
			Status: StatusFailed, Changes: EmptyChangeSet(), Issues: []Issue{}, StartedAt: time.Now(),
		}, CodePlanInvalid, errors.New("unknown execution strategy"))
	}
}

func (r *AdaptiveRuntime) respond(ctx context.Context, input RuntimeInput) StructuredOutcome {
	state := WorkflowState{
		WorkflowID: input.WorkflowID, RunID: input.RunID, WorkSpec: input.Context.WorkSpec,
		ContextID: input.Context.Manifest.ContextID, Stage: StageExecute, Strategy: StrategyRespond,
		Status: StatusRunning, Changes: EmptyChangeSet(), Issues: []Issue{}, StartedAt: time.Now(),
	}
	step := WorkflowStep{
		ID: "respond", Kind: StepAnalyze, Title: "Analyze and respond",
		Instruction: input.Context.WorkSpec.Instruction, Targets: targetRefs(input.Context),
		DependsOn: []string{}, Preconditions: []Condition{},
		Capabilities: respondCapabilities(input.Context.WorkSpec), Verifiers: []string{}, Status: StepRunning,
	}
	// Respond deliberately has no WorkflowPlan. The zero-value shell only
	// carries the operation and execution budget required by shared policy.
	state.Plan = WorkflowPlan{Operation: OperationConsult, Budget: DefaultBudget()}
	selection := ToolSelector{Registry: DefaultToolRegistry(input.Context)}.Select(state, step, RiskPolicy{})
	result, err := r.Executor.Execute(ctx, StepInput{
		State: state, Context: input.Context, Step: step, Selection: selection,
		Transaction: nil, Emitter: input.Emitter,
	})
	if err != nil {
		return r.terminal(input, state, CodeStepFailed, err)
	}
	state.Status = StatusCompleted
	outcome := outcomeFromState(state, "", "")
	outcome.Summary = result.Summary
	if input.Emitter != nil {
		input.Emitter.Emit(model.EventStatusSummary, StatusSummaryEvent{
			EventEnvelope: envelope(state), Summary: result.Summary,
		})
		input.Emitter.Emit(model.EventRunCompleted, TerminalEvent{EventEnvelope: envelope(state), Outcome: outcome})
	}
	return outcome
}

func (r *AdaptiveRuntime) direct(ctx context.Context, input RuntimeInput) (StructuredOutcome, bool) {
	state := WorkflowState{
		WorkflowID: input.WorkflowID, RunID: input.RunID, WorkSpec: input.Context.WorkSpec,
		ContextID: input.Context.Manifest.ContextID, Stage: StageExecute, Strategy: StrategyDirectAction,
		Status: StatusRunning, Changes: EmptyChangeSet(), Issues: []Issue{}, StartedAt: time.Now(),
	}
	operation := DeriveOperation(input.Context)
	step := directStep(input.Context)
	state.Plan = WorkflowPlan{
		ID: "", Version: 1, Goal: input.Context.WorkSpec.Instruction,
		Target: input.Context.WorkSpec.Target, Operation: operation,
		Assumptions: []Assumption{}, Affected: append([]ArtifactRef{}, step.Targets...),
		Steps: []WorkflowStep{step}, SuccessCriteria: []Criterion{}, Budget: DefaultBudget(),
	}
	if input.Context.WorkSpec.Interaction.Clarification == model.ClarifyBeforeApply {
		if input.Prompter == nil {
			return r.terminal(input, state, CodeStepFailed, errors.New("before_apply requires a prompter")), false
		}
		answer, err := input.Prompter.NeedsInput(ctx, "before-apply", "将执行一项局部变更，是否继续？", []string{"继续", "取消"})
		if err != nil || answer == "取消" {
			if err == nil {
				err = context.Canceled
			}
			return r.terminal(input, state, CodeWorkflowCanceled, err), false
		}
	}
	tx, err := NewTransaction(input.ProjectDir, input.RunID)
	if err != nil {
		return r.terminal(input, state, CodeStepFailed, err), false
	}
	defer tx.Cleanup()
	if input.Emitter != nil {
		input.Emitter.Emit(model.EventStageStarted, StageEvent{EventEnvelope: envelope(state)})
	}
	selection := ToolSelector{Registry: DefaultToolRegistry(input.Context)}.Select(state, step, r.Risk)
	result, executeErr := r.Executor.Execute(ctx, StepInput{
		State: state, Context: input.Context, Step: step, Selection: selection,
		Transaction: tx, Emitter: input.Emitter,
	})
	if executeErr != nil {
		if errors.Is(executeErr, ErrDirectActionUpgrade) || containsUpgradeIssue(result.Issues) {
			_ = tx.Cleanup()
			return StructuredOutcome{}, true
		}
		return r.terminal(input, state, CodeStepFailed, executeErr), false
	}
	state.Changes = tx.ChangeSet()
	if changeCount(state.Changes) == 0 {
		_ = tx.Cleanup()
		return StructuredOutcome{}, true
	}
	if input.Emitter != nil {
		for _, artifact := range result.Artifacts {
			input.Emitter.Emit(model.EventArtifactStaged, ArtifactEvent{
				EventEnvelope: envelope(state), Artifact: artifact, Change: "staged",
			})
		}
		input.Emitter.Emit(model.EventStageCompleted, StageEvent{EventEnvelope: envelope(state)})
	}
	state.Stage = StageVerify
	verify := r.directVerify(ctx, state, input.Context, tx, input.Emitter)
	state.Issues = verify.Issues
	if !verify.Passed {
		return r.terminal(input, state, CodeVerificationFailed, errors.New("direct action verification failed")), false
	}
	state.Stage = StageCommit
	if err := tx.Commit(ctx, input.CommitMetadata); err != nil {
		return r.terminal(input, state, CodeCommitFailed, err), false
	}
	state.Changes = tx.ChangeSet()
	state.Status, state.Stage = StatusCompleted, StageDeliver
	outcome := outcomeFromState(state, "", "")
	outcome.Summary = firstValue(result.Summary, "Direct action committed")
	if input.Emitter != nil {
		for _, change := range append(append(state.Changes.Created, state.Changes.Updated...), state.Changes.Deleted...) {
			input.Emitter.Emit(model.EventArtifactCommitted, ArtifactEvent{
				EventEnvelope: envelope(state), Artifact: change.Artifact, Change: "committed",
			})
		}
		input.Emitter.Emit(model.EventRunCompleted, TerminalEvent{EventEnvelope: envelope(state), Outcome: outcome})
	}
	return outcome, false
}

func (r *AdaptiveRuntime) directVerify(ctx context.Context, state WorkflowState, pack contextengine.ContextPack, tx *Transaction, emitter EventEmitter) VerifyResult {
	names := []string{"blueprint"}
	if pack.WorkSpec.Target.Artifact == model.ArtifactPresentation {
		names = []string{"presentation_static", "browser"}
	}
	combined := emptyVerifyResult()
	for _, name := range names {
		verifier, ok := r.Verifiers.Get(name)
		if !ok {
			combined.Issues = append(combined.Issues, Issue{
				Code: "VERIFIER_MISSING", Severity: SeverityFatal,
				Evidence: name + " is not registered", Verifier: "runtime",
			})
			continue
		}
		result := verifier.Verify(ctx, VerifyInput{State: state, Plan: state.Plan, Context: pack, Transaction: tx})
		combined.Issues = append(combined.Issues, result.Issues...)
		combined.Evidence = append(combined.Evidence, result.Evidence...)
		if emitter != nil {
			emitter.Emit(model.EventVerificationCompleted, VerificationEvent{
				EventEnvelope: envelope(state), Verifier: name, Result: result,
			})
		}
	}
	combined.Passed = !blocksCommit(combined.Issues)
	return combined
}

func (r *AdaptiveRuntime) terminal(input RuntimeInput, state WorkflowState, code string, err error) StructuredOutcome {
	if err == nil {
		err = errors.New(code)
	}
	event := model.EventRunFailed
	state.Status = StatusFailed
	if errors.Is(err, context.Canceled) || code == CodeWorkflowCanceled {
		event, state.Status = model.EventRunCanceled, StatusCanceled
	}
	outcome := outcomeFromState(state, code, err.Error())
	if input.Emitter != nil {
		input.Emitter.Emit(event, TerminalEvent{EventEnvelope: envelope(state), Outcome: outcome})
	}
	return outcome
}

func directStep(pack contextengine.ContextPack) WorkflowStep {
	target := targetRefs(pack)
	if pack.WorkSpec.Target.Artifact == model.ArtifactBlueprint {
		return WorkflowStep{
			ID: "direct-action", Kind: StepPatchBlueprint, Title: "Apply local blueprint patch",
			Instruction: pack.WorkSpec.Instruction, Targets: target,
			DependsOn: []string{}, Preconditions: []Condition{},
			Capabilities: []Capability{CapabilityReadBlueprint, CapabilityWriteBlueprint, CapabilityControl},
			Verifiers:    []string{"blueprint"}, Status: StepRunning,
		}
	}
	return WorkflowStep{
		ID: "direct-action", Kind: StepReviseSlide, Title: "Apply local presentation patch",
		Instruction: pack.WorkSpec.Instruction, Targets: target,
		DependsOn: []string{}, Preconditions: []Condition{},
		Capabilities: []Capability{
			CapabilityReadPresentation, CapabilityWritePresentation,
			CapabilityReadBlueprint, CapabilityReadDesignSpec, CapabilityControl,
		},
		Verifiers: []string{"presentation_static", "browser"}, Status: StepRunning,
	}
}

func respondCapabilities(spec model.WorkSpec) []Capability {
	caps := []Capability{CapabilityReadContext, CapabilityReadBlueprint, CapabilityReadDesignSpec, CapabilityControl}
	if spec.Target.Artifact == model.ArtifactPresentation {
		caps = append(caps, CapabilityReadPresentation)
	}
	return caps
}

func containsUpgradeIssue(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Code == ErrDirectActionUpgrade.Error() {
			return true
		}
	}
	return false
}

func changeCount(changes ChangeSet) int {
	return len(changes.Created) + len(changes.Updated) + len(changes.Deleted)
}

package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const (
	CodeContextAssemblyFailed = "CONTEXT_ASSEMBLY_FAILED"
	CodePlanInvalid           = "PLAN_INVALID"
	CodeTargetLevelMismatch   = "TARGET_LEVEL_MISMATCH"
	CodeStepFailed            = "STEP_FAILED"
	CodeVerificationFailed    = "VERIFICATION_FAILED"
	CodeRepairExhausted       = "REPAIR_EXHAUSTED"
	CodeRevisionConflict      = "REVISION_CONFLICT"
	CodeCommitFailed          = "COMMIT_FAILED"
	CodeWorkflowCanceled      = "WORKFLOW_CANCELED"
	CodeWorkflowBudget        = "WORKFLOW_BUDGET_EXCEEDED"
)

type Prompter interface {
	NeedsInput(context.Context, string, string, []string) (string, error)
}

type StageHook interface {
	BeforeStage(context.Context, WorkflowState) error
	AfterStage(context.Context, WorkflowState) error
}

type RuntimeInput struct {
	RunID          string
	ProjectDir     string
	Context        contextengine.ContextPack
	Emitter        EventEmitter
	Prompter       Prompter
	CommitMetadata CommitMetadata
	Decision       StrategyDecision
	PreludeEmitted bool
	WorkflowID     string
}

type Engine struct {
	Planner   Planner
	Executor  StepExecutor
	Verifiers *VerifierRegistry
	Hooks     []StageHook
	Risk      RiskPolicy
}

func NewEngine(executor StepExecutor) *Engine {
	return &Engine{
		Planner: Planner{}, Executor: executor,
		Verifiers: NewVerifierRegistry(
			BlueprintVerifier{}, PresentationStaticVerifier{},
			BrowserVerifier{}, CrossSlideVerifier{},
		),
		Hooks: []StageHook{},
	}
}

func (e *Engine) Run(ctx context.Context, input RuntimeInput) StructuredOutcome {
	workflowID := input.WorkflowID
	if workflowID == "" {
		workflowID = "workflow_" + uuid.NewString()
	}
	state := WorkflowState{
		WorkflowID: workflowID, RunID: input.RunID,
		WorkSpec: input.Context.WorkSpec, ContextID: input.Context.Manifest.ContextID,
		Issues: []Issue{}, Changes: EmptyChangeSet(), Status: StatusRunning, StartedAt: time.Now(),
		Strategy: input.Decision.Strategy,
	}
	if state.Strategy == "" {
		state.Strategy = StrategyFullPEV
	}
	if !input.PreludeEmitted && input.Emitter != nil {
		input.Emitter.Emit(model.EventRunStarted, map[string]any{
			"run_id": input.RunID, "workflow_id": state.WorkflowID,
			"target": state.WorkSpec.Target, "interaction": state.WorkSpec.Interaction,
			"user_input": state.WorkSpec.Instruction, "ts": time.Now().Unix(),
		})
	}
	fail := func(code string, err error) StructuredOutcome {
		if err == nil {
			err = errors.New(code)
		}
		status := StatusFailed
		event := model.EventRunFailed
		if errors.Is(err, context.Canceled) || code == CodeWorkflowCanceled {
			status, event = StatusCanceled, model.EventRunCanceled
		}
		state.Status = status
		now := time.Now()
		state.CompletedAt = &now
		outcome := outcomeFromState(state, code, err.Error())
		if input.Emitter != nil {
			input.Emitter.Emit(event, TerminalEvent{EventEnvelope: envelope(state), Outcome: outcome})
		}
		return outcome
	}
	if input.PreludeEmitted {
		state.Stage = StageContext
	} else {
		if err := e.enterStage(ctx, &state, StageNormalize, input.Emitter); err != nil {
			return fail(codeForContext(err), err)
		}
		if err := state.WorkSpec.Validate(); err != nil {
			return fail(CodePlanInvalid, err)
		}
		if err := e.leaveStage(ctx, state, input.Emitter, time.Now()); err != nil {
			return fail(CodeStepFailed, err)
		}

		if err := e.moveStage(ctx, &state, StageContext, input.Emitter); err != nil {
			return fail(codeForContext(err), err)
		}
		if input.Context.Manifest.ContextID == "" {
			return fail(CodeContextAssemblyFailed, errors.New("ContextPack has no manifest identity"))
		}
		if input.Emitter != nil {
			manifest := input.Context.Manifest
			input.Emitter.Emit(model.EventContextAssembled, map[string]any{
				"run_id": input.RunID, "workflow_id": state.WorkflowID, "stage": state.Stage,
				"context_id": manifest.ContextID, "profile": manifest.Profile,
				"estimated_tokens": manifest.EstimatedTokens, "budget_tokens": manifest.BudgetTokens,
				"segments": len(manifest.Segments), "refs": len(manifest.Refs),
				"warnings": manifest.Warnings, "read_only": manifest.ReadOnly, "ts": time.Now().Unix(),
			})
		}
		if err := e.leaveStage(ctx, state, input.Emitter, time.Now()); err != nil {
			return fail(CodeStepFailed, err)
		}
	}

	if err := e.moveStage(ctx, &state, StagePlan, input.Emitter); err != nil {
		return fail(codeForContext(err), err)
	}
	book, err := PlaybookFor(state.WorkSpec)
	if err != nil {
		return fail(CodePlanInvalid, err)
	}
	operation := DeriveOperation(input.Context)
	planner := e.Planner
	if state.Strategy == StrategyCompactWorkflow && !needsSemanticPlanner(input.Decision) {
		planner = Planner{}
	}
	plan, err := planner.Create(input.Context, book, operation)
	if err != nil {
		return fail(CodePlanInvalid, err)
	}
	state.Plan = plan
	if input.Emitter != nil {
		input.Emitter.Emit(model.EventPlanCreated, PlanCreatedEvent{EventEnvelope: envelope(state), Plan: plan})
	}
	if err := e.leaveStage(ctx, state, input.Emitter, time.Now()); err != nil {
		return fail(CodeStepFailed, err)
	}

	if state.WorkSpec.Interaction.Intent == model.IntentApply &&
		state.WorkSpec.Interaction.Clarification == model.ClarifyBeforeApply {
		if input.Prompter == nil {
			return fail(CodeStepFailed, errors.New("before_apply requires a prompter"))
		}
		answer, promptErr := input.Prompter.NeedsInput(ctx, "before-apply", "计划已生成，是否执行变更？", []string{"继续", "取消"})
		if promptErr != nil || answer == "取消" {
			if promptErr == nil {
				promptErr = context.Canceled
			}
			return fail(CodeWorkflowCanceled, promptErr)
		}
	}

	var tx *Transaction
	if operation != OperationConsult {
		tx, err = NewTransaction(input.ProjectDir, input.RunID)
		if err != nil {
			return fail(CodeStepFailed, err)
		}
		defer func() { _ = tx.Cleanup() }()
	}
	registry := DefaultToolRegistry(input.Context)
	selector := ToolSelector{Registry: registry}

	if err := e.moveStage(ctx, &state, StageExecute, input.Emitter); err != nil {
		return fail(codeForContext(err), err)
	}
	for index := range state.Plan.Steps {
		step := &state.Plan.Steps[index]
		if err := e.checkBudget(ctx, state); err != nil {
			return fail(codeForContext(err), err)
		}
		if !dependenciesComplete(state.Plan, *step) {
			return fail(CodePlanInvalid, fmt.Errorf("step dependencies are incomplete for %s", step.ID))
		}
		state.CurrentStepID, state.Attempt = step.ID, 1
		step.Status = StepRunning
		started := time.Now()
		if input.Emitter != nil {
			input.Emitter.Emit(model.EventStepStarted, StepEvent{
				EventEnvelope: envelope(state), Kind: step.Kind, Title: step.Title, Status: step.Status,
			})
		}
		selection := selector.Select(state, *step, e.Risk)
		result, stepErr := e.Executor.Execute(ctx, StepInput{
			State: state, Context: input.Context, Step: *step, Selection: selection,
			Transaction: tx, Emitter: input.Emitter,
		})
		if stepErr != nil {
			step.Status = StepFailed
			state.Issues = append(state.Issues, result.Issues...)
			if input.Emitter != nil {
				input.Emitter.Emit(model.EventStepFailed, StepEvent{
					EventEnvelope: envelope(state), Kind: step.Kind, Title: step.Title, Status: step.Status,
					DurationMS: time.Since(started).Milliseconds(), Summary: result.Summary, Code: CodeStepFailed,
				})
			}
			return fail(CodeStepFailed, stepErr)
		}
		step.Status = StepCompleted
		if tx != nil {
			state.Changes = tx.ChangeSet()
		}
		if input.Emitter != nil {
			input.Emitter.Emit(model.EventStepCompleted, StepEvent{
				EventEnvelope: envelope(state), Kind: step.Kind, Title: step.Title, Status: step.Status,
				DurationMS: time.Since(started).Milliseconds(), Summary: result.Summary,
			})
			for _, artifact := range result.Artifacts {
				input.Emitter.Emit(model.EventArtifactStaged, ArtifactEvent{
					EventEnvelope: envelope(state), Artifact: artifact, Change: "staged",
				})
			}
			if result.Summary != "" {
				input.Emitter.Emit(model.EventStatusSummary, StatusSummaryEvent{
					EventEnvelope: envelope(state), Summary: result.Summary,
				})
			}
		}
	}
	state.CurrentStepID = ""
	if err := e.leaveStage(ctx, state, input.Emitter, time.Now()); err != nil {
		return fail(CodeStepFailed, err)
	}

	if err := e.moveStage(ctx, &state, StageVerify, input.Emitter); err != nil {
		return fail(codeForContext(err), err)
	}
	verify := e.verify(ctx, state, input.Context, tx, book, input.Emitter)
	state.Issues = verify.Issues
	if err := e.leaveStage(ctx, state, input.Emitter, time.Now()); err != nil {
		return fail(CodeStepFailed, err)
	}

	if !verify.Passed {
		if err := e.moveStage(ctx, &state, StageRepair, input.Emitter); err != nil {
			return fail(codeForContext(err), err)
		}
		previousBlocking := blockingCount(verify.Issues)
		for round := 1; round <= state.Plan.Budget.MaxRepairRounds; round++ {
			state.RepairRound, state.Attempt = round, round
			repairStep := repairStepFor(state, verify.Issues)
			state.CurrentStepID = repairStep.ID
			if input.Emitter != nil {
				input.Emitter.Emit(model.EventRepairStarted, RepairEvent{
					EventEnvelope: envelope(state), Round: round, Artifact: firstTarget(repairStep),
				})
			}
			selection := selector.Select(state, repairStep, e.Risk)
			result, repairErr := e.Executor.Execute(ctx, StepInput{
				State: state, Context: input.Context, Step: repairStep,
				Selection: selection, Transaction: tx, Emitter: input.Emitter,
			})
			if repairErr != nil {
				state.Issues = append(state.Issues, result.Issues...)
				break
			}
			if input.Emitter != nil {
				input.Emitter.Emit(model.EventRepairCompleted, RepairEvent{
					EventEnvelope: envelope(state), Round: round, Artifact: firstTarget(repairStep), Summary: result.Summary,
				})
			}
			if err := e.leaveStage(ctx, state, input.Emitter, time.Now()); err != nil {
				return fail(CodeStepFailed, err)
			}
			if err := e.moveStage(ctx, &state, StageVerify, input.Emitter); err != nil {
				return fail(codeForContext(err), err)
			}
			verify = e.verify(ctx, state, input.Context, tx, book, input.Emitter)
			state.Issues = verify.Issues
			if verify.Passed {
				break
			}
			currentBlocking := blockingCount(verify.Issues)
			if currentBlocking >= previousBlocking {
				break
			}
			previousBlocking = currentBlocking
			if err := e.leaveStage(ctx, state, input.Emitter, time.Now()); err != nil {
				return fail(CodeStepFailed, err)
			}
			if err := e.moveStage(ctx, &state, StageRepair, input.Emitter); err != nil {
				return fail(codeForContext(err), err)
			}
		}
	}
	state.CurrentStepID = ""
	if state.Stage == StageRepair {
		if err := e.leaveStage(ctx, state, input.Emitter, time.Now()); err != nil {
			return fail(CodeStepFailed, err)
		}
	} else if state.Stage == StageVerify {
		if err := e.leaveStage(ctx, state, input.Emitter, time.Now()); err != nil {
			return fail(CodeStepFailed, err)
		}
	}
	if !verify.Passed {
		return fail(CodeRepairExhausted, errors.New("blocking verification issues remain after repair"))
	}

	if err := e.moveStage(ctx, &state, StageCommit, input.Emitter); err != nil {
		return fail(codeForContext(err), err)
	}
	if tx != nil {
		if err := tx.Commit(ctx, input.CommitMetadata); err != nil {
			return fail(CodeCommitFailed, err)
		}
		state.Changes = tx.ChangeSet()
		if input.Emitter != nil {
			for _, change := range append(append(state.Changes.Created, state.Changes.Updated...), state.Changes.Deleted...) {
				input.Emitter.Emit(model.EventArtifactCommitted, ArtifactEvent{
					EventEnvelope: envelope(state), Artifact: change.Artifact, Change: "committed",
				})
			}
		}
	}
	if err := e.leaveStage(ctx, state, input.Emitter, time.Now()); err != nil {
		return fail(CodeCommitFailed, err)
	}

	if err := e.moveStage(ctx, &state, StageDeliver, input.Emitter); err != nil {
		return fail(codeForContext(err), err)
	}
	state.Status = StatusCompleted
	now := time.Now()
	state.CompletedAt = &now
	outcome := outcomeFromState(state, "", "")
	outcome.Summary = deliverySummary(state)
	if err := e.leaveStage(ctx, state, input.Emitter, time.Now()); err != nil {
		return fail(CodeStepFailed, err)
	}
	if input.Emitter != nil {
		input.Emitter.Emit(model.EventRunCompleted, TerminalEvent{EventEnvelope: envelope(state), Outcome: outcome})
	}
	return outcome
}

func (e *Engine) enterStage(ctx context.Context, state *WorkflowState, stage Stage, emitter EventEmitter) error {
	if err := Transition(state, stage); err != nil {
		return err
	}
	for _, hook := range e.Hooks {
		if err := hook.BeforeStage(ctx, *state); err != nil {
			return err
		}
	}
	if emitter != nil {
		emitter.Emit(model.EventStageStarted, StageEvent{EventEnvelope: envelope(*state)})
	}
	return e.checkBudget(ctx, *state)
}

func (e *Engine) moveStage(ctx context.Context, state *WorkflowState, stage Stage, emitter EventEmitter) error {
	return e.enterStage(ctx, state, stage, emitter)
}

func (e *Engine) leaveStage(ctx context.Context, state WorkflowState, emitter EventEmitter, started time.Time) error {
	for _, hook := range e.Hooks {
		if err := hook.AfterStage(ctx, state); err != nil {
			return err
		}
	}
	if emitter != nil {
		emitter.Emit(model.EventStageCompleted, StageEvent{
			EventEnvelope: envelope(state), DurationMS: time.Since(started).Milliseconds(),
		})
	}
	return nil
}

func (e *Engine) checkBudget(ctx context.Context, state WorkflowState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	budget := state.Plan.Budget
	if budget.MaxDuration == 0 {
		budget = DefaultBudget()
	}
	if time.Since(state.StartedAt) > budget.MaxDuration {
		return errors.New(CodeWorkflowBudget)
	}
	return nil
}

func (e *Engine) verify(ctx context.Context, state WorkflowState, pack contextengine.ContextPack, tx *Transaction, book Playbook, emitter EventEmitter) VerifyResult {
	combined := emptyVerifyResult()
	if state.Plan.Operation == OperationConsult {
		return combined
	}
	for _, name := range book.Verifiers() {
		verifier, ok := e.Verifiers.Get(name)
		if !ok {
			combined.Issues = append(combined.Issues, Issue{
				Code: "VERIFIER_MISSING", Severity: SeverityFatal,
				Evidence: "required verifier " + name + " is not registered", Verifier: "runtime",
			})
			continue
		}
		result := verifier.Verify(ctx, VerifyInput{State: state, Plan: state.Plan, Context: pack, Transaction: tx})
		combined.Issues = append(combined.Issues, result.Issues...)
		combined.Evidence = append(combined.Evidence, result.Evidence...)
		for key, value := range result.Metrics {
			combined.Metrics[name+"."+key] = value
		}
		if emitter != nil {
			emitter.Emit(model.EventVerificationCompleted, VerificationEvent{
				EventEnvelope: envelope(state), Verifier: name, Result: result,
			})
		}
	}
	combined.Passed = !blocksCommit(combined.Issues)
	return combined
}

func repairStepFor(state WorkflowState, issues []Issue) WorkflowStep {
	targets := []ArtifactRef{}
	caps := []Capability{CapabilityControl}
	seen := map[string]bool{}
	for _, issue := range issues {
		if !issue.Severity.BlocksCommit() || seen[issue.Artifact.Key()] {
			continue
		}
		targets = append(targets, issue.Artifact)
		seen[issue.Artifact.Key()] = true
		switch issue.Artifact.Kind {
		case ArtifactDeck, ArtifactSlide:
			caps = appendUniqueCapability(caps, CapabilityReadBlueprint, CapabilityWriteBlueprint)
		case ArtifactDesign:
			caps = appendUniqueCapability(caps, CapabilityReadDesignSpec, CapabilityWriteDesignSpec)
		case ArtifactPresentation:
			caps = appendUniqueCapability(caps, CapabilityReadPresentation, CapabilityWritePresentation, CapabilityReadBlueprint, CapabilityReadDesignSpec)
		}
	}
	return WorkflowStep{
		ID: fmt.Sprintf("repair-%d", state.RepairRound), Kind: StepRepair,
		Title: "Repair verified issues", Instruction: repairInstruction(issues),
		Targets: targets, DependsOn: []string{}, Preconditions: []Condition{},
		Capabilities: caps, Verifiers: []string{}, Status: StepRunning,
	}
}

func repairInstruction(issues []Issue) string {
	message := "Repair only these verifier issues:\n"
	for _, issue := range issues {
		if issue.Severity.BlocksCommit() {
			message += fmt.Sprintf("- %s on %s: %s. Hint: %s\n", issue.Code, issue.Artifact.Key(), issue.Evidence, issue.RepairHint)
		}
	}
	return message
}

func appendUniqueCapability(existing []Capability, values ...Capability) []Capability {
	seen := map[Capability]bool{}
	for _, value := range existing {
		seen[value] = true
	}
	for _, value := range values {
		if !seen[value] {
			existing = append(existing, value)
			seen[value] = true
		}
	}
	return existing
}

func dependenciesComplete(plan WorkflowPlan, step WorkflowStep) bool {
	status := map[string]StepStatus{}
	for _, candidate := range plan.Steps {
		status[candidate.ID] = candidate.Status
	}
	for _, dependency := range step.DependsOn {
		if status[dependency] != StepCompleted {
			return false
		}
	}
	return true
}

func blockingCount(issues []Issue) int {
	count := 0
	for _, issue := range issues {
		if issue.Severity.BlocksCommit() {
			count++
		}
	}
	return count
}

func firstTarget(step WorkflowStep) ArtifactRef {
	if len(step.Targets) == 0 {
		return ArtifactRef{}
	}
	return step.Targets[0]
}

func codeForContext(err error) string {
	if errors.Is(err, context.Canceled) {
		return CodeWorkflowCanceled
	}
	if err != nil && err.Error() == CodeWorkflowBudget {
		return CodeWorkflowBudget
	}
	return CodeStepFailed
}

func outcomeFromState(state WorkflowState, code, message string) StructuredOutcome {
	return StructuredOutcome{
		WorkflowID: state.WorkflowID, Strategy: state.Strategy, Status: state.Status, Operation: state.Plan.Operation,
		Target: state.WorkSpec.Target, Affected: state.Plan.Affected,
		Changes: state.Changes, Issues: append([]Issue{}, state.Issues...),
		RepairRounds: state.RepairRound, Code: code, Message: message,
	}
}

func deliverySummary(state WorkflowState) string {
	if state.Plan.Operation == OperationConsult {
		for i := len(state.Plan.Steps) - 1; i >= 0; i-- {
			if state.Plan.Steps[i].Status == StepCompleted {
				return "Consultation completed"
			}
		}
	}
	return fmt.Sprintf("%s completed and committed %d artifact changes", state.Plan.Operation,
		len(state.Changes.Created)+len(state.Changes.Updated)+len(state.Changes.Deleted))
}

func needsSemanticPlanner(decision StrategyDecision) bool {
	for _, signal := range decision.Signals {
		if signal.Name == "semantic_ambiguity" && signal.Value == "true" {
			return true
		}
	}
	return false
}

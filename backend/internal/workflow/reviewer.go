package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"go.uber.org/zap"
)

const CodeReviewFailed = "REVIEW_FAILED"
const reviewMaxEvidenceRenders = 128
const reviewMaxOutputTokens = 4096

type TaskReviewer interface {
	Review(context.Context, ReviewInput) (model.ReviewResult, error)
}

type ReviewInput struct {
	Material      ReviewMaterial
	Images        []llm.ContentPart
	ImageResolver llm.ImageRefResolver
	CheckBudget   func(context.Context) error
	Diagnose      func(map[string]any)
}

type LLMTaskReviewer struct{ Provider llm.Provider }

func submitReviewSchema() ToolSchema {
	return ToolSchema{Name: "submit_review", OutputSchema: toolOutputSchema("submit_review"), Description: "Assess the backend-prepared evidence and submit the final artifact review. A valid submission ends this review without a reply. Call it as the only tool call in the submission response. Invalid submissions may receive error feedback and be corrected within the shared deadline, output, context and two-additional-request budget. Reasons are required for all outcomes, including approval. This does not finish the main task or change any artifact.",
		Parameters: objectSchema([]string{"decision", "reasons"}, map[string]any{
			"decision": map[string]any{"type": "string", "enum": []string{"approve", "revise", "refuse"}, "description": "Artifact review outcome: approve when checks support delivery, revise when further verification or revision is needed, or refuse when confirmed defects block delivery. This is a review assessment, not user approval or main-task completion."},
			"reasons":  map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string", "minLength": 1, "description": "One concrete review finding with the relevant page, observed evidence and impact, or the specific uncertainty or revision to address."}, "description": "Concrete reasons in the user's language, required for every outcome. For approve explain what was verified and supports delivery; for revise explain what needs verification or revision; for refuse explain the confirmed defects blocking delivery. Report findings, not internal deliberation."},
		}),
	}
}

func parseReviewSubmission(call llm.ToolCall) (model.ReviewResult, error) {
	if call.Name != "submit_review" {
		return model.ReviewResult{}, errors.New("review must end with submit_review")
	}
	if err := validateToolArguments(submitReviewSchema(), call.Args); err != nil {
		return model.ReviewResult{}, err
	}
	raw, err := json.Marshal(call.Args)
	if err != nil {
		return model.ReviewResult{}, err
	}
	var result model.ReviewResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, err
	}
	for i := range result.Reasons {
		result.Reasons[i] = strings.TrimSpace(result.Reasons[i])
	}
	return result, result.Validate()
}

const reviewCorrectionGuidance = "The review result has not been submitted. Call submit_review alone with decision and non-empty reasons, using the supplied evidence. No reading or rendering tools are available. Correct only the reported submission issue; runtime feedback does not dictate the verdict."

func validateReviewResponse(response llm.GenerateResponse) error {
	if err := llm.ValidateSubmissionEnvelope(response, "submit_review", false); err != nil {
		return err
	}
	if _, err := parseReviewSubmission(response.ToolCalls[0]); err != nil {
		field := "/"
		var argument *toolArgumentError
		if errors.As(err, &argument) {
			field = argument.Field
		}
		args := response.ToolCalls[0].Args
		if _, ok := args["reasons"]; !ok {
			field = "/reasons"
		}
		if _, ok := args["decision"]; !ok {
			field = "/decision"
		}
		return llm.SubmissionFailure("INVALID_ARGUMENTS", field, "Invalid submit_review arguments at "+field+": supply only decision (approve, revise or refuse) and reasons (a non-empty array of non-empty plain-text strings).")
	}
	return nil
}

func (r LLMTaskReviewer) Review(ctx context.Context, input ReviewInput) (model.ReviewResult, error) {
	if r.Provider == nil {
		return model.ReviewResult{}, errors.New("review provider is unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	raw, err := json.Marshal(input.Material)
	if err != nil {
		return model.ReviewResult{}, err
	}
	session := llm.NewSubmissionSession("review", reviewMaxOutputTokens)
	session.Check, session.Diagnose = input.CheckBudget, input.Diagnose
	response, err := session.Generate(ctx, r.Provider, llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: llm.TextContent(reviewerPrompt())},
			{Role: llm.RoleUser, Content: append(llm.TextContent(string(raw)), input.Images...)},
		},
		Tools: []llm.ToolSchema{llm.ToolSchema(submitReviewSchema())}, ImageResolver: input.ImageResolver, MaxOutputTokens: reviewMaxOutputTokens,
	}, validateReviewResponse, reviewCorrectionGuidance)
	if err != nil {
		return model.ReviewResult{}, err
	}
	return parseReviewSubmission(response.ToolCalls[0])
}

func (r *Runtime) runReviewTask(ctx context.Context, input RuntimeInput, state *RunState, call llm.ToolCall) ToolResult {
	r.emitProgress(input.Emitter, state, model.ActivityPresentationReviewing)
	projector := ToolPublicProjector{ProjectDir: input.ProjectDir, TextContext: state.publicTextContext()}
	if input.Emitter != nil {
		if event, ok := projector.Started(state.runID, call.ID, call.Name, call.Args, "", nil); ok {
			input.Emitter.Emit(model.EventToolStarted, event)
		}
	}
	outcome, err := r.reviewArtifacts(ctx, input, state, call)
	result := SuccessfulToolResult("artifact review completed")
	if err != nil {
		result = failedToolResult(CodeReviewFailed, err.Error(), false)
		if ctx.Err() != nil {
			result = failedToolResult(CodeCanceled, "artifact review canceled", false)
		}
	} else {
		result.Data = map[string]any{"decision": outcome.Decision, "reasons": outcome.Reasons}
		raw, _ := json.Marshal(result.Data)
		result.Observation = string(raw)
	}
	recordTrace(input.Trace, state.runID, "review.completed", map[string]any{"call_id": call.ID, "ok": result.OK, "decision": outcome.Decision, "reason_count": len(outcome.Reasons), "failure_code": result.Code, "prompt_manifest": reviewerPromptManifest()})
	return result
}

func (r *Runtime) reviewArtifacts(ctx context.Context, input RuntimeInput, state *RunState, call llm.ToolCall) (result model.ReviewResult, resultErr error) {
	// Evidence preparation and every correction share the existing five minutes.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	started := time.Now()
	stage := "evidence_preparation"
	diagnose := func(event string, d map[string]any) {
		d["parent_call_id"] = call.ID
		recordTrace(input.Trace, state.runID, event, d)
		if input.Logger != nil {
			input.Logger.Info(event, zap.String("run_id", state.runID), zap.Any("diagnostic", d))
		}
	}
	defer func() {
		termination := "accepted"
		if resultErr != nil {
			termination = stage + "_failed"
		}
		if ctx.Err() != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				termination = "timed_out"
			} else {
				termination = "canceled_or_superseded"
			}
		}
		diagnose("review.finished", map[string]any{"elapsed_ms": time.Since(started).Milliseconds(), "disposition": termination, "error_class": llm.ProviderFailureDiagnostic(resultErr)})
	}()
	if input.Reviewer == nil {
		return model.ReviewResult{}, errors.New("review provider is unavailable")
	}
	if err := r.checkBudget(ctx, state); err != nil {
		return model.ReviewResult{}, err
	}
	preparationState := *state
	var requirementsHash string
	var loadRequirements func(context.Context) ([]contextengine.ReviewUserInput, error)
	if input.Transcript != nil && input.Context.Manifest.ThreadID != "" {
		reader, ok := input.Transcript.(interface {
			LoadReviewUserInputs(context.Context, string, string) ([]contextengine.ReviewUserInput, error)
		})
		if !ok {
			return model.ReviewResult{}, errors.New("authoritative review user requirements are unavailable; compacted history cannot replace complete requirements")
		}
		loadRequirements = func(ctx context.Context) ([]contextengine.ReviewUserInput, error) {
			return reader.LoadReviewUserInputs(ctx, input.ProjectDir, input.Context.Manifest.ThreadID)
		}
		inputs, err := loadRequirements(ctx)
		if err != nil {
			return model.ReviewResult{}, err
		}
		requirementsHash = hashCheckpointValue(inputs)
		preparationState.reviewInstructions = []ReviewInstruction{}
		seen := map[string]bool{}
		for _, user := range inputs {
			instruction := ReviewInstruction{Text: user.Text, Attachments: user.Attachments, DOMSelections: user.DOMSelections}
			seen[hashCheckpointValue(instruction)] = true
			instruction.RunID = user.RunID
			preparationState.reviewInstructions = append(preparationState.reviewInstructions, instruction)
		}
		for _, instruction := range state.reviewInstructions {
			key := instruction
			key.RunID = ""
			if !seen[hashCheckpointValue(key)] {
				preparationState.reviewInstructions = append(preparationState.reviewInstructions, instruction)
			}
		}
	}
	prepareRender := func(ctx context.Context, id string, scope model.RunScope) (ToolResult, error) {
		if err := r.checkBudget(ctx, state); err != nil {
			return ToolResult{}, err
		}
		descriptor, ok := state.tools.Descriptor("render_slide")
		if !ok || !descriptor.ReadOnly {
			return ToolResult{}, errors.New("backend review renderer is unavailable")
		}
		result := descriptor.Tool.Execute(ctx, DomainToolInput{Args: map[string]any{"slide_id": id}, CallID: call.ID + ":evidence:" + id, Context: state.pack, ProjectDir: state.projectDir, RunID: state.runID, Session: state.tx, Scope: scope, Phase: PhaseExecuting, Mode: state.mode})
		state.toolCalls++
		// Review preparation owns its derived evidence. It must not change the main
		// agent's creation loop, failure counters or authored content.
		return result, nil
	}
	material, images, err := prepareReviewMaterial(ctx, &preparationState, strings.TrimSpace(stringValue(call.Args["demand"])), prepareRender)
	diagnose("review.evidence", map[string]any{"ok": err == nil, "elapsed_ms": time.Since(started).Milliseconds(), "evidence_version": material.EvidenceVersion, "source_count": len(material.Sources), "image_count": len(material.ImageEvidence)})
	if err != nil {
		return model.ReviewResult{}, err
	}
	checkCurrent := func(ctx context.Context) error {
		if err := r.checkBudget(ctx, state); err != nil {
			return err
		}
		if loadRequirements != nil {
			inputs, err := loadRequirements(ctx)
			if err != nil {
				return err
			}
			if hashCheckpointValue(inputs) != requirementsHash {
				return errors.New("user requirements changed during review; the assessment is stale")
			}
		}
		return validateReviewEvidence(ctx, state, material)
	}
	request := ReviewInput{Material: material, Images: images, ImageResolver: reviewImageResolver(material.imageData), CheckBudget: checkCurrent,
		Diagnose: func(d map[string]any) { diagnose("review.protocol", d) },
	}
	stage = "model_execution"
	result, err = input.Reviewer.Review(ctx, request)
	if err != nil {
		return model.ReviewResult{}, err
	}
	stage = "result_validation"
	if err := result.Validate(); err != nil {
		return model.ReviewResult{}, err
	}
	stage = "evidence_consistency"
	if err := checkCurrent(ctx); err != nil {
		return model.ReviewResult{}, err
	}
	return result, nil
}

func reviewerPrompt() string {
	return prompts.MustLoad("core.quality").Body + "\n\n" + prompts.MustLoad("subagent.reviewer.agent").Body
}

func reviewerPromptManifest() string {
	modules := []map[string]string{}
	for _, id := range []string{"core.quality", "subagent.reviewer.agent"} {
		m := prompts.MustLoad(id)
		modules = append(modules, map[string]string{"id": m.ID, "path": m.Path, "version": m.Version, "hash": m.Hash})
	}
	raw, _ := json.Marshal(map[string]any{"modules": modules})
	return string(raw)
}

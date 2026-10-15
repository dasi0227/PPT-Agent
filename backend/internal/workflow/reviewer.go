package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"go.uber.org/zap"
)

const CodeReviewFailed = "REVIEW_FAILED"
const reviewMaxEvidenceRenders = 128
const reviewMaxOutputTokens = 4096

func reviewMaterialError(err error) error {
	return model.NewAgentError(CodeReviewFailed, "review_material", err)
}

func reviewFailure(err error) ToolResult {
	var failure *model.AgentError
	if errors.As(err, &failure) && !model.IsCommonErrorCode(failure.Code) && failure.Code != CodeReviewFailed {
		switch failure.Code {
		case CodeRenderFailed:
			return failedToolResult(CodeReviewFailed, err.Error())
		default:
			return failedToolResult(model.CodeInternal, err.Error())
		}
	}
	return internalToolFailure(err)
}

type TaskReviewer interface {
	Review(context.Context, ReviewInput) (model.ReviewResult, error)
}

type ReviewInput struct {
	Material      ReviewMaterial
	ImageResolver llm.ImageRefResolver
	CheckBudget   func(context.Context) error
	Diagnose      func(map[string]any)
}

type LLMTaskReviewer struct{ Provider llm.Provider }

func parseReviewResponse(response llm.GenerateResponse) (model.ReviewResult, error) {
	if len(response.ToolCalls) != 0 {
		return model.ReviewResult{}, errors.New("review output cannot contain tool calls")
	}
	for _, part := range response.Content {
		if part.Type != "text" {
			return model.ReviewResult{}, errors.New("review output must contain only JSON text")
		}
	}
	return model.ParseReviewResult(response.Text())
}

func (r LLMTaskReviewer) Review(ctx context.Context, input ReviewInput) (model.ReviewResult, error) {
	if r.Provider == nil {
		return model.ReviewResult{}, errors.New("review provider is unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	check := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if input.CheckBudget != nil {
			return input.CheckBudget(ctx)
		}
		return nil
	}
	if err := check(); err != nil {
		return model.ReviewResult{}, err
	}
	content, err := input.Material.content()
	if err != nil {
		return model.ReviewResult{}, err
	}
	report := func(event string, fields map[string]any) {
		if input.Diagnose != nil {
			fields["event"] = event
			input.Diagnose(fields)
		}
	}
	req := llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: llm.TextContent(reviewerPrompt())},
			{Role: llm.RoleUser, Content: content},
		},
		ImageResolver: input.ImageResolver, MaxOutputTokens: reviewMaxOutputTokens,
		OnRequest: func(d llm.RequestDiagnostic) { report("http_request", map[string]any{"request": d}) },
		OnRetry:   func(attempt int) { report("network_retry", map[string]any{"retry_attempt": attempt}) },
	}
	if maximum := r.Provider.Capabilities().ContextWindowTokens; maximum > 0 && llm.EstimateRequestTokens(req)+req.MaxOutputTokens >= maximum {
		return model.ReviewResult{}, reviewMaterialError(errors.New("review evidence exceeds model context window; no required input was omitted"))
	}
	started := time.Now()
	response, err := r.Provider.Generate(ctx, req)
	report("request_completed", map[string]any{"elapsed_ms": time.Since(started).Milliseconds(), "provider_error": llm.ProviderFailureDiagnostic(err)})
	if checkErr := check(); checkErr != nil {
		return model.ReviewResult{}, checkErr
	}
	if err != nil {
		return model.ReviewResult{}, err
	}
	used := response.Usage.OutputTokens
	if used <= 0 {
		used = llm.EstimateTextTokens(response.Text()) + llm.EstimateValueTokens(response.ToolCalls)
	}
	if used > reviewMaxOutputTokens {
		return model.ReviewResult{}, model.NewAgentError(CodeReviewFailed, "review_output", errors.New("review output budget exceeded"))
	}
	result, err := parseReviewResponse(response)
	if err != nil {
		return model.ReviewResult{}, model.NewAgentError(CodeReviewFailed, "review_output", err)
	}
	return result, nil
}

func (r *Runtime) runReviewPPT(ctx context.Context, input RuntimeInput, state *RunState, call llm.ToolCall) ToolResult {
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
		result = reviewFailure(err)
		if errors.Is(ctx.Err(), context.Canceled) {
			result = failedToolResult(CodeCanceled, "artifact review canceled")
		}
	} else {
		result.Data = map[string]any{"decision": outcome.Decision, "reasons": outcome.Reasons}
		result.Observation = modelToolObservation(result)
	}
	recordTrace(input.Trace, state.runID, "review.completed", map[string]any{"call_id": call.ID, "ok": result.OK, "decision": outcome.Decision, "reason_count": len(outcome.Reasons), "failure_code": result.Code, "prompt_manifest": reviewerPromptManifest()})
	return result
}

func (r *Runtime) reviewArtifacts(ctx context.Context, input RuntimeInput, state *RunState, call llm.ToolCall) (result model.ReviewResult, resultErr error) {
	// Evidence preparation and the independent model request share five minutes.
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
	prepareRender := func(ctx context.Context, id string, scope model.RunScope) (ToolResult, error) {
		if err := r.checkBudget(ctx, state); err != nil {
			return ToolResult{}, err
		}
		descriptor, ok := state.tools.Descriptor("render_slide")
		if !ok || !descriptor.ReadOnly {
			return ToolResult{}, errors.New("backend review renderer is unavailable")
		}
		result := descriptor.Tool.Execute(ctx, DomainToolInput{Args: map[string]any{"slide_id": id, "need_preview": false}, CallID: call.ID + ":evidence:" + id, Context: state.pack, ProjectDir: state.projectDir, RunID: state.runID, Session: state.tx, Scope: scope, Phase: PhaseExecuting, Mode: state.mode})
		state.toolCalls++
		// Review preparation owns its derived evidence. It must not change the main
		// agent's creation loop, failure counters or authored content.
		return result, nil
	}
	material, err := prepareReviewMaterial(ctx, state, strings.TrimSpace(stringValue(call.Args["demand"])), prepareRender)
	diagnose("review.evidence", map[string]any{"ok": err == nil, "elapsed_ms": time.Since(started).Milliseconds(), "source_count": material.sourceCount, "image_count": len(material.imageData)})
	if err != nil {
		return model.ReviewResult{}, err
	}
	checkCurrent := func(ctx context.Context) error {
		if err := r.checkBudget(ctx, state); err != nil {
			return err
		}
		return validateReviewEvidence(ctx, state, material)
	}
	request := ReviewInput{Material: material, ImageResolver: reviewImageResolver(material.imageData), CheckBudget: checkCurrent,
		Diagnose: func(d map[string]any) { diagnose("review.protocol", d) },
	}
	stage = "model_execution"
	result, err = input.Reviewer.Review(ctx, request)
	if err != nil {
		return model.ReviewResult{}, err
	}
	stage = "result_validation"
	if err := result.Validate(); err != nil {
		return model.ReviewResult{}, model.NewAgentError(CodeReviewFailed, "review_output", err)
	}
	stage = "evidence_consistency"
	if err := checkCurrent(ctx); err != nil {
		return model.ReviewResult{}, err
	}
	return result, nil
}

func reviewerPrompt() string {
	modules := reviewerPromptModules()
	parts := make([]string, 0, len(modules))
	for _, m := range modules {
		parts = append(parts, m.SystemPrompt())
	}
	return strings.Join(parts, "\n\n")
}

func reviewerPromptManifest() string {
	modules := []map[string]string{}
	for _, m := range reviewerPromptModules() {
		modules = append(modules, map[string]string{"id": m.ID, "path": m.Path, "version": m.Version, "hash": m.Hash})
	}
	raw, _ := json.Marshal(map[string]any{"modules": modules})
	return string(raw)
}

func reviewerPromptModules() []prompts.Module {
	return []prompts.Module{prompts.MustLoad("subagent.reviewer.agent"), prompts.MustLoad("core.quality"), prompts.MustLoad("subagent.reviewer.output")}
}

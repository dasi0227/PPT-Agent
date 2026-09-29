package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
)

const CodeReviewFailed = "REVIEW_FAILED"
const reviewMaxTurns = 32
const reviewMaxToolCalls = 128
const reviewMaxOutputTokens = 4096

type TaskReviewer interface {
	Review(context.Context, ReviewInput) (model.ReviewResult, error)
}

type ReviewInput struct {
	Material      ReviewMaterial
	Images        []llm.ContentPart
	Tools         []ToolSchema
	ImageResolver llm.ImageRefResolver
	Execute       func(context.Context, llm.ToolCall) (ToolResult, error)
	CheckBudget   func(context.Context) error
}

type LLMTaskReviewer struct{ Provider llm.Provider }

func submitReviewSchema() ToolSchema {
	return ToolSchema{Name: "submit_review", Description: "Submit the final artifact review and end this review. Call it alone, exactly once. Reasons are required for all outcomes, including approval. This does not finish the main task or change any artifact.",
		Parameters: objectSchema([]string{"decision", "reasons"}, map[string]any{
			"decision": map[string]any{"type": "string", "enum": []string{"approve", "revise", "refuse"}},
			"reasons":  map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string", "minLength": 1}, "description": "Concrete reasons in the user's language. Name the relevant pages, observations and impact in plain text. For approval explain what was verified; for revise explain what remains uncertain; for refuse explain confirmed defects."},
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

func reviewToolAllowed(name string) bool {
	return name == "read_resource" || name == "read_image" || name == "render_slide"
}

func (r LLMTaskReviewer) Review(ctx context.Context, input ReviewInput) (model.ReviewResult, error) {
	if r.Provider == nil {
		return model.ReviewResult{}, errors.New("review provider is unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	raw, err := json.Marshal(contextengine.ModelValue(input.Material))
	if err != nil {
		return model.ReviewResult{}, err
	}
	parts := append(llm.TextContent(string(raw)), input.Images...)
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent(reviewerPrompt())},
		{Role: llm.RoleUser, Content: parts},
	}
	schemas := append([]ToolSchema{}, input.Tools...)
	for _, schema := range schemas {
		if !reviewToolAllowed(schema.Name) {
			return model.ReviewResult{}, fmt.Errorf("tool %s is not permitted in artifact review", schema.Name)
		}
	}
	schemas = append(schemas, submitReviewSchema())
	tools := make([]llm.ToolSchema, 0, len(schemas))
	for _, schema := range schemas {
		tools = append(tools, llm.ToolSchema{Name: schema.Name, Description: schema.Description, Parameters: schema.Parameters})
	}
	disclosed := schemasByName(schemas)
	calls := 0
	failures := 0
	var continuation *llm.ProviderContinuation
	for turn := 0; turn < reviewMaxTurns; turn++ {
		if err := ctx.Err(); err != nil {
			return model.ReviewResult{}, err
		}
		if input.CheckBudget != nil {
			if err := input.CheckBudget(ctx); err != nil {
				return model.ReviewResult{}, err
			}
		}
		maximum := r.Provider.Capabilities().ContextWindowTokens
		estimate := (contextengine.PromptEstimator{}).Estimate(contextengine.PromptEstimateInput{Messages: messages, Tools: tools, Max: maximum, Factor: 1})
		if maximum > 0 && estimate.Total+reviewMaxOutputTokens > maximum {
			return model.ReviewResult{}, errors.New("review material exceeds model context window; no files or screenshots were silently omitted")
		}
		response, err := r.Provider.Generate(ctx, llm.GenerateRequest{Messages: messages, Tools: tools, ImageResolver: input.ImageResolver, Continuation: continuation, MaxOutputTokens: reviewMaxOutputTokens})
		if err != nil {
			return model.ReviewResult{}, err
		}
		if err := ctx.Err(); err != nil {
			return model.ReviewResult{}, err
		}
		continuation = response.Continuation
		if len(response.ToolCalls) == 0 {
			return model.ReviewResult{}, errors.New("reviewer returned text without submit_review or an inspection tool")
		}
		for _, call := range response.ToolCalls {
			if call.Name == "submit_review" {
				if len(response.ToolCalls) != 1 {
					return model.ReviewResult{}, errors.New("submit_review must be the only tool call in the final response")
				}
				return parseReviewSubmission(call)
			}
		}
		if calls+len(response.ToolCalls) > reviewMaxToolCalls {
			return model.ReviewResult{}, errors.New("review tool budget exhausted")
		}
		results := make([]ToolResult, 0, len(response.ToolCalls))
		for _, call := range response.ToolCalls {
			if !reviewToolAllowed(call.Name) || !disclosed[call.Name] || input.Execute == nil {
				return model.ReviewResult{}, fmt.Errorf("reviewer requested unavailable tool %s", call.Name)
			}
			if err := ctx.Err(); err != nil {
				return model.ReviewResult{}, err
			}
			result, err := input.Execute(ctx, call)
			if err != nil {
				return model.ReviewResult{}, err
			}
			results = append(results, result)
			calls++
			if result.OK {
				failures = 0
			} else {
				failures++
			}
			if failures >= 3 {
				return model.ReviewResult{}, errors.New("review stopped after three consecutive inspection failures")
			}
		}
		messages = appendBatchObservations(messages, response.ToolCalls, response.Text(), results)
	}
	return model.ReviewResult{}, errors.New("review turn budget exhausted without submit_review")
}

func (r *Runtime) runReviewTask(ctx context.Context, input RuntimeInput, state *RunState, call llm.ToolCall) ToolResult {
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
	recordTrace(input.Trace, state.runID, "review.completed", map[string]any{"call_id": call.ID, "ok": result.OK, "result": result.Data, "error": result.Summary, "prompt_manifest": reviewerPromptManifest()})
	return result
}

func (r *Runtime) reviewArtifacts(ctx context.Context, input RuntimeInput, state *RunState, call llm.ToolCall) (model.ReviewResult, error) {
	if input.Reviewer == nil {
		return model.ReviewResult{}, errors.New("review provider is unavailable")
	}
	material, images, err := buildReviewMaterial(ctx, state, strings.TrimSpace(stringValue(call.Args["demand"])))
	if err != nil {
		return model.ReviewResult{}, err
	}
	registry := NewToolRegistry()
	for _, name := range []string{"read_resource", "read_image", "render_slide"} {
		desc, ok := state.tools.Descriptor(name)
		if !ok {
			continue
		}
		if !desc.ReadOnly {
			return model.ReviewResult{}, fmt.Errorf("review tool %s must not mutate authored content", name)
		}
		if err := registry.Register(desc.Tool, desc.ReadOnly, desc.Capability, desc.Risk, desc.Phases...); err != nil {
			return model.ReviewResult{}, err
		}
	}
	schemas := registry.Disclose(PhaseExecuting, state.mode, state.scope)
	sequence := 0
	request := ReviewInput{Material: material, Images: images, Tools: schemas, ImageResolver: input.ImageResolver,
		CheckBudget: func(ctx context.Context) error { return r.checkBudget(ctx, state) },
		Execute: func(ctx context.Context, inner llm.ToolCall) (ToolResult, error) {
			if err := r.checkBudget(ctx, state); err != nil {
				return ToolResult{}, err
			}
			sequence++
			result := registry.Execute(ctx, schemasByName(schemas), inner.Name, inner.Args, DomainToolInput{
				Args: inner.Args, CallID: fmt.Sprintf("%s:review:%d", call.ID, sequence), Context: state.pack, ProjectDir: input.ProjectDir,
				RunID: state.runID, Session: state.tx, Scope: state.scope, Phase: PhaseExecuting, Mode: state.mode,
			})
			state.toolCalls++
			for _, evidence := range result.Evidence {
				state.ledger.Record(evidence)
			}
			recordTrace(input.Trace, state.runID, "review.tool.completed", map[string]any{"parent_call_id": call.ID, "tool": inner.Name, "ok": result.OK, "data": result.Data})
			if inner.Name == "render_slide" {
				state.renderedImages = latestRenderedImages(state.pack, input.ProjectDir, state.tx)
				if err := r.saveCheckpoint(context.WithoutCancel(ctx), input, state, checkpointAfterRender); err != nil {
					return ToolResult{}, err
				}
			}
			return result, nil
		},
	}
	result, err := input.Reviewer.Review(ctx, request)
	if err != nil {
		return model.ReviewResult{}, err
	}
	if err := result.Validate(); err != nil {
		return model.ReviewResult{}, err
	}
	current, err := reviewSourceFiles(ctx, state.projectDir)
	if err != nil {
		return model.ReviewResult{}, err
	}
	if hashCheckpointValue(current) != material.SourceHash {
		return model.ReviewResult{}, errors.New("artifacts changed during review; the assessment no longer applies to the current content")
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

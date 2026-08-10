package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	semanticprompts "github.com/dasi0227/PPT-Agent/backend/prompts/semantic_reviewer"
)

const CodeReviewServiceUnavailable = "REVIEW_SERVICE_UNAVAILABLE"

type SemanticReviewPolicy struct {
	Enabled              bool
	ReviewPlan           bool
	ReviewTalk           bool
	ReviewExecuteDirect  bool
	ReviewExecutePlanned bool
	ReviewDeckLevel      bool
	AllowDirectBypass    bool
}

func DefaultSemanticReviewPolicy() SemanticReviewPolicy {
	return SemanticReviewPolicy{
		Enabled: true, ReviewPlan: true, ReviewExecutePlanned: true, ReviewDeckLevel: true,
	}
}

type SemanticReviewer interface {
	Review(context.Context, SemanticReviewInput) (SemanticReviewResult, error)
}

type SemanticReviewStore interface {
	SaveSemanticReview(context.Context, StoredSemanticReview) error
}

type StoredSemanticReview struct {
	ID                 string  `json:"id"`
	RunID              string  `json:"run_id"`
	FinishCallID       string  `json:"finish_call_id"`
	Accepted           bool    `json:"accepted"`
	Confidence         float64 `json:"confidence"`
	InputHash          string  `json:"input_hash"`
	OutputJSON         string  `json:"output_json"`
	PromptManifestJSON string  `json:"prompt_manifest_json"`
	CreatedAt          int64   `json:"created_at"`
}

type SemanticReviewInput struct {
	RunID             string              `json:"run_id"`
	FinishCallID      string              `json:"finish_call_id"`
	Command           model.RunCommand    `json:"run_command"`
	RequirementLedger *RequirementLedger  `json:"requirement_ledger,omitempty"`
	Plan              *Plan               `json:"plan,omitempty"`
	Changes           ChangeSet           `json:"changes"`
	GateResult        CompletionResult    `json:"gate_result"`
	Evidence          []Evidence          `json:"evidence"`
	LatestIssues      []Issue             `json:"latest_issues"`
	ContextBriefing   string              `json:"context_briefing"`
	RetrievedContext  []ReviewContextItem `json:"retrieved_context"`
	CandidateMessage  string              `json:"candidate_message,omitempty"`
	Focus             string              `json:"focus,omitempty"`
	Rubric            string              `json:"rubric"`
}

type ReviewContextItem struct {
	RefID       string      `json:"ref_id"`
	Kind        string      `json:"kind"`
	Source      string      `json:"source"`
	Target      Resource    `json:"target"`
	Revision    int         `json:"revision"`
	Hash        string      `json:"hash"`
	DetailLevel DetailLevel `json:"detail_level"`
	Summary     string      `json:"summary"`
	Reason      string      `json:"reason"`
}

type SemanticReviewResult struct {
	Checks []SemanticReviewCheck `json:"checks"`
}

type SemanticReviewCheck struct {
	Code    string `json:"code"`
	Summary string `json:"summary"`
}

func (r SemanticReviewResult) Passed() bool {
	return len(r.Checks) == 1 && r.Checks[0].Code == "REVIEW_PASS"
}

type LLMSemanticReviewer struct {
	Provider llm.Provider
}

func (r LLMSemanticReviewer) Review(ctx context.Context, input SemanticReviewInput) (SemanticReviewResult, error) {
	if r.Provider == nil {
		return SemanticReviewResult{}, errors.New("semantic reviewer provider is unavailable")
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return SemanticReviewResult{}, err
	}
	system := semanticReviewerPrompt()
	resp, err := r.Provider.Generate(ctx, llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: llm.TextContent(system)},
			{Role: llm.RoleUser, Content: llm.TextContent(string(raw))},
		},
	})
	if err != nil {
		return SemanticReviewResult{}, err
	}
	return ParseSemanticReviewResult(resp.Text())
}

func ParseSemanticReviewResult(raw string) (SemanticReviewResult, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SemanticReviewResult{}, errors.New("semantic reviewer returned empty output")
	}
	var result SemanticReviewResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return SemanticReviewResult{}, fmt.Errorf("invalid semantic review JSON: %w", err)
	}
	if err := validateSemanticReviewResult(result); err != nil {
		return SemanticReviewResult{}, err
	}
	return result, nil
}

func validateSemanticReviewResult(result SemanticReviewResult) error {
	if len(result.Checks) == 0 {
		return errors.New("semantic review checks are required")
	}
	if len(result.Checks) > 5 {
		return errors.New("semantic review checks must contain at most 5 items")
	}
	for _, check := range result.Checks {
		if !validReviewCode(check.Code) {
			return fmt.Errorf("semantic review check code %q is not allowed", check.Code)
		}
		if len([]rune(strings.TrimSpace(check.Summary))) < 20 {
			return errors.New("semantic review check summary must be specific and at least 20 characters")
		}
	}
	return nil
}

func validReviewCode(code string) bool {
	switch code {
	case "REVIEW_PASS",
		CodeReviewServiceUnavailable,
		"REVIEW_LACK_INFO",
		"REVIEW_QUALITY_POOR",
		"REVIEW_INTENT_MISMATCH",
		"REVIEW_EXECUTE_WRONG":
		return true
	default:
		return false
	}
}

func (r *Runtime) runReviewCompletion(
	ctx context.Context,
	input RuntimeInput,
	state *RunState,
	callID string,
	candidateMessage string,
	focus string,
) SemanticReviewResult {
	if focus == "" {
		focus = "all"
	}
	gate := r.Gate.Check(CompletionContext{
		Mode: state.mode, FinishPhase: state.phase, ActiveTools: state.activeTools,
		Issues: state.issues, Scope: state.scope, Session: state.tx, Changes: state.changeSet(),
		Evidence: state.ledger, Context: state.pack, Plan: state.plan,
		Requirements: state.requirements, FinishMessage: candidateMessage, Canceled: ctx.Err() != nil,
	})
	reviewInput := SemanticReviewInput{
		RunID: state.runID, FinishCallID: callID,
		Command: state.pack.Command, RequirementLedger: state.requirements, Plan: state.plan,
		Changes: state.changeSet(), GateResult: gate,
		Evidence: state.ledger.Entries(state.changeSet()), LatestIssues: state.issues,
		ContextBriefing: state.contextBriefing, RetrievedContext: reviewContextItems(state.retrievedContext),
		CandidateMessage: candidateMessage, Focus: focus,
		Rubric: semanticprompts.MustLoad("ppt_completion_rubric").Body,
	}
	recordTrace(input.Trace, state.runID, "semantic.review.started", map[string]any{
		"loop_id": state.loopID, "call_id": callID, "focus": focus,
	})
	if input.SemanticReviews == nil {
		result := reviewUnavailable("semantic reviewer provider is unavailable")
		recordTrace(input.Trace, state.runID, "semantic.review.completed", map[string]any{
			"loop_id": state.loopID, "call_id": callID, "checks": result.Checks,
		})
		return result
	}
	result, err := input.SemanticReviews.Review(ctx, reviewInput)
	if err != nil {
		result = reviewUnavailable(err.Error())
		recordTrace(input.Trace, state.runID, "semantic.review.completed", map[string]any{
			"loop_id": state.loopID, "call_id": callID, "checks": result.Checks, "error": err.Error(),
		})
		return result
	}
	if err := validateSemanticReviewResult(result); err != nil {
		result = reviewUnavailable(err.Error())
		recordTrace(input.Trace, state.runID, "semantic.review.completed", map[string]any{
			"loop_id": state.loopID, "call_id": callID, "checks": result.Checks, "error": err.Error(),
		})
		return result
	}
	raw, _ := json.Marshal(result)
	if input.SemanticReviewStore != nil {
		_ = input.SemanticReviewStore.SaveSemanticReview(ctx, StoredSemanticReview{
			ID:    "semrev_" + hashBytes([]byte(state.runID + "\x00" + callID + "\x00" + string(raw)))[:24],
			RunID: state.runID, FinishCallID: callID, Accepted: result.Passed(),
			Confidence: 0, InputHash: hashCheckpointValue(reviewInput),
			OutputJSON: string(raw), PromptManifestJSON: semanticReviewerPromptManifest(),
		})
	}
	recordTrace(input.Trace, state.runID, "semantic.review.completed", map[string]any{
		"loop_id": state.loopID, "call_id": callID, "checks": result.Checks,
	})
	return result
}

func reviewUnavailable(reason string) SemanticReviewResult {
	return SemanticReviewResult{Checks: []SemanticReviewCheck{{
		Code:    CodeReviewServiceUnavailable,
		Summary: "Reviewer service is unavailable or returned invalid output: " + strings.TrimSpace(reason),
	}}}
}

func reviewContextItems(values []RetrievedContextItem) []ReviewContextItem {
	out := make([]ReviewContextItem, 0, len(values))
	for _, item := range values {
		out = append(out, ReviewContextItem{
			RefID: item.RefID, Kind: item.Kind, Source: item.Source, Target: item.Target,
			Revision: item.Revision, Hash: item.Hash, DetailLevel: item.DetailLevel,
			Summary: item.Snippet, Reason: item.SelectionReason,
		})
	}
	return out
}

func semanticReviewerPrompt() string {
	modules := []semanticprompts.Module{
		semanticprompts.MustLoad("semantic_reviewer_policy"),
		semanticprompts.MustLoad("ppt_completion_rubric"),
		semanticprompts.MustLoad("review_output_contract"),
	}
	var b strings.Builder
	b.WriteString("<semantic_reviewer_prompt_manifest version=\"" + semanticprompts.Version + "\">\n")
	for _, module := range modules {
		b.WriteString("<prompt_module id=\"" + module.ID + "\" path=\"" + module.Path + "\" hash=\"" + module.Hash + "\">\n")
		b.WriteString(strings.TrimSpace(module.Body))
		b.WriteString("\n</prompt_module>\n")
	}
	b.WriteString("</semantic_reviewer_prompt_manifest>")
	return b.String()
}

func semanticReviewerPromptManifest() string {
	raw, _ := json.Marshal(map[string]any{
		"version": semanticprompts.Version,
		"modules": []string{
			"semantic_reviewer_policy",
			"ppt_completion_rubric",
			"review_output_contract",
		},
	})
	return string(raw)
}

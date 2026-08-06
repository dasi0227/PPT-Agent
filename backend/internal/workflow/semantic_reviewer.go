package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	semanticprompts "github.com/dasi0227/PPT-Agent/backend/prompts/semantic_reviewer"
)

const CodeSemanticReviewUnavailable = "SEMANTIC_REVIEW_UNAVAILABLE"

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
	Strategy          ExecutionStrategy   `json:"strategy"`
	WorkSpec          model.WorkSpec      `json:"work_spec"`
	RequirementLedger *RequirementLedger  `json:"requirement_ledger,omitempty"`
	Plan              *Plan               `json:"plan,omitempty"`
	Changes           ChangeSet           `json:"changes"`
	Evidence          []Evidence          `json:"evidence"`
	LatestIssues      []Issue             `json:"latest_issues"`
	ContextBriefing   string              `json:"context_briefing"`
	RetrievedContext  []ReviewContextItem `json:"retrieved_context"`
	FinishMessage     string              `json:"finish_message"`
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
	Accepted   bool                  `json:"accepted"`
	Issues     []SemanticReviewIssue `json:"issues"`
	Coverage   []RequirementCoverage `json:"coverage"`
	Confidence float64               `json:"confidence"`
	Summary    string                `json:"summary"`
}

type RequirementCoverage struct {
	RequirementID string   `json:"requirement_id"`
	Status        string   `json:"status"`
	EvidenceRefs  []string `json:"evidence_refs"`
	Reason        string   `json:"reason"`
}

type SemanticReviewIssue struct {
	Code           string                 `json:"code"`
	Severity       string                 `json:"severity"`
	RequirementID  string                 `json:"requirement_id,omitempty"`
	Target         Resource               `json:"target,omitempty"`
	Summary        string                 `json:"summary"`
	RequiredAction SemanticRequiredAction `json:"required_action,omitempty"`
}

type SemanticRequiredAction struct {
	Tool   string   `json:"tool,omitempty"`
	Target Resource `json:"target,omitempty"`
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
	if result.Confidence < 0 || result.Confidence > 1 {
		return SemanticReviewResult{}, errors.New("semantic review confidence must be between 0 and 1")
	}
	for _, issue := range result.Issues {
		if strings.TrimSpace(issue.Code) == "" || strings.TrimSpace(issue.Summary) == "" {
			return SemanticReviewResult{}, errors.New("semantic review issues require code and summary")
		}
	}
	if !result.Accepted && len(result.Issues) == 0 {
		return SemanticReviewResult{}, errors.New("semantic review rejection requires at least one issue")
	}
	return result, nil
}

func (r *Runtime) semanticReviewEnabled(state *runtimeState, pack contextengine.ContextPack) bool {
	if state == nil || !r.SemanticPolicy.Enabled {
		return false
	}
	switch state.strategy {
	case StrategyPlan:
		return r.SemanticPolicy.ReviewPlan
	case StrategyTalk, StrategyAsk:
		return r.SemanticPolicy.ReviewTalk
	case StrategyFulfill:
		return r.SemanticPolicy.ReviewExecutePlanned
	case StrategyExecute:
		if pack.WorkSpec.Target.Level == model.TargetDeck {
			return r.SemanticPolicy.ReviewDeckLevel
		}
		return r.SemanticPolicy.ReviewExecuteDirect
	default:
		return false
	}
}

func (r *Runtime) runSemanticReview(
	ctx context.Context,
	input RuntimeInput,
	state *runtimeState,
	callID string,
	message string,
	deterministic CompletionResult,
) (CompletionResult, bool, error) {
	if !r.semanticReviewEnabled(state, input.Context) {
		return deterministic, false, nil
	}
	if input.SemanticReviews != nil {
		recordTrace(input.Trace, state.runID, "semantic.review.started", map[string]any{"loop_id": state.loopID, "call_id": callID})
	}
	if input.SemanticReviews == nil {
		if state.strategy == StrategyExecute && r.SemanticPolicy.AllowDirectBypass {
			return deterministic, false, nil
		}
		issue := CompletionIssue{Code: CodeSemanticReviewUnavailable, Summary: "semantic reviewer is unavailable for this run"}
		out := deterministic
		out.Accepted = false
		out.Issues = append(out.Issues, issue)
		recordTrace(input.Trace, state.runID, "semantic.review.completed", map[string]any{
			"loop_id": state.loopID, "call_id": callID, "accepted": false, "unavailable": true,
		})
		return out, true, nil
	}
	reviewInput := SemanticReviewInput{
		RunID: state.runID, FinishCallID: callID, Strategy: state.strategy,
		WorkSpec: input.Context.WorkSpec, RequirementLedger: state.requirements, Plan: state.plan,
		Changes: state.changeSet(), Evidence: state.ledger.Entries(state.changeSet()), LatestIssues: state.issues,
		ContextBriefing: state.contextBriefing, RetrievedContext: reviewContextItems(state.retrievedContext),
		FinishMessage: message, Rubric: semanticprompts.MustLoad("ppt_completion_rubric").Body,
	}
	result, err := input.SemanticReviews.Review(ctx, reviewInput)
	if err != nil {
		out := deterministic
		out.Accepted = false
		out.Issues = append(out.Issues, CompletionIssue{
			Code: CodeSemanticReviewUnavailable, Summary: err.Error(),
		})
		recordTrace(input.Trace, state.runID, "semantic.review.completed", map[string]any{
			"loop_id": state.loopID, "call_id": callID, "accepted": false, "error": err.Error(),
		})
		return out, true, nil
	}
	raw, _ := json.Marshal(result)
	if input.SemanticReviewStore != nil {
		_ = input.SemanticReviewStore.SaveSemanticReview(ctx, StoredSemanticReview{
			ID:    "semrev_" + hashBytes([]byte(state.runID + "\x00" + callID + "\x00" + string(raw)))[:24],
			RunID: state.runID, FinishCallID: callID, Accepted: result.Accepted,
			Confidence: result.Confidence, InputHash: hashCheckpointValue(reviewInput),
			OutputJSON: string(raw), PromptManifestJSON: semanticReviewerPromptManifest(),
		})
	}
	recordTrace(input.Trace, state.runID, "semantic.review.completed", map[string]any{
		"loop_id": state.loopID, "call_id": callID, "accepted": result.Accepted,
		"confidence": result.Confidence, "issues": result.Issues,
	})
	if result.Accepted {
		return deterministic, true, nil
	}
	out := deterministic
	out.Accepted = false
	out.Issues = append(out.Issues, semanticIssuesToCompletion(result.Issues)...)
	return out, true, nil
}

func semanticIssuesToCompletion(values []SemanticReviewIssue) []CompletionIssue {
	out := make([]CompletionIssue, 0, len(values))
	for _, issue := range values {
		action := RequiredAction{Tool: issue.RequiredAction.Tool, Target: issue.RequiredAction.Target}
		if action.Tool == "" {
			action.Tool = "search_refs"
		}
		out = append(out, CompletionIssue{
			Code: issue.Code, Summary: issue.Summary,
			RequiredActions: []RequiredAction{action},
		})
	}
	return out
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

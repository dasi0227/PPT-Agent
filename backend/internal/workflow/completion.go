package workflow

import (
	"errors"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
)

type CompletionIssue struct {
	Code            string           `json:"code"`
	Summary         string           `json:"summary"`
	RequiredActions []RequiredAction `json:"required_actions,omitempty"`
}

type RequiredAction struct {
	Tool   string    `json:"tool,omitempty"`
	Target TargetRef `json:"target,omitempty"`
}

type CompletionResult struct {
	Accepted bool              `json:"accepted"`
	Issues   []CompletionIssue `json:"issues"`
}

func (r CompletionResult) RejectionKey() string {
	codes := make([]string, 0, len(r.Issues))
	for _, issue := range r.Issues {
		codes = append(codes, issue.Code)
	}
	sort.Strings(codes)
	return strings.Join(codes, "|")
}

type CompletionContext struct {
	Strategy        ExecutionStrategy
	FinishPhase     RuntimePhase
	ActiveTools     int
	Issues          []Issue
	WorkScope       Scope
	Transaction     *Transaction
	Changes         ChangeSet
	Evidence        *EvidenceLedger
	Context         contextengine.ContextPack
	Plan            *Plan
	Canceled        bool
	BudgetExhausted bool
}

type CompletionPolicy interface {
	Check(CompletionContext) []CompletionIssue
}

type EvidenceCompletionPolicy struct{}

func (EvidenceCompletionPolicy) Check(ctx CompletionContext) []CompletionIssue {
	if ctx.Strategy == StrategyChat {
		return nil
	}
	issues := []CompletionIssue{}
	referenceHash := ""
	if ctx.Transaction != nil {
		referenceHash, _ = validateReferences(ctx.Context, ctx.Transaction)
	}
	for _, change := range ctx.Changes.All() {
		target := targetForArtifact(change.Artifact)
		require := func(kind, hash, code string, action RequiredAction) {
			if hash == "" || ctx.Evidence == nil || !ctx.Evidence.HasFresh(target, hash, kind) {
				issues = append(issues, CompletionIssue{
					Code:            code,
					Summary:         kind + " evidence is missing or stale for " + target.Key(),
					RequiredActions: []RequiredAction{action},
				})
			}
		}
		if !isPPTDomainChange(change) {
			switch change.Artifact.Kind {
			case ArtifactPresentation:
				require("static", change.AfterHash, "STATIC_EVIDENCE_REQUIRED", RequiredAction{Tool: "edit_ppt", Target: target})
				require("render", change.AfterHash, "VISUAL_EVIDENCE_REQUIRED", RequiredAction{Tool: "render_slide", Target: target})
			default:
				require("schema", change.AfterHash, "SCHEMA_EVIDENCE_REQUIRED", RequiredAction{Tool: "write_ppt", Target: target})
			}
			continue
		}
		switch change.Artifact.Kind {
		case ArtifactDeck:
			require("schema", change.AfterHash, "SCHEMA_EVIDENCE_REQUIRED", RequiredAction{Tool: "write_ppt", Target: target})
			require("reference", referenceHash, "REFERENCE_EVIDENCE_REQUIRED", RequiredAction{Tool: "write_ppt", Target: target})
		case ArtifactSlide:
			require("schema", change.AfterHash, "SCHEMA_EVIDENCE_REQUIRED", RequiredAction{Tool: "write_ppt", Target: target})
			global := TargetRef{Type: "global"}
			if referenceHash == "" || ctx.Evidence == nil || !ctx.Evidence.HasFresh(global, referenceHash, "reference") {
				issues = append(issues, CompletionIssue{
					Code: "REFERENCE_EVIDENCE_REQUIRED", Summary: "global reference integrity is missing or stale",
					RequiredActions: []RequiredAction{{Tool: "write_ppt", Target: global}},
				})
			}
		case ArtifactDesign:
			require("schema", change.AfterHash, "SCHEMA_EVIDENCE_REQUIRED", RequiredAction{Tool: "write_ppt", Target: target})
			if ctx.Transaction != nil {
				if deck, err := currentDeck(ctx.Context, ctx.Transaction); err == nil {
					for _, slideID := range deck.SlideOrder {
						slideTarget := TargetRef{Type: "slide", SlideID: slideID}
						hash, hashErr := renderSourceHash(ctx.Context, ctx.Transaction, slideID)
						if hashErr != nil || ctx.Evidence == nil || !ctx.Evidence.HasFresh(slideTarget, hash, "render") {
							issues = append(issues, CompletionIssue{
								Code:            "VISUAL_EVIDENCE_REQUIRED",
								Summary:         "render evidence is missing or stale for " + slideTarget.Key(),
								RequiredActions: []RequiredAction{{Tool: "render_slide", Target: slideTarget}},
							})
						}
					}
				}
			}
		case ArtifactPresentation:
			hash := ""
			if ctx.Transaction != nil {
				hash, _ = renderSourceHash(ctx.Context, ctx.Transaction, change.Artifact.ID)
			}
			require("static", hash, "STATIC_EVIDENCE_REQUIRED", RequiredAction{Tool: "edit_ppt", Target: target})
			require("render", hash, "VISUAL_EVIDENCE_REQUIRED", RequiredAction{Tool: "render_slide", Target: target})
		}
	}
	return dedupeCompletionIssues(issues)
}

func isPPTDomainChange(change ArtifactChange) bool {
	source := strings.TrimPrefix(change.Source, "tentative:")
	return source == "write_ppt" || source == "edit_ppt"
}

func dedupeCompletionIssues(values []CompletionIssue) []CompletionIssue {
	out := []CompletionIssue{}
	seen := map[string]bool{}
	for _, value := range values {
		key := value.Code + "\x00" + value.Summary
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

type CompletionGate struct {
	Policies []CompletionPolicy
}

func NewCompletionGate() CompletionGate {
	return CompletionGate{Policies: []CompletionPolicy{EvidenceCompletionPolicy{}}}
}

func (g CompletionGate) Check(ctx CompletionContext) CompletionResult {
	issues := []CompletionIssue{}
	if ctx.FinishPhase != PhaseChat && ctx.FinishPhase != PhaseExecuting {
		issues = append(issues, CompletionIssue{Code: "FINISH_NOT_ALLOWED", Summary: "finish is not allowed in the current phase"})
	}
	if ctx.ActiveTools != 0 {
		issues = append(issues, CompletionIssue{Code: "TOOLS_RUNNING", Summary: "a tool call is still running"})
	}
	for _, issue := range ctx.Issues {
		if issue.Severity == SeverityFatal {
			issues = append(issues, CompletionIssue{Code: "FATAL_ISSUE", Summary: issue.Summary})
		}
	}
	if ctx.Canceled {
		issues = append(issues, CompletionIssue{Code: CodeCanceled, Summary: "run was canceled"})
	}
	if ctx.BudgetExhausted {
		issues = append(issues, CompletionIssue{Code: CodeBudgetExceeded, Summary: "runtime budget is exhausted"})
	}
	if ctx.Strategy != StrategyChat {
		if ctx.Transaction == nil {
			issues = append(issues, CompletionIssue{Code: "STAGING_REQUIRED", Summary: "write run has no staging transaction"})
		} else if err := ctx.Transaction.ValidateBaselines(); err != nil {
			code := CodeRevisionConflict
			if !errors.Is(err, ErrStagedHashMismatch) {
				code = "STAGING_INVALID"
			}
			issues = append(issues, CompletionIssue{Code: code, Summary: err.Error()})
		}
		if ctx.Changes.Count() == 0 {
			issues = append(issues, CompletionIssue{Code: "CHANGESET_REQUIRED", Summary: "write run has no staged changes"})
		}
		for _, change := range ctx.Changes.All() {
			if !ctx.WorkScope.AllowsArtifact(change.Artifact) {
				issues = append(issues, CompletionIssue{Code: "TARGET_OUT_OF_SCOPE", Summary: targetForArtifact(change.Artifact).Key() + " is outside the run scope"})
			}
		}
	}
	if ctx.Strategy == StrategyComplex && (ctx.Plan == nil || ctx.Plan.HasBlockingSteps()) {
		issues = append(issues, CompletionIssue{Code: "PLAN_INCOMPLETE", Summary: "complex plan still has pending, in-progress, or failed steps"})
	}
	for _, policy := range g.Policies {
		issues = append(issues, policy.Check(ctx)...)
	}
	return CompletionResult{Accepted: len(issues) == 0, Issues: issues}
}

package workflow

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type CompletionIssue struct {
	Code            string           `json:"code"`
	Summary         string           `json:"summary"`
	RequiredActions []RequiredAction `json:"required_actions,omitempty"`
}

type RequiredAction struct {
	Tool   string   `json:"tool,omitempty"`
	Target Resource `json:"target,omitempty"`
}

type CompletionResult struct {
	Accepted              bool                   `json:"accepted"`
	Issues                []CompletionIssue      `json:"issues"`
	MaterializationProofs []MaterializationProof `json:"-"`
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
	Strategy    ExecutionStrategy
	ExecuteMode ExecuteMode
	FinishPhase RuntimePhase
	ActiveTools int
	Issues      []Issue
	WorkScope   Scope
	Session     *RunSession
	Changes     ChangeSet
	Evidence    *EvidenceLedger
	Context     contextengine.ContextPack
	Plan        *Plan
	Canceled    bool
}

type CompletionPolicy interface {
	Check(CompletionContext) []CompletionIssue
}

type EvidenceCompletionPolicy struct{}

func (EvidenceCompletionPolicy) Check(ctx CompletionContext) []CompletionIssue {
	if ctx.Strategy != StrategyExecute {
		return nil
	}
	issues := []CompletionIssue{}
	referenceHash := ""
	if ctx.Session != nil {
		referenceHash, _ = validateReferences(ctx.Context, ctx.Session)
	}
	for _, change := range ctx.Changes.All() {
		target := resourceForArtifact(change.Artifact)
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
			case ArtifactSlideHTML:
				require("static", change.AfterHash, "STATIC_EVIDENCE_REQUIRED", RequiredAction{Tool: "edit_ppt", Target: target})
				require("render", change.AfterHash, "VISUAL_EVIDENCE_REQUIRED", RequiredAction{Tool: "render_slide", Target: target})
			default:
				require("schema", change.AfterHash, "SCHEMA_EVIDENCE_REQUIRED", RequiredAction{Tool: "write_ppt", Target: target})
			}
			continue
		}
		switch change.Artifact.Kind {
		case ArtifactOutline:
			require("schema", change.AfterHash, "SCHEMA_EVIDENCE_REQUIRED", RequiredAction{Tool: "write_ppt", Target: target})
			require("reference", referenceHash, "REFERENCE_EVIDENCE_REQUIRED", RequiredAction{Tool: "write_ppt", Target: target})
		case ArtifactSlideSpec:
			require("schema", change.AfterHash, "SCHEMA_EVIDENCE_REQUIRED", RequiredAction{Tool: "write_ppt", Target: target})
			outline := Resource{Type: "deck", Part: "outline"}
			if referenceHash == "" || ctx.Evidence == nil || !ctx.Evidence.HasFresh(outline, referenceHash, "reference") {
				issues = append(issues, CompletionIssue{
					Code: "REFERENCE_EVIDENCE_REQUIRED", Summary: "outline reference integrity is missing or stale",
					RequiredActions: []RequiredAction{{Tool: "write_ppt", Target: outline}},
				})
			}
			if ctx.Context.WorkSpec.Target.Artifact == model.ArtifactPresentation && ctx.Session != nil {
				htmlTarget := Resource{Type: "slide", SlideID: change.Artifact.ID, Part: "html"}
				hash, hashErr := renderSourceHash(ctx.Context, ctx.Session, change.Artifact.ID)
				if specChangeAffectsHTML(ctx.Session, change.Artifact) {
					if !hasArtifactChange(ctx.Changes, ArtifactSlideHTML, change.Artifact.ID) {
						issues = append(issues, CompletionIssue{
							Code:    "SLIDE_HTML_SYNC_REQUIRED",
							Summary: "Slide Spec changes affect Presentation HTML for " + htmlTarget.Key(),
							RequiredActions: []RequiredAction{
								{Tool: "edit_ppt", Target: htmlTarget},
								{Tool: "render_slide", Target: htmlTarget},
							},
						})
						continue
					}
					if hashErr != nil || ctx.Evidence == nil || !ctx.Evidence.HasFresh(htmlTarget, hash, "static") {
						issues = append(issues, CompletionIssue{
							Code: "STATIC_EVIDENCE_REQUIRED", Summary: "static evidence is missing or stale for " + htmlTarget.Key(),
							RequiredActions: []RequiredAction{{Tool: "edit_ppt", Target: htmlTarget}},
						})
					}
				}
				if hashErr != nil || !hasFreshMaterialization(ctx, htmlTarget, hash) {
					issues = append(issues, CompletionIssue{
						Code: "VISUAL_EVIDENCE_REQUIRED", Summary: "render evidence is missing or stale for " + htmlTarget.Key(),
						RequiredActions: []RequiredAction{{Tool: "render_slide", Target: htmlTarget}},
					})
				}
			}
		case ArtifactDesign:
			require("schema", change.AfterHash, "SCHEMA_EVIDENCE_REQUIRED", RequiredAction{Tool: "write_ppt", Target: target})
			if ctx.Context.WorkSpec.Target.Artifact == model.ArtifactPresentation && ctx.Session != nil {
				if deck, err := currentOutline(ctx.Context, ctx.Session); err == nil {
					for _, slideID := range deck.SlideOrder {
						slideTarget := Resource{Type: "slide", SlideID: slideID, Part: "html"}
						hash, hashErr := renderSourceHash(ctx.Context, ctx.Session, slideID)
						if hashErr != nil || !hasFreshMaterialization(ctx, slideTarget, hash) {
							issues = append(issues, CompletionIssue{
								Code:            "VISUAL_EVIDENCE_REQUIRED",
								Summary:         "render evidence is missing or stale for " + slideTarget.Key(),
								RequiredActions: []RequiredAction{{Tool: "render_slide", Target: slideTarget}},
							})
						}
					}
				}
			}
		case ArtifactSlideHTML:
			hash := ""
			if ctx.Session != nil {
				hash, _ = renderSourceHash(ctx.Context, ctx.Session, change.Artifact.ID)
			}
			require("static", hash, "STATIC_EVIDENCE_REQUIRED", RequiredAction{Tool: "edit_ppt", Target: target})
			if !hasFreshMaterialization(ctx, target, hash) {
				issues = append(issues, CompletionIssue{
					Code: "VISUAL_EVIDENCE_REQUIRED", Summary: "render evidence is missing or stale for " + target.Key(),
					RequiredActions: []RequiredAction{{Tool: "render_slide", Target: target}},
				})
			}
		}
	}
	return dedupeCompletionIssues(issues)
}

func hasFreshMaterialization(ctx CompletionContext, target Resource, hash string) bool {
	if ctx.Evidence == nil || ctx.Session == nil || hash == "" {
		return false
	}
	proof, ok := ctx.Evidence.FreshMaterializationProof(target, hash)
	if !ok {
		return false
	}
	expected, err := currentMaterializationProof(
		ctx.Context, ctx.Session.ProjectDir(), ctx.Session, target.SlideID, hash,
	)
	return err == nil && proof == expected
}

func hasArtifactChange(changes ChangeSet, kind ArtifactKind, id string) bool {
	for _, change := range changes.All() {
		if change.Artifact.Kind == kind && change.Artifact.ID == id {
			return true
		}
	}
	return false
}

// specChangeAffectsHTML is intentionally conservative. Only a change isolated
// to speaker_notes is known not to affect rendered HTML.
func specChangeAffectsHTML(tx *RunSession, ref ArtifactRef) bool {
	if tx == nil {
		return true
	}
	beforeRaw, err := tx.ReadBaseline(ref)
	if err != nil {
		return true
	}
	afterRaw, err := tx.Read(ref)
	if err != nil {
		return true
	}
	var before spec.SlideSpec
	var after spec.SlideSpec
	if json.Unmarshal(beforeRaw, &before) != nil || json.Unmarshal(afterRaw, &after) != nil {
		return true
	}
	clearRuntimeAndNotes := func(value *spec.SlideSpec) {
		value.SchemaVersion = ""
		value.Revision = 0
		value.ProjectID = ""
		value.SlideID = ""
		value.SourceOutlineRevision = 0
		value.SpeakerNotes = ""
		value.CreatedAt = 0
		value.UpdatedAt = 0
	}
	clearRuntimeAndNotes(&before)
	clearRuntimeAndNotes(&after)
	return !reflect.DeepEqual(before, after)
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
	if !finishAllowed(ctx.Strategy, ctx.FinishPhase) {
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
	if ctx.Strategy == StrategyExecute {
		if ctx.Session == nil {
			issues = append(issues, CompletionIssue{Code: CodeRunSessionRequired, Summary: "write run has no active run session"})
		} else if err := ctx.Session.ValidateBaselines(); err != nil {
			code := CodeRevisionConflict
			if !errors.Is(err, ErrArtifactHashMismatch) {
				code = "RUN_SESSION_INVALID"
			}
			issues = append(issues, CompletionIssue{Code: code, Summary: err.Error()})
		}
	}
	if ctx.Strategy == StrategyExecute && ctx.ExecuteMode == ExecuteModePlanned && (ctx.Plan == nil || ctx.Plan.HasBlockingSteps()) {
		issues = append(issues, CompletionIssue{Code: "PLAN_INCOMPLETE", Summary: "planned execution still has pending, in-progress, or failed steps"})
	}
	for _, policy := range g.Policies {
		issues = append(issues, policy.Check(ctx)...)
	}
	result := CompletionResult{Accepted: len(issues) == 0, Issues: issues}
	if result.Accepted {
		result.MaterializationProofs = acceptedMaterializationProofs(ctx)
	}
	return result
}

func finishAllowed(strategy ExecutionStrategy, phase RuntimePhase) bool {
	switch strategy {
	case StrategyTalk, StrategyAsk:
		return phase == PhaseChat
	case StrategyPlan:
		return phase == PhasePlanning
	case StrategyExecute:
		return phase == PhaseExecuting
	default:
		return false
	}
}

func acceptedMaterializationProofs(ctx CompletionContext) []MaterializationProof {
	if ctx.Session == nil || ctx.Evidence == nil {
		return nil
	}
	outline, err := currentOutline(ctx.Context, ctx.Session)
	if err != nil {
		return nil
	}
	proofs := make([]MaterializationProof, 0, len(outline.SlideOrder))
	for _, slideID := range outline.SlideOrder {
		if specChangeRequiresHTMLSync(ctx, slideID) &&
			!hasArtifactChange(ctx.Changes, ArtifactSlideHTML, slideID) {
			continue
		}
		hash, err := renderSourceHash(ctx.Context, ctx.Session, slideID)
		if err != nil {
			continue
		}
		target := Resource{Type: "slide", SlideID: slideID, Part: "html"}
		if proof, ok := ctx.Evidence.FreshMaterializationProof(target, hash); ok {
			expected, expectedErr := currentMaterializationProof(
				ctx.Context, ctx.Session.ProjectDir(), ctx.Session, slideID, hash,
			)
			if expectedErr != nil || proof != expected {
				continue
			}
			proofs = append(proofs, proof)
		}
	}
	return proofs
}

func specChangeRequiresHTMLSync(ctx CompletionContext, slideID string) bool {
	for _, change := range ctx.Changes.All() {
		if change.Artifact.Kind == ArtifactSlideSpec && change.Artifact.ID == slideID {
			return specChangeAffectsHTML(ctx.Session, change.Artifact)
		}
	}
	return false
}

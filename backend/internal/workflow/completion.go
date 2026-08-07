package workflow

import (
	"encoding/json"
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
	Strategy      ExecutionStrategy
	FinishPhase   RuntimePhase
	ActiveTools   int
	Issues        []Issue
	WorkScope     Scope
	Session       *RunSession
	Changes       ChangeSet
	Evidence      *EvidenceLedger
	Context       contextengine.ContextPack
	Plan          *Plan
	Requirements  *RequirementLedger
	FinishMessage string
	Canceled      bool
}

type CompletionPolicy interface {
	Check(CompletionContext) []CompletionIssue
}

type EvidenceCompletionPolicy struct{}

func (EvidenceCompletionPolicy) Check(ctx CompletionContext) []CompletionIssue {
	if !isWriteStrategy(ctx.Strategy) {
		return nil
	}
	if ctx.Changes.Count() == 0 {
		return nil
	}
	issues := []CompletionIssue{}
	var referenceErr error
	if ctx.Session != nil && hasPPTDomainChanges(ctx.Changes) {
		_, referenceErr = validateReferences(ctx.Context, ctx.Session)
	}
	if referenceErr != nil {
		issues = append(issues, asyncDeckSlideIssue(ctx, referenceErr))
	}
	for _, change := range ctx.Changes.All() {
		target := resourceForArtifact(change.Artifact)
		if !isPPTDomainChange(change) {
			switch change.Artifact.Kind {
			case ArtifactSlideHTML:
				if !hasFreshHTMLEvidence(ctx, target, change.AfterHash) {
					issues = append(issues, htmlEvidenceIssue(target))
				}
			default:
				if !hasFreshEvidence(ctx, target, change.AfterHash, "schema") {
					issues = append(issues, schemaEvidenceIssue(target))
				}
			}
			continue
		}
		switch change.Artifact.Kind {
		case ArtifactOutline:
			if !hasFreshEvidence(ctx, target, change.AfterHash, "schema") {
				issues = append(issues, schemaEvidenceIssue(target))
			}
		case ArtifactSlideSpec:
			if !hasFreshEvidence(ctx, target, change.AfterHash, "schema") {
				issues = append(issues, schemaEvidenceIssue(target))
			}
			if ctx.Context.WorkSpec.Target.Artifact == model.ArtifactPresentation && ctx.Session != nil {
				htmlTarget := Resource{Type: "slide", SlideID: change.Artifact.ID, Part: "html"}
				if specChangeAffectsHTML(ctx.Session, change.Artifact) {
					if !hasArtifactChange(ctx.Changes, ArtifactSlideHTML, change.Artifact.ID) {
						issues = append(issues, asyncSpecHTMLIssue(htmlTarget))
					}
				}
			}
		case ArtifactDesign:
			if !hasFreshEvidence(ctx, target, change.AfterHash, "schema") {
				issues = append(issues, schemaEvidenceIssue(target))
			}
			if ctx.Context.WorkSpec.Target.Artifact == model.ArtifactPresentation && ctx.Session != nil {
				if deck, err := currentOutline(ctx.Context, ctx.Session); err == nil {
					for _, slideID := range deck.SlideOrder {
						slideTarget := Resource{Type: "slide", SlideID: slideID, Part: "html"}
						if !hasArtifactChange(ctx.Changes, ArtifactSlideHTML, slideID) {
							issues = append(issues, asyncSpecHTMLIssue(slideTarget))
						}
					}
				}
			}
		case ArtifactSlideHTML:
			if !hasFreshHTMLEvidence(ctx, target, change.AfterHash) {
				issues = append(issues, htmlEvidenceIssue(target))
			}
		}
	}
	return dedupeCompletionIssues(issues)
}

func hasFreshEvidence(ctx CompletionContext, target Resource, hash string, kind string) bool {
	return hash != "" && ctx.Evidence != nil && ctx.Evidence.HasFresh(target, hash, kind)
}

func hasFreshHTMLEvidence(ctx CompletionContext, target Resource, hash string) bool {
	return hasFreshEvidence(ctx, target, hash, "static") && hasFreshMaterialization(ctx, target, hash)
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

func schemaEvidenceIssue(target Resource) CompletionIssue {
	return CompletionIssue{
		Code: "EVIDENCE_SCHEMA_MISSING", Summary: "schema evidence is missing or stale for " + target.Key(),
		RequiredActions: []RequiredAction{{Tool: "write_ppt", Target: target}},
	}
}

func htmlEvidenceIssue(target Resource) CompletionIssue {
	return CompletionIssue{
		Code: "EVIDENCE_HTML_MISSING", Summary: "HTML evidence is missing or stale for " + target.Key(),
		RequiredActions: []RequiredAction{
			{Tool: "edit_ppt", Target: target},
			{Tool: "render_slide", Target: target},
		},
	}
}

func asyncSpecHTMLIssue(target Resource) CompletionIssue {
	return CompletionIssue{
		Code: "ASYNC_SPEC_HTML", Summary: "Spec or Design changes are not materialized in HTML for " + target.Key(),
		RequiredActions: []RequiredAction{
			{Tool: "edit_ppt", Target: target},
			{Tool: "render_slide", Target: target},
		},
	}
}

func asyncDeckSlideIssue(ctx CompletionContext, cause error) CompletionIssue {
	actions := []RequiredAction{{Tool: "write_ppt", Target: Resource{Type: "deck", Part: "outline"}}}
	if ctx.Session != nil {
		if deck, err := currentOutline(ctx.Context, ctx.Session); err == nil {
			actions = actions[:0]
			for _, slideID := range deck.SlideOrder {
				if _, _, err := readArtifact(ctx.Session.ProjectDir(), ctx.Session, specSlideRef(slideID)); errorsIsNotExist(err) {
					actions = append(actions, RequiredAction{
						Tool:   "write_ppt",
						Target: Resource{Type: "slide", SlideID: slideID, Part: "spec"},
					})
				}
			}
			if len(actions) == 0 {
				actions = append(actions, RequiredAction{Tool: "write_ppt", Target: Resource{Type: "deck", Part: "outline"}})
			}
		}
	}
	return CompletionIssue{
		Code:            "ASYNC_DECK_SLIDE",
		Summary:         "outline and slide specs are not synchronized: " + cause.Error(),
		RequiredActions: actions,
	}
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

func hasPPTDomainChanges(changes ChangeSet) bool {
	for _, change := range changes.All() {
		if isPPTDomainChange(change) {
			return true
		}
	}
	return false
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
		issues = append(issues, CompletionIssue{Code: "TOOLS_STILL_RUNNING", Summary: "a tool call is still running"})
	}
	for _, issue := range ctx.Issues {
		if issue.Severity == SeverityFatal {
			issues = append(issues, CompletionIssue{Code: "RUN_FATAL_EXIST", Summary: issue.Summary})
		}
	}
	if ctx.Canceled {
		issues = append(issues, CompletionIssue{Code: CodeRunAlreadyCanceled, Summary: "run was canceled"})
	}
	if isWriteStrategy(ctx.Strategy) {
		if ctx.Session == nil {
			issues = append(issues, CompletionIssue{Code: CodeRunSessionRequired, Summary: "write run has no active run session"})
		} else if err := ctx.Session.ValidateBaselines(); err != nil {
			issues = append(issues, CompletionIssue{Code: CodeRevisionConflict, Summary: err.Error()})
		}
	}
	if ctx.Strategy == StrategyFulfill && (ctx.Plan == nil || ctx.Plan.HasBlockingSteps()) {
		issues = append(issues, CompletionIssue{Code: "PLAN_NOT_COMPLETE", Summary: "fulfill strategy still has pending, in-progress, or failed steps"})
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
	case StrategyExecute, StrategyFulfill:
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
		target := Resource{Type: "slide", SlideID: slideID, Part: "html"}
		hash, err := targetHash(ctx.Context, ctx.Session, target)
		if err != nil {
			continue
		}
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

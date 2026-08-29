package workflow

import (
	"encoding/json"
	"fmt"
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
	Op     string   `json:"op,omitempty"`
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
	Mode          model.RunMode
	FinishPhase   RunPhase
	ActiveTools   int
	Issues        []Issue
	Scope         model.RunScope
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

const (
	CodeRunLanguageUnsatisfied = "RUN_LANGUAGE_UNSATISFIED"
	CodeRunRangeUnsatisfied    = "RUN_RANGE_UNSATISFIED"
)

type ScopeCompletionPolicy struct{}

func (ScopeCompletionPolicy) Check(ctx CompletionContext) []CompletionIssue {
	if ctx.Mode != model.ModeExecute {
		return nil
	}
	issues := []CompletionIssue{}
	for _, change := range ctx.Changes.All() {
		if AllowsArtifact(ctx.Scope, change.Artifact) {
			continue
		}
		target := resourceForArtifact(change.Artifact)
		issues = append(issues, CompletionIssue{
			Code: ErrTargetOutOfScope.Error(), Summary: "changed target is outside RunCommand scope",
			RequiredActions: []RequiredAction{{Target: target}},
		})
	}
	return dedupeCompletionIssues(issues)
}

type CommandOptionsCompletionPolicy struct{}

func (CommandOptionsCompletionPolicy) Check(ctx CompletionContext) []CompletionIssue {
	command := ctx.Context.Command
	if ctx.Mode != model.ModeExecute || command.Scope.Level != model.ScopeDeck ||
		(command.Options.Language == "" && command.Options.Range == "") {
		return nil
	}
	deck, err := currentManifest(ctx.Context, ctx.Session)
	if err != nil {
		return []CompletionIssue{{
			Code: "CONTEXT_SOURCE_INVALID", Summary: "cannot verify RunCommand options against the current outline",
			RequiredActions: []RequiredAction{{Tool: "read_ppt", Target: Resource{Type: "deck", Part: "outline"}}},
		}}
	}
	issues := []CompletionIssue{}
	if command.Options.Language != "" && string(command.Options.Language) != deck.Language {
		issues = append(issues, CompletionIssue{
			Code:            CodeRunLanguageUnsatisfied,
			Summary:         fmt.Sprintf("deck language %q does not satisfy RunCommand language %q", deck.Language, command.Options.Language),
			RequiredActions: []RequiredAction{{Tool: "mutate_ppt", Op: "manifest.patch", Target: Resource{Type: "deck", Part: "manifest"}}},
		})
	}
	outline, outlineErr := currentOutline(ctx.Context, ctx.Session)
	if outlineErr != nil {
		return append(issues, CompletionIssue{Code: "CONTEXT_SOURCE_INVALID", Summary: outlineErr.Error()})
	}
	count := len(spec.FlattenOutline(outline))
	if command.Options.Range != "" && !command.Options.Range.Contains(count) {
		issues = append(issues, CompletionIssue{
			Code:            CodeRunRangeUnsatisfied,
			Summary:         fmt.Sprintf("outline has %d slides, outside RunCommand range %q", count, command.Options.Range),
			RequiredActions: []RequiredAction{{Tool: "mutate_ppt", Op: "outline.insert", Target: Resource{Type: "deck", Part: "outline"}}},
		})
	}
	return issues
}

type EvidenceCompletionPolicy struct{}

func (EvidenceCompletionPolicy) Check(ctx CompletionContext) []CompletionIssue {
	if ctx.Mode != model.ModeExecute {
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
			if ctx.Context.Command.Scope.Artifact == model.ArtifactPPT && ctx.Session != nil {
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
			if ctx.Context.Command.Scope.Artifact == model.ArtifactPPT && ctx.Session != nil {
				if deck, err := currentOutline(ctx.Context, ctx.Session); err == nil {
					for _, location := range spec.FlattenOutline(deck) {
						slideID := location.Slide.SlideID
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
		RequiredActions: []RequiredAction{{Tool: "mutate_ppt", Op: operationForTarget(target, false), Target: target}},
	}
}

func operationForTarget(target Resource, patch bool) string {
	if target.Type == "slide" {
		if target.Part == "html" {
			if patch {
				return "slide.html.patch"
			}
			return "slide.html.write"
		}
		if patch {
			return "slide.spec.patch"
		}
		return "slide.spec.write"
	}
	if target.Part == "design" {
		if patch {
			return "design.patch"
		}
		return "design.write"
	}
	if target.Part == "manifest" {
		return "manifest.patch"
	}
	return "outline.update"
}

func htmlEvidenceIssue(target Resource) CompletionIssue {
	return CompletionIssue{
		Code: "EVIDENCE_HTML_MISSING", Summary: "HTML evidence is missing or stale for " + target.Key(),
		RequiredActions: []RequiredAction{
			{Tool: "mutate_ppt", Op: "slide.html.patch", Target: target},
			{Tool: "render_slide", Target: target},
		},
	}
}

func asyncSpecHTMLIssue(target Resource) CompletionIssue {
	return CompletionIssue{
		Code: "ASYNC_SPEC_HTML", Summary: "Spec or Design changes are not materialized in HTML for " + target.Key(),
		RequiredActions: []RequiredAction{
			{Tool: "mutate_ppt", Op: "slide.html.patch", Target: target},
			{Tool: "render_slide", Target: target},
		},
	}
}

func asyncDeckSlideIssue(ctx CompletionContext, cause error) CompletionIssue {
	actions := []RequiredAction{{Tool: "mutate_ppt", Op: "outline.init", Target: Resource{Type: "deck", Part: "outline"}}}
	if ctx.Session != nil {
		if deck, err := currentOutline(ctx.Context, ctx.Session); err == nil {
			actions = actions[:0]
			for _, location := range spec.FlattenOutline(deck) {
				slideID := location.Slide.SlideID
				if _, _, err := readArtifact(ctx.Session.ProjectDir(), ctx.Session, specSlideRef(slideID)); errorsIsNotExist(err) {
					actions = append(actions, RequiredAction{
						Tool: "mutate_ppt", Op: "slide.spec.write",
						Target: Resource{Type: "slide", SlideID: slideID, Part: "spec"},
					})
				}
			}
			if len(actions) == 0 {
				actions = append(actions, RequiredAction{Tool: "mutate_ppt", Op: "outline.update", Target: Resource{Type: "deck", Part: "outline"}})
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

// specChangeAffectsHTML ignores bookkeeping fields but treats every semantic
// or placement change as presentation-affecting.
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
	clearRuntime := func(value *spec.SlideSpec) {
		value.SchemaVersion = ""
		value.Revision = 0
		value.ProjectID = ""
		value.SlideID = ""
		value.CreatedAt = 0
		value.UpdatedAt = 0
	}
	clearRuntime(&before)
	clearRuntime(&after)
	return !reflect.DeepEqual(before, after)
}

func isPPTDomainChange(change ArtifactChange) bool {
	source := strings.TrimPrefix(change.Source, "tentative:")
	return source == "mutate_ppt"
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
	return CompletionGate{Policies: []CompletionPolicy{
		ScopeCompletionPolicy{},
		CommandOptionsCompletionPolicy{},
		EvidenceCompletionPolicy{},
	}}
}

func (g CompletionGate) Check(ctx CompletionContext) CompletionResult {
	issues := []CompletionIssue{}
	if !finishAllowed(ctx.Mode, ctx.FinishPhase) {
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
	if ctx.Mode == model.ModeExecute {
		if ctx.Session == nil {
			issues = append(issues, CompletionIssue{Code: CodeRunSessionRequired, Summary: "write run has no active run session"})
		} else if err := ctx.Session.ValidateBaselines(); err != nil {
			issues = append(issues, CompletionIssue{Code: CodeRevisionConflict, Summary: err.Error()})
		}
	}
	if ctx.Mode == model.ModeExecute && ctx.Plan != nil && ctx.Plan.HasBlockingSteps() {
		issues = append(issues, CompletionIssue{Code: "PLAN_NOT_COMPLETE", Summary: "the optional execution plan still has pending, in-progress, or failed steps"})
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

func finishAllowed(mode model.RunMode, phase RunPhase) bool {
	switch mode {
	case model.ModeTalk, model.ModeAsk:
		return phase == PhaseChat
	case model.ModeExecute:
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
	flat := spec.FlattenOutline(outline)
	proofs := make([]MaterializationProof, 0, len(flat))
	for _, location := range flat {
		slideID := location.Slide.SlideID
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

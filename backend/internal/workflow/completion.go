package workflow

import (
	"fmt"
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
	NextAction      string           `json:"next_action,omitempty"`
}

type RequiredAction struct {
	Tool   string   `json:"tool,omitempty"`
	Op     string   `json:"op,omitempty"`
	Target Resource `json:"target,omitempty"`
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
	Work          *WorkLedger
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
	removed := map[string]bool{}
	for _, change := range ctx.Changes.Deleted {
		removed[change.Artifact.Key()] = true
	}
	for _, change := range ctx.Changes.All() {
		if AllowsArtifact(ctx.Scope, change.Artifact) {
			continue
		}
		// Full-deck scope is refreshed to current membership after each commit;
		// deleted pages were authorized before their files and identities vanished.
		if ctx.Scope.IncludeRunCreatedSlides && removed[change.Artifact.Key()] {
			if _, exists := spec.FindSlide(ctx.Context.Outline.Outline, change.Artifact.ID); !exists {
				continue
			}
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
	if ctx.Mode != model.ModeExecute || command.Scope.Source.Kind != model.ScopeAllPages ||
		(command.Options.Language == "" && command.Options.Range == "") {
		return nil
	}
	deck, err := currentManifest(ctx.Context, ctx.Session)
	if err != nil {
		return []CompletionIssue{{
			Code: "CONTEXT_SOURCE_INVALID", Summary: "cannot verify RunCommand options against the current manifest: " + err.Error(),
			RequiredActions: []RequiredAction{{Tool: "read_resource", Target: Resource{Type: "deck", Part: "manifest"}}},
		}}
	}
	issues := []CompletionIssue{}
	if command.Options.Language != "" && string(command.Options.Language) != deck.Language {
		issues = append(issues, CompletionIssue{
			Code:            CodeRunLanguageUnsatisfied,
			Summary:         fmt.Sprintf("deck language %q does not satisfy RunCommand language %q", deck.Language, command.Options.Language),
			RequiredActions: []RequiredAction{{Tool: "edit_manifest", Target: Resource{Type: "deck", Part: "manifest"}}},
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
			RequiredActions: []RequiredAction{{Tool: outlineEditAction(ctx), Target: Resource{Type: "deck", Part: "outline"}}},
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
	for _, change := range append(ctx.Changes.Created, ctx.Changes.Updated...) {
		target := resourceForArtifact(change.Artifact)
		if !isPPTDomainChange(change) {
			switch change.Artifact.Kind {
			case ArtifactProjectFile:
				continue
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
		case ArtifactManifest, ArtifactOutline:
			if !hasFreshEvidence(ctx, target, change.AfterHash, "schema") {
				issues = append(issues, schemaEvidenceIssue(target))
			}
		case ArtifactSlideSpec, ArtifactDesign:
			if !hasFreshEvidence(ctx, target, change.AfterHash, "schema") {
				issues = append(issues, schemaEvidenceIssue(target))
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
	return hasFreshEvidence(ctx, target, hash, "static") && hasFreshRender(ctx, target, hash)
}

func hasFreshRender(ctx CompletionContext, target Resource, hash string) bool {
	if ctx.Evidence == nil || ctx.Session == nil || hash == "" {
		return false
	}
	proof, ok := ctx.Evidence.FreshRenderProof(target, hash)
	if !ok {
		return false
	}
	expected, err := currentRenderProof(
		ctx.Context, ctx.Session.ProjectDir(), ctx.Session, target.SlideID, hash,
	)
	return err == nil && proof == expected
}

func schemaEvidenceIssue(target Resource) CompletionIssue {
	return CompletionIssue{
		Code: "EVIDENCE_SCHEMA_MISSING", Summary: "schema evidence is missing or stale for " + target.Key(),
		RequiredActions: []RequiredAction{{Tool: operationForTarget(target, false), Target: target}},
	}
}

func operationForTarget(target Resource, patch bool) string {
	if target.Type == "slide" {
		if target.Part == "html" {
			if patch {
				return "patch_html"
			}
			return "write_html"
		}
		if patch {
			return "edit_spec"
		}
		return "edit_spec"
	}
	if target.Part == "design" {
		if patch {
			return "edit_design"
		}
		return "edit_design"
	}
	if target.Part == "manifest" {
		return "edit_manifest"
	}
	return "arrange_outline"
}

func htmlEvidenceIssue(target Resource) CompletionIssue {
	return CompletionIssue{
		Code: "EVIDENCE_HTML_MISSING", Summary: "HTML evidence is missing or stale for " + target.Key(),
		RequiredActions: []RequiredAction{{Tool: "render_slide", Target: target}},
	}
}

func asyncDeckSlideIssue(ctx CompletionContext, cause error) CompletionIssue {
	actions := []RequiredAction{{Tool: outlineEditAction(ctx), Target: Resource{Type: "deck", Part: "outline"}}}
	if ctx.Session != nil {
		if deck, err := currentOutline(ctx.Context, ctx.Session); err == nil {
			actions = actions[:0]
			for _, location := range spec.FlattenOutline(deck) {
				slideID := location.Slide.SlideID
				if _, _, err := readArtifact(ctx.Session.ProjectDir(), ctx.Session, specSlideRef(slideID)); errorsIsNotExist(err) {
					actions = append(actions, RequiredAction{
						Tool:   "edit_spec",
						Target: Resource{Type: "slide", SlideID: slideID, Part: "spec"},
					})
				}
			}
			if len(actions) == 0 {
				actions = append(actions, RequiredAction{Tool: "arrange_outline", Target: Resource{Type: "deck", Part: "outline"}})
			}
		}
	}
	return CompletionIssue{
		Code:            "ASYNC_DECK_SLIDE",
		Summary:         "outline and slide specs are not synchronized: " + cause.Error(),
		RequiredActions: actions,
	}
}

func isPPTDomainChange(change ArtifactChange) bool {
	source := strings.TrimPrefix(change.Source, "tentative:")
	if !isResourceEditTool(source) && source != "run_command" {
		return false
	}
	switch change.Artifact.Kind {
	case ArtifactManifest, ArtifactOutline, ArtifactDesign, ArtifactSlideSpec, ArtifactSlideHTML:
		return true
	default:
		return false
	}
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
			issues = append(issues, CompletionIssue{Code: CodeContentConflict, Summary: err.Error()})
		}
	}
	if ctx.Mode == model.ModeExecute && ctx.Plan != nil && ctx.Plan.HasBlockingSteps() {
		unfinished := []string{}
		for _, step := range ctx.Plan.Steps {
			if step.Status != PlanStepCompleted {
				unfinished = append(unfinished, fmt.Sprintf("%s (%s)", step.ID, step.Status))
			}
		}
		issues = append(issues, CompletionIssue{
			Code: "PLAN_NOT_COMPLETE", Summary: "unfinished plan steps: " + strings.Join(unfinished, ", "),
			RequiredActions: []RequiredAction{{Tool: "update_plan"}},
			NextAction:      "Complete the actual work, then call update_plan with an updates array of {step_id, status} objects using the listed IDs before calling finish again.",
		})
	}
	if ctx.Mode == model.ModeExecute && ctx.Work != nil && ctx.Work.HasBlockingItems() {
		issues = append(issues, CompletionIssue{Code: "WORK_NOT_COMPLETE", Summary: "the explicit page work ledger still has pending, running, or failed items"})
	}
	for _, policy := range g.Policies {
		issues = append(issues, policy.Check(ctx)...)
	}
	return CompletionResult{Accepted: len(issues) == 0, Issues: issues}
}

func finishAllowed(mode model.RunMode, phase RunPhase) bool {
	switch mode {
	case model.ModeChat, model.ModeGrill:
		return phase == PhaseChat
	case model.ModeExecute:
		return phase == PhaseExecuting
	default:
		return false
	}
}

func outlineEditAction(ctx CompletionContext) string {
	if ctx.Session != nil {
		if _, err := ctx.Session.ReadPath(".outline.json"); errorsIsNotExist(err) {
			return "init_outline"
		}
	}
	return "arrange_outline"
}

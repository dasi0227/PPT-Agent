package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
)

type VerifyInput struct {
	State       WorkflowState
	Plan        WorkflowPlan
	Context     contextengine.ContextPack
	Transaction *Transaction
}

type Verifier interface {
	Name() string
	Verify(context.Context, VerifyInput) VerifyResult
}

type VerifierRegistry struct {
	verifiers map[string]Verifier
}

func NewVerifierRegistry(verifiers ...Verifier) *VerifierRegistry {
	out := &VerifierRegistry{verifiers: map[string]Verifier{}}
	for _, verifier := range verifiers {
		out.verifiers[verifier.Name()] = verifier
	}
	return out
}

func (r *VerifierRegistry) Get(name string) (Verifier, bool) {
	verifier, ok := r.verifiers[name]
	return verifier, ok
}

type BlueprintVerifier struct{}

func (BlueprintVerifier) Name() string { return "blueprint" }

func (BlueprintVerifier) Verify(_ context.Context, input VerifyInput) VerifyResult {
	result := emptyVerifyResult()
	deck := input.Context.Deck.Deck
	slides := make(map[string]blueprint.Slide, len(input.Context.Deck.Deck.SlideOrder))
	for _, id := range input.Context.Deck.Deck.SlideOrder {
		if target := input.Context.Target.Slide; target != nil && target.SlideID == id {
			slides[id] = *target
			continue
		}
		for _, summary := range input.Context.Deck.Summaries {
			if summary.ID == id {
				slides[id] = blueprint.Slide{
					SchemaVersion: blueprint.SchemaVersion, Revision: input.Context.Revisions.Slides[id],
					SlideID: id, SectionID: summary.SectionID, SubsectionID: summary.SubsectionID,
					Role: summary.Role, Title: summary.Title, KeyMessage: summary.KeyMessage,
					Content:      blueprint.Content{Summary: summary.KeyMessage, Points: []string{}},
					VisualIntent: blueprint.VisualIntent{Archetype: "content", Description: "context summary", AssetQueries: []string{}},
				}
			}
		}
	}
	if input.Transaction != nil {
		if raw, err := input.Transaction.Read(deckRef(input.Context)); err == nil {
			if err := json.Unmarshal(raw, &deck); err != nil {
				return failedVerify("BLUEPRINT_SCHEMA", deckRef(input.Context), err.Error(), "write valid deck JSON", "blueprint")
			}
		}
		ids := deck.SlideOrder
		for _, id := range ids {
			ref := blueprintSlideRef(id)
			raw, err := input.Transaction.Read(ref)
			if err != nil {
				return failedVerify("BLUEPRINT_REFERENCE_BROKEN", ref, err.Error(), "create the referenced slide blueprint", "blueprint")
			}
			var slide blueprint.Slide
			if err := json.Unmarshal(raw, &slide); err != nil {
				return failedVerify("BLUEPRINT_SCHEMA", ref, err.Error(), "write valid slide JSON", "blueprint")
			}
			slides[id] = slide
		}
	}
	if err := blueprint.ValidateDeck(deck, slides); err != nil {
		result.Issues = append(result.Issues, Issue{
			Code: "BLUEPRINT_INVALID", Severity: SeverityError, Artifact: deckRef(input.Context),
			Evidence: err.Error(), RepairHint: "repair schema and section/slide references", Verifier: "blueprint",
		})
	}
	if len(deck.SlideOrder) > 1 {
		titleSeen := map[string]string{}
		for _, id := range deck.SlideOrder {
			slide := slides[id]
			normalized := strings.ToLower(strings.TrimSpace(slide.Title))
			if other := titleSeen[normalized]; normalized != "" && other != "" {
				result.Issues = append(result.Issues, Issue{
					Code: "BLUEPRINT_DUPLICATE_TITLE", Severity: SeverityWarning, Artifact: blueprintSlideRef(id),
					Evidence: fmt.Sprintf("title duplicates slide %s", other), RepairHint: "make each page role and title distinct", Verifier: "blueprint",
				})
			}
			titleSeen[normalized] = id
		}
	}
	result.Passed = !blocksCommit(result.Issues)
	return result
}

type PresentationStaticVerifier struct{}

func (PresentationStaticVerifier) Name() string { return "presentation_static" }

func (PresentationStaticVerifier) Verify(_ context.Context, input VerifyInput) VerifyResult {
	result := emptyVerifyResult()
	for _, ref := range input.Plan.Affected {
		if ref.Kind != ArtifactPresentation {
			continue
		}
		raw, err := input.Transaction.Read(ref)
		if err != nil {
			result.Issues = append(result.Issues, Issue{
				Code: "PRESENTATION_MISSING", Severity: SeverityError, Artifact: ref,
				Evidence: err.Error(), RepairHint: "materialize the slide HTML", Verifier: "presentation_static",
			})
			continue
		}
		for _, check := range designsystem.LintSlide(raw) {
			if check.OK {
				continue
			}
			result.Issues = append(result.Issues, Issue{
				Code:     "HTML_" + strings.ToUpper(strings.ReplaceAll(check.ID, "-", "_")),
				Severity: SeverityError, Artifact: ref, Evidence: check.Reason,
				RepairHint: "update the staged HTML to satisfy " + check.ID, Verifier: "presentation_static",
			})
		}
		if slide := blueprintFor(input.Context, ref.ID); slide != nil {
			text := strings.ToLower(string(raw))
			if !strings.Contains(text, strings.ToLower(slide.Title)) &&
				!strings.Contains(text, strings.ToLower(slide.KeyMessage)) {
				result.Issues = append(result.Issues, Issue{
					Code: "HTML_BLUEPRINT_TEXT_MISSING", Severity: SeverityError, Artifact: ref,
					Evidence:   "neither blueprint title nor key message is present",
					RepairHint: "include the blueprint's primary semantic content", Verifier: "presentation_static",
				})
			}
		}
	}
	result.Passed = !blocksCommit(result.Issues)
	return result
}

type BrowserProbeResult struct {
	Loaded          bool
	ConsoleErrors   []string
	RuntimeErrors   []string
	Overflow        []string
	BrokenResources []string
	ViewportOK      bool
	TimedOut        bool
}

type BrowserProbe interface {
	Probe(context.Context, string, []byte) BrowserProbeResult
}

type IsolatedHTMLProbe struct{}

var (
	reRemoteResource = regexp.MustCompile(`(?i)(?:src|href)\s*=\s*["'](https?:)?//([^/"']+)`)
	reConsoleError   = regexp.MustCompile(`(?i)console\.error\s*\(`)
	reViewport       = regexp.MustCompile(`(?i)<meta[^>]+name=["']viewport["']`)
)

func (IsolatedHTMLProbe) Probe(_ context.Context, _ string, raw []byte) BrowserProbeResult {
	lower := bytes.ToLower(raw)
	result := BrowserProbeResult{
		Loaded:        bytes.Contains(lower, []byte("<html")) && bytes.Contains(lower, []byte("<body")),
		ConsoleErrors: []string{}, RuntimeErrors: []string{}, Overflow: []string{},
		BrokenResources: []string{}, ViewportOK: reViewport.Match(raw),
	}
	if reConsoleError.Match(raw) {
		result.ConsoleErrors = append(result.ConsoleErrors, "page contains console.error")
	}
	if bytes.Contains(lower, []byte("throw new error")) {
		result.RuntimeErrors = append(result.RuntimeErrors, "page contains an unconditional runtime error")
	}
	for _, match := range reRemoteResource.FindAllSubmatch(raw, -1) {
		host := string(match[2])
		if host == "" {
			result.BrokenResources = append(result.BrokenResources, string(match[0]))
		}
	}
	if bytes.Contains(lower, []byte("overflow-marker")) || bytes.Contains(lower, []byte("data-overflow=\"true\"")) {
		result.Overflow = append(result.Overflow, "overflow marker detected")
	}
	return result
}

type BrowserVerifier struct {
	Probe BrowserProbe
}

func (BrowserVerifier) Name() string { return "browser" }

func (v BrowserVerifier) Verify(ctx context.Context, input VerifyInput) VerifyResult {
	result := emptyVerifyResult()
	probe := v.Probe
	if probe == nil {
		probe = IsolatedHTMLProbe{}
	}
	for _, ref := range input.Plan.Affected {
		if ref.Kind != ArtifactPresentation {
			continue
		}
		raw, err := input.Transaction.Read(ref)
		if err != nil {
			result.Issues = append(result.Issues, browserIssue("BROWSER_LOAD_FAILED", ref, err.Error()))
			continue
		}
		path, _ := input.Transaction.StagedPath(ref)
		observation := probe.Probe(ctx, path, raw)
		if observation.TimedOut {
			result.Issues = append(result.Issues, browserIssue("BROWSER_TIMEOUT", ref, "sandbox render timed out"))
		}
		if !observation.Loaded {
			result.Issues = append(result.Issues, browserIssue("BROWSER_LOAD_FAILED", ref, "document did not load"))
		}
		if !observation.ViewportOK {
			result.Issues = append(result.Issues, browserIssue("BROWSER_VIEWPORT", ref, "viewport meta is missing"))
		}
		for _, message := range append(observation.ConsoleErrors, observation.RuntimeErrors...) {
			result.Issues = append(result.Issues, browserIssue("BROWSER_RUNTIME_ERROR", ref, message))
		}
		for _, message := range observation.Overflow {
			result.Issues = append(result.Issues, browserIssue("BROWSER_OVERFLOW", ref, message))
		}
		for _, message := range observation.BrokenResources {
			result.Issues = append(result.Issues, browserIssue("BROWSER_BROKEN_RESOURCE", ref, message))
		}
	}
	result.Passed = !blocksCommit(result.Issues)
	return result
}

type CrossSlideVerifier struct{}

func (CrossSlideVerifier) Name() string { return "cross_slide" }

func (CrossSlideVerifier) Verify(_ context.Context, input VerifyInput) VerifyResult {
	result := emptyVerifyResult()
	signatures := map[string]string{}
	for _, ref := range input.Plan.Affected {
		if ref.Kind != ArtifactPresentation {
			continue
		}
		raw, err := input.Transaction.Read(ref)
		if err != nil {
			continue
		}
		s := string(raw)
		if strings.Index(s, "tokens.css") > strings.Index(s, "base.css") && strings.Contains(s, "base.css") {
			result.Issues = append(result.Issues, Issue{
				Code: "CROSS_SLIDE_COMMON_ORDER", Severity: SeverityError, Artifact: ref,
				Evidence: "tokens.css must be linked before base.css", RepairHint: "normalize common layer order", Verifier: "cross_slide",
			})
		}
		digest := normalizedStructure(s)
		if other, exists := signatures[digest]; exists {
			result.Issues = append(result.Issues, Issue{
				Code: "CROSS_SLIDE_DUPLICATE", Severity: SeverityWarning, Artifact: ref,
				Evidence: "structure duplicates " + other, RepairHint: "vary composition and information rhythm", Verifier: "cross_slide",
			})
		}
		signatures[digest] = ref.ID
	}
	result.Passed = !blocksCommit(result.Issues)
	return result
}

func emptyVerifyResult() VerifyResult {
	return VerifyResult{Passed: true, Issues: []Issue{}, Evidence: []EvidenceRef{}, Metrics: map[string]float64{}}
}

func failedVerify(code string, ref ArtifactRef, evidence, hint, verifier string) VerifyResult {
	issue := Issue{Code: code, Severity: SeverityError, Artifact: ref, Evidence: evidence, RepairHint: hint, Verifier: verifier}
	return VerifyResult{Passed: false, Issues: []Issue{issue}, Evidence: []EvidenceRef{}, Metrics: map[string]float64{}}
}

func browserIssue(code string, ref ArtifactRef, evidence string) Issue {
	return Issue{
		Code: code, Severity: SeverityError, Artifact: ref, Evidence: evidence,
		RepairHint: "repair the staged slide and rerun isolated browser verification", Verifier: "browser",
	}
}

func blocksCommit(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Severity.BlocksCommit() {
			return true
		}
	}
	return false
}

func blueprintFor(pack contextengine.ContextPack, id string) *blueprint.Slide {
	if pack.Target.Slide != nil && pack.Target.Slide.SlideID == id {
		return pack.Target.Slide
	}
	return nil
}

func normalizedStructure(html string) string {
	reText := regexp.MustCompile(`>[^<]+<`)
	reSpace := regexp.MustCompile(`\s+`)
	return reSpace.ReplaceAllString(reText.ReplaceAllString(strings.ToLower(html), "><"), " ")
}

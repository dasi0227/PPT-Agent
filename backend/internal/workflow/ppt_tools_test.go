package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
)

const validToolHTML = `<!doctype html>
<html lang="zh"><head>
<link rel="stylesheet" href="../../common/tokens.css">
<link rel="stylesheet" href="../../common/base.css">
</head><body><section class="slide-stage"><h1>Original</h1></section></body></html>`

type recordingRenderer struct {
	html        string
	diagnostics RenderDiagnostics
	err         error
}

func (r *recordingRenderer) Render(_ context.Context, request RenderRequest) (RenderDiagnostics, error) {
	r.html = request.HTML
	if r.err != nil {
		return RenderDiagnostics{}, r.err
	}
	if err := os.WriteFile(request.ScreenshotPath, []byte("\x89PNG\r\n\x1a\nfake"), 0o600); err != nil {
		return RenderDiagnostics{}, err
	}
	out := r.diagnostics
	if out.ScreenshotBytes == 0 {
		out.ScreenshotBytes = 12
	}
	if out.ContentSize == nil {
		out.ContentSize = map[string]int{"width": 1600, "height": 900}
	}
	if out.Overflow == nil {
		out.Overflow = map[string]bool{"horizontal": false, "vertical": false}
	}
	if out.FontStatus == "" {
		out.FontStatus = "loaded"
	}
	return out, nil
}

func TestPPTToolSchemasExposeOnlyResourceStringContracts(t *testing.T) {
	_, pack := toolProject(t, model.ArtifactPresentation, model.TargetDeck)
	tests := []struct {
		tool     DomainTool
		required []string
		forbid   []string
	}{
		{pptReadTool{pack}, []string{`"resource"`}, []string{`"include"`, `"target"`}},
		{pptWriteTool{pack}, []string{`"resource"`, `"content"`, `"type":"string"`}, []string{`"model"`, `"target"`}},
		{pptEditTool{pack}, []string{`"resource"`, `"old_text"`, `"new_text"`}, []string{`"json_edit"`, `"path"`, `"target"`}},
	}
	for _, test := range tests {
		raw, _ := json.Marshal(test.tool.Schema().Parameters)
		text := string(raw)
		for _, value := range test.required {
			if !strings.Contains(text, value) {
				t.Fatalf("%s schema missing %s: %s", test.tool.Schema().Name, value, text)
			}
		}
		for _, value := range test.forbid {
			if strings.Contains(text, value) {
				t.Fatalf("%s schema leaked %s: %s", test.tool.Schema().Name, value, text)
			}
		}
	}
}

func TestReadPPTReturnsCompleteRawStringForAllResources(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetDeck)
	resources := []Resource{
		{Type: "deck", Part: "outline"},
		{Type: "deck", Part: "design"},
		{Type: "slide", SlideID: "slide-01", Part: "spec"},
		{Type: "slide", SlideID: "slide-01", Part: "html"},
	}
	for _, resource := range resources {
		ref, _ := refForResource(pack, resource)
		want, _, _ := readArtifact(dir, nil, ref)
		result := (pptReadTool{pack}).Execute(context.Background(), toolInput(pack, dir, nil, map[string]any{
			"resource": resourceArgs(resource),
		}))
		if !result.OK || result.Observation != string(want) || len(result.Data) != 0 {
			t.Fatalf("%s read=%+v want=%q", resource.Key(), result, want)
		}
	}
}

func TestReadPPTRejectsOversizedContentWithoutTruncation(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetDeck)
	large := strings.Repeat("x", maxPPTContentBytes+1)
	if err := os.WriteFile(filepath.Join(dir, model.SlideHTMLPath("slide-01")), []byte(large), 0o644); err != nil {
		t.Fatal(err)
	}
	result := (pptReadTool{pack}).Execute(context.Background(), toolInput(pack, dir, nil, map[string]any{
		"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-01", Part: "html"}),
	}))
	if result.OK || result.Code != CodeContentTooLarge || result.Observation == "" || result.Retryable {
		t.Fatalf("result=%+v", result)
	}
}

func TestWritePPTRequiresStringAndInjectsManagedMetadata(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactSpec, model.TargetSlide)
	tx, _ := NewRunSession(dir, "write-string")
	resource := Resource{Type: "slide", SlideID: "slide-01", Part: "spec"}
	rejected := (pptWriteTool{pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"resource": resourceArgs(resource), "content": map[string]any{"title": "bad"},
	}))
	if rejected.OK || rejected.Code != CodeContentInvalid {
		t.Fatalf("non-string accepted: %+v", rejected)
	}
	next := slideModel("model-controlled-id", "Updated")
	next.SchemaVersion, next.ProjectID, next.Revision = "wrong", "wrong", 999
	result := (pptWriteTool{pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"resource": resourceArgs(resource), "content": string(mustJSONValue(next)),
	}))
	if !result.OK {
		t.Fatalf("write=%+v", result)
	}
	raw, _ := tx.Read(specSlideRef("slide-01"))
	var saved spec.SlideSpec
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.SchemaVersion != spec.SchemaVersion || saved.ProjectID != "p1" ||
		saved.SlideID != "slide-01" || saved.Revision != 2 || saved.SourceOutlineRevision != 1 {
		t.Fatalf("runtime metadata not enforced: %+v", saved)
	}
	if err := pptschema.ValidateJSON(pptschema.SlideSpecName, raw); err != nil {
		t.Fatalf("runtime-injected resource failed complete schema validation: %v", err)
	}
}

func TestDesignWriteAtomicallyWritesDerivedTokens(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetDeck)
	tx, _ := NewRunSession(dir, "write-design")
	result := (pptWriteTool{pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"resource": resourceArgs(Resource{Type: "deck", Part: "design"}),
		"content":  string(mustJSONValue(designModel())),
	}))
	if !result.OK {
		t.Fatalf("design write=%+v", result)
	}
	tokens, err := tx.Read(designTokensRef(pack))
	if err != nil || !strings.Contains(string(tokens), "--stage-w: 1600") {
		t.Fatalf("derived tokens were not written: %q err=%v", tokens, err)
	}
	if changes := tx.ChangeSet(); changes.Count() != 1 ||
		changes.All()[0].Artifact.Kind != ArtifactDesign {
		t.Fatalf("derived runtime file leaked into public change set: %+v", changes)
	}
}

func TestLineDiffStatCountsInsertionsAndDeletions(t *testing.T) {
	insertions, deletions := lineDiffStat(
		[]byte("a\nb\nc\n"),
		[]byte("a\nb2\nc\nd\n"),
	)
	if insertions != 2 || deletions != 1 {
		t.Fatalf("stat +%d -%d, want +2 -1", insertions, deletions)
	}
}

func TestWritePPTValidatesJSONHTMLArtifactAndScope(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	tx, _ := NewRunSession(dir, "write-validation")
	invalidJSON := (pptWriteTool{pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-01", Part: "spec"}), "content": "{",
	}))
	if invalidJSON.OK || invalidJSON.Code != CodeContentInvalid {
		t.Fatalf("invalid JSON accepted: %+v", invalidJSON)
	}
	invalidHTML := (pptWriteTool{pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-01", Part: "html"}), "content": "<p>no stage</p>",
	}))
	if invalidHTML.OK || invalidHTML.Code != CodeContentInvalid {
		t.Fatalf("invalid HTML accepted: %+v", invalidHTML)
	}
	specPack := pack
	specPack.WorkSpec.Target.Artifact = model.ArtifactSpec
	forbidden := (pptWriteTool{specPack}).Execute(context.Background(), toolInput(specPack, dir, tx, map[string]any{
		"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-01", Part: "html"}), "content": validToolHTML,
	}))
	if forbidden.OK || forbidden.Code != CodeTargetOutOfScope {
		t.Fatalf("spec HTML write accepted: %+v", forbidden)
	}
	deckWrite := (pptWriteTool{pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"resource": resourceArgs(Resource{Type: "deck", Part: "design"}), "content": string(mustJSONValue(designModel())),
	}))
	if deckWrite.OK || deckWrite.Code != CodeTargetOutOfScope {
		t.Fatalf("slide-level deck write accepted: %+v", deckWrite)
	}
}

func TestEditPPTUsesOrderedUniqueAnchorsAndIsAtomic(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	tx, _ := NewRunSession(dir, "edit")
	resource := Resource{Type: "slide", SlideID: "slide-01", Part: "spec"}
	tool := pptEditTool{pack}
	ok := tool.Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"resource": resourceArgs(resource),
		"edits": []any{
			map[string]any{"old_text": `"title": "Original"`, "new_text": `"title": "Edited"`},
			map[string]any{"old_text": `"key_message": "Original"`, "new_text": `"key_message": "Edited"`},
		},
	}))
	if !ok.OK || len(tx.ChangeSet().Updated) != 1 {
		t.Fatalf("edit=%+v changes=%+v", ok, tx.ChangeSet())
	}
	before, _ := tx.Read(specSlideRef("slide-01"))
	failed := tool.Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"resource": resourceArgs(resource),
		"edits": []any{
			map[string]any{"old_text": `"title": "Edited"`, "new_text": `"title": "Must rollback"`},
			map[string]any{"old_text": "missing anchor", "new_text": "x"},
		},
	}))
	after, _ := tx.Read(specSlideRef("slide-01"))
	if failed.OK || failed.Code != CodeEditAnchorNotFound || string(before) != string(after) {
		t.Fatalf("failed edit was not atomic: result=%+v", failed)
	}
	htmlResource := Resource{Type: "slide", SlideID: "slide-01", Part: "html"}
	ambiguous := tool.Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"resource": resourceArgs(htmlResource),
		"edits":    []any{map[string]any{"old_text": "section", "new_text": "article"}},
	}))
	if ambiguous.OK || ambiguous.Code != CodeEditAnchorAmbiguous {
		t.Fatalf("ambiguous anchor accepted: %+v", ambiguous)
	}
}

func TestRenderSlideUsesDirectWrittenHTMLAndProducesEvidence(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	tx, _ := NewRunSession(dir, "render")
	written := strings.Replace(validToolHTML, "Original", "Written", 1)
	if _, err := tx.Write(slideHTMLRef("slide-01"), "test", []byte(written)); err != nil {
		t.Fatal(err)
	}
	renderer := &recordingRenderer{}
	input := toolInput(pack, dir, tx, map[string]any{"slide_id": "slide-01"})
	input.CallID = "render-call-1"
	result := (slideRenderTool{pack: pack, renderer: renderer}).Execute(context.Background(), input)
	if !result.OK || renderer.html != written || len(result.Evidence) != 1 ||
		result.Evidence[0].Target.Key() != "slide:slide-01:html" ||
		len(result.ObservationParts) != 1 || result.ObservationParts[0].Type != "text" {
		t.Fatalf("render=%+v html=%q", result, renderer.html)
	}
	var observation map[string]any
	if err := json.Unmarshal([]byte(result.ObservationParts[0].Text), &observation); err != nil ||
		observation["tool_call_id"] != "render-call-1" || observation["slide_id"] != "slide-01" ||
		observation["source_hash"] == "" {
		t.Fatalf("render observation lost call/slide/source binding: %+v err=%v", observation, err)
	}
	public, ok := (ToolPublicProjector{}).Completed(
		"run-1", "render-call-1", "render_slide", map[string]any{"slide_id": "slide-01"}, result,
	)
	publicRaw, err := json.Marshal(public)
	if !ok || err != nil || public.Preview == nil ||
		public.Preview.ImageURL != stringValue(result.Data["screenshot_url"]) {
		t.Fatalf("public render preview missing: payload=%+v err=%v", public, err)
	}
	for _, forbidden := range []string{"screenshot_ref", "run:run-1/screenshot:", "base64", dir} {
		if strings.Contains(string(publicRaw), forbidden) {
			t.Fatalf("public render event leaked %q: %s", forbidden, publicRaw)
		}
	}
}

func TestRenderSlideAttachesScreenshotOnVisualReview(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	tx, _ := NewRunSession(dir, "render")
	written := strings.Replace(validToolHTML, "Original", "Written", 1)
	if _, err := tx.Write(slideHTMLRef("slide-01"), "test", []byte(written)); err != nil {
		t.Fatal(err)
	}
	renderer := &recordingRenderer{}
	input := toolInput(pack, dir, tx, map[string]any{"slide_id": "slide-01", "visual_review": true})
	input.CallID = "render-call-1"
	result := (slideRenderTool{pack: pack, renderer: renderer}).Execute(context.Background(), input)
	if !result.OK || len(result.ObservationParts) != 2 ||
		result.ObservationParts[0].Type != "text" || result.ObservationParts[1].Type != "image" {
		t.Fatalf("visual_review render=%+v", result)
	}
}

func TestRenderSlideClassifiesWorkerInfrastructureFailureAsTransient(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	renderer := &recordingRenderer{err: renderWorkerError(
		"worker_stdin_write", errors.New("broken pipe /Users/private/worker.mjs"),
	)}
	result := (slideRenderTool{pack: pack, renderer: renderer}).Execute(
		context.Background(), toolInput(pack, dir, nil, map[string]any{"slide_id": "slide-01"}),
	)
	if result.OK || result.Code != CodeRenderWorkerUnavailable || !result.Retryable {
		t.Fatalf("worker infrastructure error classification=%+v", result)
	}
	public, ok := (ToolPublicProjector{}).Completed(
		"run-1", "render-call", "render_slide", map[string]any{"slide_id": "slide-01"}, result,
	)
	publicRaw, err := json.Marshal(public)
	if !ok || err != nil || public.Error == nil || public.Error.Code != CodeRenderWorkerUnavailable ||
		!public.Error.Retryable {
		t.Fatalf("worker public projection=%+v err=%v", public, err)
	}
	if strings.Contains(string(publicRaw), "/Users/") || strings.Contains(string(publicRaw), "broken pipe") {
		t.Fatalf("worker cause leaked to public event: %s", publicRaw)
	}
}

func TestNodeRendererStartupFailureUsesWorkerUnavailableSentinel(t *testing.T) {
	renderer := NewNodeSlideRenderer(NodeRendererConfig{
		NodePath: filepath.Join(t.TempDir(), "missing-node"),
		Timeout:  time.Second,
	})
	defer renderer.Close()
	_, err := renderer.Render(context.Background(), RenderRequest{})
	if !errors.Is(err, ErrRenderWorkerUnavailable) || errors.Is(err, context.Canceled) {
		t.Fatalf("worker startup error=%v", err)
	}
}

func TestRenderSlideKeepsPageDiagnosticsAgentRepairableAndNonRetryable(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	renderer := &recordingRenderer{diagnostics: RenderDiagnostics{
		Overflow: map[string]bool{"horizontal": true, "vertical": false},
	}}
	result := (slideRenderTool{pack: pack, renderer: renderer}).Execute(
		context.Background(), toolInput(pack, dir, nil, map[string]any{"slide_id": "slide-01"}),
	)
	if result.OK || result.Code != CodeRenderFailed || result.Retryable {
		t.Fatalf("page diagnostic classification=%+v", result)
	}
	definition := model.ErrorDefinitionFor(result.Code)
	if definition.Category != model.ErrorAgentRepairable {
		t.Fatalf("page diagnostics category=%s", definition.Category)
	}
}

func TestRenderSlideDoesNotClassifyContextCancellationAsTransient(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	result := (slideRenderTool{pack: pack, renderer: &recordingRenderer{err: context.Canceled}}).Execute(
		context.Background(), toolInput(pack, dir, nil, map[string]any{"slide_id": "slide-01"}),
	)
	if result.OK || result.Code != CodeCanceled || result.Retryable {
		t.Fatalf("render cancellation classification=%+v", result)
	}
}

func TestCompletionGateRequiresLatestStaticAndRenderEvidence(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	tx, _ := NewRunSession(dir, "gate")
	if _, err := tx.Write(slideHTMLRef("slide-01"), "edit_ppt", []byte(strings.Replace(validToolHTML, "Original", "Changed", 1))); err != nil {
		t.Fatal(err)
	}
	hash, _ := renderSourceHash(pack, tx, "slide-01")
	resource := Resource{Type: "slide", SlideID: "slide-01", Part: "html"}
	ledger := NewEvidenceLedger()
	ledger.Record(staticEvidence(resource, hash))
	ctx := CompletionContext{
		Strategy: StrategyExecute, FinishPhase: PhaseExecuting, WorkScope: ScopeFromSpec(pack.WorkSpec),
		Session: tx, Changes: tx.ChangeSet(), Evidence: ledger, Context: pack,
	}
	if result := NewCompletionGate().Check(ctx); result.Accepted || !hasCompletionCode(result, "VISUAL_EVIDENCE_REQUIRED") {
		t.Fatalf("missing render accepted: %+v", result)
	}
	rendered := (slideRenderTool{pack: pack, renderer: &recordingRenderer{}}).Execute(
		context.Background(), toolInput(pack, dir, tx, map[string]any{"slide_id": "slide-01"}))
	recordResultEvidence(ledger, rendered)
	if result := NewCompletionGate().Check(ctx); !result.Accepted {
		t.Fatalf("fresh evidence rejected: %+v", result)
	}
}

func TestSpecArtifactCanFinishAfterSlideSpecOnlyChange(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactSpec, model.TargetSlide)
	tx, _ := NewRunSession(dir, "spec-only")
	next := slideModel("slide-01", "Updated")
	result := (pptWriteTool{pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-01", Part: "spec"}),
		"content":  string(mustJSONValue(next)),
	}))
	ledger := NewEvidenceLedger()
	recordResultEvidence(ledger, result)
	recordResultEvidence(ledger, (slideRenderTool{pack: pack, renderer: &recordingRenderer{}}).Execute(
		context.Background(), toolInput(pack, dir, tx, map[string]any{"slide_id": "slide-01"})))
	gate := NewCompletionGate().Check(completionContext(pack, tx, ledger))
	if !result.OK || !gate.Accepted || len(gate.MaterializationProofs) != 0 {
		t.Fatalf("spec-only finish rejected: write=%+v gate=%+v", result, gate)
	}
}

func TestPresentationSpecHTMLAffectingChangesRequireHTMLSync(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*spec.SlideSpec)
	}{
		{name: "title", mutate: func(value *spec.SlideSpec) { value.Title = "Updated" }},
		{name: "content", mutate: func(value *spec.SlideSpec) { value.Content.Summary = "Updated" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
			tx, _ := NewRunSession(dir, "presentation-"+test.name)
			next := slideModel("slide-01", "Original")
			test.mutate(&next)
			write := (pptWriteTool{pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
				"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-01", Part: "spec"}),
				"content":  string(mustJSONValue(next)),
			}))
			ledger := NewEvidenceLedger()
			recordResultEvidence(ledger, write)
			gate := NewCompletionGate().Check(completionContext(pack, tx, ledger))
			if gate.Accepted || !hasCompletionCode(gate, "SLIDE_HTML_SYNC_REQUIRED") {
				t.Fatalf("HTML-affecting spec change accepted: write=%+v gate=%+v", write, gate)
			}
			if !hasRequiredAction(gate, "edit_ppt", "slide:slide-01:html") ||
				!hasRequiredAction(gate, "render_slide", "slide:slide-01:html") {
				t.Fatalf("gate did not direct HTML repair and render: %+v", gate)
			}
		})
	}
}

func TestPresentationSpecAndHTMLWithLatestRenderCanFinish(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	tx, _ := NewRunSession(dir, "presentation-complete")
	ledger := NewEvidenceLedger()
	next := slideModel("slide-01", "Updated")
	recordResultEvidence(ledger, (pptWriteTool{pack}).Execute(
		context.Background(), toolInput(pack, dir, tx, map[string]any{
			"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-01", Part: "spec"}),
			"content":  string(mustJSONValue(next)),
		})))
	recordResultEvidence(ledger, (pptWriteTool{pack}).Execute(
		context.Background(), toolInput(pack, dir, tx, map[string]any{
			"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-01", Part: "html"}),
			"content":  strings.Replace(validToolHTML, "Original", "Updated", 1),
		})))
	recordResultEvidence(ledger, (slideRenderTool{pack: pack, renderer: &recordingRenderer{}}).Execute(
		context.Background(), toolInput(pack, dir, tx, map[string]any{"slide_id": "slide-01"})))
	gate := NewCompletionGate().Check(completionContext(pack, tx, ledger))
	if !gate.Accepted || len(gate.MaterializationProofs) != 1 {
		t.Fatalf("latest spec+HTML render rejected: %+v", gate)
	}
}

func TestPresentationSpeakerNotesOnlyNeedsLatestMaterializationProof(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	tx, _ := NewRunSession(dir, "speaker-notes")
	ledger := NewEvidenceLedger()
	next := slideModel("slide-01", "Original")
	next.SpeakerNotes = "Updated private notes"
	recordResultEvidence(ledger, (pptWriteTool{pack}).Execute(
		context.Background(), toolInput(pack, dir, tx, map[string]any{
			"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-01", Part: "spec"}),
			"content":  string(mustJSONValue(next)),
		})))
	recordResultEvidence(ledger, (slideRenderTool{pack: pack, renderer: &recordingRenderer{}}).Execute(
		context.Background(), toolInput(pack, dir, tx, map[string]any{"slide_id": "slide-01"})))
	gate := NewCompletionGate().Check(completionContext(pack, tx, ledger))
	if !gate.Accepted || hasArtifactChange(tx.ChangeSet(), ArtifactSlideHTML, "slide-01") ||
		len(gate.MaterializationProofs) != 1 {
		t.Fatalf("speaker-notes materialization rejected: %+v changes=%+v", gate, tx.ChangeSet())
	}
}

func TestPresentationDesignOnlyNeedsLatestRenderAndKeepsHTMLUnchanged(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetDeck)
	tx, _ := NewRunSession(dir, "design-only-render")
	ledger := NewEvidenceLedger()
	next := designModel()
	next.Signature = "updated signature"
	recordResultEvidence(ledger, (pptWriteTool{pack}).Execute(
		context.Background(), toolInput(pack, dir, tx, map[string]any{
			"resource": resourceArgs(Resource{Type: "deck", Part: "design"}),
			"content":  string(mustJSONValue(next)),
		})))
	recordResultEvidence(ledger, (slideRenderTool{pack: pack, renderer: &recordingRenderer{}}).Execute(
		context.Background(), toolInput(pack, dir, tx, map[string]any{"slide_id": "slide-01"})))
	gate := NewCompletionGate().Check(completionContext(pack, tx, ledger))
	if !gate.Accepted || hasArtifactChange(tx.ChangeSet(), ArtifactSlideHTML, "slide-01") ||
		len(gate.MaterializationProofs) != 1 ||
		gate.MaterializationProofs[0].HTMLRevision != pack.Revisions.SlideHTML["slide-01"] {
		t.Fatalf("design-only render rejected or changed HTML revision: gate=%+v changes=%+v", gate, tx.ChangeSet())
	}
}

func TestRenderEvidenceBeforeSpecChangeIsStale(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	tx, _ := NewRunSession(dir, "stale-render")
	ledger := NewEvidenceLedger()
	recordResultEvidence(ledger, (slideRenderTool{pack: pack, renderer: &recordingRenderer{}}).Execute(
		context.Background(), toolInput(pack, dir, tx, map[string]any{"slide_id": "slide-01"})))
	next := slideModel("slide-01", "Original")
	next.SpeakerNotes = "changed after render"
	write := (pptWriteTool{pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-01", Part: "spec"}),
		"content":  string(mustJSONValue(next)),
	}))
	for _, target := range write.InvalidatedTargets {
		ledger.Invalidate(target)
	}
	recordResultEvidence(ledger, write)
	gate := NewCompletionGate().Check(completionContext(pack, tx, ledger))
	if gate.Accepted || !hasCompletionCode(gate, "VISUAL_EVIDENCE_REQUIRED") {
		t.Fatalf("pre-change render evidence accepted: %+v", gate)
	}
}

func TestRenderProofWithOldSourceRevisionIsRejected(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	tx, _ := NewRunSession(dir, "old-proof-revision")
	ledger := NewEvidenceLedger()
	next := slideModel("slide-01", "Original")
	next.SpeakerNotes = "updated notes"
	recordResultEvidence(ledger, (pptWriteTool{pack}).Execute(
		context.Background(), toolInput(pack, dir, tx, map[string]any{
			"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-01", Part: "spec"}),
			"content":  string(mustJSONValue(next)),
		})))
	rendered := (slideRenderTool{pack: pack, renderer: &recordingRenderer{}}).Execute(
		context.Background(), toolInput(pack, dir, tx, map[string]any{"slide_id": "slide-01"}))
	if len(rendered.Evidence) != 1 || rendered.Evidence[0].Materialization == nil {
		t.Fatalf("render proof missing: %+v", rendered)
	}
	rendered.Evidence[0].Materialization.SourceSpecRevision--
	recordResultEvidence(ledger, rendered)
	gate := NewCompletionGate().Check(completionContext(pack, tx, ledger))
	if gate.Accepted || !hasCompletionCode(gate, "VISUAL_EVIDENCE_REQUIRED") {
		t.Fatalf("old source revision passed gate: %+v", gate)
	}
}

func TestMaterializationSourceHashBindsDesignSpecAndHTML(t *testing.T) {
	base := MaterializationSourceHash("slide-01", []byte("design"), []byte("spec"), []byte("html"))
	for name, hash := range map[string]string{
		"design": MaterializationSourceHash("slide-01", []byte("design-2"), []byte("spec"), []byte("html")),
		"spec":   MaterializationSourceHash("slide-01", []byte("design"), []byte("spec-2"), []byte("html")),
		"html":   MaterializationSourceHash("slide-01", []byte("design"), []byte("spec"), []byte("html-2")),
	} {
		if hash == base {
			t.Errorf("%s change did not invalidate source hash", name)
		}
	}
}

func TestRegistryDisclosesOnlyFixedBusinessSurface(t *testing.T) {
	_, pack := toolProject(t, model.ArtifactPresentation, model.TargetDeck)
	registry := NewToolRegistry()
	if err := (DefaultDomainToolProvider{Pack: pack, Renderer: &recordingRenderer{}}).RegisterDomainTools(registry); err != nil {
		t.Fatal(err)
	}
	all := schemasByName(registry.Disclose(StrategyExecute, PhaseExecuting, model.IntentExecute))
	for _, name := range []string{"read_ppt", "write_ppt", "edit_ppt", "search_refs", "render_slide"} {
		if !all[name] {
			t.Fatalf("%s not disclosed: %v", name, all)
		}
	}
	chat := schemasByName(registry.Disclose(StrategyTalk, PhaseChat, model.IntentTalk))
	if chat["write_ppt"] || chat["edit_ppt"] {
		t.Fatalf("talk disclosed writes: %v", chat)
	}
}

func TestToolResultEnvelopeHidesInternalEvidence(t *testing.T) {
	result := SuccessfulToolResult("ok")
	result.Evidence = []Evidence{newEvidence("schema", Resource{Type: "deck", Part: "outline"}, "hash")}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"evidence"`) {
		t.Fatalf("internal evidence leaked: %s", raw)
	}
}

func hasCompletionCode(result CompletionResult, code string) bool {
	for _, issue := range result.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func hasRequiredAction(result CompletionResult, tool, target string) bool {
	for _, issue := range result.Issues {
		for _, action := range issue.RequiredActions {
			if action.Tool == tool && action.Target.Key() == target {
				return true
			}
		}
	}
	return false
}

func recordResultEvidence(ledger *EvidenceLedger, result ToolResult) {
	for _, evidence := range result.Evidence {
		ledger.Record(evidence)
	}
}

func completionContext(pack contextengine.ContextPack, tx *RunSession, ledger *EvidenceLedger) CompletionContext {
	return CompletionContext{
		Strategy: StrategyExecute, FinishPhase: PhaseExecuting, WorkScope: ScopeFromSpec(pack.WorkSpec),
		Session: tx, Changes: tx.ChangeSet(), Evidence: ledger, Context: pack,
	}
}

func resourceArgs(resource Resource) map[string]any {
	out := map[string]any{"type": resource.Type, "part": resource.Part}
	if resource.SlideID != "" {
		out["slide_id"] = resource.SlideID
	}
	return out
}

func toolInput(pack contextengine.ContextPack, dir string, tx *RunSession, args map[string]any) DomainToolInput {
	return DomainToolInput{
		Args: args, Context: pack, ProjectDir: dir, RunID: "run-1", Session: tx,
		Scope: ScopeFromSpec(pack.WorkSpec), Strategy: StrategyExecute, Phase: PhaseExecuting,
		Interaction: pack.WorkSpec.Interaction.Intent, Risk: RiskLow,
	}
}

func toolProject(t *testing.T, artifact model.Artifact, level model.TargetLevel) (string, contextengine.ContextPack) {
	t.Helper()
	dir := t.TempDir()
	for _, rel := range []string{"slides/slide-01", "common"} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	outline := deckModel("p1", []string{"slide-01"})
	slide := slideModel("slide-01", "Original")
	design := designModel()
	writeJSON := func(path string, value any) {
		raw, _ := json.MarshalIndent(value, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(path)), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeJSON("outline.json", outline)
	writeJSON(model.SlideSpecPath("slide-01"), slide)
	writeJSON("design.json", design)
	if err := os.WriteFile(filepath.Join(dir, model.SlideHTMLPath("slide-01")), []byte(validToolHTML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "common", "tokens.css"), []byte(":root{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "common", "base.css"), []byte(".slide-stage{width:1600px;height:900px}"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := model.RunTarget{Artifact: artifact, Level: level}
	if level == model.TargetSlide {
		target.SlideID = "slide-01"
	}
	pack := contextengine.ContextPack{
		SchemaVersion: contextengine.SchemaVersion,
		WorkSpec: model.WorkSpec{
			Target: target, Interaction: model.RunInteraction{Intent: model.IntentExecute}, Instruction: "test",
		},
		Project: contextengine.ProjectContext{ID: "p1", Title: "Test"},
		Outline: contextengine.OutlineContext{
			Outline: outline, Summaries: []contextengine.SlideSummary{{ID: "slide-01", Title: "Original"}},
		},
		Target: contextengine.TargetContext{
			Artifact: artifact, Level: level, SlideSpec: &slide,
			Materialization: &spec.Materialization{State: string(model.MaterializationFresh)},
		},
		Design:    contextengine.DesignContext{Design: &design},
		SlideHTML: contextengine.SlideHTMLContext{Summaries: map[string]contextengine.HTMLSummary{}},
		Revisions: contextengine.RevisionRefs{
			Outline: 1, Design: 1, SlideSpecs: map[string]int{"slide-01": 1},
			SlideHTML: map[string]int{"slide-01": 1},
		},
		Manifest: contextengine.ContextManifest{
			ContextID: "ctx", RunID: "run-1", ThreadID: "thread-1", ProjectID: "p1",
			BudgetTokens: 20000, EstimatedTokens: 1000, Refs: []contextengine.ContextRef{},
			Segments: []contextengine.ContextSegment{},
		},
	}
	return dir, pack
}

func deckModel(projectID string, order []string) spec.Outline {
	return spec.Outline{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID,
		Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN",
		CoreThesis: "Thesis", NarrativeArc: "Arc",
		Sections:   []spec.Section{{ID: "section-1", Number: "1", Title: "Section", Subsections: []spec.Subsection{}}},
		SlideOrder: append([]string{}, order...), CreatedAt: 1, UpdatedAt: 1,
	}
}

func slideModel(id, title string) spec.SlideSpec {
	return spec.SlideSpec{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "p1", SlideID: id,
		SourceOutlineRevision: 1, SectionID: "section-1",
		Role: "cover", Title: title, KeyMessage: title,
		Content:      spec.Content{Summary: title, Points: []string{}},
		VisualIntent: spec.VisualIntent{Archetype: "cover", Description: "Cover", AssetQueries: []string{}},
		SpeakerNotes: "", CreatedAt: 1, UpdatedAt: 1,
	}
}

func designModel() spec.Design {
	return spec.Design{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "p1",
		Canvas:  spec.CanvasSpec{Width: 1600, Height: 900, Ratio: "16:9"},
		Palette: []string{"#111111", "#FFFFFF", "#3366FF"},
		Typography: spec.TypographySpec{
			Display: spec.FontSpec{Family: "Arial", Weight: 700},
			Body:    spec.FontSpec{Family: "Arial", Weight: 400},
			Utility: spec.FontSpec{Family: "Arial", Weight: 500},
		},
		Spacing: spec.SpacingSpec{Unit: 8}, Radius: spec.RadiusSpec{Card: 8},
		Shadows:      spec.ShadowSpec{Card: "none"},
		LayoutSystem: spec.LayoutSystem{Grid: "12-col", Rhythm: "regular", Density: "medium"},
		Signature:    "test", Motion: spec.MotionSpec{Policy: "none"}, CreatedAt: 1, UpdatedAt: 1,
	}
}

func mustJSONValue(value any) []byte {
	raw, _ := json.MarshalIndent(value, "", "  ")
	return raw
}

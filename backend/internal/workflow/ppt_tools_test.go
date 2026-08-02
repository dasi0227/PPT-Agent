package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
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

func TestReadPPTGlobalAndSlideModelHTML(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetDeck)
	global := (pptReadTool{pack: pack}).Execute(context.Background(), toolInput(pack, dir, nil, map[string]any{
		"target":  map[string]any{"type": "global"},
		"include": []any{"model"},
	}))
	if !global.OK || global.Data["source"] != "committed" {
		t.Fatalf("global=%+v", global)
	}
	globalModel := global.Data["content"].(map[string]any)["model"].(map[string]any)
	if globalModel["deck"] == nil || globalModel["design"] == nil {
		t.Fatalf("presentation global mapping=%+v", globalModel)
	}
	slide := (pptReadTool{pack: pack}).Execute(context.Background(), toolInput(pack, dir, nil, map[string]any{
		"target":  map[string]any{"type": "slide", "slide_id": "slide-01"},
		"include": []any{"model", "html"},
	}))
	content := slide.Data["content"].(map[string]any)
	if !slide.OK || content["html"] != validToolHTML || content["model"] == nil {
		t.Fatalf("slide=%+v", slide)
	}
}

func TestPresentationGlobalWriteAndEditStageDeckAndDesignAtomically(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetDeck)
	tx, _ := NewTransaction(dir, "presentation-global")
	defer tx.Cleanup()
	deck := deckModel("p1", []string{"slide-01"})
	design := designModel()
	write := (pptWriteTool{pack: pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"target":  map[string]any{"type": "global"},
		"content": map[string]any{"model": map[string]any{"deck": deck, "design": design}},
	}))
	if !write.OK || !tx.IsStaged(deckRef(pack)) || !tx.IsStaged(designRef(pack)) {
		t.Fatalf("write=%+v changes=%+v", write, tx.ChangeSet())
	}
	edit := (pptEditTool{pack: pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"target": map[string]any{"type": "global"},
		"edits": []any{
			map[string]any{"type": "json_edit", "op": "replace", "path": "/deck/title", "value": "Edited deck"},
			map[string]any{"type": "json_edit", "op": "replace", "path": "/design/signature", "value": "Edited signature"},
		},
	}))
	if !edit.OK {
		t.Fatalf("edit=%+v", edit)
	}
	deckRaw, _, _ := readArtifact(dir, tx, deckRef(pack))
	designRaw, _, _ := readArtifact(dir, tx, designRef(pack))
	if !strings.Contains(string(deckRaw), "Edited deck") || !strings.Contains(string(designRaw), "Edited signature") {
		t.Fatalf("deck=%s design=%s", deckRaw, designRaw)
	}
}

func TestReadPPTPrefersStagedAndRejectsGlobalHTML(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetDeck)
	tx, _ := NewTransaction(dir, "read-staged")
	defer tx.Cleanup()
	staged := strings.Replace(validToolHTML, "Original", "Staged", 1)
	if _, err := tx.Stage(presentationSlideRef("slide-01"), "test", []byte(staged)); err != nil {
		t.Fatal(err)
	}
	result := (pptReadTool{pack: pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"target":  map[string]any{"type": "slide", "slide_id": "slide-01"},
		"include": []any{"html"},
	}))
	if !result.OK || result.Data["source"] != "staged" ||
		result.Data["content"].(map[string]any)["html"] != staged {
		t.Fatalf("result=%+v", result)
	}
	rejected := (pptReadTool{pack: pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"target": map[string]any{"type": "global"}, "include": []any{"html"},
	}))
	if rejected.OK || rejected.Code != CodeModelInvalid {
		t.Fatalf("rejected=%+v", rejected)
	}
}

func TestWritePPTGlobalCreateAndSlideCreateReplace(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactBlueprint, model.TargetDeck)
	if err := os.Remove(filepath.Join(dir, "deck.json")); err != nil {
		t.Fatal(err)
	}
	tx, _ := NewTransaction(dir, "create-global")
	defer tx.Cleanup()
	global := (pptWriteTool{pack: pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"target":  map[string]any{"type": "global"},
		"content": map[string]any{"model": deckModel("p1", []string{"slide-01"})},
	}))
	if !global.OK || global.Data["operation"] != "create" {
		t.Fatalf("global=%+v", global)
	}

	// Use a fresh transaction with a committed global model so slide reference
	// integrity is enforceable before staging the page.
	if _, err := tx.Stage(deckRef(pack), "test", mustJSONValue(deckModel("p1", []string{"slide-01"}))); err != nil {
		t.Fatal(err)
	}
	slide := (pptWriteTool{pack: pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"target":  map[string]any{"type": "slide", "slide_id": "slide-01"},
		"content": map[string]any{"model": slideModel("slide-01", "New title")},
	}))
	if !slide.OK || slide.ChangedTargets[0].Revision != 2 {
		t.Fatalf("slide=%+v", slide)
	}
	replaced := (pptWriteTool{pack: pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"target":  map[string]any{"type": "slide", "slide_id": "slide-01"},
		"content": map[string]any{"model": slideModel("slide-01", "Replacement")},
	}))
	if !replaced.OK || replaced.ChangedTargets[0].Revision != 3 {
		t.Fatalf("replaced=%+v", replaced)
	}
}

func TestWritePPTPresentationStagesModelAndHTMLAndBlueprintRejectsHTML(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	tx, _ := NewTransaction(dir, "presentation-write")
	defer tx.Cleanup()
	nextHTML := strings.Replace(validToolHTML, "Original", "Next", 1)
	result := (pptWriteTool{pack: pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"target":  map[string]any{"type": "slide", "slide_id": "slide-01"},
		"content": map[string]any{"model": slideModel("slide-01", "Next"), "html": nextHTML},
	}))
	if !result.OK || !tx.IsStaged(blueprintSlideRef("slide-01")) || !tx.IsStaged(presentationSlideRef("slide-01")) {
		t.Fatalf("result=%+v changes=%+v", result, tx.ChangeSet())
	}

	blueprintPack := pack
	blueprintPack.WorkSpec.Target.Artifact = model.ArtifactBlueprint
	rejected := (pptWriteTool{pack: blueprintPack}).Execute(context.Background(), toolInput(blueprintPack, dir, tx, map[string]any{
		"target":  map[string]any{"type": "slide", "slide_id": "slide-01"},
		"content": map[string]any{"model": slideModel("slide-01", "Bad"), "html": nextHTML},
	}))
	if rejected.OK || rejected.Code != ErrCapabilityDenied.Error() {
		t.Fatalf("rejected=%+v", rejected)
	}
}

func TestEditPPTJSONIsAtomicAndTextAnchorsAreUnique(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	tx, _ := NewTransaction(dir, "edit")
	defer tx.Cleanup()
	tool := pptEditTool{pack: pack}
	ok := tool.Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"target": map[string]any{"type": "slide", "slide_id": "slide-01"},
		"edits": []any{
			map[string]any{"type": "json_edit", "op": "replace", "path": "/title", "value": "Edited"},
			map[string]any{"type": "json_edit", "op": "replace", "path": "/content/summary", "value": "Edited summary"},
			map[string]any{"type": "text_edit", "field": "html", "old_text": "Original", "new_text": "Edited"},
		},
	}))
	if !ok.OK || len(tx.ChangeSet().Updated) != 2 {
		t.Fatalf("ok=%+v changes=%+v", ok, tx.ChangeSet())
	}
	before := tx.ChangeSet()
	failed := tool.Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"target": map[string]any{"type": "slide", "slide_id": "slide-01"},
		"edits": []any{
			map[string]any{"type": "json_edit", "op": "replace", "path": "/title", "value": "Must rollback"},
			map[string]any{"type": "json_edit", "op": "replace", "path": "/missing", "value": "x"},
		},
	}))
	after := tx.ChangeSet()
	if failed.OK || before.Updated[0].AfterHash != after.Updated[0].AfterHash {
		t.Fatalf("failed=%+v before=%+v after=%+v", failed, before, after)
	}
	notFound := tool.Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"target": map[string]any{"type": "slide", "slide_id": "slide-01"},
		"edits":  []any{map[string]any{"type": "text_edit", "field": "html", "old_text": "absent", "new_text": "x"}},
	}))
	if notFound.Code != CodeEditAnchorNotFound {
		t.Fatalf("notFound=%+v", notFound)
	}
	if _, err := tx.Stage(presentationSlideRef("slide-01"), "test", []byte(strings.Replace(validToolHTML, "Original", "repeat repeat", 1))); err != nil {
		t.Fatal(err)
	}
	ambiguous := tool.Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"target": map[string]any{"type": "slide", "slide_id": "slide-01"},
		"edits":  []any{map[string]any{"type": "text_edit", "field": "html", "old_text": "repeat", "new_text": "x"}},
	}))
	if ambiguous.Code != CodeEditAnchorAmbiguous {
		t.Fatalf("ambiguous=%+v", ambiguous)
	}
}

func TestPPTToolsRejectOutOfScopeTarget(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	tx, _ := NewTransaction(dir, "scope")
	defer tx.Cleanup()
	result := (pptWriteTool{pack: pack}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"target":  map[string]any{"type": "slide", "slide_id": "slide-02"},
		"content": map[string]any{"model": slideModel("slide-02", "No"), "html": validToolHTML},
	}))
	if result.OK || result.Code != ErrTargetOutOfScope.Error() {
		t.Fatalf("result=%+v", result)
	}
}

func TestSearchRefsHonorsManifestPermissionAndBudget(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	pack.Manifest.Refs = []contextengine.ContextRef{{
		ID: "opaque-ref", Kind: contextengine.RefHistoryEvidence, RunID: "run-1",
		ThreadID: "thread-1", ProjectID: "p1", Summary: "brand blue typography",
		ContentHash: strings.Repeat("a", 64), Revision: 2,
		AvailableLevels: []contextengine.DetailLevel{contextengine.DetailSummary, contextengine.DetailFull},
	}}
	result := (referenceSearchTool{pack: pack}).Execute(context.Background(), toolInput(pack, dir, nil, map[string]any{
		"query": "brand", "kinds": []any{"reference"}, "limit": 3,
	}))
	if !result.OK || len(result.Data["results"].([]map[string]any)) != 1 {
		t.Fatalf("result=%+v", result)
	}
	input := toolInput(pack, dir, nil, map[string]any{"query": "brand"})
	input.RunID = "another-run"
	forbidden := (referenceSearchTool{pack: pack}).Execute(context.Background(), input)
	if forbidden.OK || forbidden.Code != ErrCapabilityDenied.Error() {
		t.Fatalf("forbidden=%+v", forbidden)
	}
	pack.Manifest.BudgetTokens, pack.Manifest.EstimatedTokens = 1, 1
	large := strings.Repeat("brand ", 600)
	pack.Manifest.Refs[0].Summary = large
	exceeded := (referenceSearchTool{pack: pack}).Execute(context.Background(), toolInput(pack, dir, nil, map[string]any{
		"query": "brand", "limit": 20,
	}))
	if exceeded.OK || exceeded.Code != CodeContextBudget {
		t.Fatalf("exceeded=%+v", exceeded)
	}
}

func TestRenderSlideUsesStagedViewAndReturnsDiagnostics(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	tx, _ := NewTransaction(dir, "render")
	defer tx.Cleanup()
	staged := strings.Replace(validToolHTML, "Original", "Staged", 1)
	if _, err := tx.Stage(presentationSlideRef("slide-01"), "test", []byte(staged)); err != nil {
		t.Fatal(err)
	}
	renderer := &recordingRenderer{}
	result := (slideRenderTool{pack: pack, renderer: renderer}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"slide_id": "slide-01",
	}))
	if !result.OK || renderer.html != staged || result.Data["source"] != "staged" || len(result.Evidence) != 1 {
		t.Fatalf("result=%+v html=%q", result, renderer.html)
	}
	failing := &recordingRenderer{diagnostics: RenderDiagnostics{
		Overflow: map[string]bool{"horizontal": true}, ConsoleErrors: []string{"boom"},
		FailedResources: []string{"404: /x.png"}, FontStatus: "loading",
	}}
	rejected := (slideRenderTool{pack: pack, renderer: failing}).Execute(context.Background(), toolInput(pack, dir, tx, map[string]any{
		"slide_id": "slide-01",
	}))
	if rejected.OK || rejected.Code != CodeRenderFailed || len(rejected.Issues) < 3 {
		t.Fatalf("rejected=%+v", rejected)
	}
}

func TestRenderEvidenceBindsRevisionAndBecomesStaleAfterEdit(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	tx, _ := NewTransaction(dir, "evidence")
	defer tx.Cleanup()
	ledger := NewEvidenceLedger()
	hash, _ := renderSourceHash(pack, tx, "slide-01")
	ledger.Record(newEvidence("render", TargetRef{Type: "slide", SlideID: "slide-01"}, hash))
	if !ledger.HasFresh(TargetRef{Type: "slide", SlideID: "slide-01"}, hash, "render") {
		t.Fatal("fresh render evidence missing")
	}
	ledger.Invalidate(TargetRef{Type: "slide", SlideID: "slide-01"})
	if ledger.HasFresh(TargetRef{Type: "slide", SlideID: "slide-01"}, hash, "render") {
		t.Fatal("old render evidence remained fresh")
	}
}

func TestCompletionGateRequiresThenAcceptsLatestRender(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	tx, _ := NewTransaction(dir, "gate-render")
	defer tx.Cleanup()
	nextHTML := []byte(strings.Replace(validToolHTML, "Original", "Changed", 1))
	change, err := tx.Stage(presentationSlideRef("slide-01"), "edit_ppt", nextHTML)
	if err != nil {
		t.Fatal(err)
	}
	changes := tx.ChangeSet()
	hash, _ := renderSourceHash(pack, tx, "slide-01")
	ledger := NewEvidenceLedger()
	ledger.Record(staticEvidence(TargetRef{Type: "slide", SlideID: "slide-01"}, hash))
	base := CompletionContext{
		Strategy: StrategySimple, FinishPhase: PhaseExecuting, WorkScope: ScopeFromSpec(pack.WorkSpec),
		Transaction: tx, Changes: changes, Evidence: ledger, Context: pack,
	}
	rejected := NewCompletionGate().Check(base)
	if rejected.Accepted || !hasCompletionCode(rejected, "VISUAL_EVIDENCE_REQUIRED") {
		t.Fatalf("rejected=%+v change=%+v", rejected, change)
	}
	ledger.Record(newEvidence("render", TargetRef{Type: "slide", SlideID: "slide-01"}, hash))
	accepted := NewCompletionGate().Check(base)
	if !accepted.Accepted {
		t.Fatalf("accepted=%+v", accepted)
	}
}

func TestRegistryDisclosesOnlyNewSurfaceByStrategyAndPhase(t *testing.T) {
	_, pack := toolProject(t, model.ArtifactPresentation, model.TargetDeck)
	registry := NewToolRegistry()
	if err := (DefaultDomainToolProvider{Pack: pack, Renderer: &recordingRenderer{}}).RegisterDomainTools(registry); err != nil {
		t.Fatal(err)
	}
	all := map[string]bool{}
	for _, schema := range registry.Disclose(StrategySimple, PhaseExecuting, model.IntentExecute) {
		all[schema.Name] = true
	}
	for _, name := range []string{"read_ppt", "write_ppt", "edit_ppt", "search_refs", "render_slide"} {
		if !all[name] {
			t.Fatalf("%s not disclosed: %v", name, all)
		}
	}
	for _, old := range []string{
		"read_context_ref", "read_staged_artifact", "write_staged_deck_blueprint",
		"write_staged_slide_blueprint", "patch_staged_slide_blueprint", "write_staged_design_spec",
		"write_staged_presentation", "patch_staged_presentation", "search_assets", "finish_step",
	} {
		if all[old] {
			t.Fatalf("old tool %s was disclosed", old)
		}
	}
	chat := schemasByName(registry.Disclose(StrategyChat, PhaseChat, model.IntentTalk))
	if chat["write_ppt"] || chat["edit_ppt"] {
		t.Fatalf("chat disclosed write tools: %v", chat)
	}
	planning := schemasByName(registry.Disclose(StrategyComplex, PhasePlanning, model.IntentExecute))
	if planning["write_ppt"] || planning["edit_ppt"] {
		t.Fatalf("planning disclosed write tools: %v", planning)
	}
}

func TestToolResultEnvelopeHidesInternalEvidence(t *testing.T) {
	result := SuccessfulToolResult("ok")
	result.Evidence = []Evidence{newEvidence("schema", TargetRef{Type: "global"}, "hash")}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{`"ok":true`, `"summary":"ok"`, `"changed_targets":[]`, `"issues":[]`, `"retryable":false`} {
		if !strings.Contains(text, required) {
			t.Fatalf("missing %s in %s", required, text)
		}
	}
	if strings.Contains(text, `"evidence"`) {
		t.Fatalf("internal evidence leaked into ToolResult: %s", text)
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

func toolInput(pack contextengine.ContextPack, dir string, tx *Transaction, args map[string]any) DomainToolInput {
	return DomainToolInput{
		Args: args, Context: pack, ProjectDir: dir, RunID: "run-1", Transaction: tx,
		Scope: ScopeFromSpec(pack.WorkSpec), Strategy: StrategySimple, Phase: PhaseExecuting,
		Interaction: pack.WorkSpec.Interaction.Intent, Risk: RiskLow,
	}
}

func toolProject(t *testing.T, artifact model.Artifact, level model.TargetLevel) (string, contextengine.ContextPack) {
	t.Helper()
	dir := t.TempDir()
	for _, rel := range []string{"design", "slides/slide-01", "common"} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	deck := deckModel("p1", []string{"slide-01"})
	slide := slideModel("slide-01", "Original")
	design := designModel()
	writeJSON := func(path string, value any) {
		raw, _ := json.MarshalIndent(value, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(path)), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeJSON("deck.json", deck)
	writeJSON(model.SlideJSONPath("slide-01"), slide)
	writeJSON("design/design-spec.json", design)
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(model.SlideHTMLPath("slide-01"))), []byte(validToolHTML), 0o644); err != nil {
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
		Deck:    contextengine.DeckContext{Deck: deck, Summaries: []contextengine.SlideSummary{{ID: "slide-01", Title: "Original"}}},
		Target: contextengine.TargetContext{
			Artifact: artifact, Level: level, Slide: &slide,
			Materialization: &blueprint.Materialization{State: string(model.MaterializationFresh)},
		},
		Design:       contextengine.DesignContext{Spec: &design},
		Presentation: contextengine.PresentationContext{Summaries: map[string]contextengine.HTMLSummary{}},
		Revisions: contextengine.RevisionRefs{
			Deck: 1, Design: 1, Slides: map[string]int{"slide-01": 1},
			Presentations: map[string]int{"slide-01": 1},
		},
		Manifest: contextengine.ContextManifest{
			ContextID: "ctx", RunID: "run-1", ThreadID: "thread-1", ProjectID: "p1",
			BudgetTokens: 20000, EstimatedTokens: 1000, Refs: []contextengine.ContextRef{},
			Segments: []contextengine.ContextSegment{},
		},
	}
	return dir, pack
}

func deckModel(projectID string, order []string) blueprint.Deck {
	return blueprint.Deck{
		SchemaVersion: blueprint.SchemaVersion, Revision: 1, ProjectID: projectID,
		Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN",
		CoreThesis: "Thesis", NarrativeArc: "Arc",
		Sections:   []blueprint.Section{{ID: "section-1", Number: "1", Title: "Section", Subsections: []blueprint.Subsection{}}},
		SlideOrder: append([]string{}, order...), CreatedAt: 1, UpdatedAt: 1,
	}
}

func slideModel(id, title string) blueprint.Slide {
	return blueprint.Slide{
		SchemaVersion: blueprint.SchemaVersion, Revision: 1, SlideID: id, SectionID: "section-1",
		Role: "cover", Title: title, KeyMessage: title,
		Content:      blueprint.Content{Summary: title, Points: []string{}},
		VisualIntent: blueprint.VisualIntent{Archetype: "cover", Description: "Cover", AssetQueries: []string{}},
		SpeakerNotes: "", CreatedAt: 1, UpdatedAt: 1,
	}
}

func designModel() blueprint.DesignSpec {
	return blueprint.DesignSpec{
		SchemaVersion: blueprint.SchemaVersion, Revision: 1,
		Canvas:  blueprint.CanvasSpec{Width: 1600, Height: 900, Ratio: "16:9"},
		Palette: []string{"black", "white"},
		Typography: blueprint.TypographySpec{
			Display: blueprint.FontSpec{Family: "Arial"}, Body: blueprint.FontSpec{Family: "Arial"},
			Utility: blueprint.FontSpec{Family: "Arial"},
		},
		Spacing: blueprint.SpacingSpec{Unit: 8}, Radius: blueprint.RadiusSpec{Card: 8},
		Shadows:      blueprint.ShadowSpec{Card: "none"},
		LayoutSystem: blueprint.LayoutSystem{Grid: "12-col", Rhythm: "regular", Density: "medium"},
		Signature:    "test", Motion: blueprint.MotionSpec{Policy: "none"},
	}
}

func mustJSONValue(value any) []byte {
	raw, _ := json.MarshalIndent(value, "", "  ")
	return raw
}

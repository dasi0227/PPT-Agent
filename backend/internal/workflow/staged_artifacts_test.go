package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

const generationSlide = "sli_aaaaaa"
const generationHTML = `<!doctype html><html><body><section class="slide-stage"><h1>Original</h1></section></body></html>`

func generationPackFixture(t *testing.T) (string, string, contextengine.ContextPack) {
	t.Helper()
	dir, css := renderThemeFixture(t)
	manifest := spec.Manifest{Title: "Deck", Goal: "Explain", Audience: "Builders", Language: "zh-CN", SlideCount: "待明确", Requirements: []string{}, Prohibitions: []string{}}
	design := spec.Design{Requirements: []string{"A"}, Prohibitions: []string{}}
	slide := spec.SlideSpec{Core: "Message", Elements: []spec.Element{}}
	pack := testPack(model.ModeExecute, model.ScopeCurrentPage, false, "Update requirements")
	pack.Project.ThemeID = "clean"
	pack.Command.Scope = model.NewRunScope(model.ScopeCurrentPage, generationSlide)
	pack.PresentationManifest.Manifest = manifest
	pack.Design.Design = &design
	pack.Outline.Outline = spec.Outline{Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Section", Slides: []spec.SlideNode{{ID: generationSlide, Title: "Page"}}, Subsections: []spec.Subsection{}}}}
	pack.Outline.Summaries = []contextengine.SlideSummary{{ID: generationSlide, State: string(model.HTMLAvailable)}}
	for path, content := range map[string]any{".manifest.json": manifest, ".outline.json": pack.Outline.Outline, ".design.json": design, model.SlideSpecPath(generationSlide): slide} {
		raw, _ := json.Marshal(content)
		writeGenerationFile(t, dir, path, raw)
	}
	writeGenerationFile(t, dir, model.SlideHTMLPath(generationSlide), []byte(generationHTML))
	return dir, css, pack
}

func writeGenerationFile(t *testing.T, dir, path string, raw []byte) {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, raw, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestStagedHTMLCommitRollsBackAndKeepsFrozenView(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	session, err := NewRunSession(dir, "generation")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	ref := slideHTMLRef(generationSlide)
	changed := []byte(strings.Replace(generationHTML, "Original", "Changed", 1))
	// A disk edit after model dispatch must not enter this response's view.
	changedDesign := *pack.Design.Design
	changedDesign.Requirements = []string{"Unseen"}
	raw, _ := json.Marshal(changedDesign)
	writeGenerationFile(t, dir, ".design.json", raw)
	if _, err = session.Write(projectFileRef(ref.Path), "run_command", changed); err != nil {
		t.Fatal(err)
	}
	frozen := session.authoringContext(pack)
	if err := session.ValidateStagedArtifacts(frozen); err != nil {
		t.Fatal(err)
	}
	if frozen.Design.Design.Requirements[0] != "A" {
		t.Fatal("authoring view reread unobserved disk changes")
	}
	failure := errors.New("database unavailable")
	if _, err = session.CommitOperation(context.Background(), "failed", "", func(_ context.Context, c CommitContext) error {
		return failure
	}); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(filepath.Join(dir, ref.Path)); string(raw) != generationHTML {
		t.Fatal("failed write survived")
	}
	if _, err = session.Write(projectFileRef(ref.Path), "run_command", changed); err != nil {
		t.Fatal(err)
	}
	if err := session.ValidateStagedArtifacts(pack); err != nil {
		t.Fatal(err)
	}
	changes, err := session.CommitOperation(context.Background(), "success", "", func(_ context.Context, c CommitContext) error {
		if c.Changes.Updated[0].Artifact.Kind != ArtifactSlideHTML {
			t.Fatalf("command bypassed domain commit: %+v", c)
		}
		return nil
	})
	if err != nil || changes.Count() != 1 {
		t.Fatalf("commit: %+v %v", changes, err)
	}
	if _, err = session.Write(ref, "edit_html", changed); err != nil {
		t.Fatal(err)
	}
	if err := session.ValidateStagedArtifacts(pack); err != nil || session.ChangeSet().Count() != 0 {
		t.Fatalf("same bytes created an operation: %v", err)
	}
}

func TestConcurrentAuthoringViewIncludesEarlierRequirementsAndExcludesLaterCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	allowHTML, laterCommitted := make(chan struct{}), make(chan struct{})
	htmlDesign := ""
	htmlGoal, htmlTitle := "", ""
	input, state, registry := authoringBatchFixture(t, func(ctx context.Context, input DomainToolInput) error {
		if input.CallID != "html_b" {
			return nil
		}
		select {
		case <-allowHTML:
			htmlDesign = input.Context.Design.Design.Requirements[0]
			htmlGoal = input.Context.PresentationManifest.Manifest.Goal
			htmlTitle = input.Context.Outline.Outline.Sections[0].Slides[0].Title
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	input.CommitMetadata = func(_ context.Context, c CommitContext) error {
		if c.OperationID == "design_c" {
			close(laterCommitted)
		}
		return nil
	}
	calls := []llm.ToolCall{
		{ID: "manifest_b", Name: "edit_manifest", Args: map[string]any{"goal": "Earlier goal"}},
		{ID: "outline_b", Name: "edit_outline", Args: map[string]any{"edits": []any{map[string]any{"old_text": `"title":"Page"`, "new_text": `"title":"Earlier page"`}}}},
		{ID: "design_b", Name: "edit_design", Args: map[string]any{"requirements": []any{"B"}}},
		{ID: "html_b", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "content": strings.Replace(generationHTML, "Original", "B", 1)}},
		{ID: "design_c", Name: "edit_design", Args: map[string]any{"requirements": []any{"C"}}},
	}
	done := make(chan []ToolResult, 1)
	go func() {
		done <- NewRuntime(nil).executeToolBatch(ctx, input, state, registry, schemasByName(registry.Disclose(PhaseExecuting, model.ModeExecute, state.scope)), calls)
	}()
	select {
	case <-laterCommitted:
	case <-time.After(3 * time.Second):
		t.Fatal("later independent requirement could not commit")
	}
	close(allowHTML)
	for _, result := range <-done {
		if !result.OK {
			t.Fatalf("result=%+v", result)
		}
	}
	if htmlDesign != "B" || htmlGoal != "Earlier goal" || htmlTitle != "Earlier page" || state.pack.Design.Design.Requirements[0] != "C" {
		t.Fatalf("authoring view lost earlier resources or included a later commit: design=%s goal=%s title=%s", htmlDesign, htmlGoal, htmlTitle)
	}
}

func TestReplayedRequirementsAdvanceAuthoringViewWithoutRewritingNewerFiles(t *testing.T) {
	htmlDesign := ""
	input, state, registry := authoringBatchFixture(t, func(_ context.Context, input DomainToolInput) error {
		if input.CallID == "recovered_html" {
			htmlDesign = input.Context.Design.Design.Requirements[0]
		}
		return nil
	})
	frozen := cloneAuthoringPack(state.pack)
	versions := maps.Clone(state.seenVersions)
	receipts := newMemoryIdempotencyStore()
	input.Idempotency = receipts
	commits := 0
	input.CommitMetadata = func(ctx context.Context, c CommitContext) error {
		commits++
		return receipts.CompleteIdempotency(ctx, "tool_call", state.runID, c.OperationID, "completed", c.ToolResultJSON)
	}
	calls := []llm.ToolCall{
		{ID: "saved_design", Name: "edit_design", Args: map[string]any{"requirements": []any{"B"}}},
		{ID: "saved_spec", Name: "edit_spec", Args: map[string]any{"slide_id": generationSlide, "core": "B"}},
	}
	runtime := NewRuntime(nil)
	names := schemasByName(registry.Disclose(PhaseExecuting, model.ModeExecute, state.scope))
	savedContents := make([][]byte, len(calls))
	savedResults := runtime.executeToolBatch(context.Background(), input, state, registry, names, calls)
	for i, result := range savedResults {
		if !result.OK {
			t.Fatalf("initial write: %+v", result)
		}
		resource := resourceForTool(calls[i].Name, stringValue(calls[i].Args["slide_id"]))
		ref, _ := refForResource(state.pack, resource)
		var err error
		savedContents[i], err = os.ReadFile(filepath.Join(input.ProjectDir, ref.Path))
		if err != nil {
			t.Fatal(err)
		}
		assertSavedResourceObservation(t, calls[i].Name, result, savedContents[i])
	}
	// Newer disk requirements must neither replace replayed observations nor
	// be overwritten by replaying their already committed calls.
	design := *state.pack.Design.Design
	design.Requirements = []string{"C"}
	raw, _ := json.Marshal(design)
	writeGenerationFile(t, input.ProjectDir, ".design.json", raw)
	slide := spec.SlideSpec{Core: "B", Elements: []spec.Element{}}
	slide.Core = "C"
	raw, _ = json.Marshal(slide)
	writeGenerationFile(t, input.ProjectDir, model.SlideSpecPath(generationSlide), raw)
	recovered := batchState(frozen)
	recovered.tx, recovered.messages, recovered.seenVersions = state.tx, state.messages, versions
	calls = append(calls, llm.ToolCall{ID: "recovered_html", Name: "edit_html", Args: map[string]any{
		"slide_id": generationSlide, "content": strings.Replace(generationHTML, "Original", "Recovered", 1),
	}})
	for i, result := range runtime.executeToolBatch(context.Background(), input, recovered, registry, names, calls) {
		if !result.OK {
			t.Fatalf("recovered call: %+v", result)
		}
		if i < len(savedContents) {
			assertSavedResourceObservation(t, calls[i].Name, result, savedContents[i])
			if result.Observation != savedResults[i].Observation || !reflect.DeepEqual(result.ObservationMetadata, savedResults[i].ObservationMetadata) || !reflect.DeepEqual(result.ChangedTargets, savedResults[i].ChangedTargets) {
				t.Fatalf("replay lost saved observation, versions or changes: %+v", result)
			}
		}
	}
	if commits != 3 || htmlDesign != "B" {
		t.Fatalf("replay advanced the wrong view or wrote again: commits=%d design=%s", commits, htmlDesign)
	}
	if disk, _ := os.ReadFile(filepath.Join(input.ProjectDir, ".design.json")); !strings.Contains(string(disk), `"C"`) {
		t.Fatal("replay overwrote newer design")
	}
	if disk, _ := os.ReadFile(filepath.Join(input.ProjectDir, model.SlideSpecPath(generationSlide))); !strings.Contains(string(disk), `"C"`) {
		t.Fatal("replay overwrote newer spec")
	}
}

func TestSuccessfulToolOrderAndRenderDoesNotCommit(t *testing.T) {
	dir, css, pack := generationPackFixture(t)
	state := batchState(pack)
	session, err := NewRunSession(dir, state.runID)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	state.tx = session
	runtime := NewRuntime(&scriptedAgent{})
	commits := []CommitContext{}
	receipts := newMemoryIdempotencyStore()
	input := RuntimeInput{RunID: state.runID, ProjectDir: dir, Context: pack, Idempotency: receipts, Prompter: autoApprovingResourcePrompter{projectDir: dir},
		DomainTools: DefaultDomainToolProvider{Renderer: successfulScreenshotRenderer{}, Themes: staticThemeLoader{model.Theme{ResourceContentState: model.ResourceContentState{ContentState: "ready"}, ID: "clean", CSS: css}}},
		CommitMetadata: func(ctx context.Context, c CommitContext) error {
			commits = append(commits, c)
			return receipts.CompleteIdempotency(ctx, "tool_call", state.runID, c.OperationID, "completed", c.ToolResultJSON)
		},
	}
	registry, err := buildDomainToolRegistry(input)
	if err != nil {
		t.Fatal(err)
	}
	designB := *pack.Design.Design
	designB.Requirements = []string{"B"}
	designC := designB
	designC.Requirements = []string{"C"}
	calls := []llm.ToolCall{
		{ID: "design_b", Name: "edit_design", Args: map[string]any{"requirements": designB.Requirements}},
		{ID: "html_b", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "content": strings.Replace(generationHTML, "Original", "B", 1)}},
		{ID: "design_c", Name: "edit_design", Args: map[string]any{"requirements": designC.Requirements}},
		{ID: "render_c", Name: "render_slide", Args: map[string]any{"slide_id": generationSlide, "need_preview": false}},
	}
	// Provider arguments arrive as JSON objects, never Go authoring structs.
	rawCalls, _ := json.Marshal(calls)
	_ = json.Unmarshal(rawCalls, &calls)
	state.messages = testResourceMessages(t, dir, pack)
	state.rememberResourceVersions()
	results := []ToolResult{}
	for _, call := range calls {
		next := runtime.executeToolBatch(context.Background(), input, state, registry, map[string]bool{"edit_design": true, "edit_html": true, "render_slide": true}, []llm.ToolCall{call})
		state.messages = appendBatchObservations(state.messages, []llm.ToolCall{call}, "", next)
		state.rememberResourceVersions()
		results = append(results, next...)
	}
	for _, result := range results {
		if !result.OK {
			t.Fatalf("tool failed: %+v", result)
		}
	}
	if len(commits) != 3 || commits[0].Changes.Updated[0].Artifact.Kind != ArtifactDesign ||
		commits[1].Changes.Updated[0].Artifact.Kind != ArtifactSlideHTML || commits[2].Changes.Updated[0].Artifact.Kind != ArtifactDesign {
		t.Fatalf("tool commits lost order or render wrote metadata: %+v", commits)
	}
	if state.pack.Design.Design.Requirements[0] != "C" {
		t.Fatal("runtime did not retain the latest design")
	}
	gateContext := CompletionContext{Mode: model.ModeExecute, Session: session, Context: state.pack, Changes: state.committedChanges, Evidence: state.ledger}
	if issues := (EvidenceCompletionPolicy{}).Check(gateContext); len(issues) != 0 {
		t.Fatalf("render evidence rejected: %+v", issues)
	}
	replay := runtime.executeToolBatch(context.Background(), input, state, registry, map[string]bool{"edit_html": true}, calls[1:2])
	if !replay[0].OK || len(commits) != 3 {
		t.Fatal("replay repeated a committed operation")
	}
	if _, err = os.Stat(filepath.Join(dir, "materialization.json")); !os.IsNotExist(err) {
		t.Fatal("render wrote materialization")
	}
}

func TestReferenceOnlyCompletionDoesNotRequireHTMLButHTMLRequiresRender(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	session, err := NewRunSession(dir, "gate")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	changes := EmptyChangeSet()
	ledger := NewEvidenceLedger()
	for _, ref := range []ArtifactRef{manifestRef(pack), designRef(pack), specSlideRef(generationSlide)} {
		raw, err := session.Read(ref)
		if err != nil {
			t.Fatal(err)
		}
		change := ArtifactChange{Artifact: ref, Source: "edit_spec", AfterHash: hashBytes(raw)}
		changes.Updated = append(changes.Updated, change)
		ledger.Record(newEvidence("schema", ref.Resource(), change.AfterHash))
	}
	ctx := CompletionContext{Mode: model.ModeExecute, Session: session, Context: pack, Changes: changes, Evidence: ledger}
	if issues := (EvidenceCompletionPolicy{}).Check(ctx); len(issues) != 0 {
		t.Fatalf("reference-only task blocked: %+v", issues)
	}
	ref := slideHTMLRef(generationSlide)
	hash := hashBytes([]byte(generationHTML))
	ctx.Changes.Updated = append(ctx.Changes.Updated, ArtifactChange{Artifact: ref, Source: "run_command", AfterHash: hash})
	ledger.Record(newEvidence("static", ref.Resource(), hash))
	if issues := (EvidenceCompletionPolicy{}).Check(ctx); len(issues) != 1 || issues[0].Code != "EVIDENCE_HTML_MISSING" {
		t.Fatalf("HTML bypassed render: %+v", issues)
	}
}

func TestMutationJournalRecoversWithReceiptAndDetectsTampering(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "rollback", true: "committed"}[committed], func(t *testing.T) {
			dir, _, pack := generationPackFixture(t)
			session, err := NewRunSession(dir, "crash")
			if err != nil {
				t.Fatal(err)
			}
			defer session.Discard()
			ref := slideHTMLRef(generationSlide)
			after := []byte(strings.Replace(generationHTML, "Original", "After", 1))
			if _, err := session.Write(ref, "edit_spec", after); err != nil {
				t.Fatal(err)
			}
			if err := session.ValidateStagedArtifacts(pack); err != nil {
				t.Fatal(err)
			}
			journal := MutationJournal{RunID: "crash", OperationID: "write", Artifacts: session.Snapshot().Artifacts}
			journal.RequestHash, err = mutationRequestHash(journal.RunID, journal.OperationID, journal.Artifacts)
			if err != nil {
				t.Fatal(err)
			}
			path := mutationJournalPath(dir, journal.RunID, journal.OperationID)
			if err := writeMutationJournal(path, journal); err != nil {
				t.Fatal(err)
			}
			writeGenerationFile(t, dir, ref.Path, after)
			session.RollbackOperation()
			if err := RecoverMutationJournals(context.Background(), dir, func(context.Context, string, string) (string, bool, error) {
				return journal.RequestHash, committed, nil
			}); err != nil {
				t.Fatal(err)
			}
			want := generationHTML
			if committed {
				want = string(after)
			}
			if raw, _ := os.ReadFile(filepath.Join(dir, ref.Path)); string(raw) != want {
				t.Fatalf("crash recovery: %s", raw)
			}
			journal.Artifacts[0].AfterContent = []byte("tampered")
			if err := writeMutationJournal(path, journal); err != nil {
				t.Fatal(err)
			}
			if err := RecoverMutationJournals(context.Background(), dir, nil); err == nil {
				t.Fatal("tampered artifact accepted")
			}
		})
	}
}

func TestStagedCommandValidationHonorsRealPagePathsAndReferences(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	session, err := NewRunSession(dir, "paths")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	if _, err := session.Write(projectFileRef("examples/index.html"), "run_command", []byte("example")); err != nil {
		t.Fatal(err)
	}
	if err := session.ValidateStagedArtifacts(pack); err != nil {
		t.Fatalf("non-page HTML treated as a slide: %v", err)
	}
	session.RollbackOperation()
	if _, err := session.Write(projectFileRef(model.SlideHTMLPath("sli_outside")), "run_command", []byte(generationHTML)); err != nil {
		t.Fatal(err)
	}
	if err := session.ValidateStagedArtifacts(pack); err == nil {
		t.Fatal("command accepted a page absent from outline")
	}
	session.RollbackOperation()
	if _, err := session.Write(projectFileRef(model.SlideHTMLPath(generationSlide)), "run_command", []byte("<html>invalid</html>")); err != nil {
		t.Fatal(err)
	}
	if err := session.ValidateStagedArtifacts(pack); err == nil {
		t.Fatal("command bypassed HTML structure check")
	}
}

func TestDeletedPagesClearReferenceContextAndNeedNoRender(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	session, err := NewRunSession(dir, "delete")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	pack.Command.Scope = model.NewRunScope(model.ScopeAllPages, generationSlide)
	empty := spec.Outline{Sections: []spec.Section{}}
	raw, _ := json.Marshal(empty)
	if _, err := session.Write(outlineRef(pack), "edit_spec", raw); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []ArtifactRef{specSlideRef(generationSlide), slideHTMLRef(generationSlide)} {
		if err := session.Delete(ref, "edit_spec"); err != nil {
			t.Fatal(err)
		}
	}
	if err := session.ValidateStagedArtifacts(pack); err != nil {
		t.Fatalf("page deletion rejected: %v", err)
	}
	changes, err := session.CommitOperation(context.Background(), "delete", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	state := batchState(pack)
	refreshRuntimePack(dir, state, []ChangedTarget{{Type: "deck", Part: "outline"}})
	if len(state.pack.Outline.Summaries) != 0 {
		t.Fatal("deleted page retained its summary")
	}
	ledger := NewEvidenceLedger()
	ledger.Record(newEvidence("schema", Resource{Type: "deck", Part: "outline"}, hashBytes(raw)))
	ctx := CompletionContext{Mode: model.ModeExecute, Session: session, Scope: state.scope, Context: state.pack, Changes: changes, Evidence: ledger}
	issues := append((ScopeCompletionPolicy{}).Check(ctx), (EvidenceCompletionPolicy{}).Check(ctx)...)
	if len(issues) != 0 {
		t.Fatalf("deleted page blocked completion: %+v", issues)
	}
}

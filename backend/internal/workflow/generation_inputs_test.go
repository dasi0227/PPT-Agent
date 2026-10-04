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
	value := &spec.GenerationInputs{Manifest: spec.Manifest{Title: "Deck", Goal: "Explain", Audience: "Builders", Language: "zh-CN", Pages: "待明确", Requirements: []string{}, Prohibitions: []string{}}, Design: spec.Design{Demands: []string{"A"}, Decorations: spec.DefaultDecorations()}, Spec: spec.SlideSpec{Core: "Message", Elements: []spec.Element{}}}
	pack := testPack(model.ModeExecute, model.ScopeCurrentPage, false, "Update requirements")
	pack.Project.ThemeID = "clean"
	pack.Command.Scope = model.NewRunScope(model.ScopeCurrentPage, generationSlide)
	pack.Target.SlideIDs = []string{generationSlide}
	pack.Target.SlideSpec = &value.Spec
	pack.PresentationManifest.Manifest = value.Manifest
	pack.Design.Design = &value.Design
	pack.Outline.Outline = spec.Outline{Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Section", Purpose: "Explain", Slides: []spec.SlideNode{{SlideID: generationSlide, Title: "Page"}}, Subsections: []spec.Subsection{}}}}
	pack.Outline.Summaries = []contextengine.SlideSummary{{ID: generationSlide, State: string(model.HTMLAvailable)}}
	pack.GenerationInputs = map[string]*spec.GenerationInputs{generationSlide: value.Clone()}
	pack.GenerationBaselines = map[string]*spec.GenerationInputs{generationSlide: value.Clone()}
	for path, content := range map[string]any{".manifest.json": value.Manifest, ".outline.json": pack.Outline.Outline, ".design.json": value.Design, model.SpecCollectionPath: map[string]spec.SlideSpec{generationSlide: value.Spec}} {
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

func TestGenerationSnapshotCommitsFrozenViewWithHTMLAndRollsBack(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	session, err := NewRunSession(dir, "generation")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	ref := slideHTMLRef(generationSlide)
	changed := []byte(strings.Replace(generationHTML, "Original", "Changed", 1))
	// A disk edit after model dispatch must not enter this response's snapshot.
	changedDesign := *pack.Design.Design
	changedDesign.Demands = []string{"Unseen"}
	raw, _ := json.Marshal(changedDesign)
	writeGenerationFile(t, dir, ".design.json", raw)
	if _, err = session.Write(projectFileRef(ref.Path), "run_command", changed); err != nil {
		t.Fatal(err)
	}
	inputs, err := session.StageGenerationInputs(session.generationContext(pack))
	if err != nil {
		t.Fatal(err)
	}
	if value := spec.ParseGenerationInputs(inputs[generationSlide]); value == nil || value.Design.Demands[0] != "A" {
		t.Fatalf("snapshot reread disk: %s", inputs[generationSlide])
	}
	failure := errors.New("database unavailable")
	if _, err = session.CommitOperation(context.Background(), "failed", "", func(_ context.Context, c CommitContext) error {
		if !reflect.DeepEqual(c.GenerationInputs, inputs) {
			t.Fatal("snapshot missing from commit")
		}
		return failure
	}); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(filepath.Join(dir, ref.Path)); string(raw) != generationHTML {
		t.Fatal("failed write survived")
	}
	if pack.GenerationBaselines[generationSlide].Design.Demands[0] != "A" {
		t.Fatal("failure advanced baseline")
	}
	if _, err = session.Write(projectFileRef(ref.Path), "run_command", changed); err != nil {
		t.Fatal(err)
	}
	inputs, err = session.StageGenerationInputs(pack)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := session.CommitOperation(context.Background(), "success", "", func(_ context.Context, c CommitContext) error {
		if c.Changes.Updated[0].Artifact.Kind != ArtifactSlideHTML || len(c.GenerationInputs) != 1 {
			t.Fatalf("command bypassed domain commit: %+v", c)
		}
		return nil
	})
	if err != nil || changes.Count() != 1 {
		t.Fatalf("commit: %+v %v", changes, err)
	}
	contextengine.AcceptGenerationInputs(&pack, inputs)
	if _, err = session.Write(ref, "edit_html", changed); err != nil {
		t.Fatal(err)
	}
	inputs, err = session.StageGenerationInputs(pack)
	if err != nil || len(inputs) != 0 || session.ChangeSet().Count() != 0 {
		t.Fatalf("same bytes advanced snapshot: %v %v", inputs, err)
	}
}

func TestConcurrentGenerationSnapshotIncludesEarlierRequirementsAndExcludesLaterCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	allowHTML, laterCommitted := make(chan struct{}), make(chan struct{})
	input, state, registry := authoringBatchFixture(t, func(ctx context.Context, input DomainToolInput) error {
		if input.CallID != "html_b" {
			return nil
		}
		select {
		case <-allowHTML:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	var baseline *spec.GenerationInputs
	input.CommitMetadata = func(_ context.Context, c CommitContext) error {
		if c.OperationID == "design_c" {
			close(laterCommitted)
		}
		if c.OperationID == "html_b" {
			baseline = spec.ParseGenerationInputs(c.GenerationInputs[generationSlide])
		}
		return nil
	}
	calls := []llm.ToolCall{
		{ID: "design_b", Name: "edit_design", Args: map[string]any{"demands": []any{"B"}}},
		{ID: "html_b", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "content": strings.Replace(generationHTML, "Original", "B", 1)}},
		{ID: "design_c", Name: "edit_design", Args: map[string]any{"demands": []any{"C"}}},
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
	if baseline == nil || baseline.Design.Demands[0] != "B" || state.pack.Design.Design.Demands[0] != "C" {
		t.Fatalf("snapshot contaminated by later commit: %+v", baseline)
	}
	if state.pack.GenerationBaselines[generationSlide].Design.Demands[0] != "B" {
		t.Fatal("runtime baseline lost the execution view")
	}
}

func TestReplayedRequirementsUseReturnedContentInGenerationSnapshot(t *testing.T) {
	input, state, registry := authoringBatchFixture(t, nil)
	frozen := cloneAuthoringPack(state.pack)
	versions := maps.Clone(state.seenVersions)
	receipts := newMemoryIdempotencyStore()
	input.Idempotency = receipts
	var baseline *spec.GenerationInputs
	commits := 0
	input.CommitMetadata = func(ctx context.Context, c CommitContext) error {
		commits++
		if c.OperationID == "recovered_html" {
			baseline = spec.ParseGenerationInputs(c.GenerationInputs[generationSlide])
		}
		return receipts.CompleteIdempotency(ctx, "tool_call", state.runID, c.OperationID, "completed", c.ToolResultJSON)
	}
	calls := []llm.ToolCall{
		{ID: "saved_design", Name: "edit_design", Args: map[string]any{"demands": []any{"B"}}},
		{ID: "saved_spec", Name: "edit_spec", Args: map[string]any{"slide_id": generationSlide, "core": "B"}},
	}
	runtime := NewRuntime(nil)
	names := schemasByName(registry.Disclose(PhaseExecuting, model.ModeExecute, state.scope))
	for _, result := range runtime.executeToolBatch(context.Background(), input, state, registry, names, calls) {
		if !result.OK {
			t.Fatalf("initial write: %+v", result)
		}
	}
	// Newer disk requirements must neither replace replayed observations nor
	// be overwritten by replaying their already committed calls.
	design := *state.pack.Design.Design
	design.Demands = []string{"C"}
	raw, _ := json.Marshal(design)
	writeGenerationFile(t, input.ProjectDir, ".design.json", raw)
	slide := state.pack.GenerationInputs[generationSlide].Spec
	slide.Core = "C"
	raw, _ = json.Marshal(map[string]spec.SlideSpec{generationSlide: slide, otherBatchSlide: frozen.GenerationInputs[otherBatchSlide].Spec})
	writeGenerationFile(t, input.ProjectDir, model.SpecCollectionPath, raw)
	recovered := batchState(frozen)
	recovered.tx, recovered.messages, recovered.seenVersions = state.tx, state.messages, versions
	calls = append(calls, llm.ToolCall{ID: "recovered_html", Name: "edit_html", Args: map[string]any{
		"slide_id": generationSlide, "content": strings.Replace(generationHTML, "Original", "Recovered", 1),
	}})
	for _, result := range runtime.executeToolBatch(context.Background(), input, recovered, registry, names, calls) {
		if !result.OK {
			t.Fatalf("recovered call: %+v", result)
		}
	}
	if commits != 3 || baseline == nil || baseline.Design.Demands[0] != "B" || baseline.Spec.Core != "B" {
		t.Fatalf("replay advanced the wrong snapshot or wrote again: commits=%d baseline=%+v", commits, baseline)
	}
	if disk, _ := os.ReadFile(filepath.Join(input.ProjectDir, ".design.json")); !strings.Contains(string(disk), `"C"`) {
		t.Fatal("replay overwrote newer design")
	}
	if disk, _ := os.ReadFile(filepath.Join(input.ProjectDir, model.SpecCollectionPath)); !strings.Contains(string(disk), `"C"`) {
		t.Fatal("replay overwrote newer spec")
	}
}

func TestGenerationSnapshotsFollowSuccessfulToolOrderAndRenderDoesNotCommit(t *testing.T) {
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
	input := RuntimeInput{RunID: state.runID, ProjectDir: dir, Context: pack, Idempotency: receipts,
		DomainToolsForContext: func(p contextengine.ContextPack) DomainToolProvider {
			return DefaultDomainToolProvider{Pack: p, Renderer: successfulScreenshotRenderer{}, Themes: staticThemeLoader{model.Theme{ResourceContentState: model.ResourceContentState{ContentState: "ready"}, ID: "clean", CSS: css}}}
		},
		CommitMetadata: func(ctx context.Context, c CommitContext) error {
			commits = append(commits, c)
			return receipts.CompleteIdempotency(ctx, "tool_call", state.runID, c.OperationID, "completed", c.ToolResultJSON)
		},
	}
	registry, err := buildDomainToolRegistry(input, pack)
	if err != nil {
		t.Fatal(err)
	}
	designB := *pack.Design.Design
	designB.Demands = []string{"B"}
	designC := designB
	designC.Demands = []string{"C"}
	calls := []llm.ToolCall{
		{ID: "design_b", Name: "edit_design", Args: map[string]any{"demands": designB.Demands}},
		{ID: "html_b", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "content": strings.Replace(generationHTML, "Original", "B", 1)}},
		{ID: "design_c", Name: "edit_design", Args: map[string]any{"demands": designC.Demands}},
		{ID: "render_c", Name: "render_slide", Args: map[string]any{"slide_id": generationSlide}},
	}
	// Provider arguments arrive as JSON objects, never Go authoring structs.
	rawCalls, _ := json.Marshal(calls)
	_ = json.Unmarshal(rawCalls, &calls)
	state.messages = testResourceMessages(t, dir, pack)
	state.rememberResourceVersions(0)
	results := []ToolResult{}
	for _, call := range calls {
		next := runtime.executeToolBatch(context.Background(), input, state, registry, map[string]bool{"edit_design": true, "edit_html": true, "render_slide": true}, []llm.ToolCall{call})
		before := len(state.messages)
		state.messages = appendBatchObservations(state.messages, []llm.ToolCall{call}, "", next)
		state.rememberResourceVersions(before)
		results = append(results, next...)
	}
	for _, result := range results {
		if !result.OK {
			t.Fatalf("tool failed: %+v", result)
		}
	}
	if len(commits) != 3 || len(commits[0].GenerationInputs) != 0 || len(commits[2].GenerationInputs) != 0 {
		t.Fatalf("render/reference advanced snapshot: %+v", commits)
	}
	baseline := spec.ParseGenerationInputs(commits[1].GenerationInputs[generationSlide])
	if baseline == nil || baseline.Design.Demands[0] != "B" {
		t.Fatalf("wrong tool-order snapshot: %+v", baseline)
	}
	if state.pack.GenerationBaselines[generationSlide].Design.Demands[0] != "B" || state.pack.GenerationInputs[generationSlide].Design.Demands[0] != "C" {
		t.Fatal("later reference/render replaced generation baseline")
	}
	// The HTML was rendered against C, independent of its generation baseline B.
	gateContext := CompletionContext{Mode: model.ModeExecute, Session: session, Context: state.pack, Changes: state.committedChanges, Evidence: state.ledger}
	if issues := (EvidenceCompletionPolicy{}).Check(gateContext); len(issues) != 0 {
		t.Fatalf("render evidence rejected: %+v", issues)
	}
	replay := runtime.executeToolBatch(context.Background(), input, state, registry, map[string]bool{"edit_html": true}, calls[1:2])
	if !replay[0].OK || len(commits) != 3 || state.pack.GenerationBaselines[generationSlide].Design.Demands[0] != "B" {
		t.Fatal("replay advanced snapshot")
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

func TestReferenceContextDeduplicatesClearsAndRebuilds(t *testing.T) {
	_, _, pack := generationPackFixture(t)
	pack.GenerationInputs[generationSlide].Design.Demands = []string{"B"}
	req := prepareAgentRequest(AgentRequest{RunID: "refs", Mode: model.ModeExecute, Phase: PhaseExecuting, Context: pack})
	key := "html_reference_changes/" + generationSlide
	if !strings.Contains(contextSectionText(req.Messages, key), `"old":["A"],"new":["B"]`) {
		t.Fatal("missing reference changes")
	}
	same := prepareAgentRequest(req)
	if !reflect.DeepEqual(req.Messages, same.Messages) {
		t.Fatal("duplicate changes appended")
	}
	// Compaction reconstructs differences from snapshots, not previously delivered messages.
	compacted := req
	compacted.Messages = []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("summary")}}
	compacted = prepareAgentRequest(compacted)
	if contextSectionText(compacted.Messages, key) != contextSectionText(req.Messages, key) {
		t.Fatal("compaction lost changes")
	}
	req.Context.GenerationInputs[generationSlide].Design.Demands = []string{"A"}
	cleared := prepareAgentRequest(req)
	if !strings.Contains(contextSectionText(cleared.Messages, key), `"value":null`) {
		t.Fatal("net revert failed to clear prior difference")
	}
}

func TestGenerationSnapshotJournalRecoversWithReceiptAndDetectsTampering(t *testing.T) {
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
			inputs, err := session.StageGenerationInputs(pack)
			if err != nil {
				t.Fatal(err)
			}
			journal := MutationJournal{RunID: "crash", OperationID: "write", Artifacts: session.Snapshot().Artifacts, GenerationInputs: inputs}
			journal.RequestHash, err = mutationRequestHash(journal.RunID, journal.OperationID, journal.Artifacts, inputs)
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
			journal.GenerationInputs = map[string]json.RawMessage{generationSlide: json.RawMessage("null")}
			if err := writeMutationJournal(path, journal); err != nil {
				t.Fatal(err)
			}
			if err := RecoverMutationJournals(context.Background(), dir, nil); err == nil {
				t.Fatal("tampered snapshot accepted")
			}
		})
	}
	// Empty maps omitted by the journal must preserve the request digest.
	a, _ := mutationRequestHash("run", "operation", nil, map[string]json.RawMessage{})
	b, _ := mutationRequestHash("run", "operation", nil, nil)
	if a != b {
		t.Fatal("journal serialization changes request identity")
	}
}

func TestCommandSnapshotDetectionHonorsRealPagePathsAndScope(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	session, err := NewRunSession(dir, "paths")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	if _, err := session.Write(projectFileRef("examples/index.html"), "run_command", []byte("example")); err != nil {
		t.Fatal(err)
	}
	if inputs, err := session.StageGenerationInputs(pack); err != nil || len(inputs) != 0 {
		t.Fatalf("non-page HTML created a baseline: %v %v", inputs, err)
	}
	session.RollbackOperation()
	if _, err := session.Write(projectFileRef(model.SlideHTMLPath("sli_outside")), "run_command", []byte(generationHTML)); err != nil {
		t.Fatal(err)
	}
	if _, err := session.StageGenerationInputs(pack); !errors.Is(err, ErrTargetOutOfScope) {
		t.Fatalf("command bypassed page scope: %v", err)
	}
	session.RollbackOperation()
	if _, err := session.Write(projectFileRef(model.SlideHTMLPath(generationSlide)), "run_command", []byte("<html>invalid</html>")); err != nil {
		t.Fatal(err)
	}
	if _, err := session.StageGenerationInputs(pack); err == nil {
		t.Fatal("command bypassed HTML structure check")
	}
}

func TestCommandCannotRemoveOutlinePageWithoutDomainCleanup(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	session, err := NewRunSession(dir, "command-remove")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()

	empty, _ := json.Marshal(spec.Outline{Sections: []spec.Section{}})
	if _, err := session.Write(projectFileRef(".outline.json"), "run_command", empty); err != nil {
		t.Fatal(err)
	}
	if _, err := session.StageGenerationInputs(pack); err == nil || !strings.Contains(err.Error(), "edit_outline") {
		t.Fatalf("command removed page without cleaning related files: %v", err)
	}
	session.RollbackOperation()
	for _, path := range []string{".outline.json", model.SpecCollectionPath, model.SlideHTMLPath(generationSlide)} {
		if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
			t.Fatalf("rejected command changed %s: %v", path, err)
		}
	}
	var saved spec.Outline
	if raw, err := os.ReadFile(filepath.Join(dir, ".outline.json")); err != nil || json.Unmarshal(raw, &saved) != nil || len(spec.FlattenOutline(saved)) != 1 {
		t.Fatalf("rejected command changed outline: %v", err)
	}
	if err := session.Delete(projectFileRef(".outline.json"), "run_command"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.StageGenerationInputs(pack); err == nil || !strings.Contains(err.Error(), "cannot delete outline") {
		t.Fatalf("command deleted outline: %v", err)
	}
	session.RollbackOperation()

	var renamed spec.Outline
	baseline, _ := json.Marshal(pack.Outline.Outline)
	if err := json.Unmarshal(baseline, &renamed); err != nil {
		t.Fatal(err)
	}
	renamed.Sections[0].Slides[0].Title = "Renamed"
	raw, _ := json.Marshal(renamed)
	if _, err := session.Write(projectFileRef(".outline.json"), "run_command", raw); err != nil {
		t.Fatal(err)
	}
	if _, err := session.StageGenerationInputs(pack); err != nil {
		t.Fatalf("non-deleting command was rejected: %v", err)
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
	inputs, err := session.StageGenerationInputs(pack)
	if err != nil || len(inputs) != 0 {
		t.Fatalf("deletion created snapshot: %v %v", inputs, err)
	}
	changes, err := session.CommitOperation(context.Background(), "delete", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	state := batchState(pack)
	refreshRuntimePack(dir, state, []ChangedTarget{{Type: "deck", Part: "outline"}})
	if len(state.pack.GenerationInputs) != 0 || len(state.pack.GenerationBaselines) != 0 {
		t.Fatal("deleted page retained reference context")
	}
	ledger := NewEvidenceLedger()
	ledger.Record(newEvidence("schema", Resource{Type: "deck", Part: "outline"}, hashBytes(raw)))
	ctx := CompletionContext{Mode: model.ModeExecute, Session: session, Scope: state.scope, Context: state.pack, Changes: changes, Evidence: ledger}
	issues := append((ScopeCompletionPolicy{}).Check(ctx), (EvidenceCompletionPolicy{}).Check(ctx)...)
	if len(issues) != 0 {
		t.Fatalf("deleted page blocked completion: %+v", issues)
	}
}

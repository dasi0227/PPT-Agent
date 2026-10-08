package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type stagedFileTool struct{}

type autoApprovingResourcePrompter struct{ projectDir string }

func (autoApprovingResourcePrompter) Ask(context.Context, model.QuestionAskedPayload) (model.QuestionAnswer, string, error) {
	return model.QuestionAnswer{}, "", nil
}
func (p autoApprovingResourcePrompter) AskResourceEditApproval(_ context.Context, payload model.ResourceEditApprovalRequestedPayload) (model.ResourceEditApprovalAnswer, error) {
	answer := model.ResourceEditApprovalAnswer{InteractionID: payload.InteractionID, CallID: payload.CallID, Revision: payload.Revision, Decision: "approve"}
	_, err := DecideResourceEditApproval(p.projectDir, payload.RunID, payload.InteractionID, answer, func() error { return nil })
	return answer, err
}
func (autoApprovingResourcePrompter) ResumeAfterResourceEditApproval(context.Context) {}

func (stagedFileTool) Schema() ToolSchema {
	return ToolSchema{Name: "edit_spec", Parameters: objectSchema(nil, map[string]any{})}
}

func (stagedFileTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	if _, err := input.Session.Write(projectFileRef("generated.txt"), "test", []byte("durable\n")); err != nil {
		return failedToolResult(CodeCommitFailed, err.Error())
	}
	return SuccessfulToolResult("written")
}

type diskProbeTool struct{}

func (diskProbeTool) Schema() ToolSchema {
	return ToolSchema{Name: "disk_probe", Parameters: objectSchema(nil, map[string]any{})}
}

func (diskProbeTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	raw, err := os.ReadFile(filepath.Join(input.ProjectDir, "generated.txt"))
	if err != nil {
		return failedToolResult(CodeAgentFailed, err.Error())
	}
	result := SuccessfulToolResult("read")
	result.Observation = string(raw)
	return result
}

type timedBatchTool struct {
	name   string
	delay  time.Duration
	mu     *sync.Mutex
	order  *[]string
	active *int
	peak   *int
}

func (t timedBatchTool) Schema() ToolSchema {
	return ToolSchema{Name: t.name, Parameters: objectSchema([]string{"id"}, map[string]any{
		"id": map[string]any{"type": "string"}, "fail": map[string]any{"type": "boolean"},
	})}
}

func (t timedBatchTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
	id := stringValue(input.Args["id"])
	if t.mu != nil {
		t.mu.Lock()
		*t.active++
		if *t.active > *t.peak {
			*t.peak = *t.active
		}
		*t.order = append(*t.order, "start:"+id)
		t.mu.Unlock()
	}
	select {
	case <-time.After(t.delay):
	case <-ctx.Done():
		return failedToolResult(CodeCanceled, ctx.Err().Error())
	}
	if t.mu != nil {
		t.mu.Lock()
		*t.active--
		*t.order = append(*t.order, "end:"+id)
		t.mu.Unlock()
	}
	if failed, _ := input.Args["fail"].(bool); failed {
		return failedToolResult(CodeContentInvalid, "requested failure")
	}
	return SuccessfulToolResult("ok")
}

func TestIndependentReadBatchRunsWithBoundedConcurrencyAndPairedEvents(t *testing.T) {
	dir, pack := t.TempDir(), testPack(model.ModeExecute, model.ScopeAllPages, true, "batch")
	registry := NewToolRegistry()
	var mu sync.Mutex
	order := []string{}
	active, peak := 0, 0
	if err := registry.Register(
		timedBatchTool{name: "read_resource", delay: 80 * time.Millisecond, mu: &mu, order: &order, active: &active, peak: &peak},
		true, "ppt.read", RiskLow, PhaseExecuting,
	); err != nil {
		t.Fatal(err)
	}
	events := &eventRecorder{}
	state := batchState(pack)
	calls := []llm.ToolCall{
		{ID: "r1", Name: "read_resource", Args: map[string]any{"id": "one"}},
		{ID: "r2", Name: "read_resource", Args: map[string]any{"id": "two"}},
	}
	started := time.Now()
	results := NewRuntime(nil).executeToolBatch(context.Background(), RuntimeInput{
		RunID: "batch-read", ProjectDir: dir, Context: pack, Emitter: events,
	}, state, registry, map[string]bool{"read_resource": true}, calls)
	if elapsed := time.Since(started); elapsed >= 150*time.Millisecond {
		t.Fatalf("independent reads did not overlap: %s order=%v", elapsed, order)
	}
	if peak != 2 || len(results) != 2 || !results[0].OK || !results[1].OK {
		t.Fatalf("peak=%d results=%+v order=%v", peak, results, order)
	}
	if events.count(model.EventToolStarted) != 2 || events.count(model.EventToolCompleted) != 2 {
		t.Fatalf("tool events were not paired: %+v", events.events)
	}
}

func TestWriteBatchOrdersSameResourceAndSkipsFailedDependencies(t *testing.T) {
	dir, pack := t.TempDir(), testPack(model.ModeExecute, model.ScopeAllPages, true, "batch")
	tx, err := NewRunSession(dir, "batch-write")
	if err != nil {
		t.Fatal(err)
	}
	registry := NewToolRegistry()
	var mu sync.Mutex
	order := []string{}
	active, peak := 0, 0
	if err := registry.Register(
		timedBatchTool{name: "edit_spec", delay: time.Millisecond, mu: &mu, order: &order, active: &active, peak: &peak},
		false, CapabilityWrite, RiskMedium, PhaseExecuting,
	); err != nil {
		t.Fatal(err)
	}
	events := &eventRecorder{}
	state := batchState(pack)
	state.tx = tx
	calls := []llm.ToolCall{
		{ID: "w1", Name: "edit_spec", Args: map[string]any{"id": "one"}},
		{ID: "w2", Name: "edit_spec", Args: map[string]any{"id": "two", "fail": true}},
		{ID: "w3", Name: "edit_spec", Args: map[string]any{"id": "three"}},
	}
	results := NewRuntime(nil).executeToolBatch(context.Background(), RuntimeInput{
		RunID: "batch-write", ProjectDir: dir, Context: pack, Emitter: events,
	}, state, registry, map[string]bool{"edit_spec": true}, calls)
	wantOrder := []string{"start:one", "end:one", "start:two", "end:two"}
	if len(order) != len(wantOrder) {
		t.Fatalf("unexpected execution order: %v", order)
	}
	for index := range wantOrder {
		if order[index] != wantOrder[index] {
			t.Fatalf("order=%v", order)
		}
	}
	if peak != 1 || results[1].OK || results[2].Code != "DEPENDENCY_FAILED" {
		t.Fatalf("same-resource dependency was not preserved: peak=%d results=%+v", peak, results)
	}
	if events.count(model.EventToolStarted) != 2 || events.count(model.EventToolCompleted) != 3 {
		t.Fatalf("skipped call must emit only a blocked completion: %+v", events.events)
	}
}

func TestSuccessfulWriteIsVisibleToNextDiskTool(t *testing.T) {
	dir, pack := t.TempDir(), testPack(model.ModeExecute, model.ScopeAllPages, true, "batch")
	tx, err := NewRunSession(dir, "batch-durable")
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Discard()
	registry := NewToolRegistry()
	if err := registry.Register(stagedFileTool{}, false, CapabilityWrite, RiskMedium, PhaseExecuting); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(diskProbeTool{}, true, CapabilityRead, RiskLow, PhaseExecuting); err != nil {
		t.Fatal(err)
	}
	state := batchState(pack)
	state.tx = tx
	results := NewRuntime(nil).executeToolBatch(context.Background(), RuntimeInput{
		RunID: "batch-durable", ProjectDir: dir, Context: pack,
	}, state, registry, map[string]bool{"edit_spec": true, "disk_probe": true}, []llm.ToolCall{
		{ID: "write", Name: "edit_spec", Args: map[string]any{}},
		{ID: "probe", Name: "disk_probe", Args: map[string]any{}},
	})
	if len(results) != 2 || !results[0].OK || !results[1].OK || results[1].Observation != "durable\n" {
		t.Fatalf("results=%+v", results)
	}
}

func TestDependencySkipsDoNotExhaustRepairBudget(t *testing.T) {
	state := &RunState{}
	recordToolFailures(state, []ToolResult{
		failedToolResult(CodeContentInvalid, "invalid spec"),
		failedToolResult(CodeDependencyFailed, "skipped"),
		failedToolResult(CodeDependencyFailed, "skipped"),
	})
	if state.toolFailures != 1 {
		t.Fatalf("dependency skips counted as failures: %d", state.toolFailures)
	}
	recordToolFailures(state, []ToolResult{
		failedToolResult(CodeRenderFailed, "slide 1"),
		failedToolResult(CodeRenderFailed, "slide 2"),
		failedToolResult(CodeRenderFailed, "slide 3"),
	})
	if state.toolFailures != 2 {
		t.Fatalf("one failed multi-render response was not one repair round: %d", state.toolFailures)
	}
	recordToolFailures(state, []ToolResult{SuccessfulToolResult("repaired")})
	if state.toolFailures != 0 {
		t.Fatalf("successful repair did not reset failure count: %d", state.toolFailures)
	}
}

const otherBatchSlide = "sli_bbbbbb"

type gatedResourceTool struct {
	DomainTool
	before func(context.Context, DomainToolInput) error
}

func (t gatedResourceTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
	if t.before != nil {
		if err := t.before(ctx, input); err != nil {
			return failedToolResult(CodeCanceled, err.Error())
		}
	}
	return t.DomainTool.Execute(ctx, input)
}

func authoringBatchFixture(t *testing.T, hook func(context.Context, DomainToolInput) error) (RuntimeInput, *RunState, *ToolRegistry) {
	t.Helper()
	dir, _, pack := generationPackFixture(t)
	pack.Command.Scope = model.NewRunScope(model.ScopeAllPages, generationSlide, otherBatchSlide)
	pack.Target.SlideIDs = []string{generationSlide, otherBatchSlide}
	pack.Outline.Outline.Sections[0].Slides = append(pack.Outline.Outline.Sections[0].Slides, spec.SlideNode{ID: otherBatchSlide, Title: "Other"})
	pack.GenerationInputs[otherBatchSlide] = pack.GenerationInputs[generationSlide].Clone()
	pack.GenerationBaselines[otherBatchSlide] = pack.GenerationInputs[generationSlide].Clone()
	raw, _ := json.Marshal(pack.Outline.Outline)
	writeGenerationFile(t, dir, ".outline.json", raw)
	raw, _ = json.Marshal(map[string]spec.SlideSpec{generationSlide: pack.GenerationInputs[generationSlide].Spec, otherBatchSlide: pack.GenerationInputs[otherBatchSlide].Spec})
	writeGenerationFile(t, dir, model.SpecCollectionPath, raw)
	writeGenerationFile(t, dir, model.SlideHTMLPath(otherBatchSlide), []byte(generationHTML))
	state := batchState(pack)
	state.tx, _ = NewRunSession(dir, state.runID)
	t.Cleanup(state.tx.Discard)
	state.messages = testResourceMessages(t, dir, pack)
	for _, part := range []string{"spec", "html"} {
		args := map[string]any{"resource": part, "slide_id": otherBatchSlide}
		result := (pptReadTool{}).Execute(context.Background(), DomainToolInput{Args: args, Context: pack, ProjectDir: dir, Scope: pack.Command.Scope})
		state.messages = appendBatchObservations(state.messages, []llm.ToolCall{{ID: "read_" + part, Name: "read_resource", Args: args}}, "", []ToolResult{result})
	}
	state.rememberResourceVersions(0)
	registry := NewToolRegistry()
	for _, name := range []string{"edit_manifest", "edit_design", "edit_outline", "edit_spec", "edit_html"} {
		if err := registry.Register(gatedResourceTool{DomainTool: resourceEditTool{name: name}, before: hook}, false, CapabilityWrite, RiskMedium, PhaseExecuting); err != nil {
			t.Fatal(err)
		}
	}
	if err := registry.Register(pptReadTool{}, true, CapabilityRead, RiskLow, PhaseExecuting); err != nil {
		t.Fatal(err)
	}
	return RuntimeInput{RunID: state.runID, ProjectDir: dir, Context: pack, Prompter: autoApprovingResourcePrompter{projectDir: dir}}, state, registry
}

func receiveBatchStart(t *testing.T, starts <-chan string) string {
	t.Helper()
	select {
	case id := <-starts:
		return id
	case <-time.After(3 * time.Second):
		t.Fatal("independent call did not start")
		return ""
	}
}

func TestAuthoringBatchIsolatesHTMLFailureAndRunsIndependentPagesConcurrently(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	starts, release := make(chan string, 4), make(chan struct{})
	input, state, registry := authoringBatchFixture(t, func(ctx context.Context, input DomainToolInput) error {
		starts <- input.CallID
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	events := &eventRecorder{}
	input.Emitter = events
	calls := []llm.ToolCall{
		{ID: "bad_a", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "edits": []any{map[string]any{"old_text": "absent", "new_text": "X"}}}},
		{ID: "dependent_a", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "content": generationHTML}},
		{ID: "good_b", Name: "edit_html", Args: map[string]any{"slide_id": otherBatchSlide, "content": strings.Replace(generationHTML, "Original", "B", 1)}},
	}
	done := make(chan []ToolResult, 1)
	go func() {
		done <- NewRuntime(nil).executeToolBatch(ctx, input, state, registry, schemasByName(registry.Disclose(PhaseExecuting, model.ModeExecute, state.scope)), calls)
	}()
	first, second := receiveBatchStart(t, starts), receiveBatchStart(t, starts)
	if first == second || first == "dependent_a" || second == "dependent_a" {
		t.Fatalf("wrong ready calls: %s %s", first, second)
	}
	close(release)
	results := <-done
	if results[0].Code != "EDIT_ANCHOR_NOT_FOUND" || results[1].Code != CodeDependencyFailed || !results[2].OK {
		t.Fatalf("results=%+v", results)
	}
	if !strings.Contains(results[1].Observation, `"dependency_call_id":"bad_a"`) || !strings.Contains(results[0].Observation, generationSlide) {
		t.Fatal("model error lost resource or dependency identity")
	}
	raw, _ := os.ReadFile(filepath.Join(input.ProjectDir, model.SlideHTMLPath(otherBatchSlide)))
	if string(raw) != calls[2].Args["content"] {
		t.Fatal("independent HTML was not saved verbatim")
	}
	if len(starts) != 0 || events.count(model.EventToolStarted) != 2 || events.count(model.EventToolCompleted) != 3 {
		t.Fatal("skipped call executed or events were lost")
	}
}

func TestAuthoringBatchAdvancesKnownHTMLAndStructuredVersionsAfterCommit(t *testing.T) {
	input, state, registry := authoringBatchFixture(t, nil)
	source := strings.Replace(generationHTML, "Original", "Compact", 1) + "\r\n"
	calls := []llm.ToolCall{
		{ID: "html1", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "content": source}},
		{ID: "html2", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "edits": []any{map[string]any{"old_text": "Compact", "new_text": "Next"}}}},
		{ID: "design1", Name: "edit_design", Args: map[string]any{"demands": []any{"B"}}},
		{ID: "design2", Name: "edit_design", Args: map[string]any{"demands": []any{"C"}}},
	}
	results := NewRuntime(nil).executeToolBatch(context.Background(), input, state, registry, schemasByName(registry.Disclose(PhaseExecuting, model.ModeExecute, state.scope)), calls)
	for _, result := range results {
		if !result.OK {
			t.Fatalf("false version conflict: %+v", result)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(input.ProjectDir, model.SlideHTMLPath(generationSlide)))
	if string(raw) != strings.Replace(source, "Compact", "Next", 1) || strings.Contains(results[0].Observation, "doctype") {
		t.Fatal("HTML was transformed or echoed")
	}
}

func TestAuthoringBatchSerializesApprovedGlobalsAndBlocksOnlyTheirDependents(t *testing.T) {
	ctx := context.Background()
	starts, release := make(chan string, 4), make(chan struct{})
	input, state, registry := authoringBatchFixture(t, func(ctx context.Context, input DomainToolInput) error {
		starts <- input.CallID
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	calls := []llm.ToolCall{
		{ID: "outline", Name: "edit_outline", Args: map[string]any{"edits": []any{map[string]any{"old_text": `"title":"Page"`, "new_text": `"title":"Updated"`}}}},
		{ID: "design", Name: "edit_design", Args: map[string]any{"decorations": map[string]any{"page_number": "bottom-left", "deck_title": "bottom-left"}}},
		{ID: "manifest", Name: "edit_manifest", Args: map[string]any{"title": "Updated deck"}},
		{ID: "html", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "content": generationHTML}},
	}
	close(release)
	results := NewRuntime(nil).executeToolBatch(ctx, input, state, registry, schemasByName(registry.Disclose(PhaseExecuting, model.ModeExecute, state.scope)), calls)
	for _, expected := range []string{"outline", "design", "manifest"} {
		if got := receiveBatchStart(t, starts); got != expected {
			t.Fatalf("approval order: got %s, want %s", got, expected)
		}
	}
	if !results[0].OK || results[1].OK || !results[2].OK || results[3].Code != CodeDependencyFailed {
		t.Fatalf("results=%+v", results)
	}
}

func TestAuthoringBatchOutlineFailureBlocksOnlyIdentifiedPagesAndAllowsDiagnosticReads(t *testing.T) {
	for _, effects := range []string{"known", "unknown"} {
		t.Run(effects, func(t *testing.T) {
			known := effects == "known"
			input, state, registry := authoringBatchFixture(t, nil)
			old := `"title":"Page"`
			if !known {
				old = "missing outline anchor"
			}
			calls := []llm.ToolCall{
				{ID: "bad_outline", Name: "edit_outline", Args: map[string]any{"edits": []any{map[string]any{"old_text": old, "new_text": `"title":""`}}}},
				{ID: "html_a", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "content": strings.Replace(generationHTML, "Original", "A", 1)}},
				{ID: "html_b", Name: "edit_html", Args: map[string]any{"slide_id": otherBatchSlide, "content": strings.Replace(generationHTML, "Original", "B", 1)}},
				{ID: "diagnose", Name: "read_resource", Args: map[string]any{"resource": "outline"}},
			}
			results := NewRuntime(nil).executeToolBatch(context.Background(), input, state, registry, schemasByName(registry.Disclose(PhaseExecuting, model.ModeExecute, state.scope)), calls)
			if results[0].OK || !results[2].OK || !results[3].OK || known && results[1].Code != CodeDependencyFailed || !known && !results[1].OK {
				t.Fatalf("outline failure spread beyond actual dependencies: %+v", results)
			}
			if !strings.Contains(results[3].Observation, "Page") {
				t.Fatal("diagnostic read did not return the surviving outline")
			}
		})
	}
}

func TestAuthoringBatchCommitFailureRollsBackOnlyItsCallAndReplaysReceipts(t *testing.T) {
	input, state, registry := authoringBatchFixture(t, nil)
	receipts := newMemoryIdempotencyStore()
	input.Idempotency = receipts
	commits := 0
	input.CommitMetadata = func(ctx context.Context, c CommitContext) error {
		if c.OperationID == "fail_commit" {
			return errors.New("metadata unavailable")
		}
		commits++
		return receipts.CompleteIdempotency(ctx, "tool_call", state.runID, c.OperationID, "completed", c.ToolResultJSON)
	}
	calls := []llm.ToolCall{
		{ID: "fail_commit", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "content": strings.Replace(generationHTML, "Original", "A", 1)}},
		{ID: "success_commit", Name: "edit_html", Args: map[string]any{"slide_id": otherBatchSlide, "content": strings.Replace(generationHTML, "Original", "B", 1)}},
	}
	runtime := NewRuntime(nil)
	names := schemasByName(registry.Disclose(PhaseExecuting, model.ModeExecute, state.scope))
	results := runtime.executeToolBatch(context.Background(), input, state, registry, names, calls)
	if results[0].Code != CodeCommitFailed || !results[1].OK || commits != 1 {
		t.Fatalf("results=%+v commits=%d", results, commits)
	}
	a, _ := os.ReadFile(filepath.Join(input.ProjectDir, model.SlideHTMLPath(generationSlide)))
	b, _ := os.ReadFile(filepath.Join(input.ProjectDir, model.SlideHTMLPath(otherBatchSlide)))
	if string(a) != generationHTML || string(b) != calls[1].Args["content"] {
		t.Fatal("rollback crossed operation boundary")
	}
	if state.seenVersions["ppt/slide:"+generationSlide+":html"] != spec.ContentHash(a) {
		t.Fatal("failed commit advanced model version")
	}
	replay := runtime.executeToolBatch(context.Background(), input, state, registry, names, calls)
	if replay[0].Code != CodeCommitFailed || !replay[1].OK || commits != 1 || replay[1].ObservationMetadata == nil {
		t.Fatal("durable replay lost result or executed another write")
	}
}

func TestAuthoringCancellationRetainsIndependentCommitAndReceipt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input, state, registry := authoringBatchFixture(t, func(ctx context.Context, input DomainToolInput) error {
		if input.CallID == "waiting" {
			<-ctx.Done()
			return ctx.Err()
		}
		return nil
	})
	receipts := newMemoryIdempotencyStore()
	input.Idempotency = receipts
	input.CommitMetadata = func(ctx context.Context, c CommitContext) error {
		if err := receipts.CompleteIdempotency(ctx, "tool_call", state.runID, c.OperationID, "completed", c.ToolResultJSON); err != nil {
			return err
		}
		cancel()
		return nil
	}
	calls := []llm.ToolCall{
		{ID: "waiting", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "content": strings.Replace(generationHTML, "Original", "A", 1)}},
		{ID: "committed", Name: "edit_html", Args: map[string]any{"slide_id": otherBatchSlide, "content": strings.Replace(generationHTML, "Original", "B", 1)}},
	}
	results := NewRuntime(nil).executeToolBatch(ctx, input, state, registry, schemasByName(registry.Disclose(PhaseExecuting, model.ModeExecute, state.scope)), calls)
	if results[0].Code != CodeCanceled || !results[1].OK {
		t.Fatalf("cancellation crossed commit boundary: %+v", results)
	}
	a, _ := os.ReadFile(filepath.Join(input.ProjectDir, model.SlideHTMLPath(generationSlide)))
	b, _ := os.ReadFile(filepath.Join(input.ProjectDir, model.SlideHTMLPath(otherBatchSlide)))
	if string(a) != generationHTML || string(b) != calls[1].Args["content"] {
		t.Fatal("cancellation discarded independent persisted HTML")
	}
	receipt, err := receipts.GetIdempotency(context.Background(), "tool_call", state.runID, "committed")
	if err != nil || receipt.Status != "completed" || receipt.ResultJSON == "" {
		t.Fatalf("committed receipt lost: %+v %v", receipt, err)
	}
}

func TestAuthoringBatchSerializesSpecCollectionWithoutPropagatingUnrelatedFailure(t *testing.T) {
	input, state, registry := authoringBatchFixture(t, nil)
	calls := []llm.ToolCall{
		{ID: "bad_spec", Name: "edit_spec", Args: map[string]any{"slide_id": generationSlide, "role": "invalid-role"}},
		{ID: "html_a", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "content": generationHTML}},
		{ID: "spec_b", Name: "edit_spec", Args: map[string]any{"slide_id": otherBatchSlide, "core": "B"}},
		{ID: "html_b", Name: "edit_html", Args: map[string]any{"slide_id": otherBatchSlide, "content": strings.Replace(generationHTML, "Original", "B", 1)}},
	}
	names := schemasByName(registry.Disclose(PhaseExecuting, model.ModeExecute, state.scope))
	results := NewRuntime(nil).executeToolBatch(context.Background(), input, state, registry, names, calls)
	if results[0].OK || results[1].Code != CodeDependencyFailed || !results[2].OK || !results[3].OK {
		t.Fatalf("results=%+v", results)
	}
	before, _ := os.ReadFile(filepath.Join(input.ProjectDir, ".outline.json"))
	outline := state.pack.Outline.Outline
	outline.Sections[0].Slides = outline.Sections[0].Slides[1:]
	after, _ := json.Marshal(outline)
	results = NewRuntime(nil).executeToolBatch(context.Background(), input, state, registry, names, []llm.ToolCall{
		{ID: "remove_a", Name: "edit_outline", Args: map[string]any{"edits": []any{map[string]any{"old_text": string(before), "new_text": string(after)}}}},
		{ID: "spec_b2", Name: "edit_spec", Args: map[string]any{"slide_id": otherBatchSlide, "core": "B2"}},
	})
	if !results[0].OK || !results[1].OK {
		t.Fatalf("delete/spec conflict: %+v", results)
	}
	entries, err := spec.ReadCollection(func(path string) ([]byte, error) { return os.ReadFile(filepath.Join(input.ProjectDir, path)) })
	if err != nil || entries[generationSlide] != nil || !strings.Contains(string(entries[otherBatchSlide]), "B2") {
		t.Fatalf("collection overwritten or page resurrected: %s %v", entries, err)
	}
	if _, err = os.Stat(filepath.Join(input.ProjectDir, model.SlideHTMLPath(generationSlide))); !os.IsNotExist(err) {
		t.Fatal("page HTML survived deletion")
	}
}

func batchState(pack contextengine.ContextPack) *RunState {
	return &RunState{
		runID: "batch", loopID: "loop-batch", phase: PhaseExecuting,
		scope: pack.Command.Scope, mode: pack.Command.Mode, pack: pack, ledger: NewEvidenceLedger(),
	}
}

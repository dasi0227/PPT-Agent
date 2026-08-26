package workflow

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type timedBatchTool struct {
	name   string
	delay  time.Duration
	mu     *sync.Mutex
	order  *[]string
	active *int
	peak   *int
}

func (t timedBatchTool) Schema() ToolSchema {
	return ToolSchema{Name: t.name, Parameters: objectSchema(nil, map[string]any{})}
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
		return failedToolResult(CodeCanceled, ctx.Err().Error(), false)
	}
	if t.mu != nil {
		t.mu.Lock()
		*t.active--
		*t.order = append(*t.order, "end:"+id)
		t.mu.Unlock()
	}
	if failed, _ := input.Args["fail"].(bool); failed {
		return failedToolResult(CodeContentInvalid, "requested failure", false)
	}
	return SuccessfulToolResult("ok")
}

func TestIndependentReadBatchRunsWithBoundedConcurrencyAndPairedEvents(t *testing.T) {
	dir, pack := t.TempDir(), testPack(model.ModeExecute, model.ArtifactPPT, model.ScopeDeck, true, "batch")
	registry := NewToolRegistry()
	var mu sync.Mutex
	order := []string{}
	active, peak := 0, 0
	for _, name := range []string{"read_ppt", "search_refs"} {
		if err := registry.Register(
			timedBatchTool{name: name, delay: 80 * time.Millisecond, mu: &mu, order: &order, active: &active, peak: &peak},
			true, "ppt.read", RiskLow, PhaseExecuting,
		); err != nil {
			t.Fatal(err)
		}
	}
	events := &eventRecorder{}
	state := batchState(pack)
	calls := []llm.ToolCall{
		{ID: "r1", Name: "read_ppt", Args: map[string]any{"id": "one"}},
		{ID: "r2", Name: "search_refs", Args: map[string]any{"id": "two"}},
	}
	started := time.Now()
	results := NewRuntime(nil).executeToolBatch(context.Background(), RuntimeInput{
		RunID: "batch-read", ProjectDir: dir, Context: pack, Emitter: events,
	}, state, registry, map[string]bool{"read_ppt": true, "search_refs": true}, calls)
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

func TestWriteBatchIsOrderedAndFailsFast(t *testing.T) {
	dir, pack := t.TempDir(), testPack(model.ModeExecute, model.ArtifactSpec, model.ScopeDeck, true, "batch")
	tx, err := NewRunSession(dir, "batch-write")
	if err != nil {
		t.Fatal(err)
	}
	registry := NewToolRegistry()
	var mu sync.Mutex
	order := []string{}
	active, peak := 0, 0
	if err := registry.Register(
		timedBatchTool{name: "mutate_ppt", delay: time.Millisecond, mu: &mu, order: &order, active: &active, peak: &peak},
		false, "ppt.write", RiskMedium, PhaseExecuting,
	); err != nil {
		t.Fatal(err)
	}
	events := &eventRecorder{}
	state := batchState(pack)
	state.tx = tx
	calls := []llm.ToolCall{
		{ID: "w1", Name: "mutate_ppt", Args: map[string]any{"id": "one"}},
		{ID: "w2", Name: "mutate_ppt", Args: map[string]any{"id": "two", "fail": true}},
		{ID: "w3", Name: "mutate_ppt", Args: map[string]any{"id": "three"}},
	}
	results := NewRuntime(nil).executeToolBatch(context.Background(), RuntimeInput{
		RunID: "batch-write", ProjectDir: dir, Context: pack, Emitter: events,
	}, state, registry, map[string]bool{"mutate_ppt": true}, calls)
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
		t.Fatalf("writes were not serial/fail-fast: peak=%d results=%+v", peak, results)
	}
	if events.count(model.EventToolStarted) != 2 || events.count(model.EventToolCompleted) != 2 {
		t.Fatalf("dependency-skipped call should not emit public tool events: %+v", events.events)
	}
}

func TestFailFastSkipsDoNotExhaustRepairBudget(t *testing.T) {
	state := &RunState{}
	recordToolFailures(state, []ToolResult{
		failedToolResult(CodeContentInvalid, "invalid spec", true),
		failedToolResult(CodeDependencyFailed, "skipped", false),
		failedToolResult(CodeDependencyFailed, "skipped", false),
	})
	if state.toolFailures != 1 {
		t.Fatalf("dependency skips counted as failures: %d", state.toolFailures)
	}
	recordToolFailures(state, []ToolResult{
		failedToolResult(CodeRenderFailed, "slide 1", true),
		failedToolResult(CodeRenderFailed, "slide 2", true),
		failedToolResult(CodeRenderFailed, "slide 3", true),
	})
	if state.toolFailures != 2 {
		t.Fatalf("one failed multi-render response was not one repair round: %d", state.toolFailures)
	}
	recordToolFailures(state, []ToolResult{SuccessfulToolResult("repaired")})
	if state.toolFailures != 0 {
		t.Fatalf("successful repair did not reset failure count: %d", state.toolFailures)
	}
}

func batchState(pack contextengine.ContextPack) *RunState {
	return &RunState{
		runID: "batch", loopID: "loop-batch", phase: PhaseExecuting,
		scope: pack.Command.Scope, mode: pack.Command.Mode, pack: pack, ledger: NewEvidenceLedger(),
	}
}

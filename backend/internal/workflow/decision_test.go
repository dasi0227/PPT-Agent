package workflow

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/decision"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type decisionFunc func(context.Context, decision.Request) (decision.Response, error)

func (f decisionFunc) Evaluate(ctx context.Context, req decision.Request) (decision.Response, error) {
	return f(ctx, req)
}

func TestToolDecisionIsConservativeAndSurvivesRegistryRebuild(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	calls := 0
	provider := decisionFunc(func(_ context.Context, req decision.Request) (decision.Response, error) {
		calls++
		if len(req.Questions) != 4 || req.Questions["commit"] == nil {
			t.Fatal("capabilities were not batched")
		}
		return decision.Response{Model: "jev-test", Answers: map[string]decision.Answer{
			"content": decision.ChoiceAnswer{Type: "choice", Choice: "not_needed", Confidence: 1, Probabilities: map[string]float64{"not_needed": 1}},
			"visual":  decision.ChoiceAnswer{Type: "choice", Choice: "possible", Confidence: 1, Probabilities: map[string]float64{"possible": 1}},
			"outline": decision.ChoiceAnswer{Type: "choice", Choice: "not_needed", Confidence: 0.3, Probabilities: map[string]float64{"not_needed": 0.6}},
			"commit":  decision.ChoiceAnswer{Type: "choice", Choice: "not_needed", Confidence: 1, Probabilities: map[string]float64{"not_needed": 1}},
		}}, nil
	})
	runtime := NewRuntime(nil)
	runtime.Decisions = decision.Snapshot{Identity: "one", Provider: provider}
	input := RuntimeInput{ProjectDir: dir, Context: pack, DomainTools: DefaultDomainToolProvider{Pack: pack, GitCommit: func(context.Context, string, map[string]any) (map[string]any, error) {
		t.Fatal("pruned git_commit must not execute")
		return nil, nil
	}}}
	state := &RunState{pack: pack, mode: model.ModeExecute, phase: PhaseExecuting, scope: pack.Command.Scope, decisionIdentity: "one"}
	state.tools, _ = buildDomainToolRegistry(input, pack)
	if !schemasByName(state.tools.Disclose(state.phase, state.mode, state.scope))["git_commit"] {
		t.Fatal("git_commit must be available before pruning")
	}
	if err := runtime.decideTools(context.Background(), input, state); err != nil {
		t.Fatal(err)
	}
	names := schemasByName(state.tools.Disclose(state.phase, state.mode, state.scope))
	if names["edit_manifest"] || names["git_commit"] || !names["edit_design"] || !names["edit_outline"] || !names["edit_spec"] {
		t.Fatalf("incorrect pruning: %v", names)
	}
	state.tools, _ = buildDomainToolRegistry(input, pack)
	runtime.Decisions = decision.Snapshot{Identity: "changed", Provider: provider}
	if err := runtime.decideTools(context.Background(), input, state); err != nil {
		t.Fatal(err)
	}
	names = schemasByName(state.tools.Disclose(state.phase, state.mode, state.scope))
	if calls != 1 || names["edit_manifest"] || names["git_commit"] {
		t.Fatal("same run reclassified or reopened tools")
	}
	for _, name := range []string{"edit_manifest", "git_commit"} {
		result := state.tools.Execute(context.Background(), map[string]bool{name: true}, name, nil, DomainToolInput{Context: pack, Scope: state.scope, Mode: state.mode, Phase: state.phase})
		if result.OK || result.Code != ErrToolNotDisclosed.Error() {
			t.Fatalf("old disclosed name %s bypassed frozen list", name)
		}
	}
}

func TestContentPrecheckUsesFinalSnapshotAndReusesSavedScores(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	before := captureContentBefore(dir)
	writeGenerationFile(t, dir, model.SlideHTMLPath(generationSlide), []byte(strings.Replace(generationHTML, "Original", "Final content", 1)))
	count := 0
	provider := decisionFunc(func(_ context.Context, req decision.Request) (decision.Response, error) {
		count++
		if len(req.Questions) != 3 {
			t.Fatal("final page dimensions not batched")
		}
		out := decision.Response{Model: "jev-test", Answers: map[string]decision.Answer{}}
		for id, q := range req.Questions {
			question := q.(decision.ScoreQuestion)
			page := question.Instructions.(map[string]any)["page"].(map[string]any)
			if !strings.Contains(page["html_content"].(string), "Final content") || !strings.Contains(page["before_content"].(string), "Original") {
				t.Fatal("wrong version evaluated")
			}
			out.Answers[id] = decision.ScoreAnswer{Type: "score", Score: 2, Confidence: 1, Legend: map[string]any{"0": "absent", "1": "incomplete", "2": "mostly", "3": "complete"}, Probabilities: map[string]float64{"0": 0, "1": 0, "2": 1, "3": 0}}
		}
		return out, nil
	})
	runtime := NewRuntime(nil)
	runtime.Decisions = decision.Snapshot{Identity: "one", Provider: provider}
	state := &RunState{runID: "precheck-run", projectDir: dir, pack: pack, decisionIdentity: "one", reviewInstructions: []ReviewInstruction{{Text: "Preserve the message"}}, pendingContent: &PendingContentBatch{Before: before}}
	calls := []llm.ToolCall{{ID: "first", Name: "edit_html"}, {ID: "last", Name: "edit_html"}}
	results := []ToolResult{SuccessfulToolResult("saved"), SuccessfulToolResult("saved")}
	input := RuntimeInput{ProjectDir: dir}
	runtime.contentPrecheck(context.Background(), input, state, calls, results, map[string]int{generationSlide: 1})
	if count != 1 || len(results[0].ContentPrecheck) != 0 || len(results[1].ContentPrecheck) != 1 || results[1].ContentPrecheck[0].Status != "completed" {
		t.Fatalf("wrong attachment: %+v", results)
	}
	observations := appendBatchObservations(nil, calls, "", results)
	if got := observations[len(observations)-1]; len(got.Content) != 1 || got.Text() != `{"summary":"saved"}` {
		t.Fatalf("internal content assessment leaked into summary-only tool response: %+v", got)
	}
	repeated := []ToolResult{SuccessfulToolResult("saved"), SuccessfulToolResult("saved")}
	runtime.contentPrecheck(context.Background(), input, state, calls, repeated, map[string]int{generationSlide: 1})
	if count != 1 {
		t.Fatal("saved score requested twice")
	}
	state.reviewInstructions = append(state.reviewInstructions, ReviewInstruction{Text: "Change the content requirement"})
	runtime.invalidateContentPrechecks(input, state)
	record, err := loadContentRecord(dir, state.runID, results[1].ContentPrecheck[0].AssessmentID)
	if err != nil || record.Result.Status != "stale" {
		t.Fatal("changed requirements reused old score")
	}
}

func TestPrecheckFailureDoesNotChangeSuccessfulWrite(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	before := captureContentBefore(dir)
	writeGenerationFile(t, dir, model.SlideHTMLPath(generationSlide), []byte(strings.Replace(generationHTML, "Original", "Changed", 1)))
	runtime := NewRuntime(nil)
	runtime.Decisions = decision.Snapshot{Identity: "one", Provider: decisionFunc(func(context.Context, decision.Request) (decision.Response, error) {
		return decision.Response{}, errors.New("offline")
	})}
	state := &RunState{runID: "failed-assessment", projectDir: dir, pack: pack, decisionIdentity: "one", pendingContent: &PendingContentBatch{Before: before}}
	results := []ToolResult{SuccessfulToolResult("saved")}
	runtime.contentPrecheck(context.Background(), RuntimeInput{ProjectDir: dir}, state, []llm.ToolCall{{ID: "write", Name: "edit_html"}}, results, map[string]int{generationSlide: 0})
	if !results[0].OK || results[0].Code != "" || len(results[0].ContentPrecheck) != 1 || results[0].ContentPrecheck[0].Status != "unavailable" {
		t.Fatal("assessment failure changed write success")
	}
}

type precheckReceiptStore struct {
	records map[string]model.IdempotencyRecord
}

func (s precheckReceiptStore) AcquireIdempotency(context.Context, model.IdempotencyRecord) (model.IdempotencyRecord, bool, error) {
	panic("recovery must not acquire or replay a tool")
}
func (s precheckReceiptStore) CompleteIdempotency(context.Context, string, string, string, string, string) error {
	panic("recovery must not rewrite receipts")
}
func (s precheckReceiptStore) GetIdempotency(_ context.Context, scope, owner, key string) (model.IdempotencyRecord, error) {
	v, ok := s.records[scope+":"+key]
	if !ok {
		return v, errors.New("missing")
	}
	return v, nil
}

func TestPendingPrecheckRecoversReceiptsWithoutReplayingEdits(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	before := captureContentBefore(dir)
	final := []byte(strings.Replace(generationHTML, "Original", "Committed", 1))
	writeGenerationFile(t, dir, model.SlideHTMLPath(generationSlide), final)
	saved := SuccessfulToolResult("saved")
	saved.ChangedTargets = []ChangedTarget{{Type: "slide", Part: "html", SlideID: generationSlide, Hash: hashBytes(final)}}
	raw, err := marshalPersistedToolResult(saved)
	if err != nil {
		t.Fatal(err)
	}
	receipts := precheckReceiptStore{records: map[string]model.IdempotencyRecord{
		"tool_call:write": {Status: "completed", ResultJSON: raw}, "artifact_commit:write": {Status: "completed"},
	}}
	state := &RunState{runID: "recover-check", projectDir: dir, pack: pack, decisionIdentity: "old", pendingContent: &PendingContentBatch{Calls: []llm.ToolCall{{ID: "write", Name: "edit_html"}, {ID: "unstarted", Name: "edit_html"}}, Before: before}}
	runtime := NewRuntime(nil) // Original decision credentials cannot be reconstructed.
	if err := runtime.resumeContentBatch(context.Background(), RuntimeInput{ProjectDir: dir, Idempotency: receipts}, state); err != nil {
		t.Fatal(err)
	}
	if state.pendingContent != nil || len(state.messages) != 3 {
		t.Fatal("recovered batch was not observed")
	}
	if state.messages[1].Text() != `{"summary":"saved"}` || !strings.Contains(state.messages[2].Text(), "interrupted") {
		t.Fatal("recovery changed the saved tool reply or replayed an unstarted call")
	}
	current, err := readPrecheckFile(filepath.Join(dir, model.SlideHTMLPath(generationSlide)))
	if err != nil || string(current) != string(final) {
		t.Fatal("assessment recovery changed the saved artifact")
	}
}

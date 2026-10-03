package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/decision"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
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
	before := captureHTMLHashes(dir)
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
			if !strings.Contains(page["html_content"].(string), "Final content") {
				t.Fatal("wrong version evaluated")
			}
			for _, removed := range []string{"before_content", "run_start_content", "run_start_content_status"} {
				if _, exists := page[removed]; exists {
					t.Fatalf("historical content sent to scorer: %s", removed)
				}
			}
			out.Answers[id] = decision.ScoreAnswer{Type: "score", Score: 2, Confidence: 1, Legend: map[string]any{"0": "absent", "1": "incomplete", "2": "mostly", "3": "complete"}, Probabilities: map[string]float64{"0": 0, "1": 0, "2": 1, "3": 0}}
		}
		return out, nil
	})
	runtime := NewRuntime(nil)
	runtime.Decisions = decision.Snapshot{Identity: "one", Provider: provider}
	state := &RunState{runID: "precheck-run", projectDir: dir, pack: pack, decisionIdentity: "one", reviewInstructions: []ReviewInstruction{{Text: "Preserve the message"}}, pendingContent: &PendingContentBatch{BeforeHTMLHashes: before}}
	calls := []llm.ToolCall{{ID: "first", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide}}, {ID: "last", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide}}}
	results := []ToolResult{SuccessfulToolResult("saved"), SuccessfulToolResult("saved")}
	input := RuntimeInput{ProjectDir: dir}
	runtime.contentPrecheck(context.Background(), input, state, calls, results, map[string]int{generationSlide: 1})
	if count != 1 || len(results[0].ContentPrecheck) != 0 || len(results[1].ContentPrecheck) != 1 || results[1].ContentPrecheck[0].Status != "completed" {
		t.Fatalf("wrong attachment: %+v", results)
	}
	observations := appendBatchObservations(nil, calls, "", results)
	if observations[1].Text() != `{"summary":"saved"}` {
		t.Fatal("earlier edit was given the final page assessment")
	}
	got := observations[len(observations)-1]
	assessment := assertPrecheckObservation(t, got, "completed")
	if got.ToolCallID != "last" || len(assessment["scores"].(map[string]any)) != 3 {
		t.Fatal("final scores lost their tool call association")
	}
	if assessment["slide_id"] != nil {
		t.Fatal("single-page reply repeated its page identity")
	}
	if err := model.ValidatePublicEvent(model.EventContentPrechecked, model.ContentPrecheckedPayload{
		PublicEventBase: model.NewPublicEventBase(state.runID), CallID: "last", ContentPrecheck: results[1].ContentPrecheck,
	}); err != nil {
		t.Fatalf("new score dimensions were rejected by public events: %v", err)
	}
	persisted, err := loadContentRecord(dir, state.runID, results[1].ContentPrecheck[0].AssessmentID)
	if err != nil || persisted.Result.Scores[model.ContentCoverage].Confidence != 1 || len(persisted.Result.Scores[model.ContentCoverage].Probabilities) != 4 {
		t.Fatal("compact model reply discarded the durable distribution")
	}
	repeated := []ToolResult{SuccessfulToolResult("saved"), SuccessfulToolResult("saved")}
	runtime.contentPrecheck(context.Background(), input, state, calls, repeated, map[string]int{generationSlide: 1})
	if count != 1 {
		t.Fatal("saved score requested twice")
	}
	state.messages = observations
	state.reviewInstructions = append(state.reviewInstructions, ReviewInstruction{Text: "Change the content requirement"})
	runtime.invalidateContentPrechecks(input, state)
	record, err := loadContentRecord(dir, state.runID, results[1].ContentPrecheck[0].AssessmentID)
	if err != nil || record.Result.Status != "stale" {
		t.Fatal("changed requirements reused old score")
	}
	notice := assertPrecheckObservation(t, state.messages[len(state.messages)-1], "stale")
	if notice["slide_id"] != generationSlide || notice["tool_call_id"] != "last" || notice["scores"] != nil {
		t.Fatal("standalone invalidation lost the page or original call identity")
	}
}

func TestPrecheckFailureDoesNotChangeSuccessfulWrite(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	before := captureHTMLHashes(dir)
	writeGenerationFile(t, dir, model.SlideHTMLPath(generationSlide), []byte(strings.Replace(generationHTML, "Original", "Changed", 1)))
	runtime := NewRuntime(nil)
	runtime.Decisions = decision.Snapshot{Identity: "one", Provider: decisionFunc(func(context.Context, decision.Request) (decision.Response, error) {
		return decision.Response{}, errors.New("offline")
	})}
	state := &RunState{runID: "failed-assessment", projectDir: dir, pack: pack, decisionIdentity: "one", pendingContent: &PendingContentBatch{BeforeHTMLHashes: before}}
	results := []ToolResult{SuccessfulToolResult("saved")}
	runtime.contentPrecheck(context.Background(), RuntimeInput{ProjectDir: dir}, state, []llm.ToolCall{{ID: "write", Name: "edit_html"}}, results, map[string]int{generationSlide: 0})
	if !results[0].OK || results[0].Code != "" || len(results[0].ContentPrecheck) != 1 || results[0].ContentPrecheck[0].Status != "unavailable" {
		t.Fatal("assessment failure changed write success")
	}
	observation := appendBatchObservations(nil, []llm.ToolCall{{ID: "write", Name: "edit_html"}}, "", results)[1]
	assessment := assertPrecheckObservation(t, observation, "unavailable")
	if assessment["scores"] != nil || assessment["reason"] != "unavailable" {
		t.Fatal("failed precheck exposed scores or lost its reason")
	}
}

func TestContentPrecheckBatchesPagesAndRefreshesCachedCommandReply(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	const other = "sli_bbbbbb"
	pack.Outline.Outline.Sections[0].Slides = append(pack.Outline.Outline.Sections[0].Slides, spec.SlideNode{SlideID: other, Title: "Other"})
	for path, value := range map[string]any{
		".outline.json":          pack.Outline.Outline,
		model.SpecCollectionPath: map[string]spec.SlideSpec{generationSlide: pack.GenerationInputs[generationSlide].Spec, other: pack.GenerationInputs[generationSlide].Spec},
	} {
		raw, _ := json.Marshal(value)
		writeGenerationFile(t, dir, path, raw)
	}
	before := captureHTMLHashes(dir)
	writeGenerationFile(t, dir, model.SlideHTMLPath(generationSlide), []byte(strings.Replace(generationHTML, "Original", "First final", 1)))
	writeGenerationFile(t, dir, model.SlideHTMLPath(other), []byte(strings.Replace(generationHTML, "Original", "Other final", 1)))
	requests := 0
	runtime := NewRuntime(nil)
	runtime.Decisions = decision.Snapshot{Identity: "one", Provider: decisionFunc(func(_ context.Context, req decision.Request) (decision.Response, error) {
		requests++
		if len(req.Questions) != 6 {
			t.Fatal("page dimensions were not combined into one request")
		}
		response := decision.Response{Model: "jev-test", Answers: map[string]decision.Answer{}}
		for id, q := range req.Questions {
			page := q.(decision.ScoreQuestion).Instructions.(map[string]any)["page"].(map[string]any)
			score := float64(2)
			if page["slide_id"] == other {
				score = 3
			}
			response.Answers[id] = decision.ScoreAnswer{Type: "score", Score: score, Confidence: 1, Legend: map[string]any{"0": "absent", "1": "incomplete", "2": "mostly", "3": "complete"}}
		}
		return response, nil
	})}
	state := &RunState{runID: "batched-precheck", projectDir: dir, pack: pack, decisionIdentity: "one", pendingContent: &PendingContentBatch{BeforeHTMLHashes: before}}
	calls := []llm.ToolCall{{ID: "command", Name: "run_command"}, {ID: "html", Name: "edit_html", Args: map[string]any{"slide_id": other}}}
	command := SuccessfulToolResult("command completed")
	command.Data = map[string]any{"stdout": "unchanged output", "stderr": "", "exit_code": 0}
	command.Observation = modelToolObservation(command)
	results := []ToolResult{command, SuccessfulToolResult("HTML 已保存。")}
	runtime.contentPrecheck(context.Background(), RuntimeInput{ProjectDir: dir}, state, calls, results, map[string]int{generationSlide: 0, other: 1})
	if requests != 1 {
		t.Fatalf("expected one scoring request, got %d", requests)
	}
	observations := appendBatchObservations(nil, calls, "", results)
	for i, expected := range []float64{2, 3} {
		assessment := assertPrecheckObservation(t, observations[i+1], "completed")
		if observations[i+1].ToolCallID != calls[i].ID || assessment["scores"].(map[string]any)[model.ContentCoverage] != expected {
			t.Fatal("scores were assigned to the wrong page or tool call")
		}
	}
	if !strings.Contains(observations[1].Text(), `"stdout":"unchanged output"`) || command.Data["content_precheck"] != nil {
		t.Fatal("adding the assessment replaced command output or mutated the original data")
	}
}

func assertPrecheckObservation(t *testing.T, message llm.Message, status string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(message.Text()), &value); err != nil {
		t.Fatal(err)
	}
	assessment, ok := value["content_precheck"].(map[string]any)
	if !ok || assessment["status"] != status {
		t.Fatalf("expected single %s assessment: %s", status, message.Text())
	}
	for _, forbidden := range []string{"assessment_id", "content_hash", "material_hash", "max_score", "confidence", "probabilities", "legend", "rubric"} {
		if strings.Contains(message.Text(), `"`+forbidden+`"`) {
			t.Fatalf("assessment metadata exposed to model: %s", forbidden)
		}
	}
	return assessment
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
	before := captureHTMLHashes(dir)
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
	state := &RunState{runID: "recover-check", projectDir: dir, pack: pack, decisionIdentity: "old", pendingContent: &PendingContentBatch{Calls: []llm.ToolCall{{ID: "write", Name: "edit_html"}, {ID: "unstarted", Name: "edit_html"}}, BeforeHTMLHashes: before}}
	runtime := NewRuntime(nil) // Original decision credentials cannot be reconstructed.
	if err := runtime.resumeContentBatch(context.Background(), RuntimeInput{ProjectDir: dir, Idempotency: receipts}, state); err != nil {
		t.Fatal(err)
	}
	if state.pendingContent != nil || len(state.messages) != 3 {
		t.Fatal("recovered batch was not observed")
	}
	assessment := assertPrecheckObservation(t, state.messages[1], "unavailable")
	if assessment["reason"] != "configuration_changed" || !strings.Contains(state.messages[2].Text(), "interrupted") {
		t.Fatal("recovery lost the assessment reason or replayed an unstarted call")
	}
	current, err := readPrecheckFile(filepath.Join(dir, model.SlideHTMLPath(generationSlide)))
	if err != nil || string(current) != string(final) {
		t.Fatal("assessment recovery changed the saved artifact")
	}
}

package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

// Workflow tests substitute only the durable interaction boundary. The actual
// persistence contract belongs to the SQLite store, never to temporary files.
type resourceApprovalFixture struct {
	mu   sync.Mutex
	rows map[string]model.ResourceEditApproval
	next CheckpointSink
}

func withResourceApprovals(input RuntimeInput) RuntimeInput {
	f := &resourceApprovalFixture{rows: map[string]model.ResourceEditApproval{}, next: input.Checkpoint}
	input.Checkpoint, input.ResourceApproval = f, f.read
	input.Prompter = autoApprovingResourcePrompter{approvals: f}
	return input
}

func (f *resourceApprovalFixture) SaveCheckpoint(ctx context.Context, cp RuntimeCheckpoint) error {
	if pending := cp.PendingResourceApproval; pending != nil && pending.Candidate != nil {
		f.mu.Lock()
		if _, exists := f.rows[pending.InteractionID]; !exists {
			f.rows[pending.InteractionID] = model.ResourceEditApproval{ResourceEditCandidate: *pending.Candidate,
				RunID: cp.RunID, CallID: pending.CallID, InteractionID: pending.InteractionID,
				Resource: pending.Request.Resource, Revision: pending.Request.Revision, Target: pending.Request.Target, State: "pending"}
		}
		f.mu.Unlock()
	}
	if f.next != nil {
		return f.next.SaveCheckpoint(ctx, cp)
	}
	return nil
}

func (f *resourceApprovalFixture) read(_ context.Context, runID, id string) (model.ResourceEditApproval, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	record, ok := f.rows[id]
	if !ok || record.RunID != runID {
		return model.ResourceEditApproval{}, errors.New("approval not saved with checkpoint")
	}
	return record, nil
}

func (f *resourceApprovalFixture) put(record model.ResourceEditApproval) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[record.InteractionID] = record
}

func (f *resourceApprovalFixture) accept(answer model.ResourceEditApprovalAnswer) (model.ResourceEditApprovalAnswer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	record, ok := f.rows[answer.InteractionID]
	if !ok || record.CallID != answer.CallID || record.Revision != answer.Revision {
		return answer, ErrResourceApprovalConflict
	}
	answer.Feedback = strings.TrimSpace(answer.Feedback)
	record.State, record.Answer = "answered", &answer
	f.rows[answer.InteractionID] = record
	return answer, nil
}

type waitingResourcePrompter struct {
	approvals *resourceApprovalFixture
	requested chan model.ResourceEditApprovalRequestedPayload
	answered  chan model.ResourceEditApprovalAnswer
}

func (waitingResourcePrompter) Ask(context.Context, model.QuestionAskedPayload) (model.QuestionAnswer, error) {
	return model.QuestionAnswer{}, nil
}
func (p waitingResourcePrompter) AskResourceEditApproval(ctx context.Context, payload model.ResourceEditApprovalRequestedPayload) (model.ResourceEditApprovalAnswer, error) {
	p.requested <- payload
	select {
	case answer := <-p.answered:
		return p.approvals.accept(answer)
	case <-ctx.Done():
		return model.ResourceEditApprovalAnswer{}, ctx.Err()
	}
}

func TestResourceApprovalBlocksCommitAndPublishesEditedDraft(t *testing.T) {
	input, state, registry := authoringBatchFixture(t, nil)
	input.Idempotency = newMemoryIdempotencyStore()
	before, err := os.ReadFile(filepath.Join(input.ProjectDir, ".constraints.json"))
	if err != nil {
		t.Fatal(err)
	}
	prompter := waitingResourcePrompter{approvals: input.Checkpoint.(*resourceApprovalFixture), requested: make(chan model.ResourceEditApprovalRequestedPayload, 1), answered: make(chan model.ResourceEditApprovalAnswer, 1)}
	input.Prompter = prompter
	call := llm.ToolCall{ID: "edit_for_approval", Name: "edit_constraints", Args: map[string]any{"type": "visual", "requirements": []any{"Agent suggestion"}}}
	finished := make(chan []ToolResult, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var approvalRequired atomic.Bool
	approvalRequired.Store(true)
	runtime := newTestRuntime(nil)
	runtime.ResourceApprovalRequired = func() (bool, error) { return approvalRequired.Load(), nil }
	go func() {
		finished <- runtime.executeToolBatch(ctx, input, state, registry,
			schemasByName(registry.Disclose(PhaseExecuting, model.ModeCreate, state.scope)), []llm.ToolCall{call})
	}()
	var requested model.ResourceEditApprovalRequestedPayload
	select {
	case requested = <-prompter.requested:
	case <-ctx.Done():
		t.Fatal("approval was not requested")
	}
	approvalRequired.Store(false)
	select {
	case <-finished:
		t.Fatal("disabling approval released an already pending interaction")
	default:
	}
	actual, err := os.ReadFile(filepath.Join(input.ProjectDir, ".constraints.json"))
	if err != nil || string(actual) != string(before) {
		t.Fatalf("candidate was written before approval: %q %v", actual, err)
	}
	record, err := input.ResourceApproval(ctx, state.runID, requested.InteractionID)
	if err != nil {
		t.Fatal(err)
	}
	var userDraft spec.Constraints
	if err := json.Unmarshal(record.Draft, &userDraft); err != nil {
		t.Fatal(err)
	}
	userDraft.Visual.Requirements = []string{"User choice"}
	userDraft.Content.Prohibitions = []string{"No unsupported claims"}
	draft, _ := json.Marshal(userDraft)
	record, err = PrepareResourceEditDraft(input.ProjectDir, record, record.Revision, draft)
	if err != nil {
		t.Fatal(err)
	}
	prompter.approvals.put(record)
	answer := model.ResourceEditApprovalAnswer{InteractionID: requested.InteractionID, CallID: call.ID, Revision: record.Revision, Decision: "approve"}
	prompter.answered <- answer
	var results []ToolResult
	select {
	case results = <-finished:
	case <-ctx.Done():
		t.Fatal("tool did not finish after approval")
	}
	if len(results) != 1 || !results[0].OK || len(results[0].OperationTargets) == 0 {
		t.Fatalf("approved tool result: %+v", results)
	}
	diff, _ := json.Marshal(results[0].OperationTargets)
	if !strings.Contains(string(diff), "User choice") || strings.Contains(string(diff), "Agent suggestion") {
		t.Fatalf("final diff does not reflect edited draft: %s", diff)
	}
	actual, err = os.ReadFile(filepath.Join(input.ProjectDir, ".constraints.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved spec.Constraints
	if err := json.Unmarshal(actual, &saved); err != nil || len(saved.Visual.Requirements) != 1 || saved.Visual.Requirements[0] != "User choice" || len(saved.Content.Prohibitions) != 1 || saved.Content.Prohibitions[0] != "No unsupported claims" {
		t.Fatalf("final draft was not committed: %+v %v", saved, err)
	}
	assertSavedResourceObservation(t, call.Name, results[0], actual)
	replayed, execute := runtime.acquireDispatchedToolCall(ctx, input, state, call, true)
	if execute || !reflect.DeepEqual(replayed.ChangedTargets, results[0].ChangedTargets) || !reflect.DeepEqual(replayed.ObservationMetadata, results[0].ObservationMetadata) {
		t.Fatalf("approved replay lost changes or source versions: %+v", replayed)
	}
	assertSavedResourceObservation(t, call.Name, replayed, actual)
	approvedMessages := appendBatchObservations(nil, []llm.ToolCall{call}, "", []ToolResult{replayed})
	if visibleResourceHashes(approvedMessages)["ppt/deck:constraints"] != resourceContentHash(Resource{Part: "constraints"}, actual) {
		t.Fatal("user-revised approval did not supply its complete final version")
	}
	input.Prompter = nil
	next := llm.ToolCall{ID: "edit_after_toggle", Name: "edit_meta", Args: map[string]any{"goal": "Directly saved goal"}}
	results = runtime.executeToolBatch(ctx, input, state, registry,
		schemasByName(registry.Disclose(PhaseExecuting, model.ModeCreate, state.scope)), []llm.ToolCall{next})
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("later call still required approval: %+v", results)
	}
	raw, err := os.ReadFile(filepath.Join(input.ProjectDir, ".meta.json"))
	if err != nil || !strings.Contains(string(raw), "Directly saved goal") {
		t.Fatalf("direct mutation was not committed: %q %v", raw, err)
	}
	assertSavedResourceObservation(t, next.Name, results[0], raw)
}

func TestResourceApprovalRejectionLeavesSourceUntouched(t *testing.T) {
	input, state, registry := authoringBatchFixture(t, nil)
	input.Idempotency = newMemoryIdempotencyStore()
	state.toolFailures = 2
	htmlPath := filepath.Join(input.ProjectDir, model.SlideHTMLPath(generationSlide))
	htmlBefore, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(input.ProjectDir, ".meta.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	prompter := waitingResourcePrompter{approvals: input.Checkpoint.(*resourceApprovalFixture), requested: make(chan model.ResourceEditApprovalRequestedPayload, 1), answered: make(chan model.ResourceEditApprovalAnswer, 1)}
	input.Prompter = prompter
	call := llm.ToolCall{ID: "edit_to_reject", Name: "edit_meta", Args: map[string]any{"goal": "Agent goal"}}
	dependent := llm.ToolCall{ID: "dependent_html", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide, "content": strings.Replace(generationHTML, "Original", "Changed", 1)}}
	finished := make(chan []ToolResult, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		finished <- newTestRuntime(nil).executeToolBatch(ctx, input, state, registry,
			schemasByName(registry.Disclose(PhaseExecuting, model.ModeCreate, state.scope)), []llm.ToolCall{call, dependent})
	}()
	var requested model.ResourceEditApprovalRequestedPayload
	select {
	case requested = <-prompter.requested:
	case <-ctx.Done():
		t.Fatal("approval was not requested")
	}
	answer := model.ResourceEditApprovalAnswer{InteractionID: requested.InteractionID, CallID: call.ID, Revision: requested.Revision, Decision: "reject", Feedback: "  保留原目标\n不要改写  "}
	prompter.answered <- answer
	select {
	case results := <-finished:
		if len(results) != 2 || !results[0].OK || results[0].Applied || results[0].Code != "" || results[1].Code != CodeDependencyFailed || results[1].Applied {
			t.Fatalf("rejected tool result: %+v", results)
		}
		var observation map[string]any
		if err := json.Unmarshal([]byte(modelToolObservation(results[0])), &observation); err != nil || !reflect.DeepEqual(observation, map[string]any{"status": "refuse", "feedback": strings.TrimSpace(answer.Feedback)}) {
			t.Fatalf("rejection feedback missing: %v %v", observation, err)
		}
		projector := ToolPublicProjector{ProjectDir: input.ProjectDir}
		event, ok := projector.Completed(state.runID, call.ID, call.Name, call.Args, results[0])
		if !ok || event.Status != "refuse" || event.Changes != nil || event.Error != nil {
			t.Fatalf("refused edit projected as a committed change: %+v", event)
		}
		if err := model.ValidatePublicEvent(model.EventToolCompleted, event); err != nil {
			t.Fatalf("refused edit event cannot be consumed: %v", err)
		}

		recordToolFailures(state, results)
		if state.toolFailures != 2 || len(results[0].ChangedTargets) != 0 || len(results[0].Evidence) != 0 || len(results[0].InvalidatedTargets) != 0 {
			t.Fatalf("refusal counted as a failure or committed effect: %+v", results[0])
		}
		// Replaying the refused call must neither prompt again nor authorize the dependency.
		replay := recordedToolResults(t, state, []llm.ToolCall{call, dependent})
		if !replay[0].OK || replay[0].Applied || replay[1].Code != CodeDependencyFailed || replay[1].Applied {
			t.Fatalf("replay lost the refused effect: %+v", replay)
		}
		messages := appendBatchObservations(nil, []llm.ToolCall{call}, "", replay[:1])
		var replayObservation map[string]any
		if err := json.Unmarshal([]byte(messages[len(messages)-1].Text()), &replayObservation); err != nil || !reflect.DeepEqual(replayObservation, observation) {
			t.Fatalf("replay changed the refusal observation: %v (%v)", replayObservation, err)
		}
	case <-ctx.Done():
		t.Fatal("tool did not finish after rejection")
	}
	htmlAfter, err := os.ReadFile(htmlPath)
	if err != nil || string(htmlAfter) != string(htmlBefore) {
		t.Fatalf("dependent HTML was applied after refusal: %q %v", htmlAfter, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("rejected edit changed source: %q %v", after, err)
	}
}

func TestResourceApprovalChangedSourceFailsWithoutOverwriting(t *testing.T) {
	input, state, registry := authoringBatchFixture(t, nil)
	path := filepath.Join(input.ProjectDir, ".meta.json")
	prompter := waitingResourcePrompter{approvals: input.Checkpoint.(*resourceApprovalFixture), requested: make(chan model.ResourceEditApprovalRequestedPayload, 1), answered: make(chan model.ResourceEditApprovalAnswer, 1)}
	input.Prompter = prompter
	call := llm.ToolCall{ID: "edit_with_conflict", Name: "edit_meta", Args: map[string]any{"goal": "Agent goal"}}
	finished := make(chan []ToolResult, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		finished <- newTestRuntime(nil).executeToolBatch(ctx, input, state, registry,
			schemasByName(registry.Disclose(PhaseExecuting, model.ModeCreate, state.scope)), []llm.ToolCall{call})
	}()
	var requested model.ResourceEditApprovalRequestedPayload
	select {
	case requested = <-prompter.requested:
	case <-ctx.Done():
		t.Fatal("approval was not requested")
	}
	manifest := spec.DefaultMeta("Changed elsewhere")
	changed, _ := json.Marshal(manifest)
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	answer := model.ResourceEditApprovalAnswer{InteractionID: requested.InteractionID, CallID: call.ID, Revision: requested.Revision, Decision: "approve"}
	prompter.answered <- answer
	select {
	case results := <-finished:
		if len(results) != 1 || results[0].Code != model.CodeContentConflict {
			t.Fatalf("changed baseline result: %+v", results)
		}
	case <-ctx.Done():
		t.Fatal("tool did not finish after baseline conflict")
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != string(changed) {
		t.Fatalf("changed source was overwritten: %q %v", actual, err)
	}
}

func TestResourceApprovalDraftDoesNotWriteSourceAndChecksRevision(t *testing.T) {
	root := t.TempDir()
	base := spec.DefaultMeta("Deck")
	before, _ := json.Marshal(base)
	path := filepath.Join(root, ".meta.json")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	proposal := base
	proposal.Goal = "Agent goal"
	proposed, _ := json.Marshal(proposal)
	record := model.ResourceEditApproval{RunID: "run", CallID: "call", InteractionID: resourceApprovalID("run", "call"),
		Resource: "meta", Revision: 1, ResourceEditCandidate: model.ResourceEditCandidate{BaseExists: true, BaseHash: hashBytes(before), Base: before,
			Proposal: proposed, Draft: proposed}, Target: model.PublicTarget{Type: "deck", Part: "meta"}, State: "pending"}
	for _, invalid := range []json.RawMessage{json.RawMessage(`null`), json.RawMessage(`{}`)} {
		if _, err := PrepareResourceEditDraft(root, record, 1, invalid); !errors.Is(err, pptmutation.ErrInvalid) {
			t.Fatalf("invalid approval draft was accepted: %v", err)
		}
		corrupt := record
		corrupt.Draft = invalid
		session, err := NewRunSession(root, "invalid-approval")
		if err != nil {
			t.Fatal(err)
		}
		_, err = stageApprovedResource(root, contextengine.ContextPack{}, session, corrupt)
		session.Discard()
		if err == nil || resourceMutationFailure(err, Resource{Type: "deck", Part: "meta"}).Code != model.CodeInternal {
			t.Fatalf("invalid persisted draft was blamed on model content: %v", err)
		}
	}
	changed := proposal
	changed.Goal = "User goal"
	draft, _ := json.Marshal(changed)
	largeDraft := append([]byte(strings.Repeat(" ", (2<<20)+1)), draft...)
	updated, err := PrepareResourceEditDraft(root, record, 1, largeDraft)
	if err != nil || updated.Revision != 2 || updated.Target.Diff == nil {
		t.Fatalf("draft update: %+v %v", updated, err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != string(before) {
		t.Fatalf("approval draft changed source: %q %v", actual, err)
	}
	if _, err := PrepareResourceEditDraft(root, updated, 1, draft); !errors.Is(err, ErrResourceApprovalConflict) {
		t.Fatalf("stale draft revision accepted: %v", err)
	}
}

func TestOutlineApprovalReturnsFinalSourceWithNoSizeLimit(t *testing.T) {
	for _, changed := range []bool{false, true} {
		dir, _, pack := generationPackFixture(t)
		before, err := os.ReadFile(filepath.Join(dir, ".outline.json"))
		if err != nil {
			t.Fatal(err)
		}
		var outline spec.Outline
		if err := json.Unmarshal(before, &outline); err != nil {
			t.Fatal(err)
		}
		if changed {
			outline.Sections[0].Title = "User revision"
		}
		draft, _ := json.Marshal(outline)
		record := model.ResourceEditApproval{RunID: "outline-approval", CallID: "call", InteractionID: resourceApprovalID("outline-approval", "call"),
			Resource: "outline", Revision: 1, ResourceEditCandidate: model.ResourceEditCandidate{BaseExists: true, BaseHash: hashBytes(before), Base: before,
				Proposal: before, Draft: before}, Target: model.PublicTarget{Type: "deck", Part: "outline"}, State: "pending"}
		largeDraft := append([]byte(strings.Repeat(" ", 2*1024*1024+1)), draft...)
		record, err = PrepareResourceEditDraft(dir, record, record.Revision, largeDraft)
		if err != nil {
			t.Fatal(err)
		}
		session, err := NewRunSession(dir, record.RunID)
		if err != nil {
			t.Fatal(err)
		}
		result, err := stageApprovedResource(dir, pack, session, record)
		if err != nil {
			session.Discard()
			t.Fatal(err)
		}
		saved, _, err := readArtifact(dir, session, outlineRef(pack))
		session.Discard()
		if err != nil {
			t.Fatal(err)
		}
		assertSavedResourceObservation(t, "edit_outline", result, saved)
		messages := appendBatchObservations(nil, []llm.ToolCall{{ID: "call", Name: "edit_outline"}}, "", []ToolResult{result})
		if visibleResourceHashes(messages)["ppt/deck:outline"] != spec.ContentHash(saved) {
			t.Fatal("approved outline did not preserve its exact saved source")
		}
		var actual spec.Outline
		if err := json.Unmarshal(saved, &actual); err != nil || !reflect.DeepEqual(actual, outline) {
			t.Fatalf("approval did not return the user's final outline: %s (%v)", saved, err)
		}
		if (len(result.ChangedTargets) > 0) != changed {
			t.Fatalf("approval change state differs from saved source: %+v", result)
		}
	}
}

func TestResourceApprovalRejectsChangedBaseline(t *testing.T) {
	root := t.TempDir()
	manifest := spec.DefaultMeta("Deck")
	base, _ := json.Marshal(manifest)
	path := filepath.Join(root, ".meta.json")
	if err := os.WriteFile(path, base, 0o600); err != nil {
		t.Fatal(err)
	}
	record := model.ResourceEditApproval{RunID: "run", CallID: "call", InteractionID: resourceApprovalID("run", "call"),
		Resource: "meta", Revision: 1, ResourceEditCandidate: model.ResourceEditCandidate{BaseExists: true, BaseHash: hashBytes(base), Base: base,
			Proposal: base, Draft: base}, Target: model.PublicTarget{Type: "deck", Part: "meta"}, State: "pending"}
	manifest.Goal = "Changed elsewhere"
	changed, _ := json.Marshal(manifest)
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if ResourceEditApprovalBaselineMatches(root, record) {
		t.Fatal("stale baseline accepted")
	}
	if _, err := PrepareResourceEditDraft(root, record, record.Revision, base); !errors.Is(err, ErrResourceApprovalConflict) {
		t.Fatalf("stale baseline accepted for draft: %v", err)
	}
}

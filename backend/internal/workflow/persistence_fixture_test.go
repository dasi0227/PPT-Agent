package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

// Runtime tests substitute the atomic storage boundary, not historical replay.
// SQLite owns revision fencing, idempotency, and transaction tests.
type testRuntime struct{ *Runtime }

func recordedToolResults(t *testing.T, state *RunState, calls []llm.ToolCall) []ToolResult {
	t.Helper()
	results := make([]ToolResult, len(calls))
	for i, call := range calls {
		result, err := storedExecutionResult(state.toolExecutions[call.ID], state)
		if err != nil {
			t.Fatal(err)
		}
		results[i] = result
	}
	return results
}

func newTestRuntime(agent ReActAgent) *testRuntime { return &testRuntime{NewRuntime(agent)} }

func (r *testRuntime) Run(ctx context.Context, input RuntimeInput) StructuredOutcome {
	return r.Runtime.Run(ctx, runtimeFixtureInput(input))
}

func runtimeFixtureInput(input RuntimeInput) RuntimeInput {
	if _, wrapped := input.Checkpoint.(*fixtureCheckpoint); wrapped {
		return input
	}
	if input.Transcript == nil && input.Checkpoint == nil {
		return input
	}
	store, ok := input.Transcript.(*fixtureContextStore)
	if !ok {
		store = newFixtureContextStore(nil)
	}
	if input.Context.Manifest.ThreadID == "" {
		input.Context.Manifest.ThreadID = "fixture_thread"
	}
	if store.snapshot.ThreadID == "" {
		store.snapshot.ThreadID = input.Context.Manifest.ThreadID
		if input.ResumeCheckpoint != nil {
			store.snapshot.Revision = input.ResumeCheckpoint.ContextRevision
		}
		store.versions[store.snapshot.Revision] = store.snapshot
	}
	if input.ResumeCheckpoint != nil {
		// A test can choose an earlier saved boundary to represent a crash there.
		if snapshot, exists := store.versions[input.ResumeCheckpoint.ContextRevision]; exists {
			store.snapshot = snapshot
		}
		cp := *input.ResumeCheckpoint
		cp.ToolExecutions = nil
		for _, row := range store.tools {
			if row.RunID == input.RunID {
				cp.ToolExecutions = append(cp.ToolExecutions, row)
			}
		}
		if cp.PendingToolBatch != nil {
			pending := *cp.PendingToolBatch
			pending.Calls = nil
			for _, id := range pending.CallIDs {
				if row, exists := store.tools[input.RunID+":"+id]; exists {
					call, err := row.Call()
					if err == nil {
						pending.Calls = append(pending.Calls, call)
					}
				}
			}
			cp.PendingToolBatch = &pending
		}
		input.ResumeCheckpoint = &cp
	}
	emitter := &fixtureEmitter{source: input.Emitter}
	sink := &fixtureCheckpoint{store: store, next: input.Checkpoint, emitter: emitter}
	input.Transcript, input.Checkpoint, input.ToolExecutions, input.Emitter = store, sink, store, emitter
	if commit := input.CommitPlanApproval; commit != nil {
		input.CommitPlanApproval = func(ctx context.Context, mode model.RunMode, pack contextengine.ContextPack, cp RuntimeCheckpoint) error {
			if err := commit(ctx, mode, pack, cp); err != nil {
				return err
			}
			return sink.SaveCheckpoint(ctx, cp)
		}
	}
	if commit := input.CommitScopeExpansion; commit != nil {
		input.CommitScopeExpansion = func(ctx context.Context, scope model.RunScope, cp RuntimeCheckpoint) error {
			if err := commit(ctx, scope, cp); err != nil {
				return err
			}
			return sink.SaveCheckpoint(ctx, cp)
		}
	}
	return input
}

func (r *testRuntime) executeToolBatch(ctx context.Context, input RuntimeInput, state *RunState, registry *ToolRegistry, disclosed map[string]bool, calls []llm.ToolCall) []ToolResult {
	if input.Checkpoint != nil {
		store := newFixtureContextStore(state.messages)
		if state.contextSession != nil {
			store.snapshot = state.contextSession.Snapshot
		}
		for _, row := range state.toolExecutions {
			store.tools[row.RunID+":"+row.CallID] = row
		}
		input.Transcript = store
		input = runtimeFixtureInput(input)
		state.contextSession, _ = store.Open(ctx, input.Context.Manifest.ThreadID, nil)
		input.publicUpdates = &publicUpdates{source: input.Emitter}
		input.Emitter = input.publicUpdates
	}
	results := r.Runtime.executeToolBatch(ctx, input, state, registry, disclosed, calls)
	if toolExecutionStopError(input, state) == nil && state.pendingToolBatch != nil {
		complete := true
		for _, id := range state.pendingToolBatch.CallIDs {
			complete = complete && state.toolExecutions[id].Terminal()
		}
		if complete {
			if err := r.finishToolBatch(ctx, input, state); err != nil {
				state.toolRecoveryErr = err
			}
		}
	}
	return results
}

type fixtureContextStore struct {
	snapshot sessionstate.ContextSnapshot
	versions map[int64]sessionstate.ContextSnapshot
	tools    map[string]sessionstate.ToolExecution
}

func newFixtureContextStore(messages []llm.Message) *fixtureContextStore {
	store := &fixtureContextStore{
		snapshot: sessionstate.ContextSnapshot{SceneRevision: 1},
		versions: map[int64]sessionstate.ContextSnapshot{},
		tools:    map[string]sessionstate.ToolExecution{},
	}
	for i, message := range messages {
		if message.PersistentID == "" {
			message.PersistentID = fmt.Sprintf("fixture_message_%d", i)
		}
		store.snapshot.Messages = append(store.snapshot.Messages, sessionstate.Message{ID: message.PersistentID, Native: message})
	}
	return store
}

func (s *fixtureContextStore) Messages() []llm.Message {
	messages := make([]llm.Message, len(s.snapshot.Messages))
	for i, message := range s.snapshot.Messages {
		messages[i] = message.Native
	}
	return messages
}

func (s *fixtureContextStore) LoadContext(_ context.Context, _ string) (sessionstate.ContextSnapshot, error) {
	return s.snapshot, nil
}

func (s *fixtureContextStore) LoadContextVersion(_ context.Context, _ string, revision int64) (sessionstate.ContextSnapshot, error) {
	snapshot, exists := s.versions[revision]
	if !exists {
		return sessionstate.ContextSnapshot{}, sessionstate.ErrInvalidState
	}
	return snapshot, nil
}

func (s *fixtureContextStore) Open(ctx context.Context, threadID string, revision *int64) (*contextengine.ContextSession, error) {
	return contextengine.NewContextStore(s).Open(ctx, threadID, revision)
}

func (s *fixtureContextStore) GetToolExecution(_ context.Context, runID, callID string) (sessionstate.ToolExecution, error) {
	row, exists := s.tools[runID+":"+callID]
	if !exists {
		return sessionstate.ToolExecution{}, model.ErrRunNotFound
	}
	return row, nil
}

type fixtureCheckpoint struct {
	store   *fixtureContextStore
	next    CheckpointSink
	emitter *fixtureEmitter
}

func (s *fixtureCheckpoint) SaveCheckpoint(ctx context.Context, cp RuntimeCheckpoint) error {
	next := s.store.snapshot
	if update := cp.ContextUpdate; update != nil {
		if update.ExpectedRevision != next.Revision {
			return sessionstate.ErrVersionConflict
		}
		bodies := map[string]sessionstate.Message{}
		before := make([]string, len(next.Messages))
		for i, message := range next.Messages {
			before[i], bodies[message.ID] = message.ID, message
		}
		for _, message := range update.NewMessages {
			bodies[message.ID] = message
		}
		next.Messages = make([]sessionstate.Message, len(update.MessageIDs))
		for i, id := range update.MessageIDs {
			message, exists := bodies[id]
			if !exists {
				return sessionstate.ErrInvalidState
			}
			message.Native.PersistentID = id
			next.Messages[i] = message
		}
		if !slices.Equal(before, update.MessageIDs) {
			next.Revision++
		}
	}
	cp.ContextRevision = next.Revision
	if s.next != nil {
		if err := s.next.SaveCheckpoint(ctx, cp); err != nil {
			return err
		}
	}
	s.store.snapshot = next
	s.store.versions[next.Revision] = next
	if cp.CommitReceipt != nil {
		*cp.CommitReceipt = sessionstate.Receipt{SceneRevision: next.SceneRevision, ContextRevision: next.Revision}
	}
	for _, row := range cp.ToolChanges {
		row.EntityRevision++
		s.store.tools[row.RunID+":"+row.CallID] = row
	}
	return s.emitter.publish(cp.PublicUpdates)
}

func (s *fixtureCheckpoint) PersistenceFailed(ctx context.Context, reason string) {
	if next, ok := s.next.(interface{ PersistenceFailed(context.Context, string) }); ok {
		next.PersistenceFailed(ctx, reason)
	}
}

// Preserve typed payloads used by workflow assertions while publishing them only
// after the fake atomic write, as the production state store does.
type fixtureEmitter struct {
	source  EventEmitter
	pending []any
}

func (e *fixtureEmitter) Emit(kind model.EventType, payload any) {
	if e.source != nil {
		e.source.Emit(kind, payload)
	}
}

func (e *fixtureEmitter) PreparePublicUpdate(_ model.EventType, payload any) (any, bool) {
	e.pending = append(e.pending, payload)
	return payload, true
}

func (e *fixtureEmitter) publish(updates []sessionstate.RunUpdate) error {
	for i, update := range updates {
		var value any
		if i < len(e.pending) {
			value = reflect.New(reflect.TypeOf(e.pending[i])).Interface()
		} else if update.Type == model.EventContextCompacted {
			// Auto-compaction appends its candidate fact directly to the transaction.
			value = &model.ContextCompactedPayload{}
		} else {
			return sessionstate.ErrInvalidState
		}
		if err := json.Unmarshal(update.Payload, value); err != nil {
			return err
		}
		e.Emit(update.Type, reflect.ValueOf(value).Elem().Interface())
	}
	e.pending = e.pending[min(len(updates), len(e.pending)):]
	return nil
}

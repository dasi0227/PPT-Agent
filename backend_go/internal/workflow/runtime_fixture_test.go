package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func resumedProject(t *testing.T, dir, runID string) string {
	t.Helper()
	if err := ensureReviewBaseline(context.Background(), dir, runID, false); err != nil {
		t.Fatal(err)
	}
	return dir
}

type scriptedAgent struct {
	mu        sync.Mutex
	responses []AgentResponse
	requests  []AgentRequest
	err       error
}

type memoryIdempotencyStore struct {
	mu      sync.Mutex
	records map[string]model.IdempotencyRecord
}

func newMemoryIdempotencyStore() *memoryIdempotencyStore {
	return &memoryIdempotencyStore{records: map[string]model.IdempotencyRecord{}}
}

func idempotencyRecordKey(scope, ownerID, key string) string {
	return scope + "|" + ownerID + "|" + key
}

func (s *memoryIdempotencyStore) saveArtifactCommit(_ context.Context, runID string, commit CommitContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[idempotencyRecordKey("artifact_commit", runID, commit.OperationID)] = model.IdempotencyRecord{Scope: "artifact_commit", OwnerID: runID, Key: commit.OperationID, Status: "completed", ResultJSON: commit.ToolResultJSON}
	return nil
}

func (s *memoryIdempotencyStore) GetIdempotency(_ context.Context, scope, ownerID, key string) (model.IdempotencyRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[idempotencyRecordKey(scope, ownerID, key)]
	if !ok {
		return model.IdempotencyRecord{}, model.ErrRunNotFound
	}
	return record, nil
}

func (a *scriptedAgent) Next(_ context.Context, req AgentRequest) (AgentResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requests = append(a.requests, req)
	if len(a.responses) == 0 {
		if a.err != nil {
			return AgentResponse{}, a.err
		}
		return AgentResponse{}, errors.New("script exhausted")
	}
	response := a.responses[0]
	a.responses = a.responses[1:]
	return response, nil
}

type recordedEvent struct {
	kind    model.EventType
	payload any
}

type eventRecorder struct {
	events []recordedEvent
}

func (r *eventRecorder) Emit(kind model.EventType, payload any) {
	r.events = append(r.events, recordedEvent{kind: kind, payload: payload})
}

func (r *eventRecorder) count(kind model.EventType) int {
	count := 0
	for _, event := range r.events {
		if event.kind == kind {
			count++
		}
	}
	return count
}

type fakeProvider struct {
	kind ArtifactKind
}

func (p fakeProvider) RegisterDomainTools(registry *ToolRegistry) error {
	return registry.Register(fakeWriteTool{kind: p.kind}, false, CapabilityPPTMutate, RiskMedium, PhaseExecuting)
}

type fakeWriteTool struct{ kind ArtifactKind }

func (fakeWriteTool) Schema() ToolSchema {
	return ToolSchema{Name: "edit_spec", Description: "test write", Parameters: objectSchema(nil, map[string]any{
		"slide_id": map[string]any{"type": "string"},
		"content":  map[string]any{"type": "string"}, "evidence": map[string]any{"type": "boolean"},
		"target": objectSchema([]string{"type", "slide_id"}, map[string]any{
			"type": map[string]any{"type": "string"}, "slide_id": map[string]any{"type": "string"},
		}),
	})}
}

func (t fakeWriteTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	ref := ArtifactRef{Kind: t.kind, ID: "sli_1", Path: model.SlideSpecPath("sli_1")}
	if t.kind == ArtifactSlideHTML {
		ref.Path = model.SlideHTMLPath("sli_1")
	}
	content := []byte(stringValue(input.Args["content"]))
	if len(content) == 0 {
		content = []byte("changed")
	}
	if t.kind == ArtifactSlideSpec {
		content, _ = json.Marshal(spec.SlideSpec{Purpose: "content_explanation", Layout: "Title above content", Core: string(content), Elements: []spec.Element{}})
	}
	change, err := input.Session.Write(ref, "write_fake", content)
	if err != nil {
		return persistenceToolFailure(err)
	}
	result := SuccessfulToolResult("written")
	part := "spec"
	if t.kind == ArtifactSlideHTML {
		part = "html"
	}
	result.ChangedTargets = []ChangedTarget{{
		Type: "slide", SlideID: "sli_1", Part: part, Hash: change.AfterHash, Fields: []string{part},
	}}
	if input.Args["evidence"] != false {
		kind := "schema"
		if t.kind == ArtifactSlideHTML {
			kind = "static"
		}
		result.Evidence = []Evidence{newEvidence(kind, Resource{Type: "slide", SlideID: "sli_1", Part: part}, change.AfterHash)}
	}
	return result
}

func toolCall(id, name string, args map[string]any) AgentResponse {
	return AgentResponse{ToolCalls: []llm.ToolCall{{ID: id, Name: name, Args: args}}}
}

func finalReply() AgentResponse {
	return AgentResponse{Text: "done"}
}

type commandPermissionPrompter struct {
	decision string
	feedback string
	err      error
	requests []model.CommandPermissionRequestedPayload
}

func (p *commandPermissionPrompter) Ask(context.Context, model.QuestionAskedPayload) (model.QuestionAnswer, error) {
	return model.QuestionAnswer{}, errors.New("ordinary question was not expected")
}

func (p *commandPermissionPrompter) AskCommandPermission(
	_ context.Context,
	request model.CommandPermissionRequestedPayload,
) (model.CommandPermissionAnswer, error) {
	p.requests = append(p.requests, request)
	if p.err != nil {
		return model.CommandPermissionAnswer{}, p.err
	}
	return model.CommandPermissionAnswer{
		InteractionID: request.InteractionID,
		CallID:        request.CallID,
		CommandHash:   request.CommandHash,
		Decision:      p.decision,
		Feedback:      p.feedback,
	}, nil
}

type checkpointRecorder struct {
	checkpoints []RuntimeCheckpoint
}

func (r *checkpointRecorder) SaveCheckpoint(_ context.Context, checkpoint RuntimeCheckpoint) error {
	r.checkpoints = append(r.checkpoints, checkpoint)
	return nil
}

type postCommitFailingCheckpoint struct {
	failures int
}

func (s *postCommitFailingCheckpoint) SaveCheckpoint(_ context.Context, checkpoint RuntimeCheckpoint) error {
	if checkpoint.Boundary == string(checkpointAfterCommit) || checkpoint.Boundary == string(checkpointTerminal) {
		s.failures++
		return errors.New("checkpoint store unavailable after commit")
	}
	return nil
}

func testPack(mode model.RunMode, selection model.ScopeSelectionKind, empty bool, instruction string) contextengine.ContextPack {
	sections := []spec.Section{{ID: "sec_test", Title: "Section", Slides: []spec.SlideNode{{ID: "sli_1", Title: "Old"}}, Subsections: []spec.Subsection{}}}
	summaries := []contextengine.SlideSummary{{ID: "sli_1", Title: "Old", State: string(model.HTMLAvailable)}}
	if empty {
		sections, summaries = []spec.Section{}, []contextengine.SlideSummary{}
	}
	target := model.NewRunScope(selection)
	if selection == model.ScopeCurrentPage {
		target.SlideIDs = []string{"sli_1"}
	}
	return contextengine.ContextPack{
		Command: model.RunCommand{
			Scope: target, Mode: mode, Instruction: instruction,
		},
		Project: contextengine.ProjectContext{ID: "p1", Title: "Deck"},
		Outline: contextengine.OutlineContext{Outline: spec.Outline{
			Sections: sections,
		}, Summaries: summaries},
		Components: []contextengine.ComponentCandidate{},

		Manifest: contextengine.ContextManifest{
			ContextID: "ctx", RunID: "run", ThreadID: "thread",
			Warnings: []string{},
		},
	}
}

func testProject(t *testing.T, kind ArtifactKind) string {
	t.Helper()
	dir := t.TempDir()
	path := model.SlideSpecPath("sli_1")
	if kind == ArtifactSlideHTML {
		path = model.SlideHTMLPath("sli_1")
	}
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("formal")
	if kind == ArtifactSlideSpec {
		content = []byte(`{"purpose":"content_explanation","layout":"Title above content","core":"formal","elements":[]}`)
	}
	if err := os.WriteFile(full, content, 0o644); err != nil {
		t.Fatal(err)
	}
	outline := spec.Outline{Sections: []spec.Section{{ID: "sec_test", Title: "Section", Slides: []spec.SlideNode{{ID: "sli_1", Title: "Old"}}, Subsections: []spec.Subsection{}}}}
	raw, _ := json.Marshal(outline)
	if err := os.WriteFile(filepath.Join(dir, ".outline.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readTestSpecText(dir string) ([]byte, error) {
	raw, err := spec.ReadSlideSpec(func(path string) ([]byte, error) { return os.ReadFile(filepath.Join(dir, path)) }, "sli_1")
	if err != nil {
		return nil, err
	}
	var value spec.SlideSpec
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return []byte(value.Core), nil
}

type planTestAgentFunc func(context.Context, AgentRequest) (AgentResponse, error)

func (f planTestAgentFunc) Next(ctx context.Context, request AgentRequest) (AgentResponse, error) {
	return f(ctx, request)
}

type recoveryPauseRecorder struct {
	checkpointRecorder
	reason string
}

func (r *recoveryPauseRecorder) PersistenceFailed(_ context.Context, reason string) {
	r.reason = reason
}

func testToolExecution(t *testing.T, runID string, call llm.ToolCall, policy sessionstate.ToolRecoveryPolicy, status string) sessionstate.ToolExecution {
	t.Helper()
	hash, args, err := sessionstate.ToolArgumentsHash(call.Name, call.Args)
	if err != nil {
		t.Fatal(err)
	}
	return sessionstate.ToolExecution{ID: sessionstate.ToolAttemptID(runID, call.ID, 1), RunID: runID, CallID: call.ID, AttemptNo: 1, Name: call.Name, Arguments: args, ArgumentsHash: hash, Status: status, RecoveryPolicy: policy, EntityRevision: 2}
}

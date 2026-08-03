package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
)

type scriptedAgent struct {
	mu        sync.Mutex
	responses []AgentResponse
	requests  []AgentRequest
	err       error
}

type cancelingAgent struct {
	cancel context.CancelFunc
}

type scriptedSteering struct {
	mu       sync.Mutex
	batches  [][]SteeringInput
	injected []string
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

func (s *memoryIdempotencyStore) AcquireIdempotency(_ context.Context, record model.IdempotencyRecord) (model.IdempotencyRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := idempotencyRecordKey(record.Scope, record.OwnerID, record.Key)
	if existing, ok := s.records[key]; ok {
		return existing, false, nil
	}
	if record.Status == "" {
		record.Status = "in_progress"
	}
	s.records[key] = record
	return record, true, nil
}

func (s *memoryIdempotencyStore) CompleteIdempotency(_ context.Context, scope, ownerID, key, status, resultJSON string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	recordKey := idempotencyRecordKey(scope, ownerID, key)
	record, ok := s.records[recordKey]
	if !ok {
		return errors.New("idempotency record not found")
	}
	record.Status, record.ResultJSON = status, resultJSON
	s.records[recordKey] = record
	return nil
}

func (s *memoryIdempotencyStore) GetIdempotency(_ context.Context, scope, ownerID, key string) (model.IdempotencyRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[idempotencyRecordKey(scope, ownerID, key)]
	if !ok {
		return model.IdempotencyRecord{}, errors.New("idempotency record not found")
	}
	return record, nil
}

func (s *scriptedSteering) DrainInputs(context.Context) ([]SteeringInput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.batches) == 0 {
		return nil, nil
	}
	next := append([]SteeringInput(nil), s.batches[0]...)
	s.batches = s.batches[1:]
	return next, nil
}

func (s *scriptedSteering) MarkInputsInjected(_ context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.injected = append(s.injected, ids...)
	return nil
}

func (a cancelingAgent) Next(_ context.Context, _ AgentRequest) (AgentResponse, error) {
	a.cancel()
	return AgentResponse{}, errors.New("provider request interrupted")
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

type cancelOnToolStartedEmitter struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	events []recordedEvent
}

func (e *cancelOnToolStartedEmitter) Emit(kind model.EventType, payload any) {
	e.mu.Lock()
	e.events = append(e.events, recordedEvent{kind: kind, payload: payload})
	e.mu.Unlock()
	if kind == model.EventToolStarted {
		e.cancel()
	}
}

type blockingReadProvider struct{}

func (blockingReadProvider) RegisterDomainTools(registry *ToolRegistry) error {
	return registry.RegisterDomainTool(blockingReadTool{}, true, PhaseChat, PhasePlanning, PhaseExecuting)
}

type blockingReadTool struct{}

func (blockingReadTool) Schema() ToolSchema {
	return ToolSchema{Name: "search_refs", Description: "blocking cancellation test", Parameters: objectSchema(nil, map[string]any{})}
}

func (blockingReadTool) Execute(ctx context.Context, _ DomainToolInput) ToolResult {
	<-ctx.Done()
	return failedToolResult(CodeCanceled, "run canceled", false)
}

type fakeProvider struct {
	kind ArtifactKind
}

func (p fakeProvider) RegisterDomainTools(registry *ToolRegistry) error {
	if err := registry.RegisterDomainTool(fakeWriteTool{kind: p.kind}, false, PhaseExecuting); err != nil {
		return err
	}
	if err := registry.RegisterDomainTool(fakeRenderTool{}, true, PhaseChat, PhasePlanning, PhaseExecuting); err != nil {
		return err
	}
	return registry.RegisterDomainTool(fakeExpansionTool{}, false, PhaseExecuting)
}

type fakeWriteTool struct{ kind ArtifactKind }

func (fakeWriteTool) Schema() ToolSchema {
	return ToolSchema{Name: "write_ppt", Description: "test write", Parameters: objectSchema(nil, map[string]any{})}
}

func (t fakeWriteTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	ref := ArtifactRef{Kind: t.kind, ID: "s1", Path: model.SlideSpecPath("s1")}
	if t.kind == ArtifactSlideHTML {
		ref.Path = model.SlideHTMLPath("s1")
	}
	content := []byte(stringValue(input.Args["content"]))
	if len(content) == 0 {
		content = []byte("changed")
	}
	change, err := input.Transaction.Stage(ref, "write_fake", content)
	if err != nil {
		return failedToolResult("STAGING_FAILED", err.Error(), false)
	}
	result := SuccessfulToolResult("staged")
	part := "spec"
	if t.kind == ArtifactSlideHTML {
		part = "html"
	}
	result.ChangedTargets = []ChangedTarget{{
		Type: "slide", SlideID: "s1", Part: part, Hash: change.AfterHash, Fields: []string{part},
	}}
	if input.Args["evidence"] != false {
		kind := "schema"
		if t.kind == ArtifactSlideHTML {
			kind = "static"
		}
		result.Evidence = []Evidence{newEvidence(kind, Resource{Type: "slide", SlideID: "s1", Part: part}, change.AfterHash)}
	}
	return result
}

type fakeRenderTool struct{}

func (fakeRenderTool) Schema() ToolSchema {
	return ToolSchema{Name: "render_slide", Description: "test render", Parameters: objectSchema(nil, map[string]any{})}
}

func (fakeRenderTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	ref := ArtifactRef{Kind: ArtifactSlideHTML, ID: "s1", Path: model.SlideHTMLPath("s1")}
	raw, err := input.Transaction.Read(ref)
	if err != nil {
		return failedToolResult("RENDER_FAILED", err.Error(), false)
	}
	result := SuccessfulToolResult("rendered")
	result.Evidence = []Evidence{newEvidence("render", Resource{Type: "slide", SlideID: "s1", Part: "html"}, hashBytes(raw))}
	return result
}

type fakeExpansionTool struct{}

func (fakeExpansionTool) Schema() ToolSchema {
	return ToolSchema{Name: "edit_ppt", Description: "test scope expansion", Parameters: objectSchema(nil, map[string]any{})}
}

func (fakeExpansionTool) Execute(_ context.Context, _ DomainToolInput) ToolResult {
	return failedToolResult(CodeScopeExpansion, "second target required", false)
}

func toolCall(id, name string, args map[string]any) AgentResponse {
	return AgentResponse{ToolCalls: []llm.ToolCall{{ID: id, Name: name, Args: args}}}
}

func planCall(id string, completed bool) AgentResponse {
	status := "in_progress"
	if completed {
		status = "completed"
	}
	return toolCall(id, "update_plan", map[string]any{
		"explanation": "test plan",
		"steps":       []any{map[string]any{"id": "work", "title": "Do the work", "status": status}},
	})
}

func finishCall(id string) AgentResponse {
	return toolCall(id, "finish", map[string]any{"message": "done"})
}

func TestRouterMapsTalkAndAskToChat(t *testing.T) {
	for _, intent := range []model.InteractionIntent{model.IntentTalk, model.IntentAsk} {
		pack := testPack(intent, model.ArtifactPresentation, model.TargetDeck, false, "生成整份演示")
		if got := (StrategyRouter{}).Decide(pack).Strategy; got != StrategyChat {
			t.Fatalf("%s routed to %s", intent, got)
		}
	}
}

func TestRouterSelectsSimpleForExplicitSingleTarget(t *testing.T) {
	pack := testPack(model.IntentExecute, model.ArtifactSpec, model.TargetSlide, false, "修改当前页标题")
	if got := (StrategyRouter{}).Decide(pack).Strategy; got != StrategySimple {
		t.Fatalf("strategy=%s", got)
	}
}

func TestRouterSelectsComplexForEmptyWholeDeck(t *testing.T) {
	pack := testPack(model.IntentExecute, model.ArtifactSpec, model.TargetDeck, true, "生成产品发布演示")
	if got := (StrategyRouter{}).Decide(pack).Strategy; got != StrategyComplex {
		t.Fatalf("strategy=%s", got)
	}
}

func TestChatPlainTextRequiresLaterExplicitFinish(t *testing.T) {
	events := &eventRecorder{}
	agent := &scriptedAgent{responses: []AgentResponse{
		{Text: "这是尚未通过 finish 提交的分析。"},
		finishCall("finish"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "chat-explicit-finish", ProjectDir: t.TempDir(),
		Context: testPack(model.IntentTalk, model.ArtifactSpec, model.TargetSlide, false, "分析当前页"),
		Emitter: events, DomainTools: fakeProvider{kind: ArtifactSlideSpec},
	})
	if outcome.Status != StatusCompleted || outcome.Strategy != StrategyChat {
		t.Fatalf("outcome=%+v", outcome)
	}
	if len(agent.requests) != 2 {
		t.Fatalf("plain text ended the run instead of starting another turn: requests=%d", len(agent.requests))
	}
	messages := agent.requests[1].Messages
	if len(messages) != 2 {
		t.Fatalf("next-turn context=%+v", messages)
	}
	if messages[0].Role != llm.RoleAssistant ||
		messages[0].Text() != "这是尚未通过 finish 提交的分析。" {
		t.Fatalf("assistant text was not retained: %+v", messages[0])
	}
	if messages[1].Role != llm.RoleUser || messages[1].Text() != noToolCallGuidance {
		t.Fatalf("explicit finish guidance missing: %+v", messages[1])
	}
	if events.count(model.EventMessageFinal) != 1 || events.count(model.EventRunFinished) != 1 {
		t.Fatalf("terminal events=%+v", events.events)
	}
	final := ""
	for _, event := range events.events {
		if event.kind == model.EventMessageFinal {
			final = event.payload.(model.MessageFinalPayload).Text
		}
	}
	if final != "done" {
		t.Fatalf("final text must come from finish.message, got %q", final)
	}
}

func TestRuntimePassesOpaqueProviderContinuationWithoutParsingIt(t *testing.T) {
	continuation := &llm.ProviderContinuation{
		Provider: "deepseek", Model: "deepseek-v4-pro",
		Opaque: json.RawMessage(`{"private_reasoning_state":"opaque"}`),
	}
	agent := &scriptedAgent{responses: []AgentResponse{
		{Text: "continue", Continuation: continuation},
		finishCall("finish"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "opaque-continuation", ProjectDir: t.TempDir(),
		Context:     testPack(model.IntentTalk, model.ArtifactSpec, model.TargetSlide, false, "inspect"),
		DomainTools: fakeProvider{kind: ArtifactSlideSpec},
	})
	if outcome.Status != StatusCompleted || len(agent.requests) != 2 {
		t.Fatalf("continuation run failed: outcome=%+v requests=%d", outcome, len(agent.requests))
	}
	got := agent.requests[1].Continuation
	if got != continuation || got.Provider != "deepseek" ||
		string(got.Opaque) != `{"private_reasoning_state":"opaque"}` {
		t.Fatalf("Runtime changed provider continuation: %+v", got)
	}
	raw, _ := json.Marshal(agent.requests[1].Messages)
	if strings.Contains(string(raw), "private_reasoning_state") {
		t.Fatalf("provider continuation entered ordinary message history: %s", raw)
	}
}

func TestSteeringInjectsIndependentUserMessagesInAcceptanceOrderBeforeFirstNext(t *testing.T) {
	agent := &scriptedAgent{responses: []AgentResponse{finishCall("finish")}}
	steering := &scriptedSteering{batches: [][]SteeringInput{{
		{ID: "msg-1", Content: "use dark colors"},
		{ID: "msg-2", Content: "keep the typography compact"},
	}}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "steer-initial", ProjectDir: t.TempDir(),
		Context:     testPack(model.IntentTalk, model.ArtifactSpec, model.TargetSlide, false, "review"),
		DomainTools: fakeProvider{kind: ArtifactSlideSpec}, Steering: steering,
	})
	if outcome.Status != StatusCompleted || len(agent.requests) != 1 {
		t.Fatalf("outcome=%+v requests=%d", outcome, len(agent.requests))
	}
	messages := agent.requests[0].Messages
	if len(messages) != 2 ||
		messages[0].Role != llm.RoleUser || messages[0].Text() != "User steering: use dark colors" ||
		messages[1].Role != llm.RoleUser || messages[1].Text() != "User steering: keep the typography compact" {
		t.Fatalf("steering messages were merged or reordered: %+v", messages)
	}
	if strings.Join(steering.injected, ",") != "msg-1,msg-2" {
		t.Fatalf("injected status was not acknowledged: %+v", steering.injected)
	}
}

func TestSteeringWaitsUntilCompleteToolBatchObservation(t *testing.T) {
	dir := testProject(t, ArtifactSlideSpec)
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("write", "write_ppt", map[string]any{"content": "next"}),
		finishCall("finish"),
	}}
	steering := &scriptedSteering{batches: [][]SteeringInput{
		nil,
		{{ID: "msg-after-batch", Content: "apply this after the current batch"}},
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "steer-boundary", ProjectDir: dir,
		Context:     testPack(model.IntentExecute, model.ArtifactSpec, model.TargetSlide, false, "modify"),
		DomainTools: fakeProvider{kind: ArtifactSlideSpec}, Steering: steering,
	})
	if outcome.Status != StatusCompleted || len(agent.requests) != 2 {
		t.Fatalf("outcome=%+v requests=%d", outcome, len(agent.requests))
	}
	messages := agent.requests[1].Messages
	if len(messages) < 3 ||
		messages[len(messages)-2].Role != llm.RoleTool ||
		messages[len(messages)-2].ToolCallID != "write" ||
		messages[len(messages)-1].Role != llm.RoleUser ||
		messages[len(messages)-1].Text() != "User steering: apply this after the current batch" {
		t.Fatalf("steering was not injected after the complete batch observation: %+v", messages)
	}
}

func TestToolCallIdempotencyReplaysEvidenceWithoutDuplicateSideEffects(t *testing.T) {
	dir := testProject(t, ArtifactSlideSpec)
	events := &eventRecorder{}
	idempotencyStore := newMemoryIdempotencyStore()
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("stable-call", "write_ppt", map[string]any{"content": "idempotent"}),
		toolCall("stable-call", "write_ppt", map[string]any{"content": "idempotent"}),
		finishCall("finish"),
	}}
	commits := 0
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "idempotent-tool", ProjectDir: dir,
		Context:        testPack(model.IntentExecute, model.ArtifactSpec, model.TargetSlide, false, "modify"),
		DomainTools:    fakeProvider{kind: ArtifactSlideSpec},
		Emitter:        events,
		Idempotency:    idempotencyStore,
		CommitMetadata: func(context.Context, CommitContext) error { commits++; return nil },
	})
	if outcome.Status != StatusCompleted || commits != 1 {
		t.Fatalf("outcome=%+v commits=%d", outcome, commits)
	}
	if events.count(model.EventToolStarted) != 1 || events.count(model.EventToolCompleted) != 1 {
		t.Fatalf("tool replay emitted duplicate business lifecycle: %+v", events.events)
	}
	raw, err := os.ReadFile(filepath.Join(dir, model.SlideSpecPath("s1")))
	if err != nil || string(raw) != "idempotent" {
		t.Fatalf("formal content=%q err=%v", raw, err)
	}
	record, err := idempotencyStore.GetIdempotency(context.Background(), "tool_call", "idempotent-tool", "stable-call")
	if err != nil || record.Status != "completed" || !strings.Contains(record.ResultJSON, `"evidence"`) {
		t.Fatalf("tool result did not persist replay evidence: %+v err=%v", record, err)
	}
}

func TestContextCompactionDropsOnlySupersededSlideImages(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "old", Content: []llm.ContentPart{
			{Type: "text", Text: `{"slide_id":"slide-1","source_hash":"old"}`},
			{Type: "image", ImageRef: "run:r/screenshot:old"},
		}},
		{Role: llm.RoleTool, ToolCallID: "other", Content: []llm.ContentPart{
			{Type: "text", Text: `{"slide_id":"slide-2","source_hash":"other"}`},
			{Type: "image", ImageRef: "run:r/screenshot:other"},
		}},
		{Role: llm.RoleTool, ToolCallID: "new", Content: []llm.ContentPart{
			{Type: "text", Text: `{"slide_id":"slide-1","source_hash":"new"}`},
			{Type: "image", ImageRef: "run:r/screenshot:new"},
		}},
	}
	got := pruneSupersededRenderImages(messages)
	if len(got[0].Content) != 1 || len(got[1].Content) != 2 || len(got[2].Content) != 2 {
		t.Fatalf("superseded image pruning mismatch: %+v", got)
	}
	if got[0].Content[0].Type != "text" || got[2].Content[1].ImageRef != "run:r/screenshot:new" {
		t.Fatalf("latest screenshot/source binding was not preserved: %+v", got)
	}
}

func TestRuntimePromptAndFinishSchemaRequireExplicitFinishForEveryStrategy(t *testing.T) {
	for _, strategy := range []ExecutionStrategy{StrategyChat, StrategySimple, StrategyComplex} {
		prompt := runtimeSystemPrompt(PhaseExecuting, strategy, "{}")
		if !strings.Contains(prompt, "All normal successful exits require finish(message=...). Ordinary assistant text never completes a run.") {
			t.Fatalf("%s prompt does not require explicit finish: %q", strategy, prompt)
		}
	}
	schemas := controlSchemas(StrategyChat, PhaseChat, model.IntentTalk)
	var finishDescription string
	var finishParameters map[string]any
	for _, schema := range schemas {
		if schema.Name == "finish" {
			finishDescription = schema.Description
			finishParameters = schema.Parameters
		}
	}
	if !strings.Contains(finishDescription, "所有策略的最终答复都必须通过 finish(message=...) 提交。普通 assistant 文本不是结束信号。") {
		t.Fatalf("finish schema description=%q", finishDescription)
	}
	properties, _ := finishParameters["properties"].(map[string]any)
	if len(properties) != 1 || properties["message"] == nil {
		t.Fatalf("finish schema must expose only message: %+v", finishParameters)
	}
}

func TestRuntimePromptAgentContractsDoNotRequireManagedFields(t *testing.T) {
	prompt := runtimeSystemPrompt(PhaseExecuting, StrategySimple, "{}")
	prefix := "authoritative schemas: "
	start := strings.Index(prompt, prefix)
	if start < 0 {
		t.Fatal("compiled contracts are missing from the system prompt")
	}
	start += len(prefix)
	end := strings.Index(prompt[start:], "\n</current_resource_contracts>")
	if end < 0 {
		t.Fatal("compiled contract boundary is missing")
	}
	var contracts map[string]struct {
		Fields      []string       `json:"agent_fields"`
		Required    []string       `json:"required"`
		FieldSchema map[string]any `json:"field_schema"`
		Example     map[string]any `json:"example"`
	}
	if err := json.Unmarshal([]byte(prompt[start:start+end]), &contracts); err != nil {
		t.Fatalf("compiled contracts are not JSON: %v", err)
	}
	for name, contract := range contracts {
		managed, err := pptschema.RuntimeManagedFields(name)
		if err != nil {
			t.Fatal(err)
		}
		for field := range managed {
			if contains(contract.Fields, field) || contains(contract.Required, field) ||
				contract.FieldSchema[field] != nil || contract.Example[field] != nil {
				t.Errorf("%s prompt contract leaks runtime-managed field %q: %+v", name, field, contract)
			}
		}
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestComplexPlanningDisclosesNoWriteAndFirstPlanEntersExecuting(t *testing.T) {
	dir := testProject(t, ArtifactSlideSpec)
	agent := &scriptedAgent{responses: []AgentResponse{
		planCall("plan", true), toolCall("write", "write_ppt", map[string]any{"content": "next"}), finishCall("finish"),
	}}
	events := &eventRecorder{}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "complex", ProjectDir: dir,
		Context: testPack(model.IntentExecute, model.ArtifactSpec, model.TargetSlide, false, "重建当前页结构"),
		Emitter: events, DomainTools: fakeProvider{kind: ArtifactSlideSpec},
	})
	if outcome.Status != StatusCompleted {
		t.Fatalf("outcome=%+v", outcome)
	}
	for _, schema := range agent.requests[0].Tools {
		if schema.Name == "write_ppt" {
			t.Fatal("planning disclosed a write tool")
		}
	}
	if agent.requests[1].Phase != PhaseExecuting || events.count(model.EventPlanUpdated) != 1 {
		t.Fatalf("requests=%+v events=%+v", agent.requests, events.events)
	}
	if events.count(model.EventToolStarted) != 1 {
		t.Fatalf("runtime control tools leaked into business tool events: %+v", events.events)
	}
}

func TestPlanStepsNeverCreateAnotherLoop(t *testing.T) {
	dir := testProject(t, ArtifactSlideSpec)
	agent := &scriptedAgent{responses: []AgentResponse{
		planCall("plan", false),
		planCall("progress", true),
		toolCall("write", "write_ppt", map[string]any{"content": "next"}),
		finishCall("finish"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "one-loop", ProjectDir: dir,
		Context:     testPack(model.IntentExecute, model.ArtifactSpec, model.TargetSlide, false, "重建当前页结构"),
		DomainTools: fakeProvider{kind: ArtifactSlideSpec},
	})
	for _, request := range agent.requests {
		if request.LoopID != outcome.LoopID {
			t.Fatalf("step update created another loop: %s != %s", request.LoopID, outcome.LoopID)
		}
	}
}

func TestSimpleProducesNoPlan(t *testing.T) {
	dir := testProject(t, ArtifactSlideSpec)
	events := &eventRecorder{}
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("write", "write_ppt", map[string]any{"content": "next"}), finishCall("finish"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "simple", ProjectDir: dir,
		Context: testPack(model.IntentExecute, model.ArtifactSpec, model.TargetSlide, false, "修改当前页标题"),
		Emitter: events, DomainTools: fakeProvider{kind: ArtifactSlideSpec},
	})
	if outcome.Strategy != StrategySimple || events.count(model.EventPlanUpdated) != 0 {
		t.Fatalf("outcome=%+v events=%+v", outcome, events.events)
	}
}

func TestSimpleScopeExpansionUpgradesSameRunToComplex(t *testing.T) {
	dir := testProject(t, ArtifactSlideSpec)
	events := &eventRecorder{}
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("expand", "edit_ppt", nil), planCall("plan", true),
		toolCall("write", "write_ppt", map[string]any{"content": "next"}), finishCall("finish"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "upgrade", ProjectDir: dir,
		Context: testPack(model.IntentExecute, model.ArtifactSpec, model.TargetSlide, false, "修改当前页标题"),
		Emitter: events, DomainTools: fakeProvider{kind: ArtifactSlideSpec},
	})
	if outcome.Strategy != StrategyComplex {
		t.Fatalf("outcome=%+v events=%+v", outcome, events.events)
	}
	for _, request := range agent.requests {
		if request.LoopID != outcome.LoopID {
			t.Fatal("upgrade replaced the loop")
		}
	}
}

func TestGateRejectionContinuesSameLoop(t *testing.T) {
	dir := testProject(t, ArtifactSlideSpec)
	events := &eventRecorder{}
	agent := &scriptedAgent{responses: []AgentResponse{
		{
			Text: "我先尝试提交当前结果。",
			ToolCalls: []llm.ToolCall{{
				ID: "first", Name: "finish", Args: map[string]any{"message": "not ready"},
			}},
		},
		toolCall("write", "write_ppt", map[string]any{"content": "next"}),
		finishCall("second"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "gate-continue", ProjectDir: dir,
		Context: testPack(model.IntentExecute, model.ArtifactSpec, model.TargetSlide, false, "修改当前页标题"),
		Emitter: events, DomainTools: fakeProvider{kind: ArtifactSlideSpec},
	})
	if outcome.Status != StatusCompleted {
		t.Fatalf("outcome=%+v events=%+v", outcome, events.events)
	}
	for _, request := range agent.requests {
		if request.LoopID != outcome.LoopID {
			t.Fatal("gate rejection replaced the loop")
		}
	}
	if len(agent.requests) < 2 || len(agent.requests[1].Messages) != 2 {
		t.Fatalf("completion rejection did not return to the same context: %+v", agent.requests)
	}
	rejectedCall := agent.requests[1].Messages[0]
	rejectionObservation := agent.requests[1].Messages[1]
	if rejectedCall.Role != llm.RoleAssistant || len(rejectedCall.ToolCalls) != 1 ||
		rejectedCall.ToolCalls[0].ID != "first" || rejectedCall.ToolCalls[0].Name != "finish" ||
		rejectedCall.Text() != "我先尝试提交当前结果。" ||
		rejectionObservation.Role != llm.RoleTool ||
		rejectionObservation.ToolCallID != "first" ||
		!strings.Contains(rejectionObservation.Text(), CodeCompletionGateBlocked) {
		t.Fatalf("completion rejection context=%+v", agent.requests[1].Messages)
	}
}

func TestStaleEvidenceDoesNotSatisfyGate(t *testing.T) {
	dir := testProject(t, ArtifactSlideHTML)
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("write-1", "write_ppt", map[string]any{"content": "one"}),
		toolCall("render", "render_slide", nil),
		toolCall("write-2", "write_ppt", map[string]any{"content": "two"}),
		finishCall("finish-1"), finishCall("finish-2"), finishCall("finish-3"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "stale", ProjectDir: dir,
		Context:     testPack(model.IntentExecute, model.ArtifactPresentation, model.TargetSlide, false, "修改当前页文案"),
		DomainTools: fakeProvider{kind: ArtifactSlideHTML},
	})
	if outcome.Status != StatusFailed || outcome.Code != CodeGateRejectedRepeated {
		t.Fatalf("outcome=%+v", outcome)
	}
}

func TestIdenticalGateRejectionThreeTimesBlowsFuse(t *testing.T) {
	dir := testProject(t, ArtifactSlideSpec)
	agent := &scriptedAgent{responses: []AgentResponse{finishCall("1"), finishCall("2"), finishCall("3")}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "fuse", ProjectDir: dir,
		Context:     testPack(model.IntentExecute, model.ArtifactSpec, model.TargetSlide, false, "修改当前页标题"),
		DomainTools: fakeProvider{kind: ArtifactSlideSpec},
	})
	if outcome.Code != CodeGateRejectedRepeated || outcome.Status != StatusFailed {
		t.Fatalf("outcome=%+v", outcome)
	}
}

type fakePrompter struct {
	calls int
}

func (p *fakePrompter) Ask(_ context.Context, question model.QuestionAskedPayload) (model.QuestionAnswer, string, error) {
	p.calls++
	if question.QuestionID == "" {
		return model.QuestionAnswer{}, "", errors.New("missing question id")
	}
	return model.QuestionAnswer{CustomText: "用户回答"}, "用户回答", nil
}

type checkpointRecorder struct {
	checkpoints []RuntimeCheckpoint
}

func (r *checkpointRecorder) SaveCheckpoint(_ context.Context, checkpoint RuntimeCheckpoint) error {
	r.checkpoints = append(r.checkpoints, checkpoint)
	return nil
}

func TestAskUserCheckpointsAndResumesSameLoop(t *testing.T) {
	dir := testProject(t, ArtifactSlideSpec)
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("question", "ask_user", map[string]any{"question": "选择方向？"}), finishCall("finish"),
	}}
	prompter, checkpoints := &fakePrompter{}, &checkpointRecorder{}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "ask", ProjectDir: dir,
		Context:  testPack(model.IntentAsk, model.ArtifactSpec, model.TargetSlide, false, "讨论当前页"),
		Prompter: prompter, Checkpoint: checkpoints, DomainTools: fakeProvider{kind: ArtifactSlideSpec},
	})
	if outcome.Status != StatusCompleted || prompter.calls != 1 || len(checkpoints.checkpoints) != 1 {
		t.Fatalf("outcome=%+v calls=%d checkpoints=%d", outcome, prompter.calls, len(checkpoints.checkpoints))
	}
	if agent.requests[0].LoopID != agent.requests[1].LoopID || checkpoints.checkpoints[0].Phase != PhaseWaitingInput {
		t.Fatal("ask_user did not resume the same waiting loop")
	}
}

func TestCommitOnlyAfterGateAcceptance(t *testing.T) {
	dir := testProject(t, ArtifactSlideSpec)
	commits := 0
	agent := &scriptedAgent{responses: []AgentResponse{
		finishCall("reject"), toolCall("write", "write_ppt", map[string]any{"content": "committed"}), finishCall("accept"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "commit", ProjectDir: dir,
		Context:        testPack(model.IntentExecute, model.ArtifactSpec, model.TargetSlide, false, "修改当前页标题"),
		DomainTools:    fakeProvider{kind: ArtifactSlideSpec},
		CommitMetadata: func(context.Context, CommitContext) error { commits++; return nil },
	})
	if outcome.Status != StatusCompleted || commits != 1 {
		t.Fatalf("outcome=%+v commits=%d", outcome, commits)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, model.SlideSpecPath("s1")))
	if string(raw) != "committed" {
		t.Fatalf("formal content=%q", raw)
	}
}

func TestAcceptedRenderProofFlowsThroughRuntimeCommitContext(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetSlide)
	next := slideModel("slide-01", "Original")
	next.SpeakerNotes = "updated notes"
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("spec", "write_ppt", map[string]any{
			"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-01", Part: "spec"}),
			"content":  string(mustJSONValue(next)),
		}),
		toolCall("render", "render_slide", map[string]any{"slide_id": "slide-01"}),
		finishCall("done"),
	}}
	var committed CommitContext
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "proof-commit", ProjectDir: dir, Context: pack,
		DomainTools: DefaultDomainToolProvider{Pack: pack, Renderer: &recordingRenderer{}},
		CommitMetadata: func(_ context.Context, value CommitContext) error {
			committed = value
			return nil
		},
	})
	if outcome.Status != StatusCompleted || len(committed.MaterializationProofs) != 1 {
		t.Fatalf("outcome=%+v commit=%+v", outcome, committed)
	}
	proof := committed.MaterializationProofs[0]
	if proof.SlideID != "slide-01" || proof.HTMLRevision != 1 || proof.SourceSpecRevision != 2 {
		t.Fatalf("unexpected proof=%+v", proof)
	}
}

func TestFailedAndCanceledRunsDoNotModifyFormalFiles(t *testing.T) {
	for _, test := range []struct {
		name   string
		cancel bool
	}{
		{name: "failed"}, {name: "canceled", cancel: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := testProject(t, ArtifactSlideSpec)
			ctx, cancel := context.WithCancel(context.Background())
			agent := &scriptedAgent{responses: []AgentResponse{toolCall("write", "write_ppt", map[string]any{"content": "dirty"})}, err: errors.New("agent stopped")}
			if test.cancel {
				agent.err = context.Canceled
			}
			outcome := NewRuntime(agent).Run(ctx, RuntimeInput{
				RunID: test.name, ProjectDir: dir,
				Context:     testPack(model.IntentExecute, model.ArtifactSpec, model.TargetSlide, false, "修改当前页标题"),
				DomainTools: fakeProvider{kind: ArtifactSlideSpec},
			})
			cancel()
			raw, _ := os.ReadFile(filepath.Join(dir, model.SlideSpecPath("s1")))
			if string(raw) != "formal" || (test.cancel && outcome.Status != StatusCanceled) || (!test.cancel && outcome.Status != StatusFailed) {
				t.Fatalf("outcome=%+v formal=%q", outcome, raw)
			}
			if _, err := os.Stat(filepath.Join(dir, ".staging", test.name)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("uncommitted staging was not cleaned up: %v", err)
			}
		})
	}
}

func TestProviderErrorAfterContextCancellationFinishesCanceled(t *testing.T) {
	dir := testProject(t, ArtifactSlideSpec)
	ctx, cancel := context.WithCancel(context.Background())
	events := &eventRecorder{}
	outcome := NewRuntime(cancelingAgent{cancel: cancel}).Run(ctx, RuntimeInput{
		RunID: "provider-canceled", ProjectDir: dir,
		Context:     testPack(model.IntentExecute, model.ArtifactSpec, model.TargetSlide, false, "修改当前页标题"),
		DomainTools: fakeProvider{kind: ArtifactSlideSpec}, Emitter: events,
	})
	if outcome.Status != StatusCanceled || outcome.Code != CodeCanceled {
		t.Fatalf("outcome=%+v", outcome)
	}
	for _, event := range events.events {
		if event.kind != model.EventRunFinished {
			continue
		}
		finished := event.payload.(model.RunFinishedPayload)
		if finished.Status != "canceled" || finished.Error != nil {
			t.Fatalf("canceled projection=%+v", finished)
		}
		return
	}
	t.Fatal("canceled run.finished was not emitted")
}

func TestProviderUnavailableUsesAuthoritativeTransientProjection(t *testing.T) {
	events := &eventRecorder{}
	cause := "POST https://provider.example/v1/chat body=secret api_key=sk-secret"
	agent := &scriptedAgent{err: fmt.Errorf("%w: %s", llm.ErrUnavailable, cause)}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "provider-unavailable", ProjectDir: t.TempDir(),
		Context:     testPack(model.IntentTalk, model.ArtifactSpec, model.TargetSlide, false, "review"),
		DomainTools: fakeProvider{kind: ArtifactSlideSpec}, Emitter: events,
	})
	if outcome.Status != StatusFailed || outcome.Code != "PROVIDER_UNAVAILABLE" {
		t.Fatalf("outcome=%+v", outcome)
	}
	var finished model.RunFinishedPayload
	for _, event := range events.events {
		if event.kind == model.EventRunFinished {
			finished = event.payload.(model.RunFinishedPayload)
		}
	}
	if finished.Error == nil || finished.Error.Code != "PROVIDER_UNAVAILABLE" || !finished.Error.Retryable ||
		finished.Error.Message != model.ErrorDefinitionFor("PROVIDER_UNAVAILABLE").SafeMessage {
		t.Fatalf("run.finished projection=%+v", finished)
	}
	publicRaw, err := json.Marshal(finished)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"provider.example", "body=secret", "sk-secret", "POST "} {
		if strings.Contains(string(publicRaw), forbidden) {
			t.Fatalf("public terminal error leaked %q: %s", forbidden, publicRaw)
		}
	}
}

func TestCancellationPairsEveryStartedToolBeforeCanceledTerminal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	emitter := &cancelOnToolStartedEmitter{cancel: cancel}
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("blocking-call", "search_refs", map[string]any{"query": "x"}),
	}}
	outcome := NewRuntime(agent).Run(ctx, RuntimeInput{
		RunID: "cancel-tool", ProjectDir: t.TempDir(),
		Context: testPack(model.IntentTalk, model.ArtifactSpec, model.TargetSlide, false, "search"),
		Emitter: emitter, DomainTools: blockingReadProvider{},
	})
	if outcome.Status != StatusCanceled || outcome.Code != CodeCanceled {
		t.Fatalf("outcome=%+v", outcome)
	}
	emitter.mu.Lock()
	events := append([]recordedEvent(nil), emitter.events...)
	emitter.mu.Unlock()
	startedIndex, completedIndex, terminalIndex := -1, -1, -1
	for index, event := range events {
		switch event.kind {
		case model.EventToolStarted:
			startedIndex = index
		case model.EventToolCompleted:
			payload := event.payload.(model.ToolCompletedPayload)
			if payload.CallID == "blocking-call" && payload.Error != nil && payload.Error.Code == CodeCanceled {
				completedIndex = index
			}
		case model.EventMessageFinal:
			t.Fatal("canceled run emitted a successful message.final")
		case model.EventRunFinished:
			if event.payload.(model.RunFinishedPayload).Status == "canceled" {
				terminalIndex = index
			}
		}
	}
	if startedIndex < 0 || completedIndex <= startedIndex || terminalIndex <= completedIndex {
		t.Fatalf("canceled lifecycle is not paired and ordered: %+v", events)
	}
}

func TestEmptyProjectComplexPresentationGenerationUsesUnifiedPPTTargets(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetDeck)
	emptyDeck := deckModel("p1", []string{})
	if err := os.WriteFile(filepath.Join(dir, "outline.json"), mustJSONValue(emptyDeck), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "slides")); err != nil {
		t.Fatal(err)
	}
	pack.Outline.Outline = emptyDeck
	pack.Outline.Summaries = []contextengine.SlideSummary{}
	pack.Target.SlideSpec = nil
	pack.Revisions.SlideSpecs = map[string]int{}
	pack.Revisions.SlideHTML = map[string]int{}
	pack.WorkSpec.Options.DesiredSlideCount = 2
	targetDeck := deckModel("p1", []string{"slide-01", "slide-02"})
	slideOne := slideModel("slide-01", "One")
	slideTwo := slideModel("slide-02", "Two")
	agent := &scriptedAgent{responses: []AgentResponse{
		planCall("plan", false),
		toolCall("outline", "write_ppt", map[string]any{
			"resource": resourceArgs(Resource{Type: "deck", Part: "outline"}),
			"content":  string(mustJSONValue(targetDeck)),
		}),
		toolCall("design", "write_ppt", map[string]any{
			"resource": resourceArgs(Resource{Type: "deck", Part: "design"}),
			"content":  string(mustJSONValue(designModel())),
		}),
		toolCall("spec-1", "write_ppt", map[string]any{
			"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-01", Part: "spec"}),
			"content":  string(mustJSONValue(slideOne)),
		}),
		toolCall("html-1", "write_ppt", map[string]any{
			"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-01", Part: "html"}),
			"content":  strings.Replace(validToolHTML, "Original", "One", 1),
		}),
		toolCall("render-1", "render_slide", map[string]any{"slide_id": "slide-01"}),
		toolCall("spec-2", "write_ppt", map[string]any{
			"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-02", Part: "spec"}),
			"content":  string(mustJSONValue(slideTwo)),
		}),
		toolCall("html-2", "write_ppt", map[string]any{
			"resource": resourceArgs(Resource{Type: "slide", SlideID: "slide-02", Part: "html"}),
			"content":  strings.Replace(validToolHTML, "Original", "Two", 1),
		}),
		toolCall("render-2", "render_slide", map[string]any{"slide_id": "slide-02"}),
		planCall("complete-plan", true),
		finishCall("finish"),
	}}
	renderer := &recordingRenderer{}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "empty-complex", ProjectDir: dir, Context: pack,
		DomainTools: DefaultDomainToolProvider{Pack: pack, Renderer: renderer},
	})
	if outcome.Status != StatusCompleted || outcome.Strategy != StrategyComplex {
		t.Fatalf("outcome=%+v", outcome)
	}
	for _, slideID := range []string{"slide-01", "slide-02"} {
		if _, err := os.Stat(filepath.Join(dir, model.SlideSpecPath(slideID))); err != nil {
			t.Fatalf("%s model missing: %v", slideID, err)
		}
		if _, err := os.Stat(filepath.Join(dir, model.SlideHTMLPath(slideID))); err != nil {
			t.Fatalf("%s HTML missing: %v", slideID, err)
		}
	}
}

type traceRecorder struct {
	events []TraceEvent
}

func (r *traceRecorder) Record(event TraceEvent) {
	r.events = append(r.events, event)
}

func TestPublicReasoningToolProjectionAndTerminalOrder(t *testing.T) {
	dir := testProject(t, ArtifactSlideSpec)
	events := &eventRecorder{}
	traces := &traceRecorder{}
	agent := &scriptedAgent{responses: []AgentResponse{
		{
			Text: "我会先确认当前页面结构，再进行局部更新。",
			ToolCalls: []llm.ToolCall{{
				ID: "write", Name: "write_ppt",
				Args: map[string]any{
					"target":  map[string]any{"type": "slide", "slide_id": "s1"},
					"content": "<section>private html</section>",
				},
			}},
		},
		finishCall("finish"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "public", ProjectDir: dir,
		Context: testPack(model.IntentExecute, model.ArtifactSpec, model.TargetSlide, false, "修改当前页标题"),
		Emitter: events, Trace: traces, DomainTools: fakeProvider{kind: ArtifactSlideSpec},
	})
	if outcome.Status != StatusCompleted {
		t.Fatalf("outcome=%+v", outcome)
	}
	if events.count(model.EventMessageReasoning) != 1 ||
		events.count(model.EventToolStarted) != 1 ||
		events.count(model.EventToolCompleted) != 1 ||
		events.count(model.EventMessageFinal) != 1 ||
		events.count(model.EventRunFinished) != 1 {
		t.Fatalf("public events=%+v", events.events)
	}
	raw, _ := json.Marshal(events.events)
	publicText := string(raw)
	for _, forbidden := range []string{
		"<section>", `"args"`, `"content"`,
		"screenshot_path", `"hash"`, `"data"`,
	} {
		if strings.Contains(publicText, forbidden) {
			t.Fatalf("public payload leaked %q: %s", forbidden, publicText)
		}
	}
	if len(agent.requests) < 2 || len(agent.requests[1].Messages) < 2 {
		t.Fatalf("assistant tool turn was not retained: %+v", agent.requests)
	}
	retained := agent.requests[1].Messages[0]
	if retained.Text() != "我会先确认当前页面结构，再进行局部更新。" ||
		len(retained.ToolCalls) != 1 || retained.ToolCalls[0].ID != "write" ||
		retained.ToolCalls[0].Name != "write_ppt" {
		t.Fatalf("assistant context=%+v", retained)
	}
	finalIndex, finishedIndex := -1, -1
	for index, event := range events.events {
		if event.kind == model.EventMessageFinal {
			finalIndex = index
		}
		if event.kind == model.EventRunFinished {
			finishedIndex = index
		}
	}
	if finalIndex < 0 || finishedIndex != len(events.events)-1 || finalIndex >= finishedIndex {
		t.Fatalf("terminal order=%+v", events.events)
	}
	foundInternalTrace := false
	for _, event := range traces.events {
		if event.Type == "context.assembled" || event.Type == "strategy.selected" ||
			event.Type == "phase.changed" || event.Type == "completion.checked" {
			foundInternalTrace = true
		}
	}
	if !foundInternalTrace {
		t.Fatalf("internal trace was not recorded: %+v", traces.events)
	}
}

func TestPlanDiffProducesOneMilestonePerNewCompletion(t *testing.T) {
	dir := testProject(t, ArtifactSlideSpec)
	events := &eventRecorder{}
	agent := &scriptedAgent{responses: []AgentResponse{
		planCall("plan-1", false),
		planCall("plan-2", true),
		planCall("plan-3", true),
		toolCall("write", "write_ppt", map[string]any{"content": "next"}),
		finishCall("finish"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "milestone", ProjectDir: dir,
		Context: testPack(model.IntentExecute, model.ArtifactSpec, model.TargetSlide, false, "重建当前页结构"),
		Emitter: events, DomainTools: fakeProvider{kind: ArtifactSlideSpec},
	})
	if outcome.Status != StatusCompleted {
		t.Fatalf("outcome=%+v", outcome)
	}
	if events.count(model.EventPlanUpdated) != 3 || events.count(model.EventMessageMilestone) != 1 {
		t.Fatalf("events=%+v", events.events)
	}
	for _, event := range events.events {
		if event.kind == model.EventMessageMilestone {
			payload := event.payload.(model.MessageMilestonePayload)
			if len(payload.CompletedStepIDs) != 1 || payload.CompletedStepIDs[0] != "work" {
				t.Fatalf("milestone=%+v", payload)
			}
		}
	}
}

func testPack(intent model.InteractionIntent, artifact model.Artifact, level model.TargetLevel, empty bool, instruction string) contextengine.ContextPack {
	order := []string{"s1"}
	summaries := []contextengine.SlideSummary{{ID: "s1", Title: "Old", State: string(model.MaterializationFresh)}}
	if empty {
		order, summaries = []string{}, []contextengine.SlideSummary{}
	}
	target := model.RunTarget{Artifact: artifact, Level: level}
	if level == model.TargetSlide {
		target.SlideID = "s1"
	}
	slide := &spec.SlideSpec{SchemaVersion: spec.SchemaVersion, Revision: 1, SlideID: "s1"}
	if empty || level == model.TargetDeck {
		slide = nil
	}
	return contextengine.ContextPack{
		SchemaVersion: contextengine.SchemaVersion,
		WorkSpec: model.WorkSpec{
			Target: target, Interaction: model.RunInteraction{Intent: intent}, Instruction: instruction,
		},
		Project: contextengine.ProjectContext{ID: "p1", Title: "Deck"},
		Outline: contextengine.OutlineContext{Outline: spec.Outline{
			SchemaVersion: spec.SchemaVersion, ProjectID: "p1", SlideOrder: order,
		}, Summaries: summaries},
		Target: contextengine.TargetContext{
			Artifact: artifact, Level: level, SlideSpec: slide,
			Materialization: &spec.Materialization{State: string(model.MaterializationFresh)},
		},
		SlideHTML: contextengine.SlideHTMLContext{Summaries: map[string]contextengine.HTMLSummary{}},
		Assets:    []contextengine.AssetCandidate{}, RelatedSlides: []contextengine.SlideSummary{},
		RecentTurns: []contextengine.RecentTurn{},
		Revisions: contextengine.RevisionRefs{
			SlideSpecs: map[string]int{"s1": 1}, SlideHTML: map[string]int{"s1": 1},
		},
		Manifest: contextengine.ContextManifest{
			ContextID: "ctx", RunID: "run", ThreadID: "thread", ProjectID: "p1",
			ReadOnly: intent != model.IntentExecute, BudgetTokens: 20000,
			Segments: []contextengine.ContextSegment{}, Refs: []contextengine.ContextRef{},
			Dropped: []contextengine.DroppedSegment{}, Warnings: []string{},
		},
	}
}

func testProject(t *testing.T, kind ArtifactKind) string {
	t.Helper()
	dir := t.TempDir()
	path := model.SlideSpecPath("s1")
	if kind == ArtifactSlideHTML {
		path = model.SlideHTMLPath("s1")
	}
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("formal"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

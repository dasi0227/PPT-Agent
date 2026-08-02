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

	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type scriptedAgent struct {
	mu        sync.Mutex
	responses []AgentResponse
	requests  []AgentRequest
	err       error
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
	ref := ArtifactRef{Kind: t.kind, ID: "s1", Path: model.SlideJSONPath("s1")}
	if t.kind == ArtifactPresentation {
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
	result.ChangedTargets = []ChangedTarget{{
		Type: "slide", SlideID: "s1", Hash: change.AfterHash, Fields: []string{"model"},
	}}
	if input.Args["evidence"] != false {
		kind := "schema"
		if t.kind == ArtifactPresentation {
			kind = "static"
		}
		result.Evidence = []Evidence{newEvidence(kind, TargetRef{Type: "slide", SlideID: "s1"}, change.AfterHash)}
	}
	return result
}

type fakeRenderTool struct{}

func (fakeRenderTool) Schema() ToolSchema {
	return ToolSchema{Name: "render_slide", Description: "test render", Parameters: objectSchema(nil, map[string]any{})}
}

func (fakeRenderTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	ref := ArtifactRef{Kind: ArtifactPresentation, ID: "s1", Path: model.SlideHTMLPath("s1")}
	raw, err := input.Transaction.Read(ref)
	if err != nil {
		return failedToolResult("RENDER_FAILED", err.Error(), false)
	}
	result := SuccessfulToolResult("rendered")
	result.Evidence = []Evidence{newEvidence("render", TargetRef{Type: "slide", SlideID: "s1"}, hashBytes(raw))}
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
	return AgentResponse{ToolCall: &llm.ToolCall{ID: id, Name: name, Args: args}}
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
	pack := testPack(model.IntentExecute, model.ArtifactBlueprint, model.TargetSlide, false, "修改当前页标题")
	if got := (StrategyRouter{}).Decide(pack).Strategy; got != StrategySimple {
		t.Fatalf("strategy=%s", got)
	}
}

func TestRouterSelectsComplexForEmptyWholeDeck(t *testing.T) {
	pack := testPack(model.IntentExecute, model.ArtifactBlueprint, model.TargetDeck, true, "生成产品发布演示")
	if got := (StrategyRouter{}).Decide(pack).Strategy; got != StrategyComplex {
		t.Fatalf("strategy=%s", got)
	}
}

func TestChatPlainTextRequiresLaterExplicitFinish(t *testing.T) {
	events := &eventRecorder{}
	agent := &scriptedAgent{responses: []AgentResponse{
		{Text: "这是尚未通过 finish 提交的分析。", ProviderReasoning: "provider state"},
		finishCall("finish"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "chat-explicit-finish", ProjectDir: t.TempDir(),
		Context: testPack(model.IntentTalk, model.ArtifactBlueprint, model.TargetSlide, false, "分析当前页"),
		Emitter: events, DomainTools: fakeProvider{kind: ArtifactSlide},
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
		messages[0].Content != "这是尚未通过 finish 提交的分析。" ||
		messages[0].ReasoningContent != "provider state" {
		t.Fatalf("assistant text or provider reasoning was not retained: %+v", messages[0])
	}
	if messages[1].Role != llm.RoleUser || messages[1].Content != noToolCallGuidance {
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

func TestRuntimePromptAndFinishSchemaRequireExplicitFinishForEveryStrategy(t *testing.T) {
	for _, strategy := range []ExecutionStrategy{StrategyChat, StrategySimple, StrategyComplex} {
		prompt := runtimeSystemPrompt(PhaseExecuting, strategy, "{}")
		if !strings.Contains(prompt, "所有策略的最终答复都必须通过 finish(message=...) 提交。普通 assistant 文本不是结束信号。") {
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

func TestComplexPlanningDisclosesNoWriteAndFirstPlanEntersExecuting(t *testing.T) {
	dir := testProject(t, ArtifactSlide)
	agent := &scriptedAgent{responses: []AgentResponse{
		planCall("plan", true), toolCall("write", "write_ppt", map[string]any{"content": "next"}), finishCall("finish"),
	}}
	events := &eventRecorder{}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "complex", ProjectDir: dir,
		Context: testPack(model.IntentExecute, model.ArtifactBlueprint, model.TargetSlide, false, "重建当前页结构"),
		Emitter: events, DomainTools: fakeProvider{kind: ArtifactSlide},
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
	dir := testProject(t, ArtifactSlide)
	agent := &scriptedAgent{responses: []AgentResponse{
		planCall("plan", false),
		planCall("progress", true),
		toolCall("write", "write_ppt", map[string]any{"content": "next"}),
		finishCall("finish"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "one-loop", ProjectDir: dir,
		Context:     testPack(model.IntentExecute, model.ArtifactBlueprint, model.TargetSlide, false, "重建当前页结构"),
		DomainTools: fakeProvider{kind: ArtifactSlide},
	})
	for _, request := range agent.requests {
		if request.LoopID != outcome.LoopID {
			t.Fatalf("step update created another loop: %s != %s", request.LoopID, outcome.LoopID)
		}
	}
}

func TestSimpleProducesNoPlan(t *testing.T) {
	dir := testProject(t, ArtifactSlide)
	events := &eventRecorder{}
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("write", "write_ppt", map[string]any{"content": "next"}), finishCall("finish"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "simple", ProjectDir: dir,
		Context: testPack(model.IntentExecute, model.ArtifactBlueprint, model.TargetSlide, false, "修改当前页标题"),
		Emitter: events, DomainTools: fakeProvider{kind: ArtifactSlide},
	})
	if outcome.Strategy != StrategySimple || events.count(model.EventPlanUpdated) != 0 {
		t.Fatalf("outcome=%+v events=%+v", outcome, events.events)
	}
}

func TestSimpleScopeExpansionUpgradesSameRunToComplex(t *testing.T) {
	dir := testProject(t, ArtifactSlide)
	events := &eventRecorder{}
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("expand", "edit_ppt", nil), planCall("plan", true),
		toolCall("write", "write_ppt", map[string]any{"content": "next"}), finishCall("finish"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "upgrade", ProjectDir: dir,
		Context: testPack(model.IntentExecute, model.ArtifactBlueprint, model.TargetSlide, false, "修改当前页标题"),
		Emitter: events, DomainTools: fakeProvider{kind: ArtifactSlide},
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
	dir := testProject(t, ArtifactSlide)
	events := &eventRecorder{}
	agent := &scriptedAgent{responses: []AgentResponse{
		{
			Text:              "我先尝试提交当前结果。",
			ProviderReasoning: "finish provider state",
			ToolCall: &llm.ToolCall{
				ID: "first", Name: "finish", Args: map[string]any{"message": "not ready"},
			},
		},
		toolCall("write", "write_ppt", map[string]any{"content": "next"}),
		finishCall("second"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "gate-continue", ProjectDir: dir,
		Context: testPack(model.IntentExecute, model.ArtifactBlueprint, model.TargetSlide, false, "修改当前页标题"),
		Emitter: events, DomainTools: fakeProvider{kind: ArtifactSlide},
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
		rejectedCall.Content != "我先尝试提交当前结果。" ||
		rejectedCall.ReasoningContent != "finish provider state" ||
		rejectionObservation.Role != llm.RoleTool ||
		rejectionObservation.ToolCallID != "first" ||
		!strings.Contains(rejectionObservation.Content, CodeCompletionGateBlocked) {
		t.Fatalf("completion rejection context=%+v", agent.requests[1].Messages)
	}
}

func TestStaleEvidenceDoesNotSatisfyGate(t *testing.T) {
	dir := testProject(t, ArtifactPresentation)
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("write-1", "write_ppt", map[string]any{"content": "one"}),
		toolCall("render", "render_slide", nil),
		toolCall("write-2", "write_ppt", map[string]any{"content": "two"}),
		finishCall("finish-1"), finishCall("finish-2"), finishCall("finish-3"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "stale", ProjectDir: dir,
		Context:     testPack(model.IntentExecute, model.ArtifactPresentation, model.TargetSlide, false, "修改当前页文案"),
		DomainTools: fakeProvider{kind: ArtifactPresentation},
	})
	if outcome.Status != StatusFailed || outcome.Code != CodeGateRejectedRepeated {
		t.Fatalf("outcome=%+v", outcome)
	}
}

func TestIdenticalGateRejectionThreeTimesBlowsFuse(t *testing.T) {
	dir := testProject(t, ArtifactSlide)
	agent := &scriptedAgent{responses: []AgentResponse{finishCall("1"), finishCall("2"), finishCall("3")}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "fuse", ProjectDir: dir,
		Context:     testPack(model.IntentExecute, model.ArtifactBlueprint, model.TargetSlide, false, "修改当前页标题"),
		DomainTools: fakeProvider{kind: ArtifactSlide},
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
	dir := testProject(t, ArtifactSlide)
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("question", "ask_user", map[string]any{"question": "选择方向？"}), finishCall("finish"),
	}}
	prompter, checkpoints := &fakePrompter{}, &checkpointRecorder{}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "ask", ProjectDir: dir,
		Context:  testPack(model.IntentAsk, model.ArtifactBlueprint, model.TargetSlide, false, "讨论当前页"),
		Prompter: prompter, Checkpoint: checkpoints, DomainTools: fakeProvider{kind: ArtifactSlide},
	})
	if outcome.Status != StatusCompleted || prompter.calls != 1 || len(checkpoints.checkpoints) != 1 {
		t.Fatalf("outcome=%+v calls=%d checkpoints=%d", outcome, prompter.calls, len(checkpoints.checkpoints))
	}
	if agent.requests[0].LoopID != agent.requests[1].LoopID || checkpoints.checkpoints[0].Phase != PhaseWaitingInput {
		t.Fatal("ask_user did not resume the same waiting loop")
	}
}

func TestCommitOnlyAfterGateAcceptance(t *testing.T) {
	dir := testProject(t, ArtifactSlide)
	commits := 0
	agent := &scriptedAgent{responses: []AgentResponse{
		finishCall("reject"), toolCall("write", "write_ppt", map[string]any{"content": "committed"}), finishCall("accept"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "commit", ProjectDir: dir,
		Context:        testPack(model.IntentExecute, model.ArtifactBlueprint, model.TargetSlide, false, "修改当前页标题"),
		DomainTools:    fakeProvider{kind: ArtifactSlide},
		CommitMetadata: func(context.Context, ChangeSet) error { commits++; return nil },
	})
	if outcome.Status != StatusCompleted || commits != 1 {
		t.Fatalf("outcome=%+v commits=%d", outcome, commits)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, model.SlideJSONPath("s1")))
	if string(raw) != "committed" {
		t.Fatalf("formal content=%q", raw)
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
			dir := testProject(t, ArtifactSlide)
			ctx, cancel := context.WithCancel(context.Background())
			agent := &scriptedAgent{responses: []AgentResponse{toolCall("write", "write_ppt", map[string]any{"content": "dirty"})}, err: errors.New("agent stopped")}
			if test.cancel {
				agent.err = context.Canceled
			}
			outcome := NewRuntime(agent).Run(ctx, RuntimeInput{
				RunID: test.name, ProjectDir: dir,
				Context:     testPack(model.IntentExecute, model.ArtifactBlueprint, model.TargetSlide, false, "修改当前页标题"),
				DomainTools: fakeProvider{kind: ArtifactSlide},
			})
			cancel()
			raw, _ := os.ReadFile(filepath.Join(dir, model.SlideJSONPath("s1")))
			if string(raw) != "formal" || (test.cancel && outcome.Status != StatusCanceled) || (!test.cancel && outcome.Status != StatusFailed) {
				t.Fatalf("outcome=%+v formal=%q", outcome, raw)
			}
		})
	}
}

func TestEmptyProjectComplexPresentationGenerationUsesUnifiedPPTTargets(t *testing.T) {
	dir, pack := toolProject(t, model.ArtifactPresentation, model.TargetDeck)
	emptyDeck := deckModel("p1", []string{})
	if err := os.WriteFile(filepath.Join(dir, "deck.json"), mustJSONValue(emptyDeck), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "slides")); err != nil {
		t.Fatal(err)
	}
	pack.Deck.Deck = emptyDeck
	pack.Deck.Summaries = []contextengine.SlideSummary{}
	pack.Target.Slide = nil
	pack.Revisions.Slides = map[string]int{}
	pack.Revisions.Presentations = map[string]int{}
	pack.WorkSpec.Options.DesiredSlideCount = 2
	targetDeck := deckModel("p1", []string{"slide-01", "slide-02"})
	slideOne := slideModel("slide-01", "One")
	slideTwo := slideModel("slide-02", "Two")
	agent := &scriptedAgent{responses: []AgentResponse{
		planCall("plan", false),
		toolCall("global", "write_ppt", map[string]any{
			"target": map[string]any{"type": "global"},
			"content": map[string]any{"model": map[string]any{
				"deck": targetDeck, "design": designModel(),
			}},
		}),
		toolCall("slide-1", "write_ppt", map[string]any{
			"target": map[string]any{"type": "slide", "slide_id": "slide-01"},
			"content": map[string]any{
				"model": slideOne, "html": strings.Replace(validToolHTML, "Original", "One", 1),
			},
		}),
		toolCall("render-1", "render_slide", map[string]any{"slide_id": "slide-01"}),
		toolCall("slide-2", "write_ppt", map[string]any{
			"target": map[string]any{"type": "slide", "slide_id": "slide-02"},
			"content": map[string]any{
				"model": slideTwo, "html": strings.Replace(validToolHTML, "Original", "Two", 1),
			},
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
		if _, err := os.Stat(filepath.Join(dir, model.SlideJSONPath(slideID))); err != nil {
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
	dir := testProject(t, ArtifactSlide)
	events := &eventRecorder{}
	traces := &traceRecorder{}
	agent := &scriptedAgent{responses: []AgentResponse{
		{
			Text:              "我会先确认当前页面结构，再进行局部更新。",
			ProviderReasoning: "hidden provider chain of thought",
			ToolCall: &llm.ToolCall{
				ID: "write", Name: "write_ppt",
				Args: map[string]any{
					"target":  map[string]any{"type": "slide", "slide_id": "s1"},
					"content": "<section>private html</section>",
				},
			},
		},
		finishCall("finish"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "public", ProjectDir: dir,
		Context: testPack(model.IntentExecute, model.ArtifactBlueprint, model.TargetSlide, false, "修改当前页标题"),
		Emitter: events, Trace: traces, DomainTools: fakeProvider{kind: ArtifactSlide},
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
		"hidden provider chain of thought", "<section>", `"args"`, `"content"`,
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
	if retained.Content != "我会先确认当前页面结构，再进行局部更新。" ||
		retained.ReasoningContent != "hidden provider chain of thought" ||
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
	dir := testProject(t, ArtifactSlide)
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
		Context: testPack(model.IntentExecute, model.ArtifactBlueprint, model.TargetSlide, false, "重建当前页结构"),
		Emitter: events, DomainTools: fakeProvider{kind: ArtifactSlide},
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
	slide := &blueprint.Slide{SchemaVersion: blueprint.SchemaVersion, Revision: 1, SlideID: "s1"}
	if empty || level == model.TargetDeck {
		slide = nil
	}
	return contextengine.ContextPack{
		SchemaVersion: contextengine.SchemaVersion,
		WorkSpec: model.WorkSpec{
			Target: target, Interaction: model.RunInteraction{Intent: intent}, Instruction: instruction,
		},
		Project: contextengine.ProjectContext{ID: "p1", Title: "Deck"},
		Deck: contextengine.DeckContext{Deck: blueprint.Deck{
			SchemaVersion: blueprint.SchemaVersion, ProjectID: "p1", SlideOrder: order,
		}, Summaries: summaries},
		Target: contextengine.TargetContext{
			Artifact: artifact, Level: level, Slide: slide,
			Materialization: &blueprint.Materialization{State: string(model.MaterializationFresh)},
		},
		Presentation: contextengine.PresentationContext{Summaries: map[string]contextengine.HTMLSummary{}},
		Assets:       []contextengine.AssetCandidate{}, RelatedSlides: []contextengine.SlideSummary{},
		RecentTurns: []contextengine.RecentTurn{},
		Revisions: contextengine.RevisionRefs{
			Slides: map[string]int{"s1": 1}, Presentations: map[string]int{"s1": 1},
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
	path := model.SlideJSONPath("s1")
	if kind == ArtifactPresentation {
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

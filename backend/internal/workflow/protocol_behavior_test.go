package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
)

func TestSnapshotPreparationDoesNotMarkUnreadSourcesAsSeen(t *testing.T) {
	_, _, pack := generationPackFixture(t)
	image := llm.ContentPart{Type: "image", ImageRef: "project:p1/render:retained", MIMEType: "image/png"}
	for _, changed := range []bool{false, true} {
		req := prepareAgentRequest(AgentRequest{RunID: "versions", Context: pack})
		req.Messages = append(req.Messages, llm.Message{Role: llm.RoleTool, ToolCallID: "read-again", Content: []llm.ContentPart{{Type: "image", ImageRef: image.ImageRef, MIMEType: image.MIMEType}}})
		state := &RunState{messages: req.Messages}
		state.rememberResourceVersions()
		designKey := "ppt/" + (Resource{Type: "deck", Part: "constraints"}).Key()
		state.seenVersions[designKey] = "newer-checkpoint-version"
		if changed {
			req.Context.PresentationMeta.Meta.Title = "Fresh visible context"
		}
		state.messages = prepareAgentRequest(req).Messages
		manifestKey := "ppt/" + (Resource{Type: "deck", Part: "meta"}).Key()
		if state.seenVersions[manifestKey] != "" {
			t.Fatalf("unread source marked as seen after snapshot preparation: %v", state.seenVersions)
		}
		if state.seenVersions[designKey] != "newer-checkpoint-version" {
			t.Fatal("existing context overwrote the retained newer version")
		}
	}
}

func TestProtocolSeenVersionConflictAndRereadAfterCheckpoint(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	session, err := NewRunSession(dir, "versions")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	input := DomainToolInput{ProjectDir: dir, Session: session, Context: pack, Scope: pack.Command.Scope, Args: map[string]any{"type": "meta"}}
	read := (pptReadTool{}).Execute(context.Background(), input)
	state := &RunState{messages: appendBatchObservations(nil, []llm.ToolCall{{ID: "read", Name: "read_resource"}}, "", []ToolResult{read})}
	state.rememberResourceVersions()
	// Old checkpoint hashes must not substitute for content lost to compaction.
	raw, _ := json.Marshal(RuntimeCheckpoint{SeenVersions: state.seenVersions})
	var restored RuntimeCheckpoint
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	compacted := &RunState{seenVersions: restored.SeenVersions, messages: []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("summary"), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "summary"}}}}
	compacted.rememberResourceVersions()
	input.SeenVersions = compacted.seenVersions
	if _, err := modelSeenResourceVersion(input, Resource{Type: "deck", Part: "meta"}); !errors.Is(err, errResourceUnseen) {
		t.Fatalf("checkpoint hash authorized compacted content: %v", err)
	}
	input.SeenVersions, input.Messages = state.seenVersions, state.messages
	manifest := pack.PresentationMeta.Meta
	manifest.Title = "External change"
	current, _ := json.Marshal(manifest)
	writeGenerationFile(t, dir, ".meta.json", current)
	input.Args = map[string]any{"goal": "Agent edit"}
	tool := resourceEditTool{name: "edit_meta"}
	rejected := tool.Execute(context.Background(), input)
	if rejected.OK || rejected.Code != model.CodeContentConflict || !strings.Contains(rejected.Observation, "read") {
		t.Fatalf("stale edit=%+v", rejected)
	}
	unchanged, _ := session.Read(metaRef(pack))
	if string(unchanged) != string(current) {
		t.Fatal("conflict changed content")
	}
	input.Args = map[string]any{"type": "meta"}
	fresh := (pptReadTool{}).Execute(context.Background(), input)
	state.messages = appendBatchObservations(nil, []llm.ToolCall{{ID: "fresh", Name: "read_resource"}}, "", []ToolResult{fresh})
	state.rememberResourceVersions()
	input.SeenVersions = state.seenVersions
	input.Messages = state.messages
	input.Args = map[string]any{"goal": "Agent edit"}
	saved := tool.Execute(context.Background(), input)
	if !saved.OK {
		t.Fatalf("fresh edit=%+v", saved)
	}
	content := saved.Data["content"].(map[string]any)
	if content["title"] != "External change" || content["goal"] != "Agent edit" {
		t.Fatalf("saved=%v", content)
	}
	input.SeenVersions = nil
	input.Messages = nil
	if got := tool.Execute(context.Background(), input); got.OK {
		t.Fatal("unseen latest version authorized edit")
	}
}

func TestProtocolHTMLExactEditsAreAtomicAndStatusOnly(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	session, _ := NewRunSession(dir, "html")
	defer session.Discard()
	input := DomainToolInput{ProjectDir: dir, Session: session, Context: pack, Scope: pack.Command.Scope, Messages: testResourceMessages(t, dir, pack)}
	tool := resourceEditTool{name: "edit_html"}
	input.Args = map[string]any{"slide_id": generationSlide, "edits": []any{map[string]any{"old_text": "Original", "new_text": "New"}, map[string]any{"old_text": "missing anchor", "new_text": "Never"}}}
	result := tool.Execute(context.Background(), input)
	if result.OK || result.Data["edit_index"] != 1 {
		t.Fatalf("result=%+v", result)
	}
	after, _ := session.Read(slideHTMLRef(generationSlide))
	if string(after) != generationHTML {
		t.Fatal("partial replacement saved")
	}
	input.Args = map[string]any{"slide_id": generationSlide, "content": generationHTML}
	result = tool.Execute(context.Background(), input)
	if !result.OK || result.Observation != `{"status":"success"}` {
		t.Fatalf("result=%+v", result)
	}
	// A status-only write remains usable through its complete assistant input.
	writeCall := llm.ToolCall{ID: "write", Name: "edit_html", Args: input.Args}
	messages := appendBatchObservations(nil, []llm.ToolCall{writeCall}, "", []ToolResult{result})
	key := "ppt/slide:" + generationSlide + ":html"
	if visibleResourceHashes(messages)[key] != resourceContentHash(Resource{Part: "html"}, []byte(generationHTML)) {
		t.Fatal("full HTML input was not retained as usable source")
	}
	writeRound := append([]llm.Message(nil), messages...)
	for index, replacement := range []string{"Second", "Third"} {
		old := "Original"
		if index > 0 {
			old = "Second"
		}
		input.Messages, input.SeenVersions = messages, visibleResourceHashes(messages)
		input.Args = map[string]any{"slide_id": generationSlide, "edits": []any{map[string]any{"old_text": old, "new_text": replacement}}}
		patch := tool.Execute(context.Background(), input)
		if !patch.OK || patch.Observation != `{"status":"success"}` {
			t.Fatalf("patch=%+v", patch)
		}
		// Recovery must retain the dependency proof without adding source bodies.
		raw, err := marshalPersistedToolResult(patch, nil)
		if err != nil {
			t.Fatal(err)
		}
		var receipt persistedToolResult
		if err := json.Unmarshal([]byte(raw), &receipt); err != nil {
			t.Fatal(err)
		}
		call := llm.ToolCall{ID: replacement, Name: "edit_html", Args: input.Args}
		messages = appendBatchObservations(messages, []llm.ToolCall{call}, "", []ToolResult{receipt.restoreResult()})
		current, _ := session.Read(slideHTMLRef(generationSlide))
		if visibleResourceHashes(messages)[key] != resourceContentHash(Resource{Part: "html"}, current) {
			t.Fatal("successful patch required a redundant read")
		}
	}
	if visibleResourceHashes(messages[len(writeRound):])[key] != "" {
		t.Fatal("patch receipts survived without their complete source")
	}
	if visibleResourceHashes(writeRound[1:])[key] != "" {
		t.Fatal("status/hash alone was treated as full HTML")
	}
	// Failed or refused receipts must not use even a complete input as proof.
	for _, failed := range []ToolResult{failedToolResult(model.CodeContentConflict, "conflict"), refusedResourceEditResult()} {
		failed.ObservationMetadata = result.ObservationMetadata
		if visibleResourceHashes(appendBatchObservations(nil, []llm.ToolCall{writeCall}, "", []ToolResult{failed}))[key] != "" {
			t.Fatal("unsuccessful write supplied a version")
		}
	}
	for _, args := range []map[string]any{
		{"slide_id": generationSlide},
		{"slide_id": generationSlide, "content": generationHTML, "edits": []any{}},
		{"slide_id": generationSlide, "content": generationHTML, "expected_hash": "old"},
	} {
		input.Args = args
		if got := tool.Execute(context.Background(), input); got.OK {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestProtocolCommandPreservesFailureOutput(t *testing.T) {
	result := commandFailure(commandexec.Decision{}, commandexec.Result{Stdout: "partial", Stderr: "", ExitCode: -1}, &commandexec.Error{Code: commandexec.CodeTimeout, Message: "command exceeded the execution deadline"})
	view := errorObservation(t, result)
	if view["stdout"] != "partial" || view["exit_code"] != float64(-1) || view["code"] != commandexec.CodeTimeout || view["reason"] == nil {
		t.Fatalf("lost command failure output: %v", view)
	}
}

func TestProtocolRenderFailureKeepsImageAndOnlyPublicDiagnostics(t *testing.T) {
	d := RenderDiagnostics{ContentSize: map[string]int{"width": 1921, "height": 1140}, Overflow: map[string]bool{"horizontal": false, "vertical": true}, FontStatus: "loaded"}
	diagnostics := modelRenderDiagnostics(d, 1920, 1080)
	axes := diagnostics["overflow"].(map[string]string)
	if !strings.HasPrefix(axes["horizontal"], "未溢出") || !strings.Contains(axes["vertical"], "超出 60px") {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
	result := slideRenderResult(renderimage.Entry{ProjectID: "p1", SlideID: "sli_one", RunID: "run", ScreenshotID: "shot"}, RenderProof{}, d, "saved", false)
	bound := bindToolObservation(result, llm.ToolCall{ID: "render", Name: "render_slide", Args: map[string]any{"slide_id": generationSlide, "assess_quality": false}})
	messages := appendBatchObservations(nil, []llm.ToolCall{{ID: "render", Name: "render_slide"}}, "", []ToolResult{bound})
	toolReply := messages[len(messages)-1]
	if len(toolReply.Content) != 2 || toolReply.Content[1].Type != "image" || strings.Contains(toolReply.Text(), "private") || strings.Contains(toolReply.Text(), "font_status") {
		t.Fatalf("reply=%+v", toolReply)
	}
	if got := llm.NormalizeHistory(messages); !reflect.DeepEqual(got, messages) {
		t.Fatal("render result or image association lost in history")
	}
}

type protocolPlanPrompter struct {
	approvingPrompter
	decision, feedback string
}

func (p *protocolPlanPrompter) AskPlanApproval(_ context.Context, q model.PlanApprovalRequestedPayload) (model.PlanApprovalAnswer, error) {
	return model.PlanApprovalAnswer{InteractionID: q.InteractionID, Decision: p.decision, Feedback: p.feedback}, nil
}
func TestProtocolPlanRefusalReturnsFeedbackAndKeepsReplanningAvailable(t *testing.T) {
	for _, feedback := range []string{"", "  保留原文\n修改第二步  "} {
		t.Run(feedback, func(t *testing.T) {
			call := llm.ToolCall{ID: "proposal", Name: "create_plan", Args: map[string]any{"content": "Work", "steps": []any{"Step"}}}
			state := &RunState{runID: "plan-protocol", mode: model.ModePlan, phase: PhaseWaitingInput, scope: model.NewRunScope(model.ScopeAllPages), ledger: NewEvidenceLedger(), activeSkills: &ActiveSkillSet{}, plan: &Plan{Steps: []PlanStep{{Title: "Step", Status: PlanStepPending}}}, pendingPlanCall: pendingProposal(call.ID, "Work", "Step")}
			cp := &checkpointRecorder{}
			if result, stop := newTestRuntime(nil).awaitPlanApproval(context.Background(), RuntimeInput{Prompter: &protocolPlanPrompter{decision: "refuse", feedback: feedback}, Checkpoint: cp}, state); stop {
				t.Fatalf("result=%+v", result)
			}
			if state.phase != PhasePlanning || state.plan.Steps[0].Status != PlanStepPending || len(state.messages) != 2 || state.messages[1].ToolCallID != call.ID {
				t.Fatalf("refusal did not preserve planning: %+v", state)
			}
			var visible map[string]any
			_ = json.Unmarshal([]byte(state.messages[1].Text()), &visible)
			want := map[string]any{"status": "refuse"}
			if feedback != "" {
				want["feedback"] = feedback
			}
			if !reflect.DeepEqual(visible, want) {
				t.Fatalf("wrong refusal outcome: %v", visible)
			}
			if feedback != "" {
				projected := requestConversation(state.messages, state.runID)
				if !strings.Contains(transcriptText(projected), state.messages[1].Text()) {
					t.Fatal("feedback missing from current request")
				}
			}
			tools := schemasByName(controlSchemas(state.phase, state.mode))
			if tools["update_plan"] || !tools["create_plan"] || tools["finish_task"] {
				t.Fatalf("tools=%v", tools)
			}
			raw, _ := json.Marshal(cp.checkpoints[len(cp.checkpoints)-1])
			var restored RuntimeCheckpoint
			_ = json.Unmarshal(raw, &restored)
			if restored.PendingPlanCall != nil || restored.Phase != PhasePlanning {
				t.Fatal("refusal not durable")
			}
		})
	}
}

func TestProtocolQuestionAnswersUseStatesOriginalTextAndOrder(t *testing.T) {
	args := map[string]any{"questions": []any{
		map[string]any{"question": "问题一？", "reason": "确定范围", "options": []any{map[string]any{"label": "原选项", "description": "含义"}}},
		map[string]any{"question": "问题二？", "reason": "确定内容"}, map[string]any{"question": "问题三？", "reason": "确定顺序"},
	}}
	question := publicQuestion("run", "call", args)
	answer := model.QuestionAnswer{Answers: []model.QuestionFieldAnswer{
		{QuestionID: question.Questions[2].ID, Skipped: true}, {QuestionID: question.Questions[1].ID, CustomText: "  用户原文\n "}, {QuestionID: question.Questions[0].ID, SelectedOptionID: question.Questions[0].Options[0].ID},
	}}
	got := modelQuestionAnswers(args, question, answer)
	var observation struct {
		Answers []map[string]any `json:"answers"`
	}
	if err := json.Unmarshal([]byte(modelSuccessObservation(map[string]any{"answers": got})), &observation); err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{
		{"state": "answered", "text": "原选项"},
		{"state": "answered", "text": "  用户原文\n "},
		{"state": "skipped"},
	}
	if !reflect.DeepEqual(observation.Answers, want) {
		t.Fatalf("answers=%v", observation.Answers)
	}
}

type protocolScopePrompter struct {
	approvingPrompter
	decision, feedback string
}

func (p *protocolScopePrompter) AskScopeExpansion(_ context.Context, q model.ScopeExpansionRequestedPayload) (model.ScopeExpansionAnswer, error) {
	decision := p.decision
	if decision == "" {
		decision = "revise"
	}
	return model.ScopeExpansionAnswer{InteractionID: q.InteractionID, CallID: q.CallID, BaseRevision: q.BaseRevision, Decision: decision, Feedback: p.feedback}, nil
}
func (*protocolScopePrompter) ResumeAfterScopeExpansion(context.Context) {}
func TestProtocolScopeReviseAuthorizesAllIncludingNewPages(t *testing.T) {
	pack := scopeExpansionPack()
	original := model.NewRunScope(model.ScopeCurrentPage, "sli_one")
	pack.Command.Mode = model.ModeCreate
	pack.Command.Scope = original
	state := &RunState{runID: "scope", mode: model.ModeCreate, phase: PhaseExecuting, scope: original, pack: pack, ledger: NewEvidenceLedger(), activeSkills: &ActiveSkillSet{}}
	call := llm.ToolCall{ID: "expand", Name: "request_privilege", Args: map[string]any{"slide_ids": []any{"sli_two"}, "reason": "More content"}}
	committed := false
	input := RuntimeInput{Prompter: &protocolScopePrompter{}, CommitScopeExpansion: func(_ context.Context, s model.RunScope, cp RuntimeCheckpoint) error {
		committed = s.IncludeRunCreatedSlides && len(s.SlideIDs) == 3 && cp.PendingScopeExpansion == nil && len(cp.InteractionApplications) == 1 && cp.InteractionApplications[0].ID != ""
		return nil
	}}
	if outcome, stop := newTestRuntime(nil).executeControl(context.Background(), input, state, call, ""); stop {
		t.Fatalf("outcome=%+v", outcome)
	}
	if !committed || !state.scope.IncludeRunCreatedSlides || state.scope.Revision != 2 {
		t.Fatalf("scope=%+v", state.scope)
	}
	reply := state.messages[len(state.messages)-1]
	if reply.ToolCallID != call.ID || reply.Text() != `{"status":"success"}` {
		t.Fatalf("reply=%+v", reply)
	}
	prepared := prepareAgentRequest(agentRequestForState(input, state, nil))
	var runState struct {
		SlideIDs                []string `json:"run_scope"`
		IncludeRunCreatedSlides bool     `json:"include_run_created_slides"`
	}
	if err := json.Unmarshal([]byte(runtimeBody(contextSectionText(prepared.RuntimeContext, "run_state"))), &runState); err != nil || !runState.IncludeRunCreatedSlides || !reflect.DeepEqual(runState.SlideIDs, state.scope.SlideIDs) {
		t.Fatalf("revised authorization missing from model context: %+v (%v)", runState, err)
	}
}

func TestProtocolScopeRefusalKeepsAuthorizationAndOriginalFeedback(t *testing.T) {
	pack := scopeExpansionPack()
	original := model.NewRunScope(model.ScopeCurrentPage, "sli_one")
	pack.Command.Mode, pack.Command.Scope = model.ModeCreate, original
	state := &RunState{runID: "scope", mode: model.ModeCreate, phase: PhaseExecuting, scope: original, pack: pack, ledger: NewEvidenceLedger(), activeSkills: &ActiveSkillSet{}}
	call := llm.ToolCall{ID: "expand", Name: "request_privilege", Args: map[string]any{"slide_ids": []any{"sli_two"}, "reason": "More content"}}
	const feedback = "  保持当前页\n删去第二页工作  "
	events := &eventRecorder{}
	input := RuntimeInput{Prompter: &protocolScopePrompter{decision: "refuse", feedback: feedback}, Emitter: events,
		CommitScopeExpansion: func(context.Context, model.RunScope, RuntimeCheckpoint) error {
			t.Fatal("refusal changed authorization")
			return nil
		}}
	if outcome, stop := newTestRuntime(nil).executeControl(context.Background(), input, state, call, ""); stop {
		t.Fatal(outcome)
	}
	if !reflect.DeepEqual(state.scope, original) || state.phase != PhaseExecuting || state.pendingScopeExpansion != nil {
		t.Fatalf("state=%+v", state)
	}
	var reply map[string]any
	if json.Unmarshal([]byte(state.messages[len(state.messages)-1].Text()), &reply) != nil || !reflect.DeepEqual(reply, map[string]any{"status": "refuse", "feedback": feedback}) {
		t.Fatalf("reply=%v", reply)
	}
	if len(state.interactionApplications) != 1 || state.interactionApplications[0].ID == "" {
		t.Fatalf("refusal was not staged for atomic consumption: %+v", state.interactionApplications)
	}
	var accepted model.ScopeExpansionAnswer
	if err := json.Unmarshal(state.interactionApplications[0].Answer, &accepted); err != nil || accepted.Decision != "refuse" || accepted.Feedback != feedback {
		t.Fatalf("answer lost refusal or feedback: %+v %v", accepted, err)
	}

}

func TestProtocolRefusedPlanCannotFinishWithoutReplanning(t *testing.T) {
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("create", "create_plan", map[string]any{"content": "Work", "steps": []any{"Write"}}),
		finalReply(),
	}}
	outcome := newTestRuntime(agent).Run(context.Background(), RuntimeInput{RunID: "refusal", ProjectDir: t.TempDir(), Context: testPack(model.ModePlan, model.ScopeAllPages, false, "规划后等待决定"), Prompter: &protocolPlanPrompter{decision: "refuse"}})
	if outcome.Status == StatusCompleted || len(agent.requests) < 2 {
		t.Fatalf("outcome=%+v requests=%d", outcome, len(agent.requests))
	}
	schemas := schemasByName(agent.requests[1].Tools)
	if schemas["edit_html"] || !schemas["create_plan"] || schemas["update_plan"] || schemas["finish_task"] {
		t.Fatalf("post-refusal tools=%v", schemas)
	}
}

func TestProtocolDraftReplacementRetainsOnlyLatestProposal(t *testing.T) {
	pack := scopeExpansionPack()
	pack.Command.Scope = model.NewRunScope(model.ScopeAllPages, "sli_one", "sli_two", "sli_three")
	pack.Command.Mode = model.ModePlan
	state := &RunState{runID: "drafts", mode: model.ModePlan, phase: PhasePlanning, pack: pack, scope: pack.Command.Scope, ledger: NewEvidenceLedger(), activeSkills: &ActiveSkillSet{}}
	runtime := newTestRuntime(nil)
	input := RuntimeInput{Prompter: &protocolPlanPrompter{decision: "refuse"}}
	for i, title := range []string{"First draft", "Second draft"} {
		call := llm.ToolCall{ID: []string{"draft-one", "draft-two"}[i], Name: "create_plan", Args: map[string]any{"content": title, "steps": []any{title}}}
		if outcome, stop := runtime.executeControl(context.Background(), input, state, call, ""); stop {
			t.Fatalf("draft=%+v", outcome)
		}
	}
	if len(state.plan.Steps) != 1 || state.plan.Steps[0].Title != "Second draft" || state.plan.Steps[0].Status != PlanStepPending {
		t.Fatalf("old draft survived replacement: %+v", state.plan)
	}
}

func TestProtocolToolCatalog(t *testing.T) {
	registry := NewToolRegistry()
	_ = (DefaultDomainToolProvider{}).RegisterDomainTools(registry)
	for _, name := range []string{"init_outline", "arrange_outline", "write_html", "patch_html", "finish"} {
		if _, ok := registry.Descriptor(name); ok {
			t.Fatalf("legacy alias %s", name)
		}
	}
	for _, schema := range registry.Disclose(PhaseExecuting, model.ModeCreate, model.NewRunScope(model.ScopeAllPages)) {
		raw, _ := json.Marshal(schema.Parameters)
		if strings.Contains(string(raw), `"expected_hash"`) {
			t.Fatalf("hash input in %s", schema.Name)
		}
	}
}

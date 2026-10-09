package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestSnapshotPreparationDoesNotMarkUnreadSourcesAsSeen(t *testing.T) {
	_, _, pack := generationPackFixture(t)
	image := RunReadImage{CallID: "render", SlideID: generationSlide, ImageRef: "project/render:retained", MIMEType: "image/png"}
	for _, changed := range []bool{false, true} {
		req := prepareAgentRequest(AgentRequest{RunID: "versions", Context: pack, ReadImages: []RunReadImage{image}})
		req.Messages = append(req.Messages, llm.Message{Role: llm.RoleTool, ToolCallID: "read-again", Content: []llm.ContentPart{{Type: "image", ImageRef: image.ImageRef, MIMEType: image.MIMEType}}})
		state := &RunState{messages: req.Messages}
		state.rememberResourceVersions(0)
		designKey := "ppt/" + (Resource{Type: "deck", Part: "design"}).Key()
		state.seenVersions[designKey] = "newer-checkpoint-version"
		if changed {
			req.Context.PresentationManifest.Manifest.Title = "Fresh visible context"
		}
		state.messages = prepareAgentRequest(req).Messages
		manifestKey := "ppt/" + (Resource{Type: "deck", Part: "manifest"}).Key()
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
	input := DomainToolInput{ProjectDir: dir, Session: session, Context: pack, Scope: pack.Command.Scope, Args: map[string]any{"resource": "manifest"}}
	read := (pptReadTool{pack: pack}).Execute(context.Background(), input)
	state := &RunState{messages: appendBatchObservations(nil, []llm.ToolCall{{ID: "read", Name: "read_resource"}}, "", []ToolResult{read})}
	state.rememberResourceVersions(0)
	// A checkpoint retains the version even when all text messages are compacted.
	raw, _ := json.Marshal(RuntimeCheckpoint{SeenVersions: state.seenVersions})
	var restored RuntimeCheckpoint
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	input.SeenVersions = restored.SeenVersions
	manifest := pack.PresentationManifest.Manifest
	manifest.Title = "External change"
	current, _ := json.Marshal(manifest)
	writeGenerationFile(t, dir, ".manifest.json", current)
	input.Args = map[string]any{"goal": "Agent edit"}
	tool := resourceEditTool{pack: pack, name: "edit_manifest"}
	rejected := tool.Execute(context.Background(), input)
	if rejected.OK || rejected.Code != CodeContentConflict || !strings.Contains(rejected.Observation, "read") {
		t.Fatalf("stale edit=%+v", rejected)
	}
	unchanged, _ := session.Read(manifestRef(pack))
	if string(unchanged) != string(current) {
		t.Fatal("conflict changed content")
	}
	input.Args = map[string]any{"resource": "manifest"}
	fresh := (pptReadTool{pack: pack}).Execute(context.Background(), input)
	state.messages = appendBatchObservations(nil, []llm.ToolCall{{ID: "fresh", Name: "read_resource"}}, "", []ToolResult{fresh})
	state.rememberResourceVersions(0)
	input.SeenVersions = state.seenVersions
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

func TestProtocolHTMLExactEditsAreAtomicAndSummaryOnly(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	session, _ := NewRunSession(dir, "html")
	defer session.Discard()
	input := DomainToolInput{ProjectDir: dir, Session: session, Context: pack, Scope: pack.Command.Scope, Messages: testResourceMessages(t, dir, pack)}
	tool := resourceEditTool{pack: pack, name: "edit_html"}
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
	if !result.OK || result.Data["changed"] != false {
		t.Fatalf("result=%+v", result)
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

func TestProtocolCommandRequiresSeenVersionAndPreservesTruncation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	_ = os.WriteFile(path, []byte("old"), 0600)
	session, _ := NewRunSession(dir, "command-version")
	defer session.Discard()
	tool := projectCommandTool{}
	input := DomainToolInput{ProjectDir: dir, Session: session, Mode: model.ModeExecute, Phase: PhaseExecuting, Args: map[string]any{"command": "cat notes.txt"}}
	decision := tool.Preflight(context.Background(), input)
	input.Decision = &decision
	read := tool.Execute(context.Background(), input)
	if !read.OK || read.ObservationMetadata == nil {
		t.Fatalf("read=%+v", read)
	}
	input.Messages = appendBatchObservations(nil, []llm.ToolCall{{ID: "cat", Name: "run_command"}}, "", []ToolResult{read})
	input.Args = map[string]any{"command": `sed -i '' 's/old/new/g' notes.txt`}
	if got := tool.Preflight(context.Background(), input); got.Outcome != "confirm" {
		t.Fatalf("read version rejected: %+v", got)
	}
	_ = os.WriteFile(path, []byte("changed externally"), 0600)
	if got := tool.Preflight(context.Background(), input); got.Outcome != "deny" || got.ReasonCode != CodeContentConflict {
		t.Fatalf("stale sed allowed: %+v", got)
	}
	result := commandFailure(commandexec.Decision{}, commandexec.Result{Stdout: "partial", Stderr: "limit", ExitCode: 1, OutputTruncated: true}, errors.New("output limit exceeded"))
	view := errorObservation(t, result)
	if view["stdout"] != "partial" || view["exit_code"] != float64(1) || view["output_truncated"] != true || view["reason"] == nil {
		t.Fatalf("lost truncated output: %v", view)
	}
}

func TestProtocolRenderFailureKeepsImageAndOnlyPublicDiagnostics(t *testing.T) {
	d := RenderDiagnostics{ContentSize: map[string]int{"width": 1921, "height": 1140}, Overflow: map[string]bool{"horizontal": false, "vertical": true}, FontStatus: "loaded"}
	diagnostics := modelRenderDiagnostics(d, 1920, 1080)
	axes := diagnostics["overflow"].(map[string]string)
	if !strings.HasPrefix(axes["horizontal"], "未溢出") || !strings.Contains(axes["vertical"], "超出 60px") || len(diagnostics) != 4 {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
	raw, _ := json.Marshal(map[string]any{"slide_id": generationSlide, "diagnostics": diagnostics, "code": CodeRenderFailed, "reason": "overflow"})
	result := ToolResult{OK: false, Code: CodeRenderFailed, Data: map[string]any{"image_path": "private", "hash": "private", "font_status": "loaded"}, ObservationParts: []llm.ContentPart{{Type: "text", Text: string(raw)}, {Type: "image", ImageRef: "project:p1/render:sli_one/shot"}}}
	bound := bindToolErrorObservation(result, llm.ToolCall{ID: "render", Name: "render_slide", Args: map[string]any{"slide_id": generationSlide}})
	messages := appendBatchObservations(nil, []llm.ToolCall{{ID: "render", Name: "render_slide"}}, "", []ToolResult{bound})
	toolReply := messages[len(messages)-1]
	if len(toolReply.Content) != 2 || toolReply.Content[1].Type != "image" || strings.Contains(toolReply.Text(), "private") || strings.Contains(toolReply.Text(), "font_status") {
		t.Fatalf("reply=%+v", toolReply)
	}
	state := &RunState{}
	state.rememberReadImages([]llm.ToolCall{{ID: "render", Name: "render_slide", Args: map[string]any{"slide_id": generationSlide}}}, []ToolResult{bound})
	if len(state.readImages) != 1 || state.readImages[0].CallID != "render" || state.readImages[0].SlideID != generationSlide {
		t.Fatal("render association lost")
	}
}

type protocolPlanPrompter struct {
	approvingPrompter
	decision, feedback string
}

func (p *protocolPlanPrompter) AskPlanApproval(_ context.Context, q model.PlanApprovalRequestedPayload) (model.PlanApprovalAnswer, error) {
	return model.PlanApprovalAnswer{InteractionID: q.InteractionID, PlanID: q.Plan.PlanID, Decision: p.decision, Feedback: p.feedback}, nil
}
func TestProtocolPlanRefusalReturnsFeedbackAndKeepsReplanningAvailable(t *testing.T) {
	for _, feedback := range []string{"", "  保留原文\n修改第二步  "} {
		t.Run(feedback, func(t *testing.T) {
			call := llm.ToolCall{ID: "proposal", Name: "create_plan", Args: map[string]any{"title": "Plan", "content": "Work", "steps": []any{map[string]any{"title": "Step"}}}}
			state := &RunState{runID: "plan-protocol", mode: model.ModeExecute, phase: PhaseWaitingInput, scope: model.NewRunScope(model.ScopeAllPages), ledger: NewEvidenceLedger(), activeSkills: &ActiveSkillSet{}, plan: &Plan{ID: "plan", ApprovalID: "approval", Status: PlanAwaitingApproval}, pendingPlanCall: &PendingPlan{Call: call, OriginMode: model.ModeExecute}}
			cp := &checkpointRecorder{}
			if result, stop := NewRuntime(nil).awaitPlanApproval(context.Background(), RuntimeInput{Prompter: &protocolPlanPrompter{decision: "refuse", feedback: feedback}, Checkpoint: cp}, state); stop {
				t.Fatalf("result=%+v", result)
			}
			if state.phase != PhasePlanning || state.plan.Status != PlanAwaitingApproval || len(state.messages) != 2 || state.messages[1].ToolCallID != call.ID {
				t.Fatalf("refusal did not preserve planning: %+v", state)
			}
			var visible map[string]any
			_ = json.Unmarshal([]byte(state.messages[1].Text()), &visible)
			if visible["decision"] != "refuse" || !strings.Contains(visible["summary"].(string), "create_plan") {
				t.Fatalf("wrong refusal outcome: %v", visible)
			}
			if feedback != "" {
				if visible["feedback"] != feedback {
					t.Fatal("feedback was rewritten")
				}
				projected := requestConversation(state.messages, state.runID)
				if !strings.Contains(projected[len(projected)-1].Text(), state.messages[1].Text()) {
					t.Fatal("feedback missing from current request")
				}
			} else if visible["feedback"] != nil || !strings.Contains(visible["summary"].(string), "未提供") {
				t.Fatal("empty feedback was not handled")
			}
			tools := schemasByName(controlSchemas(state.phase, state.mode, state.plan))
			if tools["update_plan"] || !tools["create_plan"] || tools["finish_task"] {
				t.Fatalf("tools=%v", tools)
			}
			raw, _ := json.Marshal(cp.checkpoints[len(cp.checkpoints)-1])
			var restored RuntimeCheckpoint
			_ = json.Unmarshal(raw, &restored)
			if restored.PendingPlanCall != nil || restored.Phase != PhasePlanning || restored.Plan.Status != PlanAwaitingApproval {
				t.Fatal("refusal not durable")
			}
		})
	}
}

func TestProtocolQuestionAnswersUseOriginalTextAndOrder(t *testing.T) {
	args := map[string]any{"questions": []any{
		map[string]any{"question": "问题一？", "reason": "确定范围", "options": []any{map[string]any{"label": "原选项", "description": "含义"}}},
		map[string]any{"question": "问题二？", "reason": "确定内容"}, map[string]any{"question": "问题三？", "reason": "确定顺序"},
	}}
	question := publicQuestion("run", "call", args)
	answer := model.QuestionAnswer{Answers: []model.QuestionFieldAnswer{
		{QuestionID: question.Questions[2].ID, Skipped: true}, {QuestionID: question.Questions[1].ID, CustomText: "  用户原文\n "}, {QuestionID: question.Questions[0].ID, SelectedOptionID: question.Questions[0].Options[0].ID},
	}}
	got := modelQuestionAnswers(args, question, answer)
	want := []map[string]string{{"question": "问题一？", "answer": "原选项"}, {"question": "问题二？", "answer": "  用户原文\n "}, {"question": "问题三？", "answer": skippedQuestionAnswer}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("answers=%v", got)
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
	pack.Command.Mode = model.ModeExecute
	pack.Command.Scope = original
	state := &RunState{runID: "scope", mode: model.ModeExecute, phase: PhaseExecuting, scope: original, pack: pack, ledger: NewEvidenceLedger(), activeSkills: &ActiveSkillSet{}}
	call := llm.ToolCall{ID: "expand", Name: "request_privilege", Args: map[string]any{"slide_ids": []any{"sli_two"}, "reason": "More content"}}
	committed := false
	input := RuntimeInput{Prompter: &protocolScopePrompter{}, CommitScopeExpansion: func(_ context.Context, s model.RunScope, cp RuntimeCheckpoint) error {
		committed = s.IncludeRunCreatedSlides && len(s.SlideIDs) == 3 && cp.PendingScopeExpansion.Call.ID == call.ID
		return nil
	}}
	if outcome, stop := NewRuntime(nil).executeControl(context.Background(), input, state, call, ""); stop {
		t.Fatalf("outcome=%+v", outcome)
	}
	if !committed || !state.scope.IncludeRunCreatedSlides || state.scope.Revision != 2 {
		t.Fatalf("scope=%+v", state.scope)
	}
	reply := state.messages[len(state.messages)-1]
	if reply.ToolCallID != call.ID || reply.Text() != `{"decision":"revise","summary":"用户已将编辑范围扩大至所有页面，包括本次运行中新建的页面，可以继续执行。"}` {
		t.Fatalf("reply=%+v", reply)
	}
}

func TestProtocolScopeRefusalKeepsAuthorizationAndOriginalFeedback(t *testing.T) {
	pack := scopeExpansionPack()
	original := model.NewRunScope(model.ScopeCurrentPage, "sli_one")
	pack.Command.Mode, pack.Command.Scope = model.ModeExecute, original
	state := &RunState{runID: "scope", mode: model.ModeExecute, phase: PhaseExecuting, scope: original, pack: pack, ledger: NewEvidenceLedger(), activeSkills: &ActiveSkillSet{}}
	call := llm.ToolCall{ID: "expand", Name: "request_privilege", Args: map[string]any{"slide_ids": []any{"sli_two"}, "reason": "More content"}}
	const feedback = "  保持当前页\n删去第二页工作  "
	events := &eventRecorder{}
	input := RuntimeInput{Prompter: &protocolScopePrompter{decision: "refuse", feedback: feedback}, Emitter: events,
		CommitScopeExpansion: func(context.Context, model.RunScope, RuntimeCheckpoint) error {
			t.Fatal("refusal changed authorization")
			return nil
		}}
	if outcome, stop := NewRuntime(nil).executeControl(context.Background(), input, state, call, ""); stop {
		t.Fatal(outcome)
	}
	if !reflect.DeepEqual(state.scope, original) || state.phase != PhaseExecuting || state.pendingScopeExpansion != nil {
		t.Fatalf("state=%+v", state)
	}
	var reply map[string]any
	if json.Unmarshal([]byte(state.messages[len(state.messages)-1].Text()), &reply) != nil || reply["decision"] != "refuse" || reply["feedback"] != feedback {
		t.Fatalf("reply=%v", reply)
	}
	answered := 0
	for _, event := range events.events {
		if event.kind == model.EventScopeExpansionAnswered {
			answered++
			payload := event.payload.(model.ScopeExpansionAnsweredPayload)
			if payload.Decision != "refuse" || payload.Feedback != feedback || payload.AppliedScope != nil {
				t.Fatalf("answer=%+v", payload)
			}
		}
	}
	if answered != 1 {
		t.Fatalf("answered=%d", answered)
	}
}

func TestProtocolRefusedPlanCannotFinishWithoutReplanning(t *testing.T) {
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("create", "create_plan", map[string]any{"title": "Plan", "content": "Work", "steps": []any{map[string]any{"title": "Write"}}}),
		finishCall("done"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{RunID: "refusal", ProjectDir: t.TempDir(), Context: testPack(model.ModePlan, model.ScopeAllPages, false, "规划后等待决定"), Prompter: &protocolPlanPrompter{decision: "refuse"}})
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
	pack.Command.Mode = model.ModeExecute
	state := &RunState{runID: "drafts", mode: model.ModeExecute, phase: PhaseExecuting, pack: pack, scope: pack.Command.Scope, ledger: NewEvidenceLedger(), activeSkills: &ActiveSkillSet{}}
	runtime := NewRuntime(nil)
	input := RuntimeInput{Prompter: &protocolPlanPrompter{decision: "refuse"}}
	for i, title := range []string{"First draft", "Second draft"} {
		call := llm.ToolCall{ID: []string{"draft-one", "draft-two"}[i], Name: "create_plan", Args: map[string]any{"title": "Plan", "content": title, "steps": []any{map[string]any{"title": title}}}}
		if outcome, stop := runtime.executeControl(context.Background(), input, state, call, ""); stop {
			t.Fatalf("draft=%+v", outcome)
		}
	}
	if len(state.plan.Steps) != 1 || state.plan.Steps[0].Title != "Second draft" || state.plan.Content != "Second draft" || state.plan.Status != PlanAwaitingApproval {
		t.Fatalf("old draft survived replacement: %+v", state.plan)
	}
}

func TestProtocolToolCatalogAndCommitProjection(t *testing.T) {
	registry := NewToolRegistry()
	_ = (DefaultDomainToolProvider{}).RegisterDomainTools(registry)
	for _, name := range []string{"init_outline", "arrange_outline", "write_html", "patch_html", "finish"} {
		if _, ok := registry.Descriptor(name); ok {
			t.Fatalf("legacy alias %s", name)
		}
	}
	for _, schema := range registry.Disclose(PhaseExecuting, model.ModeExecute, model.NewRunScope(model.ScopeAllPages)) {
		raw, _ := json.Marshal(schema.Parameters)
		if strings.Contains(string(raw), `"expected_hash"`) {
			t.Fatalf("hash input in %s", schema.Name)
		}
	}
	for _, empty := range []bool{false, true} {
		tool := gitCommitTool{execute: func(context.Context, string, map[string]any) (map[string]any, error) {
			return map[string]any{"empty": empty, "hash": "abc123", "branch": "main", "title": "private", "files_changed": 9, "model_profile": "private"}, nil
		}}
		result := tool.Execute(context.Background(), DomainToolInput{})
		var visible map[string]any
		_ = json.Unmarshal([]byte(result.Observation), &visible)
		if empty {
			if len(visible) != 1 || visible["summary"] != "当前项目没有可提交的变更。" {
				t.Fatalf("empty=%v", visible)
			}
		} else if len(visible) != 3 || visible["hash"] != "abc123" || visible["branch"] != "main" {
			t.Fatalf("commit=%v", visible)
		}
	}
}

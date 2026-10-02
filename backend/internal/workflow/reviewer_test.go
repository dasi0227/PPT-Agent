package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestReviewerCorrectsWithoutLosingInspectionContext(t *testing.T) {
	content := []llm.ContentPart{{Type: "text", Text: "first paragraph"}, {Type: "text", Text: "second paragraph"}}
	continuation := &llm.ProviderContinuation{Provider: "fake", Model: "fake-model"}
	read := llm.ToolCall{ID: "pixels", Name: "read_image", Args: map[string]any{"image_path": "latest"}}
	invalid := llm.ToolCall{ID: "bad-submit", Name: "submit_review", Args: map[string]any{"decision": "approve"}}
	p := &llmtest.FakeProvider{Script: []llm.GenerateResponse{
		{Content: content, Continuation: continuation}, {ToolCalls: []llm.ToolCall{read}},
		{ToolCalls: []llm.ToolCall{invalid}}, {Content: llm.TextContent("checked"), ToolCalls: []llm.ToolCall{reviewSubmission("approve", "内容与当前截图一致。")}},
	}}
	diagnostics := []map[string]any{}
	result, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{
		Tools:    []ToolSchema{(readImageTool{}).Schema()},
		Diagnose: func(d map[string]any) { diagnostics = append(diagnostics, d) },
		Execute: func(context.Context, llm.ToolCall) (ToolResult, error) {
			out := SuccessfulToolResult("pixels")
			out.ObservationParts = []llm.ContentPart{{Type: "image", ImageRef: "test:current", MIMEType: "image/png"}}
			return out, nil
		},
	})
	if err != nil || result.Decision != "approve" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	requests := p.Requests()
	if len(requests) != 4 || requests[1].Continuation != continuation || len(requests[1].Messages[2].Content) != 2 {
		t.Fatal("raw assistant content or continuation lost")
	}
	guidance := requests[1].Messages[3]
	if guidance.Metadata == nil || guidance.Metadata.Origin != "runtime" || !strings.Contains(guidance.Text(), "inspection tools: read_image") {
		t.Fatal("runtime guidance missing")
	}
	if requests[1].RequiredTool != "" || requests[1].ParallelToolCalls != nil {
		t.Fatal("Reviewer was forced to submit")
	}
	if requestImageCounts(requests[3].Messages)["test:current"] != 1 {
		t.Fatal("inspection image lost across correction")
	}
	paired := false
	for _, message := range requests[3].Messages {
		if message.Role == llm.RoleTool && message.ToolCallID == invalid.ID {
			var feedback map[string]any
			if json.Unmarshal([]byte(message.Text()), &feedback) != nil || feedback["field"] != "/reasons" {
				t.Fatal("missing specific reasons feedback")
			}
			paired = true
		}
	}
	if !paired || len(diagnostics) != 3 || diagnostics[2]["corrections"] != 2 || diagnostics[2]["disposition"] != "accepted" {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
}

func TestReviewerRejectsEntireBatchBeforeInspection(t *testing.T) {
	read := llm.ToolCall{ID: "read", Name: "read_image"}
	for _, bad := range []llm.ToolCall{reviewSubmission("approve", "已核对。"), {ID: "unknown", Name: "edit_html"}} {
		p := &llmtest.FakeProvider{Script: []llm.GenerateResponse{{ToolCalls: []llm.ToolCall{read, bad}}, {ToolCalls: []llm.ToolCall{reviewSubmission("revise", "需要核对引用。")}}}}
		executed := 0
		result, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{Tools: []ToolSchema{(readImageTool{}).Schema()}, Execute: func(context.Context, llm.ToolCall) (ToolResult, error) {
			executed++
			return SuccessfulToolResult("pixels"), nil
		}})
		if err != nil || result.Decision != "revise" || executed != 0 {
			t.Fatalf("result=%v executed=%d err=%v", result, executed, err)
		}
		messages := p.Requests()[1].Messages
		if len(messages) != 6 || messages[3].ToolCallID != read.ID || messages[4].ToolCallID != bad.ID {
			t.Fatal("rejected batch has missing tool results")
		}
	}
}

func TestReviewerCorrectionBudgetDoesNotResetAfterInspection(t *testing.T) {
	text := llm.GenerateResponse{Content: llm.TextContent("not submitted")}
	read := llm.GenerateResponse{ToolCalls: []llm.ToolCall{{ID: "read", Name: "read_image"}}}
	p := &llmtest.FakeProvider{Script: []llm.GenerateResponse{text, read, text, read, text}}
	executed := 0
	result, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{Tools: []ToolSchema{(readImageTool{}).Schema()}, Execute: func(context.Context, llm.ToolCall) (ToolResult, error) {
		executed++
		return SuccessfulToolResult("pixels"), nil
	}})
	if err == nil || result.Decision != "" || len(p.Requests()) != 5 || executed != 2 {
		t.Fatalf("result=%v requests=%d executed=%d err=%v", result, len(p.Requests()), executed, err)
	}
}

func reviewSubmission(kind string, reasons ...any) llm.ToolCall {
	return llm.ToolCall{ID: "submit", Name: "submit_review", Args: map[string]any{"decision": kind, "reasons": reasons}}
}

func TestReviewSubmissionRequiresReasonsForEveryOutcome(t *testing.T) {
	for _, kind := range []string{"approve", "revise", "refuse"} {
		result, err := parseReviewSubmission(reviewSubmission(kind, "  第 3 页的数据已核对。  "))
		if err != nil || result.Decision != kind || result.Reasons[0] != "第 3 页的数据已核对。" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		for _, call := range []llm.ToolCall{reviewSubmission(kind), reviewSubmission(kind, " "), reviewSubmission(kind, 3)} {
			if _, err := parseReviewSubmission(call); err == nil {
				t.Fatalf("invalid reasons accepted: %+v", call)
			}
		}
	}
	wrong := reviewSubmission("approve", "已核对。")
	wrong.Args["checks"] = []any{}
	if _, err := parseReviewSubmission(wrong); err == nil {
		t.Fatal("extra fields accepted")
	}
	if _, err := parseReviewSubmission(reviewSubmission("unknown", "已核对。")); err == nil {
		t.Fatal("unknown verdict accepted")
	}
}

func TestReviewerCanRenderReadPixelsThenSubmit(t *testing.T) {
	render := llm.ToolCall{ID: "render", Name: "render_slide", Args: map[string]any{"slide_id": "sli_1"}}
	read := llm.ToolCall{ID: "image", Name: "read_image", Args: map[string]any{"image_path": "latest"}}
	provider := &llmtest.FakeProvider{Script: []llm.GenerateResponse{
		{ToolCalls: []llm.ToolCall{render}}, {ToolCalls: []llm.ToolCall{read}}, {ToolCalls: []llm.ToolCall{reviewSubmission("refuse", "第 1 页正文被截断。")}},
	}}
	calls := []string{}
	result, err := (LLMTaskReviewer{Provider: provider}).Review(context.Background(), ReviewInput{
		Material: ReviewMaterial{Demand: "检查版式", UserInstructions: []ReviewInstruction{{Text: "保留全部正文"}}},
		Tools:    []ToolSchema{(slideRenderTool{}).Schema(), (readImageTool{}).Schema()},
		Execute: func(_ context.Context, call llm.ToolCall) (ToolResult, error) {
			calls = append(calls, call.Name)
			out := SuccessfulToolResult("inspection completed")
			if call.Name == "render_slide" {
				out.Data = map[string]any{"image_path": "latest"}
			} else {
				out.ObservationParts = []llm.ContentPart{{Type: "image", ImageRef: "project:p1/render:sli_1/shot_latest", MIMEType: "image/png"}}
			}
			return out, nil
		},
	})
	if err != nil || result.Decision != "refuse" || strings.Join(calls, ",") != "render_slide,read_image" {
		t.Fatalf("result=%+v calls=%v err=%v", result, calls, err)
	}
	requests := provider.Requests()
	if requestImageCounts(requests[2].Messages)["project:p1/render:sli_1/shot_latest"] != 1 {
		t.Fatal("read_image pixels did not reach the review model")
	}
	if !strings.Contains(transcriptText(requests[0].Messages), "保留全部正文") {
		t.Fatal("user instructions missing")
	}
}

func TestReviewerRejectsTextMixedSubmissionAndUnavailableTools(t *testing.T) {
	for _, response := range []llm.GenerateResponse{
		{Content: llm.TextContent(`{"type":"approve","reasons":["ok"]}`)},
		{ToolCalls: []llm.ToolCall{reviewSubmission("approve", "已核对。"), {ID: "second", Name: "submit_review", Args: map[string]any{"decision": "approve", "reasons": []any{"已核对。"}}}}},
		{ToolCalls: []llm.ToolCall{{ID: "edit", Name: "edit_html"}}},
	} {
		p := &llmtest.FakeProvider{Script: []llm.GenerateResponse{response, response, response}}
		if _, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{}); err == nil {
			t.Fatal("invalid reviewer output was accepted")
		}
		if len(p.Requests()) != 3 {
			t.Fatalf("correction budget not exhausted: %d", len(p.Requests()))
		}
	}
	p := &llmtest.FakeProvider{Caps: llm.Capabilities{ContextWindowTokens: 100}}
	if _, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{}); err == nil || len(p.Requests()) != 0 {
		t.Fatal("over-budget material must fail before provider invocation")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (LLMTaskReviewer{Provider: p}).Review(ctx, ReviewInput{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
}

func TestReviewTaskFailureHasNoAssessment(t *testing.T) {
	agent := &scriptedAgent{responses: []AgentResponse{toolCall("review", "review_task", map[string]any{"demand": "检查成果"}), finishCall("finish_task")}}
	events := &eventRecorder{}
	NewRuntime(agent).Run(context.Background(), RuntimeInput{RunID: "review-failure", ProjectDir: t.TempDir(), Context: testPack(model.ModeExecute, model.ScopeAllPages, false, "检查成果"), Reviewer: &scriptedReviewer{err: errors.New("provider unavailable")}, Emitter: events})
	for _, event := range events.events {
		if event.kind != model.EventToolCompleted {
			continue
		}
		payload := event.payload.(model.ToolCompletedPayload)
		if payload.Tool != "review_task" {
			continue
		}
		if payload.Status != "failed" || payload.Review != nil || payload.Error == nil {
			t.Fatalf("failure was represented as an assessment: %+v", payload)
		}
		if err := model.ValidatePublicEvent(model.EventToolCompleted, payload); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatal("review failure event missing")
}

func TestReviewRecoveryPreservesSavedAssessmentOrReportsInterruption(t *testing.T) {
	for _, saved := range []bool{false, true} {
		root := t.TempDir()
		pack := testPack(model.ModeExecute, model.ScopeAllPages, false, "检查成果")
		if err := ensureReviewBaseline(context.Background(), root, "review-resume", false); err != nil {
			t.Fatal(err)
		}
		pending := &PendingReview{Call: llm.ToolCall{ID: "review", Name: "review_task", Args: map[string]any{"demand": "检查成果"}}}
		if saved {
			result := SuccessfulToolResult("artifact review completed")
			result.Data = map[string]any{"decision": "approve", "reasons": []string{"内容与用户要求一致。"}}
			pending.Result = &result
		}
		checkpoint := RuntimeCheckpoint{RunID: "review-resume", Mode: model.ModeExecute, Phase: PhaseExecuting, Scope: pack.Command.Scope, PendingReview: pending,
			ReviewInstructions: []ReviewInstruction{{Text: "original"}, {Text: "correction"}}}
		events := &eventRecorder{}
		checkpoints := &checkpointRecorder{}
		agent := &scriptedAgent{responses: []AgentResponse{finishCall("finish_task")}}
		outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{RunID: "review-resume", ProjectDir: root, Context: pack, ResumeCheckpoint: &checkpoint, Emitter: events, Checkpoint: checkpoints})
		if outcome.Status != StatusCompleted {
			t.Fatalf("outcome=%+v", outcome)
		}
		found := false
		for _, event := range events.events {
			if event.kind != model.EventToolCompleted {
				continue
			}
			payload := event.payload.(model.ToolCompletedPayload)
			if payload.Tool != "review_task" {
				continue
			}
			found = true
			if saved && (payload.Status != "completed" || payload.Review == nil || payload.Review.Decision != "approve") {
				t.Fatalf("saved approval lost: %+v", payload)
			}
			if !saved && (payload.Status != "failed" || payload.Review != nil) {
				t.Fatalf("invented interrupted assessment: %+v", payload)
			}
		}
		if !found {
			t.Fatal("recovery left review running")
		}
		if len(agent.requests) == 0 || !hasToolResponse(agent.requests[0].Messages, "review") {
			t.Fatal("recovered observation did not reach main agent")
		}
		if len(checkpoints.checkpoints) == 0 {
			t.Fatal("recovered review was not checkpointed")
		}
		last := checkpoints.checkpoints[len(checkpoints.checkpoints)-1]
		if last.PendingReview != nil || len(last.ReviewInstructions) != 2 || last.ReviewInstructions[1].Text != "correction" {
			t.Fatal("recovery must clear the pending review and preserve user corrections")
		}
	}
}

func hasToolResponse(messages []llm.Message, id string) bool {
	for _, m := range messages {
		if m.Role == llm.RoleTool && m.ToolCallID == id {
			return true
		}
	}
	return false
}

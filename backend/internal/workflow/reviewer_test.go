package workflow

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func reviewSubmission(kind string, reasons ...any) llm.ToolCall {
	return llm.ToolCall{ID: "submit", Name: "submit_review", Args: map[string]any{"type": kind, "reasons": reasons}}
}

func TestReviewSubmissionRequiresReasonsForEveryOutcome(t *testing.T) {
	for _, kind := range []string{"approve", "check", "refuse"} {
		result, err := parseReviewSubmission(reviewSubmission(kind, "  第 3 页的数据已核对。  "))
		if err != nil || result.Type != kind || result.Reasons[0] != "第 3 页的数据已核对。" {
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
	if err != nil || result.Type != "refuse" || strings.Join(calls, ",") != "render_slide,read_image" {
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
		{ToolCalls: []llm.ToolCall{reviewSubmission("approve", "已核对。"), reviewSubmission("approve", "已核对。")}},
		{ToolCalls: []llm.ToolCall{{ID: "edit", Name: "write_html"}}},
	} {
		p := &llmtest.FakeProvider{Script: []llm.GenerateResponse{response}}
		if _, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{}); err == nil {
			t.Fatal("invalid reviewer output was accepted")
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
	agent := &scriptedAgent{responses: []AgentResponse{toolCall("review", "review_task", map[string]any{"demand": "检查成果"}), finishCall("finish")}}
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
			result.Data = map[string]any{"type": "approve", "reasons": []string{"内容与用户要求一致。"}}
			pending.Result = &result
		}
		checkpoint := RuntimeCheckpoint{RunID: "review-resume", Mode: model.ModeExecute, Phase: PhaseExecuting, Scope: pack.Command.Scope, PendingReview: pending,
			ReviewInstructions: []ReviewInstruction{{Text: "original"}, {Text: "correction"}}}
		events := &eventRecorder{}
		checkpoints := &checkpointRecorder{}
		agent := &scriptedAgent{responses: []AgentResponse{finishCall("finish")}}
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
			if saved && (payload.Status != "completed" || payload.Review == nil || payload.Review.Type != "approve") {
				t.Fatalf("saved approval lost: %+v", payload)
			}
			if !saved && (payload.Status != "failed" || payload.Review != nil) {
				t.Fatalf("invented interrupted assessment: %+v", payload)
			}
		}
		if !found {
			t.Fatal("recovery left review running")
		}
		if len(agent.requests) == 0 || !strings.Contains(transcriptText(agent.requests[0].Messages), "review") {
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

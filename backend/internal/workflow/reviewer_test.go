package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestReviewerCorrectsWithoutLosingEvidenceContext(t *testing.T) {
	content := []llm.ContentPart{{Type: "text", Text: "first paragraph"}, {Type: "text", Text: "second paragraph"}}
	continuation := &llm.ProviderContinuation{Provider: "fake", Model: "fake-model"}
	invalid := llm.ToolCall{ID: "bad-submit", Name: "submit_review", Args: map[string]any{"decision": "approve"}}
	p := &llmtest.FakeProvider{Script: []llm.GenerateResponse{
		{Content: content, Continuation: continuation}, {ToolCalls: []llm.ToolCall{invalid}},
		{Content: llm.TextContent("checked"), ToolCalls: []llm.ToolCall{reviewSubmission("approve", "内容与当前截图一致。")}},
	}}
	diagnostics := []map[string]any{}
	result, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{
		Images:   []llm.ContentPart{{Type: "image", ImageRef: "test:current", MIMEType: "image/png"}},
		Diagnose: func(d map[string]any) { diagnostics = append(diagnostics, d) },
	})
	if err != nil || result.Decision != "approve" {
		t.Fatalf("result=%v err=%v", result, err)
	}
	requests := p.Requests()
	if len(requests) != 3 || requests[1].Continuation != continuation || len(requests[1].Messages[2].Content) != 2 {
		t.Fatal("raw assistant content or continuation lost")
	}
	for _, req := range requests {
		if len(req.Tools) != 1 || req.Tools[0].Name != "submit_review" || req.RequiredTool != "submit_review" || req.ParallelToolCalls == nil || *req.ParallelToolCalls {
			t.Fatal("Reviewer must disclose/require only submit_review")
		}
		if requestImageCounts(req.Messages)["test:current"] != 1 {
			t.Fatal("prepared image lost")
		}
	}
	guidance := requests[1].Messages[3]
	if guidance.Metadata == nil || guidance.Metadata.Origin != "runtime" {
		t.Fatal("runtime provenance lost")
	}
	paired := false
	for _, m := range requests[2].Messages {
		if m.Role == llm.RoleTool && m.ToolCallID == invalid.ID {
			var feedback map[string]any
			if json.Unmarshal([]byte(m.Text()), &feedback) != nil || feedback["field"] != "/reasons" {
				t.Fatal("specific failure feedback missing")
			}
			paired = true
		}
	}
	last := diagnostics[len(diagnostics)-1]
	if !paired || last["corrections"] != 2 || last["disposition"] != "accepted" {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
}

func TestReviewerRejectsEntireInspectionBatch(t *testing.T) {
	read := llm.ToolCall{ID: "read", Name: "read_image"}
	p := &llmtest.FakeProvider{Script: []llm.GenerateResponse{{ToolCalls: []llm.ToolCall{read, reviewSubmission("approve", "已核对。")}}, {ToolCalls: []llm.ToolCall{reviewSubmission("revise", "需要核对引用。")}}}}
	result, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{})
	if err != nil || result.Decision != "revise" {
		t.Fatalf("result=%v err=%v", result, err)
	}
	messages := p.Requests()[1].Messages
	if len(messages) != 6 || messages[3].ToolCallID != read.ID || messages[4].ToolCallID != "submit" {
		t.Fatal("rejected batch has unpaired tool calls")
	}
}

func TestReviewerInspectionCallsConsumeCorrectionBudget(t *testing.T) {
	p := &llmtest.FakeProvider{}
	for i := range 3 {
		p.Script = append(p.Script, llm.GenerateResponse{ToolCalls: []llm.ToolCall{{ID: "read-" + strconv.Itoa(i), Name: "read_image"}}})
	}
	result, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{})
	if err == nil || result.Decision != "" || len(p.Requests()) != 3 {
		t.Fatalf("result=%v requests=%d err=%v", result, len(p.Requests()), err)
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

func TestReviewerNormalPathUsesOneSubmission(t *testing.T) {
	p := &llmtest.FakeProvider{Script: []llm.GenerateResponse{{ToolCalls: []llm.ToolCall{reviewSubmission("refuse", "第 1 页正文被截断。")}}}}
	result, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{Material: ReviewMaterial{Demand: "检查版式", UserInstructions: []ReviewInstruction{{Text: "保留全部正文"}}}, Images: []llm.ContentPart{{Type: "image", ImageRef: "prepared:slide-1"}}})
	if err != nil || result.Decision != "refuse" || len(p.Requests()) != 1 {
		t.Fatalf("result=%v err=%v", result, err)
	}
	req := p.Requests()[0]
	if len(req.Tools) != 1 || !strings.Contains(transcriptText(req.Messages), "保留全部正文") || requestImageCounts(req.Messages)["prepared:slide-1"] != 1 {
		t.Fatal("evidence or submit-only contract missing")
	}
}

func TestReviewerRejectsTextMixedSubmissionAndUnavailableTools(t *testing.T) {
	for _, response := range []llm.GenerateResponse{
		{Content: llm.TextContent(`{"type":"approve","reasons":["ok"]}`)},
		{ToolCalls: []llm.ToolCall{reviewSubmission("approve", "已核对。"), {ID: "second", Name: "submit_review", Args: map[string]any{"decision": "approve", "reasons": []any{"已核对。"}}}}},
		{ToolCalls: []llm.ToolCall{{ID: "edit", Name: "edit_html"}}},
	} {
		p := &llmtest.FakeProvider{}
		for i := range 3 {
			next := response
			next.ToolCalls = append([]llm.ToolCall(nil), response.ToolCalls...)
			for index := range next.ToolCalls {
				next.ToolCalls[index].ID += "-" + strconv.Itoa(i)
			}
			p.Script = append(p.Script, next)
		}
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

type reviewHook func(context.Context, ReviewInput) (model.ReviewResult, error)

func (f reviewHook) Review(ctx context.Context, input ReviewInput) (model.ReviewResult, error) {
	return f(ctx, input)
}

type reviewUserTranscript struct {
	recordingTranscript
	inputs []contextengine.ReviewUserInput
}

func (s *reviewUserTranscript) LoadReviewUserInputs(context.Context, string, string) ([]contextengine.ReviewUserInput, error) {
	return append([]contextengine.ReviewUserInput(nil), s.inputs...), nil
}

func TestReviewArtifactsDiscardsCanceledOrStaleAssessment(t *testing.T) {
	for _, kind := range []string{"source", "requirements", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			root := t.TempDir()
			path := filepath.Join(root, "content.txt")
			if err := os.WriteFile(path, []byte("current content"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := ensureReviewBaseline(ctx, root, "run", false); err != nil {
				t.Fatal(err)
			}
			pack := testPack(model.ModeExecute, model.ScopeAllPages, false, "当前要求")
			pack.Manifest.ThreadID = "thread"
			tx, err := NewRunSession(root, "run")
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Discard()
			transcript := &reviewUserTranscript{inputs: []contextengine.ReviewUserInput{{RunID: "previous", Text: "完整的历史要求"}, {RunID: "run", Text: "当前要求"}}}
			state := &RunState{runID: "run", projectDir: root, pack: pack, tx: tx, scope: pack.Command.Scope, reviewInstructions: []ReviewInstruction{{Text: "当前要求"}}, budget: RuntimeBudget{MaxTurns: 128, MaxDuration: 10 * time.Minute}}
			called := false
			reviewer := reviewHook(func(_ context.Context, input ReviewInput) (model.ReviewResult, error) {
				called = true
				if len(input.Material.UserInstructions) != 2 || input.Material.UserInstructions[0].Text != "完整的历史要求" {
					t.Fatal("authoritative requirements missing or duplicated")
				}
				if input.CheckBudget(ctx) != nil {
					t.Fatal("fresh evidence failed its consistency check")
				}
				switch kind {
				case "source":
					if err := os.WriteFile(path, []byte("newer content"), 0o600); err != nil {
						t.Fatal(err)
					}
				case "requirements":
					transcript.inputs = append(transcript.inputs, contextengine.ReviewUserInput{RunID: "next", Text: "新的纠正要求"})
				case "cancel":
					cancel()
				}
				return model.ReviewResult{Decision: "approve", Reasons: []string{"stale assessment"}}, nil
			})
			result := NewRuntime(nil).runReviewTask(ctx, RuntimeInput{Context: pack, ProjectDir: root, Transcript: transcript, Reviewer: reviewer}, state, llm.ToolCall{ID: "review", Name: "review_task", Args: map[string]any{"demand": "检查成果"}})
			if !called || result.OK || len(result.Data) != 0 {
				t.Fatalf("expired assessment applied: %+v called=%v", result, called)
			}
			if kind == "cancel" && result.Code != CodeCanceled {
				t.Fatalf("cancellation lost: %+v", result)
			}
		})
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

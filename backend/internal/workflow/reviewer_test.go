package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestReviewOutputRequiresStrictJSONForEveryOutcome(t *testing.T) {
	for _, kind := range []string{"approve", "revise", "refuse"} {
		result, err := parseReviewResponse(llm.GenerateResponse{Content: llm.TextContent(`{"decision":"` + kind + `","reasons":["  第 3 页的数据已核对。  "]}`)})
		if err != nil || result.Decision != kind || result.Reasons[0] != "第 3 页的数据已核对。" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		for _, reasons := range []string{`null`, `[]`, `[" "]`, `[3]`, `[null]`, `"ok"`} {
			if _, err := model.ParseReviewResult(`{"decision":"` + kind + `","reasons":` + reasons + `}`); err == nil {
				t.Fatalf("invalid reasons accepted: %s", reasons)
			}
		}
	}
	for _, text := range []string{
		`{"decision":"approve"}`, `{"reasons":["ok"]}`, `{"decision":"unknown","reasons":["ok"]}`,
		`{"decision":"approve","reasons":["ok"],"status":"success"}`,
		`{"decision":"approve","decision":"refuse","reasons":["ok"]}`,
		`{"Decision":"approve","reasons":["ok"]}`, `{"decision":null,"reasons":["ok"]}`,
		`null`, `[]`, `{"decision":"approve","reasons":["ok"]`,
		`{"decision":"approve","reasons":["ok"]} {}`,
		"```json\n" + `{"decision":"approve","reasons":["ok"]}` + "\n```",
		`Here is the result: {"decision":"approve","reasons":["ok"]}`,
		`{"decision":"approve","reasons":["ok"]} Done.`,
	} {
		if _, err := model.ParseReviewResult(text); err == nil {
			t.Fatalf("noncompliant JSON accepted: %s", text)
		}
	}
}

func TestReviewerNormalPathUsesOneToolFreeRequest(t *testing.T) {
	p := &llmtest.FakeProvider{Script: []llm.GenerateResponse{{Content: llm.TextContent(`{"decision":"refuse","reasons":["第 1 页正文被截断。"]}`)}}}
	result, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{Material: ReviewMaterial{
		Demand: "检查第 1 页版式并保留全部正文", Sources: []llm.ContentPart{{Type: "text", Text: "完整 HTML"}, {Type: "image", ImageRef: "prepared:slide-1"}},
	}})
	if err != nil || result.Decision != "refuse" || len(p.Requests()) != 1 {
		t.Fatalf("result=%v err=%v", result, err)
	}
	req := p.Requests()[0]
	if len(req.Tools) != 0 || req.RequiredTool != "" || req.ParallelToolCalls != nil || req.ToolConstraintPolicy != nil || len(req.Messages) != 2 {
		t.Fatal("independent JSON review must not disclose tools or inherit a transcript")
	}
	parts := req.Messages[1].Content
	if len(parts) != 5 || !strings.HasPrefix(parts[0].Text, "demand\n") || parts[1].Text != "changes\n[]" || parts[2].Text != "current_sources" || requestImageCounts(req.Messages)["prepared:slide-1"] != 1 {
		t.Fatal("ordered evidence missing")
	}
}

func TestReviewerRejectsInvalidOutputWithoutInventingAssessment(t *testing.T) {
	for _, response := range []llm.GenerateResponse{
		{Content: llm.TextContent(`{"type":"approve","reasons":["ok"]}`)},
		{Content: llm.TextContent(`{"decision":"approve","reasons":["ok"]}`), ToolCalls: []llm.ToolCall{{ID: "edit", Name: "edit_html"}}},
		{Content: []llm.ContentPart{{Type: "image", ImageRef: "unexpected"}}},
		{Content: llm.TextContent(`{"decision":"approve","reasons":["ok"]}`), Usage: llm.Usage{OutputTokens: reviewMaxOutputTokens + 1}},
	} {
		p := &llmtest.FakeProvider{Script: []llm.GenerateResponse{response}}
		result, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{})
		if err == nil || result.Decision != "" || len(p.Requests()) != 1 || reviewFailure(err).Code != CodeReviewFailed {
			t.Fatalf("invalid output became an assessment or retried: %+v %v", result, err)
		}
	}
	p := &llmtest.FakeProvider{GenerateErr: llm.ErrUnavailable}
	result, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{})
	if !errors.Is(err, llm.ErrUnavailable) || result.Decision != "" || len(p.Requests()) != 1 {
		t.Fatal("provider failure became a business verdict")
	}
}

func TestReviewerHonorsContextAndCancellationLimits(t *testing.T) {
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
	loads  int
}

func (s *reviewUserTranscript) LoadReviewUserInputs(context.Context, string, string) ([]contextengine.ReviewUserInput, error) {
	s.loads++
	return append([]contextengine.ReviewUserInput(nil), s.inputs...), nil
}

func TestReviewArtifactsDiscardsCanceledOrStaleAssessment(t *testing.T) {
	for _, kind := range []string{"source", "cancel"} {
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
				content, err := input.Material.content()
				if err != nil || strings.Contains((llm.Message{Content: content}).Text(), "完整的历史要求") || transcript.loads != 0 {
					t.Fatal("review must not load or disclose user history")
				}
				if input.CheckBudget(ctx) != nil {
					t.Fatal("fresh evidence failed its consistency check")
				}
				switch kind {
				case "source":
					if err := os.WriteFile(path, []byte("newer content"), 0o600); err != nil {
						t.Fatal(err)
					}
				case "cancel":
					cancel()
				}
				return model.ReviewResult{Decision: "approve", Reasons: []string{"stale assessment"}}, nil
			})
			result := NewRuntime(nil).runReviewPPT(ctx, RuntimeInput{Context: pack, ProjectDir: root, Transcript: transcript, Reviewer: reviewer}, state, llm.ToolCall{ID: "review", Name: "review_ppt", Args: map[string]any{"demand": "检查成果"}})
			if !called || result.OK || len(result.Data) != 0 {
				t.Fatalf("expired assessment applied: %+v called=%v", result, called)
			}
			if kind == "cancel" && result.Code != CodeCanceled {
				t.Fatalf("cancellation lost: %+v", result)
			}
		})
	}
}

func TestReviewPPTFailureHasNoAssessment(t *testing.T) {
	agent := &scriptedAgent{responses: []AgentResponse{toolCall("review", "review_ppt", map[string]any{"demand": "检查成果"}), finishCall("finish_task")}}
	events := &eventRecorder{}
	NewRuntime(agent).Run(context.Background(), RuntimeInput{RunID: "review-failure", ProjectDir: t.TempDir(), Context: testPack(model.ModeExecute, model.ScopeAllPages, false, "检查成果"), Reviewer: &scriptedReviewer{err: errors.New("provider unavailable")}, Emitter: events})
	for _, event := range events.events {
		if event.kind != model.EventToolCompleted {
			continue
		}
		payload := event.payload.(model.ToolCompletedPayload)
		if payload.Tool != "review_ppt" {
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
		pending := &PendingReview{Call: llm.ToolCall{ID: "review", Name: "review_ppt", Args: map[string]any{"demand": "检查成果"}}}
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
			if payload.Tool != "review_ppt" {
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

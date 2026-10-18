package workflow

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextcompact"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type runImageProvider struct{}

func (runImageProvider) RegisterDomainTools(registry *ToolRegistry) error {
	return registry.Register(runImageTool{}, true, CapabilityRead, RiskLow, PhaseChat)
}

type runImageTool struct{}

func (runImageTool) Schema() ToolSchema {
	return ToolSchema{Name: "read_image", Parameters: objectSchema([]string{"ref"}, map[string]any{"ref": map[string]any{"type": "string"}})}
}

func (runImageTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	ref := stringValue(input.Args["ref"])
	result := SuccessfulToolResult("read image")
	result.ObservationParts = []llm.ContentPart{
		{Type: "text", Text: modelSuccessObservation(nil)},
		{Type: "image", ImageRef: ref, MIMEType: "image/png", Detail: "high"},
	}
	return result
}

func requestImageCounts(messages []llm.Message) map[string]int {
	counts := map[string]int{}
	for _, message := range messages {
		for _, part := range message.Content {
			if part.Type == "image" {
				counts[part.ImageRef]++
			}
		}
	}
	return counts
}

func TestToolImagesSurviveTurnsTranscriptAndResume(t *testing.T) {
	const first = "project:p1/attachment:att_one/original"
	const second = "project:p1/attachment:att_two/original"
	const attachment = "project:p1/attachment:att_read/original"
	const selected = "project:p1/attachment:att_user/original"
	transcript := &recordingTranscript{messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentPart{
		{Type: "text", Text: "previous render and user reference"},
		{Type: "image", ImageRef: "run:old/screenshot:shot_old"},
		{Type: "image", ImageRef: selected},
	}}}}
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("a", "read_image", map[string]any{"ref": first}),
		toolCall("b", "read_image", map[string]any{"ref": second}),
		toolCall("attachment", "read_image", map[string]any{"ref": attachment}),
		toolCall("a-again", "read_image", map[string]any{"ref": first}),
		finalReply(),
	}}
	checkpoints := &checkpointRecorder{}
	input := RuntimeInput{RunID: "image-lifecycle", ProjectDir: t.TempDir(), Transcript: transcript,
		Context:     testPack(model.ModeChat, model.ScopeCurrentPage, false, "inspect"),
		DomainTools: runImageProvider{}, Checkpoint: checkpoints}
	outcome := NewRuntime(agent).Run(context.Background(), input)
	if outcome.Status != StatusCompleted || len(agent.requests) != 5 {
		t.Fatalf("unexpected result: %+v, requests=%d", outcome, len(agent.requests))
	}
	for i, request := range agent.requests {
		for _, message := range request.Messages {
			if message.Role == llm.RoleTool && message.Text() != `{"status":"success"}` {
				t.Fatalf("image status lost across turns: %+v", message)
			}
		}
		if i > 0 {
			previous := agent.requests[i-1].Messages
			if len(request.Messages) < len(previous) || !reflect.DeepEqual(request.Messages[:len(previous)], previous) {
				t.Fatalf("request %d rewrote unchanged image history needed by provider replay", i)
			}
		}
		want := map[string]int{selected: 1, "run:old/screenshot:shot_old": 1}
		if i >= 1 {
			want[first] = 1
		}
		if i >= 2 {
			want[second] = 1
		}
		if i >= 3 {
			want[attachment] = 1
		}
		if i >= 4 {
			want[first] = 2
		}
		if got := requestImageCounts(request.Messages); !reflect.DeepEqual(got, want) {
			t.Fatalf("request %d images=%v, want %v", i, got, want)
		}
	}
	want := map[string]int{selected: 1, "run:old/screenshot:shot_old": 1, first: 2, second: 1, attachment: 1}
	if got := requestImageCounts(transcript.messages); !reflect.DeepEqual(got, want) {
		t.Fatalf("native images lost in thread history: %v", got)
	}
	var checkpoint RuntimeCheckpoint
	for _, cp := range checkpoints.checkpoints {
		if cp.Boundary == "tool_batch_observed" {
			checkpoint = cp
		}
	}
	if checkpoint.RunID == "" {
		t.Fatal("tool round was not checkpointed after reading")
	}
	// Round-trip through JSON, as the SQLite checkpoint does.
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &checkpoint); err != nil {
		t.Fatal(err)
	}
	resumed := &scriptedAgent{responses: []AgentResponse{finalReply()}}
	input.ResumeCheckpoint = &checkpoint
	if got := NewRuntime(resumed).Run(context.Background(), input); got.Status != StatusCompleted {
		t.Fatalf("resume failed: %+v", got)
	}
	if got := requestImageCounts(resumed.requests[0].Messages); !reflect.DeepEqual(got, want) {
		t.Fatalf("checkpoint lost images: %v", got)
	}
	input.RunID, input.ResumeCheckpoint = "new-run", nil
	fresh := &scriptedAgent{responses: []AgentResponse{finalReply()}}
	NewRuntime(fresh).Run(context.Background(), input)
	if got := requestImageCounts(fresh.requests[0].Messages); !reflect.DeepEqual(got, want) {
		t.Fatalf("new Run lost native historical images: %v", got)
	}
}

type runImageCompactor struct{ inputs [][]llm.Message }

func (c *runImageCompactor) Compact(_ context.Context, messages []llm.Message) (contextcompact.Result, error) {
	c.inputs = append(c.inputs, messages)
	return contextcompact.Result{Messages: []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("text summary")}}}, nil
}

func TestCompactionDoesNotReinjectToolImages(t *testing.T) {
	state := &RunState{runID: "run", mode: model.ModeChat, phase: PhaseChat,
		pack:   testPack(model.ModeChat, model.ScopeCurrentPage, false, "compare"),
		ledger: NewEvidenceLedger(), activeSkills: &ActiveSkillSet{}, budget: DefaultRuntimeBudget(200000),
		messages: []llm.Message{
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "render", Name: "render_slide"}}},
			{Role: llm.RoleTool, ToolCallID: "render", Content: []llm.ContentPart{
				{Type: "text", Text: "render diagnostics"},
				{Type: "image", ImageRef: "project:p1/render:sli_1/shot", MIMEType: "image/png"},
			}},
		},
	}
	c := &runImageCompactor{}
	runtime := NewRuntime(nil)
	runtime.Compactor = c
	runtime.measureContextWindow(RuntimeInput{}, state, nil)
	if got := requestImageCounts(state.messages); got["project:p1/render:sli_1/shot"] != 1 {
		t.Fatal("request preparation removed native tool pixels")
	}
	state.tokens, state.nextCompactionTokens = state.budget.ContextCompactionThreshold, 0
	if err := runtime.compactIfNeeded(context.Background(), RuntimeInput{}, state, nil); err != nil {
		t.Fatal(err)
	}
	if len(c.inputs) != 1 || len(requestImageCounts(c.inputs[0])) != 0 {
		t.Fatal("text-only compaction input changed")
	}
	if len(requestImageCounts(state.messages)) != 0 {
		t.Fatal("compaction reintroduced a separate image injection")
	}
}

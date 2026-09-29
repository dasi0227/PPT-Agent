package workflow

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextcompact"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type runImageProvider struct{}

func (runImageProvider) RegisterDomainTools(registry *ToolRegistry) error {
	return registry.RegisterDomainTool(runImageTool{}, true, PhaseChat)
}

type runImageTool struct{}

func (runImageTool) Schema() ToolSchema {
	return ToolSchema{Name: "read_image", Parameters: objectSchema([]string{"ref"}, map[string]any{"ref": map[string]any{"type": "string"}})}
}

func (runImageTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	ref := stringValue(input.Args["ref"])
	result := SuccessfulToolResult("read image")
	result.ObservationParts = []llm.ContentPart{
		{Type: "text", Text: "image read: " + ref},
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

func TestRunReadImagesSurviveTurnsAndResumeWithoutLeakingToNewRun(t *testing.T) {
	const first = "project:p1/render:sli_1/shot_one"
	const second = "project:p1/render:sli_2/shot_two"
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
		{Text: "visual finding: both pages use consistent spacing"},
		finishCall("finished"),
	}}
	checkpoints := &checkpointRecorder{}
	input := RuntimeInput{RunID: "image-lifecycle", ProjectDir: t.TempDir(), Transcript: transcript,
		Context:     testPack(model.ModeChat, model.ScopeCurrentPage, false, "inspect"),
		DomainTools: runImageProvider{}, Checkpoint: checkpoints}
	outcome := NewRuntime(agent).Run(context.Background(), input)
	if outcome.Status != StatusCompleted || len(agent.requests) != 6 {
		t.Fatalf("unexpected result: %+v, requests=%d", outcome, len(agent.requests))
	}
	for i, request := range agent.requests {
		if i > 0 {
			previous := agent.requests[i-1].Messages
			if len(request.Messages) < len(previous) || !reflect.DeepEqual(request.Messages[:len(previous)], previous) {
				t.Fatalf("request %d rewrote unchanged image history needed by provider replay", i)
			}
		}
		want := map[string]int{selected: 1}
		if i >= 1 {
			want[first] = 1
		}
		if i >= 2 {
			want[second] = 1
		}
		if i >= 3 {
			want[attachment] = 1
		}
		if got := requestImageCounts(request.Messages); !reflect.DeepEqual(got, want) {
			t.Fatalf("request %d images=%v, want %v", i, got, want)
		}
	}
	if got := requestImageCounts(transcript.messages); !reflect.DeepEqual(got, map[string]int{selected: 1}) {
		t.Fatalf("Run-owned images leaked into thread history: %v", got)
	}
	var checkpoint RuntimeCheckpoint
	for _, cp := range checkpoints.checkpoints {
		if cp.Boundary == string(checkpointAfterImageRead) && len(cp.ReadImages) == 3 {
			checkpoint = cp
		}
	}
	if checkpoint.RunID == "" {
		t.Fatal("images were not checkpointed immediately after reading")
	}
	// Round-trip through JSON, as the SQLite checkpoint does.
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &checkpoint); err != nil {
		t.Fatal(err)
	}
	resumed := &scriptedAgent{responses: []AgentResponse{finishCall("resumed")}}
	input.ResumeCheckpoint = &checkpoint
	if got := NewRuntime(resumed).Run(context.Background(), input); got.Status != StatusCompleted {
		t.Fatalf("resume failed: %+v", got)
	}
	if got := requestImageCounts(resumed.requests[0].Messages); !reflect.DeepEqual(got, map[string]int{selected: 1, first: 1, second: 1, attachment: 1}) {
		t.Fatalf("checkpoint lost images: %v", got)
	}
	input.RunID, input.ResumeCheckpoint = "new-run", nil
	fresh := &scriptedAgent{responses: []AgentResponse{finishCall("new run")}}
	NewRuntime(fresh).Run(context.Background(), input)
	if got := requestImageCounts(fresh.requests[0].Messages); !reflect.DeepEqual(got, map[string]int{selected: 1}) {
		t.Fatalf("new Run inherited read images: %v", got)
	}
}

type runImageCompactor struct{ inputs [][]llm.Message }

func (c *runImageCompactor) Compact(_ context.Context, messages []llm.Message) (contextcompact.Result, error) {
	c.inputs = append(c.inputs, messages)
	return contextcompact.Result{Messages: []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("text summary")}}}, nil
}

func TestRunReadImagesSurviveRepeatedCompactionAndVersionChanges(t *testing.T) {
	old := RenderedImageContext{SlideID: "sli_1", ImagePath: "old.png", SourceHash: "old"}
	newer := RenderedImageContext{SlideID: "sli_1", ImagePath: "new.png", SourceHash: "new"}
	oldRaw, _ := json.Marshal(old)
	newRaw, _ := json.Marshal(newer)
	state := &RunState{runID: "run", mode: model.ModeChat, phase: PhaseChat,
		pack:   testPack(model.ModeChat, model.ScopeCurrentPage, false, "compare"),
		ledger: NewEvidenceLedger(), activeSkills: &ActiveSkillSet{},
		budget: DefaultRuntimeBudget(200000), renderedImages: []RenderedImageContext{newer},
		readImages: []RunReadImage{
			{ImageRef: "project:p1/render:sli_1/shot_old", MIMEType: "image/png", Observation: string(oldRaw)},
			{ImageRef: "project:p1/render:sli_1/shot_new", MIMEType: "image/png", Observation: string(newRaw)},
			{ImageRef: "project:p1/attachment:att_read/original", MIMEType: "image/png"},
		},
	}
	c := &runImageCompactor{}
	runtime := NewRuntime(nil)
	runtime.Compactor = c
	for i := 0; i < 2; i++ {
		runtime.measureContextWindow(RuntimeInput{}, state, nil)
		state.tokens, state.nextCompactionTokens = state.budget.ContextCompactionThreshold, 0
		if err := runtime.compactIfNeeded(context.Background(), RuntimeInput{}, state, nil); err != nil {
			t.Fatal(err)
		}
		if len(requestImageCounts(c.inputs[i])) != 0 {
			t.Fatal("compactor received Run pixels")
		}
		if got := requestImageCounts(state.messages); len(got) != 3 {
			t.Fatalf("compaction %d lost images: %v", i, got)
		}
	}
	var labels string
	for _, message := range state.messages {
		labels += message.Text()
	}
	if !strings.Contains(labels, `"render_state":"superseded"`) || !strings.Contains(labels, `"render_state":"current"`) {
		t.Fatalf("render versions are indistinguishable: %s", labels)
	}
	state.renderedImages[0].Stale = true
	runtime.measureContextWindow(RuntimeInput{}, state, nil)
	for _, message := range state.messages {
		if message.Metadata != nil && message.Metadata.Key == state.readImages[1].ImageRef && !strings.Contains(message.Text(), `"render_state":"stale"`) {
			t.Fatal("dependency changes did not invalidate retained render status")
		}
	}
}

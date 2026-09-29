package workflow

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type loopReadTool struct {
	result func(string) string
}

func (t loopReadTool) RegisterDomainTools(registry *ToolRegistry) error {
	return registry.RegisterDomainTool(t, true, PhaseChat)
}

func (loopReadTool) Schema() ToolSchema {
	return ToolSchema{Name: "read_image", Parameters: objectSchema([]string{"image_path"}, map[string]any{
		"image_path": map[string]any{"type": "string"},
	})}
}

func (t loopReadTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	path := stringValue(input.Args["image_path"])
	content := path
	if t.result != nil {
		content = t.result(path)
	}
	result := SuccessfulToolResult("image read")
	result.ObservationParts = []llm.ContentPart{
		{Type: "text", Text: content},
		{Type: "image", ImageRef: "project:p1/render:sli_1/" + path, MIMEType: "image/png"},
	}
	return result
}

func readLoopCall(index int, paths ...string) AgentResponse {
	response := AgentResponse{}
	for j, path := range paths {
		response.ToolCalls = append(response.ToolCalls, llm.ToolCall{
			ID: fmt.Sprintf("read-%d-%d", index, j), Name: "read_image", Args: map[string]any{"image_path": path},
		})
	}
	return response
}

func TestReadLoopStopsSuccessfulRepeatedAndAlternatingReads(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paths [][]string
		turns int
	}{
		{"same image", [][]string{{"shot_a"}}, 7},
		{"alternating images", [][]string{{"shot_a"}, {"shot_b"}}, 8},
		{"batch counts as one round", [][]string{{"shot_a", "shot_b"}}, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent := &scriptedAgent{}
			for i := 0; i < 16; i++ {
				agent.responses = append(agent.responses, readLoopCall(i, tc.paths[i%len(tc.paths)]...))
			}
			transcript := &recordingTranscript{}
			checkpoints := &checkpointRecorder{}
			events := &eventRecorder{}
			outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
				RunID: "read-loop", ProjectDir: t.TempDir(), DomainTools: loopReadTool{},
				Context:    testPack(model.ModeChat, model.ScopeCurrentPage, false, "inspect"),
				Transcript: transcript, Checkpoint: checkpoints, Emitter: events,
			})
			if outcome.Status != StatusFailed || outcome.Code != CodeReadLoop || len(agent.requests) != tc.turns {
				t.Fatalf("read loop was not bounded: outcome=%+v requests=%d", outcome, len(agent.requests))
			}
			if len(requestImageCounts(transcript.messages)) > 0 {
				t.Fatal("loop protection persisted render pixels")
			}
			last := checkpoints.checkpoints[len(checkpoints.checkpoints)-1]
			if !last.ContinuationAllowed || last.ReadLoop.RepeatRounds != maxRepeatedReadRounds {
				t.Fatalf("terminal checkpoint lost rule stop: %+v", last)
			}
			found := false
			for _, message := range transcript.messages {
				found = found || strings.Contains(message.Text(), "Runtime stopped repeated reads")
			}
			if !found {
				t.Fatal("continuation has no explanation of the stopped loop")
			}
			if events.events[len(events.events)-1].kind != model.EventRunFailed {
				t.Fatal("read loop was not surfaced as a rule-based task failure")
			}
		})
	}
}

func TestReadLoopAllowsNewResourcesAndChangedResults(t *testing.T) {
	for _, freshPaths := range []bool{false, true} {
		t.Run(fmt.Sprintf("fresh_paths=%t", freshPaths), func(t *testing.T) {
			agent := &scriptedAgent{}
			for i := 0; i < 15; i++ {
				path := "shot_a"
				if freshPaths {
					path = fmt.Sprintf("shot_%d", i)
				}
				agent.responses = append(agent.responses, readLoopCall(i, path))
			}
			agent.responses = append(agent.responses, finishCall("finish_task"))
			calls := 0
			reader := loopReadTool{result: func(path string) string {
				calls++
				// A changing observation at the same path is new information.
				return fmt.Sprintf("%s version %d", path, (calls-1)/5)
			}}
			outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
				RunID: "fresh-reads", ProjectDir: t.TempDir(), DomainTools: reader,
				Context: testPack(model.ModeChat, model.ScopeCurrentPage, false, "inspect"),
			})
			if outcome.Status != StatusCompleted || len(agent.requests) != 16 {
				t.Fatalf("legitimate reads stopped: %+v requests=%d", outcome, len(agent.requests))
			}
		})
	}
}

func TestReadLoopAllowsReadsAfterUserSteering(t *testing.T) {
	agent := &scriptedAgent{}
	steering := &scriptedSteering{batches: make([][]SteeringInput, 11)}
	for i := 0; i < 10; i++ {
		agent.responses = append(agent.responses, readLoopCall(i, "shot_a"))
	}
	steering.batches[5] = []SteeringInput{{ID: "new-question", Content: "再检查一下标题的位置"}}
	agent.responses = append(agent.responses, finishCall("finish_task"))
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "steered-reads", ProjectDir: t.TempDir(), DomainTools: loopReadTool{}, Steering: steering,
		Context: testPack(model.ModeChat, model.ScopeCurrentPage, false, "inspect"),
	})
	if outcome.Status != StatusCompleted || len(agent.requests) != 11 {
		t.Fatalf("new user intent did not renew inspection: %+v requests=%d", outcome, len(agent.requests))
	}
}

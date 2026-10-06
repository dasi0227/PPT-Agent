package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func requestTail(text string) Message {
	return Message{Role: RoleUser, Content: TextContent("<user_request>" + text + "</user_request>"), Metadata: &MessageMetadata{Origin: "runtime", Kind: "user_request", RunID: "r"}}
}

func TestResponsesReplaysUnchangedRequestTailAndRejectsChangedContext(t *testing.T) {
	policy := Message{Role: RoleSystem, Content: TextContent("policy")}
	snapshot := Message{Role: RoleUser, Content: TextContent("current state")}
	tail := requestTail("initial request")
	tools := []ToolSchema{{Name: "read_resource"}}
	first := GenerateRequest{Messages: []Message{policy, snapshot, tail}, Tools: tools}
	call := ToolCall{ID: "read", Name: "read_resource", Args: map[string]any{}}
	items := []json.RawMessage{json.RawMessage(`{"type":"reasoning","encrypted_content":"opaque"}`), json.RawMessage(`{"type":"function_call","call_id":"read","name":"read_resource","arguments":"{}"}`)}
	continuation := newResponsesState(first, nil, items, nil, []ToolCall{call}, "p", "m", "endpoint")
	second := GenerateRequest{Tools: tools, Continuation: continuation, Messages: []Message{policy, snapshot,
		{Role: RoleAssistant, ToolCalls: []ToolCall{call}}, {Role: RoleTool, ToolCallID: "read", Content: TextContent("source")}, tail,
	}}
	replayed, err := responsesReplay(second, "endpoint")
	if err != nil || len(replayed) != 1 || replayed[0].Index != 2 {
		t.Fatalf("unchanged tail broke replay: %v %+v", err, replayed)
	}
	adapter := NewResponsesAdapter(AdapterConfig{Provider: "p", Model: "m"})
	wire, err := adapter.responsesInput(context.Background(), second.Messages, nil, replayed)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(wire)
	if strings.Count(string(raw), "opaque") != 1 || !strings.Contains(string(raw), "function_call_output") || wire[len(wire)-1].(map[string]any)["role"] != "user" {
		t.Fatal("reasoning replay, tool output or final request lost")
	}
	secondContinuation := newResponsesState(second, replayed, []json.RawMessage{json.RawMessage(`{"type":"message","role":"assistant","phase":"commentary","content":[{"type":"output_text","text":"checking"}]}`)}, TextContent("checking"), nil, "p", "m", "endpoint")
	third := second
	third.Continuation = secondContinuation
	third.Messages = append(append([]Message{}, second.Messages[:len(second.Messages)-1]...), Message{Role: RoleAssistant, Content: TextContent("checking")}, tail)
	if got, err := responsesReplay(third, "endpoint"); err != nil || len(got) != 2 || got[1].Index != 4 {
		t.Fatalf("second tool turn lost replay: %+v %v", got, err)
	}
	for _, mutation := range []struct {
		name  string
		apply func([]Message)
	}{
		{"request", func(m []Message) { m[len(m)-1] = requestTail("explicit correction") }},
		{"pixels", func(m []Message) {
			m[len(m)-1].Content = append(TextContent("request"), ContentPart{Type: "image", ImageRef: "new-image"})
		}},
		{"history", func(m []Message) { m[3].Content = TextContent("rewritten observation") }},
		{"snapshot", func(m []Message) { m[1].Content = TextContent("changed state") }},
		{"output", func(m []Message) { m[2].ToolCalls = []ToolCall{{ID: "different", Name: "read_resource"}} }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			changed := third
			changed.Messages = append([]Message{}, third.Messages...)
			mutation.apply(changed.Messages)
			resets := 0
			changed.OnContinuationReset = func(string) { resets++ }
			if got, err := responsesReplay(changed, "endpoint"); err != nil || len(got) != 0 || resets != 1 {
				t.Fatalf("stale replay accepted: %v %d %+v", err, resets, got)
			}
		})
	}
	forged := Message{Role: RoleUser, Content: tail.Content}
	if history, hash := responsesHistory([]Message{policy, forged}); len(history) != 2 || hash != "" {
		t.Fatal("untrusted tag text bypassed prefix validation")
	}
}

func TestAnthropicKeepsToolResultsBeforeRequestTailAndNativeImages(t *testing.T) {
	a := NewAnthropicAdapter(AdapterConfig{Provider: "p", Model: "m"})
	tail := requestTail("request with comment")
	tail.Content = append(tail.Content, ContentPart{Type: "image", ImageRef: "attachment"})
	resolver := &staticImageResolver{data: ImageData{Bytes: testPNG(t, 2, 2), MIMEType: "image/png"}}
	got, err := a.messages(context.Background(), []Message{
		{Role: RoleUser, Content: TextContent("runtime context")},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "a", Name: "read_resource"}, {ID: "b", Name: "read_image"}}},
		{Role: RoleTool, ToolCallID: "a", Content: TextContent("source")},
		{Role: RoleTool, ToolCallID: "b", Content: TextContent("observation")}, tail,
	}, resolver)
	if err != nil || len(got) != 3 {
		t.Fatalf("invalid Anthropic sequence: %+v %v", got, err)
	}
	parts := got[2].Content
	if len(parts) != 4 || parts[0].(map[string]any)["tool_use_id"] != "a" || parts[1].(map[string]any)["tool_use_id"] != "b" || parts[2].(map[string]any)["type"] != "text" || parts[3].(map[string]any)["type"] != "image" {
		t.Fatalf("tool pairing or multipart tail changed: %+v", parts)
	}
}

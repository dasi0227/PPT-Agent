package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func requestInstruction(text string) Message {
	return Message{Role: RoleUser, Content: TextContent("<user_instruction>" + text + "</user_instruction>"), Metadata: &MessageMetadata{Origin: "runtime", Kind: "user_instruction", RunID: "r"}}
}

func TestResponsesReplaysChronologicalTurnsAndRejectsChangedContext(t *testing.T) {
	policy := Message{Role: RoleSystem, Content: TextContent("policy")}
	snapshot := Message{Role: RoleUser, Content: TextContent("current state")}
	instruction := requestInstruction("increase the font size")
	tools := []ToolSchema{{Name: "edit_html"}, {Name: "render_slide"}}
	open := Message{Role: RoleUser, Content: TextContent("<conversation_history>"), Metadata: &MessageMetadata{Origin: "runtime", Kind: "conversation_history"}}
	close := Message{Role: RoleUser, Content: TextContent("</conversation_history>"), Metadata: &MessageMetadata{Origin: "runtime", Kind: "conversation_history"}}
	first := GenerateRequest{Messages: []Message{policy, snapshot, open, close, instruction}, Tools: tools}
	call := ToolCall{ID: "edit", Name: "edit_html", Args: map[string]any{}}
	items := []json.RawMessage{json.RawMessage(`{"type":"reasoning","encrypted_content":"opaque"}`), json.RawMessage(`{"type":"function_call","call_id":"edit","name":"edit_html","arguments":"{}"}`)}
	continuation := newResponsesState(first, nil, items, nil, []ToolCall{call}, "p", "m", "endpoint")
	second := GenerateRequest{Tools: tools, Continuation: continuation, Messages: append(append([]Message{}, first.Messages...),
		Message{Role: RoleAssistant, ToolCalls: []ToolCall{call}}, Message{Role: RoleTool, ToolCallID: "edit", Content: TextContent("success")},
	)}
	replayed, err := responsesReplay(second, "endpoint")
	if err != nil || len(replayed) != 1 || replayed[0].Index != 5 {
		t.Fatalf("chronological tool turn lost replay: %v %+v", err, replayed)
	}
	adapter := NewResponsesAdapter(AdapterConfig{Provider: "p", Model: "m"})
	wire, err := adapter.responsesInput(context.Background(), second.Messages, nil, replayed)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(wire)
	if strings.Count(string(raw), "opaque") != 1 || strings.Count(string(raw), "increase the font size") != 1 || wire[len(wire)-1].(map[string]any)["type"] != "function_call_output" {
		t.Fatal("reasoning replay lost or original instruction followed its tool result")
	}
	render := ToolCall{ID: "render", Name: "render_slide", Args: map[string]any{}}
	secondContinuation := newResponsesState(second, replayed, []json.RawMessage{json.RawMessage(`{"type":"function_call","call_id":"render","name":"render_slide","arguments":"{}"}`)}, nil, []ToolCall{render}, "p", "m", "endpoint")
	addition := Message{Role: RoleUser, Content: TextContent("<user_addition>keep the color</user_addition>"), Metadata: &MessageMetadata{Origin: "runtime", Kind: "user_addition", RunID: "r"}}
	directive := Message{Role: RoleUser, Content: TextContent("<runtime_instruction>resolve the reported issue</runtime_instruction>"), Metadata: &MessageMetadata{Origin: "runtime", Kind: "runtime_instruction_projection", RunID: "r"}}
	third := GenerateRequest{Tools: tools, Continuation: secondContinuation, Messages: append(append([]Message{}, second.Messages...),
		Message{Role: RoleAssistant, ToolCalls: []ToolCall{render}},
		Message{Role: RoleTool, ToolCallID: "render", Content: append(TextContent("rendered"), ContentPart{Type: "image", ImageRef: "render-image"})},
		addition, directive,
	)}
	replayed, err = responsesReplay(third, "endpoint")
	if err != nil || len(replayed) != 2 || replayed[1].Index != 7 {
		t.Fatalf("new input invalidated earlier tool rounds: %+v %v", replayed, err)
	}
	thirdContinuation := newResponsesState(third, replayed, []json.RawMessage{json.RawMessage(`{"type":"message","role":"assistant","phase":"commentary","content":[{"type":"output_text","text":"checking"}]}`)}, TextContent("checking"), nil, "p", "m", "endpoint")
	fourth := GenerateRequest{Tools: tools, Continuation: thirdContinuation, Messages: append(append([]Message{}, third.Messages...), Message{Role: RoleAssistant, Content: TextContent("checking")})}
	replayed, err = responsesReplay(fourth, "endpoint")
	if err != nil || len(replayed) != 3 || replayed[2].Index != 11 {
		t.Fatalf("consumed instructions changed their prefix: %+v %v", replayed, err)
	}
	resolver := &staticImageResolver{data: ImageData{Bytes: testPNG(t, 2, 2), MIMEType: "image/png"}}
	wire, err = adapter.responsesInput(context.Background(), fourth.Messages, resolver, replayed)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(wire)
	if !strings.Contains(string(raw), `"phase":"commentary"`) || !strings.Contains(string(raw), `"type":"input_image"`) || strings.Count(string(raw), "keep the color") != 1 {
		t.Fatal("native output metadata, image or addition lost")
	}
	for _, mutation := range []struct {
		name  string
		apply func([]Message)
	}{
		{"request", func(m []Message) { m[4] = requestInstruction("explicit correction") }},
		{"request pixels", func(m []Message) {
			m[4].Content = append(TextContent("request"), ContentPart{Type: "image", ImageRef: "new-image"})
		}},
		{"tool result", func(m []Message) { m[6].Content = TextContent("rewritten observation") }},
		{"tool pixels", func(m []Message) { m[8].Content = []ContentPart{{Type: "image", ImageRef: "different-image"}} }},
		{"addition", func(m []Message) { m[9].Content = TextContent("changed addition") }},
		{"runtime instruction", func(m []Message) { m[10].Content = TextContent("changed directive") }},
		{"snapshot", func(m []Message) { m[1].Content = TextContent("changed state") }},
		{"output", func(m []Message) { m[5].ToolCalls = []ToolCall{{ID: "different", Name: "edit_html"}} }},
		{"history boundary", func(m []Message) { m[3].Content = TextContent("changed boundary") }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			changed := fourth
			changed.Messages = append([]Message{}, fourth.Messages...)
			mutation.apply(changed.Messages)
			resets := 0
			changed.OnContinuationReset = func(string) { resets++ }
			if got, err := responsesReplay(changed, "endpoint"); err != nil || len(got) != 0 || resets != 1 {
				t.Fatalf("stale replay accepted: %v %d %+v", err, resets, got)
			}
		})
	}
}

func TestAnthropicKeepsInputsBeforeTheirToolRoundsAndNativeImages(t *testing.T) {
	a := NewAnthropicAdapter(AdapterConfig{Provider: "p", Model: "m"})
	instruction := requestInstruction("increase the font size")
	instruction.Content = append(instruction.Content, ContentPart{Type: "image", ImageRef: "attachment"})
	resolver := &staticImageResolver{data: ImageData{Bytes: testPNG(t, 2, 2), MIMEType: "image/png"}}
	got, err := a.messages(context.Background(), []Message{
		{Role: RoleUser, Content: TextContent("runtime context")},
		{Role: RoleUser, Content: TextContent("<conversation_history>")},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "old", Name: "read_resource"}}},
		{Role: RoleTool, ToolCallID: "old", Content: TextContent("previous source")},
		{Role: RoleUser, Content: TextContent("</conversation_history>")}, instruction,
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "a", Name: "read_resource"}, {ID: "b", Name: "read_image"}}},
		{Role: RoleTool, ToolCallID: "a", Content: TextContent("source")},
		{Role: RoleTool, ToolCallID: "b", Content: append(TextContent("observation"), ContentPart{Type: "image", ImageRef: "tool-image"})},
		{Role: RoleUser, Content: TextContent("<user_addition>keep the color</user_addition>")},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c", Name: "edit_html"}}},
		{Role: RoleTool, ToolCallID: "c", Content: TextContent("success")},
	}, resolver)
	if err != nil || len(got) != 7 {
		t.Fatalf("invalid Anthropic sequence: %+v %v", got, err)
	}
	requestParts := got[2].Content
	if len(requestParts) != 4 || requestParts[0].(map[string]any)["tool_use_id"] != "old" || requestParts[1].(map[string]any)["text"] != "</conversation_history>" ||
		requestParts[2].(map[string]any)["text"] != instruction.Text() || requestParts[3].(map[string]any)["type"] != "image" {
		t.Fatal("original request or its references moved out of the initial user turn")
	}
	parts := got[4].Content
	if len(parts) != 3 || parts[0].(map[string]any)["tool_use_id"] != "a" || parts[1].(map[string]any)["tool_use_id"] != "b" || parts[2].(map[string]any)["text"] != "<user_addition>keep the color</user_addition>" {
		t.Fatalf("tool pairing or chronological addition changed: %+v", parts)
	}
	toolParts := parts[1].(map[string]any)["content"].([]any)
	if len(toolParts) != 2 || toolParts[1].(map[string]any)["type"] != "image" {
		t.Fatal("tool image escaped its result")
	}
	if got[5].Role != "assistant" || got[6].Role != "user" || len(got[6].Content) != 1 || got[6].Content[0].(map[string]any)["tool_use_id"] != "c" {
		t.Fatal("answered user input was repeated after its tool result")
	}
}

package llm

import (
	"encoding/json"
	"testing"
)

func TestEstimateTextTokensUsesO200kBase(t *testing.T) {
	for _, test := range []struct {
		text string
		want int
	}{
		{"", 0},
		{"'RE", 2},
		{"hello world", 2},
		{"supercalifragilistic", 6},
	} {
		if got := EstimateTextTokens(test.text); got != test.want {
			t.Errorf("EstimateTextTokens(%q)=%d, want %d", test.text, got, test.want)
		}
	}
}

func TestEstimateReplayTokensDoesNotCountNormalizedContentTwice(t *testing.T) {
	thinking := json.RawMessage(`{"type":"thinking","thinking":"inspect layout","signature":"signed-value"}`)
	message := Message{
		Role:      RoleAssistant,
		Content:   TextContent("I will read the page."),
		ToolCalls: []ToolCall{{ID: "read-1", Name: "read_resource", Args: map[string]any{"path": "slide.html"}}},
	}
	withoutReplay := EstimateMessageTokens(message)
	message.Replay = &AssistantReplay{Source: "anthropic", Blocks: []json.RawMessage{
		thinking,
		json.RawMessage(`{"type":"text","text":"I will read the page."}`),
		json.RawMessage(`{"type":"tool_use","id":"read-1","name":"read_resource","input":{"path":"slide.html"}}`),
	}}
	if got, want := EstimateMessageTokens(message)-withoutReplay, EstimateTextTokens(string(thinking)); got != want {
		t.Fatalf("replay adds %d tokens, want only %d thinking tokens", got, want)
	}
}

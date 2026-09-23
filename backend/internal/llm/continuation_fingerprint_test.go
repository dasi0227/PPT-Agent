package llm

import "testing"

func TestContinuationRejectsChangedPrefixAndOmittedOutput(t *testing.T) {
	input := []Message{{Role: RoleSystem, Content: TextContent("policy")}, {Role: RoleUser, Content: TextContent("five pages")}}
	output := Message{Role: RoleAssistant, Content: TextContent("working")}
	c := openAIContinuation{MessageCount: len(input), InputHash: continuationFingerprint(input), ToolsHash: continuationFingerprint([]ToolSchema(nil)), OutputHash: continuationMessageFingerprint(output)}
	req := GenerateRequest{Messages: append(append([]Message{}, input...), output, Message{Role: RoleUser, Content: TextContent("continue")})}
	if reason := openAIContinuationMismatch(c, req); reason != "" {
		t.Fatal(reason)
	}
	req.Messages[1].Content = TextContent("six pages")
	if reason := openAIContinuationMismatch(c, req); reason != "history_prefix_changed" {
		t.Fatal(reason)
	}
	req.Messages[1] = input[1]
	req.Messages[2].Content = TextContent("rewritten")
	if reason := openAIContinuationMismatch(c, req); reason != "previous_output_changed" {
		t.Fatal(reason)
	}
}

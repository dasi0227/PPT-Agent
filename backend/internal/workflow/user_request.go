package workflow

import "github.com/dasi0227/PPT-Agent/backend/internal/llm"

// requestConversation is a model-facing projection, never a transcript edit.
// Ordinary inputs move to the tail; tool-delivered answers also stay paired
// with their originating calls in history.
func requestConversation(messages []llm.Message, runID string) []llm.Message {
	history := make([]llm.Message, 0, len(messages)+1)
	parts := llm.TextContent(`<user_request desc="Contains the current Run's original request, subsequent user inputs and clarification answers in their original order.">`)
	for _, message := range messages {
		if !llm.IsRunInput(message, runID) {
			history = append(history, message)
			continue
		}
		if message.Role == llm.RoleTool {
			history = append(history, message)
		}
		parts = append(parts, llm.ContentPart{Type: "text", Text: `<request_input kind="` + message.Metadata.Kind + `">`})
		parts = append(parts, message.Content...)
		parts = append(parts, llm.ContentPart{Type: "text", Text: "</request_input>"})
	}
	parts = append(parts, llm.ContentPart{Type: "text", Text: "</user_request>"})
	return append(history, llm.Message{Role: llm.RoleUser, Content: parts,
		Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "user_request", RunID: runID}})
}

func requestContextMessages(req AgentRequest) []llm.Message {
	out := append([]llm.Message{}, req.RuntimeContext...)
	return append(out, requestConversation(req.Messages, req.RunID)...)
}

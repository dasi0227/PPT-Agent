package workflow

import "github.com/dasi0227/PPT-Agent/backend/internal/llm"

// requestConversation preserves chronology in both the transcript and the
// provider request. Only prior Runs belong inside conversation_history; current
// inputs stay before the assistant/tool rounds that respond to them.
func requestConversation(messages []llm.Message, runID string) []llm.Message {
	var summaries, history, current []llm.Message
	inCurrentRun := false
	for _, message := range messages {
		if m := message.Metadata; message.Role == llm.RoleUser && m != nil && m.Origin == "runtime" && m.Kind == "summary" {
			summaries = append(summaries, message)
			continue
		}
		// The original instruction is retained by compaction and marks a safe
		// boundary before this Run's first assistant/tool round.
		if message.Role == llm.RoleUser && llm.IsRunInput(message, runID) && message.Metadata.Kind == "instruction" {
			inCurrentRun = true
		}
		if !inCurrentRun {
			history = append(history, message)
			continue
		}
		if message.Role == llm.RoleUser && llm.IsRunInput(message, runID) {
			if message.Metadata.Kind == "instruction" {
				message = wrapRequestMessage(message, "user_instruction", runID, "user_instruction")
			} else {
				message = wrapRequestMessage(message, "user_addition", runID, "user_addition")
			}
		} else if m := message.Metadata; message.Role == llm.RoleUser && m != nil && m.Origin == "runtime" && m.RunID == runID && m.Kind == "runtime_instruction" {
			message = wrapRequestMessage(message, "runtime_instruction_projection", runID, "runtime_instruction")
		}
		current = append(current, message)
	}
	out := append(summaries, llm.Message{Role: llm.RoleUser, Content: llm.TextContent("<conversation_history>"),
		Metadata: runtimeControlMetadata("conversation_history", runID)})
	out = append(out, history...)
	out = append(out, llm.Message{Role: llm.RoleUser, Content: llm.TextContent("</conversation_history>"),
		Metadata: runtimeControlMetadata("conversation_history", runID)})
	return append(out, current...)
}

func wrapRequestMessage(message llm.Message, kind, runID, tag string) llm.Message {
	parts := make([]llm.ContentPart, 0, len(message.Content)+2)
	parts = append(parts, llm.ContentPart{Type: "text", Text: "<" + tag + ">"})
	parts = append(parts, message.Content...)
	parts = append(parts, llm.ContentPart{Type: "text", Text: "</" + tag + ">"})
	message.Content = parts
	message.Metadata = runtimeControlMetadata(kind, runID)
	return message
}

func requestContextMessages(req AgentRequest) []llm.Message {
	out := append([]llm.Message{}, req.RuntimeContext...)
	return append(out, requestConversation(req.Messages, req.RunID)...)
}

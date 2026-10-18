package workflow

import "github.com/dasi0227/PPT-Agent/backend/internal/llm"

// requestConversation projects the request without changing durable chronology.
// Only ordinary current-Run inputs move; answers remain in their tool rounds.
func requestConversation(messages []llm.Message, runID string) []llm.Message {
	var instruction *llm.Message
	instructionIndex := -1
	for i := len(messages) - 1; i >= 0; i-- {
		last := messages[i]
		// New user additions move into user_instruction and do not consume a
		// pending Runtime directive before the next assistant turn.
		if last.Role == llm.RoleUser && llm.IsRunInput(last, runID) {
			continue
		}
		if last.Role == llm.RoleUser && last.Metadata != nil && last.Metadata.Origin == "runtime" && last.Metadata.RunID == runID && last.Metadata.Kind == "runtime_instruction" {
			instruction = &last
			instructionIndex = i
		}
		break
	}

	var summaries, history []llm.Message
	parts := llm.TextContent("<user_instruction>")
	for i, message := range messages {
		if i == instructionIndex {
			continue
		}
		if m := message.Metadata; message.Role == llm.RoleUser && m != nil && m.Origin == "runtime" && m.Kind == "summary" {
			summaries = append(summaries, message)
			continue
		}
		if message.Role != llm.RoleUser || !llm.IsRunInput(message, runID) {
			history = append(history, message)
			continue
		}
		tag := "user_addition"
		if message.Metadata.Kind == "instruction" {
			tag = "user_request"
		}
		parts = append(parts, llm.ContentPart{Type: "text", Text: "<" + tag + ">"})
		parts = append(parts, message.Content...)
		parts = append(parts, llm.ContentPart{Type: "text", Text: "</" + tag + ">"})
	}
	parts = append(parts, llm.ContentPart{Type: "text", Text: "</user_instruction>"})
	out := append(summaries, llm.Message{Role: llm.RoleUser, Content: llm.TextContent("<conversation_history>"),
		Metadata: runtimeControlMetadata("conversation_history", runID)})
	out = append(out, history...)
	out = append(out, llm.Message{Role: llm.RoleUser, Content: llm.TextContent("</conversation_history>"),
		Metadata: runtimeControlMetadata("conversation_history", runID)},
		llm.Message{Role: llm.RoleUser, Content: parts, Metadata: runtimeControlMetadata("user_instruction", runID)})
	if instruction != nil {
		content := llm.TextContent("<runtime_instruction>")
		content = append(content, instruction.Content...)
		content = append(content, llm.ContentPart{Type: "text", Text: "</runtime_instruction>"})
		out = append(out, llm.Message{Role: llm.RoleUser, Content: content,
			Metadata: runtimeControlMetadata("runtime_instruction_projection", runID)})
	}
	return out
}

func requestContextMessages(req AgentRequest) []llm.Message {
	out := append([]llm.Message{}, req.RuntimeContext...)
	return append(out, requestConversation(req.Messages, req.RunID)...)
}

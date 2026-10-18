package llm

// NormalizeHistory prepares current-protocol messages for replay and compaction.
// Native content, including tool images, remains attached to its source message.
func NormalizeHistory(messages []Message) []Message {
	return WithoutRequestContext(messages)
}

// Derived request snapshots are never conversation history. Match trusted
// metadata only; text supplied by users or tools is preserved verbatim.
func WithoutRequestContext(messages []Message) []Message {
	out := make([]Message, 0, len(messages))
	for _, message := range messages {
		if m := message.Metadata; m != nil && m.Origin == "runtime" {
			switch m.Kind {
			case "context", "user_instruction", "conversation_history", "runtime_instruction_projection":
				continue
			}
		}
		out = append(out, message)
	}
	return out
}

// IsRunInput recognizes only application-stamped user input, never tag text.
// Clarifications and approval decisions remain paired with their original calls.
func IsRunInput(message Message, runID string) bool {
	m := message.Metadata
	if m == nil || m.Origin != "user" || m.RunID != runID {
		return false
	}
	switch message.Role {
	case RoleUser:
		return m.Kind == "instruction" || m.Kind == "steering" || m.Kind == "feedback"
	case RoleTool:
		return m.Kind == "clarification" || m.Kind == "plan_feedback" || m.Kind == "command_feedback" || m.Kind == "scope_feedback" || m.Kind == "resource_edit_feedback"
	}
	return false
}

func LatestInputRunID(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if m := messages[i].Metadata; m != nil && m.RunID != "" && IsRunInput(messages[i], m.RunID) {
			return m.RunID
		}
	}
	return ""
}

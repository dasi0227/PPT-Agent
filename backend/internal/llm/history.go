package llm

// NormalizeHistory prepares current-protocol messages for replay and compaction.
// It omits Run-owned pixels, without translating historical tool protocols.
func NormalizeHistory(messages []Message) []Message {
	return WithoutRenderImages(WithoutRunImageMessages(WithoutRequestContext(messages)))
}

// Derived request snapshots are never conversation history. Match trusted
// metadata only; text supplied by users or tools is preserved verbatim.
func WithoutRequestContext(messages []Message) []Message {
	out := make([]Message, 0, len(messages))
	for _, message := range messages {
		if m := message.Metadata; m != nil && m.Origin == "runtime" && (m.Kind == "context" || m.Kind == "user_request") {
			continue
		}
		out = append(out, message)
	}
	return out
}

// IsRunInput recognizes only application-stamped user input, never tag text.
// Clarifications remain tool results so their call/result protocol survives.
func IsRunInput(message Message, runID string) bool {
	m := message.Metadata
	if m == nil || m.Origin != "user" || m.RunID != runID {
		return false
	}
	switch message.Role {
	case RoleUser:
		return m.Kind == "instruction" || m.Kind == "steering" || m.Kind == "feedback"
	case RoleTool:
		return m.Kind == "clarification" || m.Kind == "plan_feedback"
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

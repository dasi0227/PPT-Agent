package llm

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

func continuationFingerprint(value any) string {
	raw, _ := json.Marshal(value)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

// Validate the old input and the response that native continuation omits.
// Counting messages alone misses edits of equal-length histories.
func openAIContinuationMismatch(c openAIContinuation, req GenerateRequest) string {
	if c.MessageCount < 0 || c.MessageCount >= len(req.Messages) {
		return "history_length_changed"
	}
	if c.InputHash == "" || c.InputHash != continuationFingerprint(req.Messages[:c.MessageCount]) {
		return "history_prefix_changed"
	}
	if c.ToolsHash != continuationFingerprint(req.Tools) {
		return "tools_changed"
	}
	if c.OutputHash != continuationMessageFingerprint(req.Messages[c.MessageCount]) {
		return "previous_output_changed"
	}
	for _, message := range req.Messages[c.MessageCount+1:] {
		if message.Role == RoleAssistant {
			return "additional_assistant_message"
		}
	}
	return ""
}

func continuationMessageFingerprint(message Message) string {
	// Providers may omit empty arrays that the runtime reconstructs as nil.
	message.Metadata = nil
	if len(message.Content) == 0 {
		message.Content = nil
	}
	if len(message.ToolCalls) == 0 {
		message.ToolCalls = nil
	}
	return continuationFingerprint(message)
}

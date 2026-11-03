package sessionstate

import (
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

// NativeMessage includes local provenance and provider replay metadata, which
// llm.Message deliberately excludes from provider-facing JSON serialization.
type NativeMessage struct {
	FormatVersion int                  `json:"format_version"`
	Role          llm.Role             `json:"role"`
	Content       []llm.ContentPart    `json:"content"`
	ToolCallID    string               `json:"tool_call_id,omitempty"`
	ToolCalls     []llm.ToolCall       `json:"tool_calls,omitempty"`
	Metadata      *llm.MessageMetadata `json:"metadata,omitempty"`
	Replay        *llm.AssistantReplay `json:"assistant_replay,omitempty"`
}

func EncodeMessage(message llm.Message) ([]byte, error) {
	return json.Marshal(NativeMessage{FormatVersion: 1, Role: message.Role, Content: message.Content, ToolCallID: message.ToolCallID, ToolCalls: message.ToolCalls, Metadata: message.Metadata, Replay: message.Replay})
}

func DecodeMessage(raw []byte) (llm.Message, error) {
	var message NativeMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		return llm.Message{}, err
	}
	if message.FormatVersion != 1 {
		return llm.Message{}, ErrInvalidState
	}
	switch message.Role {
	case llm.RoleUser, llm.RoleAssistant, llm.RoleTool:
	default:
		return llm.Message{}, ErrInvalidState
	}
	return llm.Message{Role: message.Role, Content: message.Content, ToolCallID: message.ToolCallID, ToolCalls: message.ToolCalls, Metadata: message.Metadata, Replay: message.Replay}, nil
}

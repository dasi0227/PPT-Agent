package llm

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// Replayable output is client-owned state, not a server-side conversation ID.
// Keep reasoning and message metadata intact without adding them to Runtime text.
type responsesTurn struct {
	Index      int               `json:"index"`
	PrefixHash string            `json:"prefix_hash"`
	OutputHash string            `json:"output_hash"`
	Items      []json.RawMessage `json:"items"`
}
type responsesState struct {
	Protocol     string          `json:"protocol"`
	EndpointHash string          `json:"endpoint_hash"`
	ToolsHash    string          `json:"tools_hash"`
	RequestHash  string          `json:"request_hash"`
	Turns        []responsesTurn `json:"turns"`
}

// The current Run request is projected after history on every call. Fingerprint
// it separately so moving the unchanged tail past a new tool round preserves
// replay, while any change to its text, references, pixels or Run resets it.
// Only trusted metadata can identify the tail; user-provided tags cannot.
func responsesHistory(messages []Message) ([]Message, string) {
	if len(messages) > 0 {
		last := messages[len(messages)-1]
		if m := last.Metadata; m != nil && m.Origin == "runtime" && m.Kind == "user_request" && last.Role == RoleUser {
			return messages[:len(messages)-1], continuationFingerprint(struct {
				RunID string
				Input Message
			}{m.RunID, last})
		}
	}
	return messages, ""
}

func continuationFingerprint(value any) string {
	raw, _ := json.Marshal(value)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func continuationMessageFingerprint(message Message) string {
	message.Metadata = nil
	if len(message.Content) == 0 {
		message.Content = nil
	}
	if len(message.ToolCalls) == 0 {
		message.ToolCalls = nil
	}
	return continuationFingerprint(message)
}

func responsesReplay(req GenerateRequest, endpoint string) ([]responsesTurn, error) {
	if req.Continuation == nil || len(req.Continuation.Opaque) == 0 {
		return nil, nil
	}
	var state responsesState
	if err := json.Unmarshal(req.Continuation.Opaque, &state); err != nil || state.Protocol != ProtocolResponses {
		return nil, fmt.Errorf("%w: invalid responses state", ErrBadRequest)
	}
	if state.EndpointHash != continuationFingerprint(endpoint) {
		return nil, fmt.Errorf("%w: state belongs to another endpoint", ErrBadRequest)
	}
	if state.ToolsHash != continuationFingerprint(req.Tools) {
		if req.OnContinuationReset != nil {
			req.OnContinuationReset("tools_changed")
		}
		return nil, nil
	}
	history, requestHash := responsesHistory(req.Messages)
	if state.RequestHash != requestHash {
		if req.OnContinuationReset != nil {
			req.OnContinuationReset("user_request_changed")
		}
		return nil, nil
	}
	for _, turn := range state.Turns {
		if turn.Index < 0 || turn.Index >= len(history) || history[turn.Index].Role != RoleAssistant ||
			turn.PrefixHash != continuationFingerprint(history[:turn.Index]) ||
			turn.OutputHash != continuationMessageFingerprint(history[turn.Index]) {
			if req.OnContinuationReset != nil {
				req.OnContinuationReset("history_changed")
			}
			return nil, nil
		}
	}
	return state.Turns, nil
}

func replayItems(turns []responsesTurn, index int) []json.RawMessage {
	for _, turn := range turns {
		if turn.Index == index {
			return turn.Items
		}
	}
	return nil
}

func newResponsesState(req GenerateRequest, turns []responsesTurn, items []json.RawMessage, content []ContentPart, calls []ToolCall, provider, model, endpoint string) *ProviderContinuation {
	history, requestHash := responsesHistory(req.Messages)
	turns = append(turns, responsesTurn{
		Index: len(history), PrefixHash: continuationFingerprint(history),
		OutputHash: continuationMessageFingerprint(Message{Role: RoleAssistant, Content: content, ToolCalls: calls}), Items: items,
	})
	raw, _ := json.Marshal(responsesState{Protocol: ProtocolResponses, EndpointHash: continuationFingerprint(endpoint), ToolsHash: continuationFingerprint(req.Tools), RequestHash: requestHash, Turns: turns})
	return &ProviderContinuation{Provider: provider, Model: model, Opaque: raw}
}

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

// Fingerprint the current instruction independently of its moving position.
// History boundaries do not occupy replay positions. A pending Runtime directive
// occupies its original history position, before the response it will produce.
func responsesHistory(messages []Message) ([]Message, string) {
	history := make([]Message, 0, len(messages))
	requestHash := ""
	for _, message := range messages {
		if m := message.Metadata; m != nil && m.Origin == "runtime" && m.Kind == "user_instruction" && message.Role == RoleUser {
			requestHash = continuationFingerprint(struct {
				RunID string
				Input Message
			}{m.RunID, message})
		}
		if canonical, ok := responsesHistoryMessage(message); ok {
			history = append(history, canonical)
		}
	}
	return history, requestHash
}

// Only application provenance identifies projection markup, never tag text.
func responsesHistoryMessage(message Message) (Message, bool) {
	if m := message.Metadata; m != nil && m.Origin == "runtime" && message.Role == RoleUser {
		switch m.Kind {
		case "user_instruction", "conversation_history":
			return Message{}, false
		case "runtime_instruction_projection":
			if len(message.Content) >= 2 {
				message.Content = message.Content[1 : len(message.Content)-1]
			}
		}
	}
	return message, true
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
			req.OnContinuationReset("user_instruction_changed")
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

package llm

import "encoding/json"

const (
	MessageEnvelopeTokens = 4
	ImageApproxTokens     = 1024
)

// Text uses one fixed o200k_base vocabulary for every model. Message framing
// and images remain estimates shared by context accounting and capacity checks.
func EstimateTextTokens(value string) int {
	if value == "" {
		return 0
	}
	return contextTokenCounter().count(value)
}

func EstimateValueTokens(value any) int {
	raw, _ := json.Marshal(value)
	return EstimateTextTokens(string(raw))
}

// Tool definitions already contain the final description and input annotations.
func EstimateToolTokens(tools []ToolSchema) int {
	if len(tools) == 0 {
		return 0
	}
	return EstimateValueTokens(tools)
}

func EstimateMessageEnvelopeTokens(message Message) int {
	return MessageEnvelopeTokens + EstimateTextTokens(message.ToolCallID)
}

func EstimateContentPartTokens(part ContentPart) int {
	if part.Type == "image" {
		return ImageApproxTokens
	}
	return EstimateTextTokens(part.Text)
}

// Replay blocks include the text and tool calls already counted on Message.
// Count only the remaining blocks, from source rather than a persisted estimate.
func EstimateReplayTokens(replay *AssistantReplay) int {
	if replay == nil {
		return 0
	}
	total := 0
	for _, raw := range replay.Blocks {
		var block struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(raw, &block) == nil && (block.Type == "text" || block.Type == "tool_use") {
			continue
		}
		total += EstimateTextTokens(string(raw))
	}
	return total
}

func EstimateMessageTokens(message Message) int {
	total := EstimateMessageEnvelopeTokens(message) + EstimateReplayTokens(message.Replay)
	for _, call := range message.ToolCalls {
		total += EstimateValueTokens(call)
	}
	for _, part := range message.Content {
		total += EstimateContentPartTokens(part)
	}
	return total
}

func EstimateRequestTokens(req GenerateRequest) int {
	total := EstimateToolTokens(req.Tools)
	for _, message := range req.Messages {
		total += EstimateMessageTokens(message)
	}
	return total
}

package llm

import (
	"encoding/json"
	"unicode/utf8"
)

// These estimates are shared by context accounting and route capability checks.
func EstimateTextTokens(value string) int {
	runes := utf8.RuneCountInString(value)
	if runes == 0 {
		return 0
	}
	return (runes+2)/3 + 1
}
func EstimateValueTokens(value any) int {
	raw, _ := json.Marshal(value)
	return EstimateTextTokens(string(raw))
}
func EstimateMessageTokens(message Message) int {
	total := 4
	for _, part := range message.Content {
		if part.Type == "image" {
			total += 1024
		} else {
			total += EstimateTextTokens(part.Text)
		}
	}
	return total + EstimateValueTokens(message.ToolCalls) + EstimateTextTokens(message.ToolCallID)
}
func EstimateRequestTokens(req GenerateRequest) int {
	total := EstimateValueTokens(req.Tools)
	for _, message := range req.Messages {
		total += EstimateMessageTokens(message)
	}
	return total
}

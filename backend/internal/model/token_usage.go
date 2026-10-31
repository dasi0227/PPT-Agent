package model

import (
	"errors"
	"strings"
)

// TokenUsage contains provider-reported consumption, never a prompt estimate.
type TokenUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}

// A nil Usage means the request ended without a usable consumption report.
type ModelCallUsage struct {
	CallID     string      `json:"call_id"`
	Provider   string      `json:"provider"`
	Model      string      `json:"model"`
	DurationMS int64       `json:"duration_ms"`
	Usage      *TokenUsage `json:"usage"`
}

type RunUsageUpdatedPayload struct {
	PublicEventBase
	Call ModelCallUsage `json:"call"`
}

func validateModelCallUsage(value any) error {
	call, ok := value.(map[string]any)
	if !ok || strings.TrimSpace(stringValue(call["call_id"])) == "" ||
		strings.TrimSpace(stringValue(call["provider"])) == "" || strings.TrimSpace(stringValue(call["model"])) == "" ||
		!isInteger(call["duration_ms"]) || int64Value(call["duration_ms"]) < 0 {
		return errors.New("invalid model call usage identity")
	}
	raw, exists := call["usage"]
	if !exists {
		return errors.New("model call usage must be reported or null")
	}
	if raw == nil {
		return nil
	}
	usage, ok := raw.(map[string]any)
	if !ok {
		return errors.New("invalid model call token counts")
	}
	for _, key := range []string{"input_tokens", "output_tokens", "total_tokens"} {
		if !isInteger(usage[key]) || int64Value(usage[key]) < 0 || int64Value(usage[key]) > 1<<53-1 {
			return errors.New("invalid model call token counts")
		}
	}
	if int64Value(usage["total_tokens"]) < int64Value(usage["input_tokens"])+int64Value(usage["output_tokens"]) {
		return errors.New("model call total is smaller than its input and output")
	}
	return nil
}

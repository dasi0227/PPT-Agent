package llm

import (
	"encoding/json"
	"errors"
	"strings"
)

// SubmissionError contains constraint descriptions, never submitted values.
type SubmissionError struct{ Code, Field, Message string }

func (e *SubmissionError) Error() string { return e.Message }

func SubmissionFailure(code, field, message string) error {
	return &SubmissionError{code, field, message}
}

// Missing/duplicate call IDs cannot be safely paired with failure replies.
func CanReplaySubmission(response GenerateResponse) bool {
	seen := map[string]bool{}
	for _, call := range response.ToolCalls {
		if strings.TrimSpace(call.ID) == "" || seen[call.ID] {
			return false
		}
		seen[call.ID] = true
	}
	return true
}

func SubmissionDiagnostic(response GenerateResponse, err error) map[string]any {
	names := make([]string, 0, len(response.ToolCalls))
	for _, call := range response.ToolCalls {
		names = append(names, sanitizeProviderDiagnostic(call.Name, 96))
	}
	d := map[string]any{"tool_count": len(response.ToolCalls), "tool_names": names, "has_text": strings.TrimSpace(response.Text()) != ""}
	var invalid *SubmissionError
	if errors.As(err, &invalid) {
		d["validation_code"] = invalid.Code
		d["field"] = invalid.Field
		d["validation_error"] = invalid.Message
	}
	return d
}

// RejectedSubmissionOutput is a failed tool reply, not a success acknowledgement.
func RejectedSubmissionOutput(err error, guidance string) string {
	var invalid *SubmissionError
	if !errors.As(err, &invalid) {
		invalid = &SubmissionError{"INVALID_SUBMISSION", "/", "Submission is invalid."}
	}
	raw, _ := json.Marshal(map[string]any{"code": invalid.Code, "field": invalid.Field, "reason": invalid.Message, "next_action": guidance})
	return string(raw)
}

func SubmissionNoReplyOutput(description string) map[string]any {
	out := NoReplyOutput(description)
	props := map[string]any{}
	for name, description := range map[string]string{"code": "Protocol validation failure code.", "field": "Invalid field path or response constraint; submitted values are omitted.", "reason": "Specific submission constraint that failed; the call was not accepted or executed.", "next_action": "How to correct the submission within the remaining task budget."} {
		props[name] = map[string]any{"type": "string", "description": description}
	}
	out["x-error-schema"] = map[string]any{"type": "object", "required": []string{"code", "field", "reason", "next_action"}, "additionalProperties": false, "properties": props}
	return out
}

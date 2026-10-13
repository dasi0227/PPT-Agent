package llm

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
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
		raw, _ := json.Marshal(model.NewAgentError(model.CodeInternal, "tool_submission", err).ModelObservation())
		return string(raw)
	}
	reason := invalid.Message
	if invalid.Field != "" && invalid.Field != "/" && !strings.Contains(reason, invalid.Field) {
		reason = "At " + invalid.Field + ": " + reason
	}
	raw, _ := json.Marshal(map[string]any{"code": invalid.Code, "category": model.ErrorRepairable, "reason": reason, "measure": guidance})
	return string(raw)
}

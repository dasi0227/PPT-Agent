package workflow

import (
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
)

// Model observations deliberately exclude public UI links, command cards,
// persistence proofs and internal artifact locations. Durable ToolResult stays
// unchanged for recovery, audit and completion checks.
func modelToolObservation(result ToolResult) string {
	if !result.OK {
		raw, _ := json.Marshal(contextengine.ModelValue(toolResultError(result).ModelObservation()))
		return string(raw)
	}
	value := result.Data
	if len(value) == 0 {
		value = map[string]any{"summary": result.Summary}
	}
	raw, _ := json.Marshal(contextengine.ModelValue(value))
	return string(raw)
}

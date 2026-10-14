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
	return modelSuccessObservation(result.Data)
}

// data contains the tool's model-visible business fields. Status belongs to the
// shared reply protocol and must not mutate the durable business result.
func modelSuccessObservation(data map[string]any) string {
	value := make(map[string]any, len(data)+1)
	for key, item := range data {
		value[key] = item
	}
	value["status"] = "success"
	raw, _ := json.Marshal(contextengine.ModelValue(value))
	return string(raw)
}

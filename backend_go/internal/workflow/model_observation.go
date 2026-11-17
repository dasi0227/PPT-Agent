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
	status := "success"
	if !result.Applied {
		status = "refuse"
	}
	return modelResultObservation(result.Data, status)
}

// data contains the tool's model-visible business fields. Status belongs to the
// shared reply protocol and must not mutate the durable business result.
func modelSuccessObservation(data map[string]any) string {
	return modelResultObservation(data, "success")
}

func modelResultObservation(data map[string]any, status string) string {
	value := make(map[string]any, len(data)+1)
	for key, item := range data {
		value[key] = item
	}
	value["status"] = status
	raw, _ := json.Marshal(contextengine.ModelValue(value))
	return string(raw)
}

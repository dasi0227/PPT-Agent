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
	value := map[string]any{"ok": result.OK, "summary": result.Summary}
	if result.Data != nil {
		value["data"] = result.Data
	}
	if len(result.ChangedTargets) > 0 {
		targets := []any{}
		for _, target := range result.ChangedTargets {
			targets = append(targets, map[string]any{"resource": target.Target(), "artifact_hash": target.Hash})
		}
		value["changed_targets"] = targets
	}
	if len(result.Issues) > 0 {
		value["issues"] = result.Issues
	}
	raw, _ := json.Marshal(contextengine.ModelValue(value))
	return string(raw)
}

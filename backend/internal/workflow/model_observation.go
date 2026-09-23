package workflow

import (
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Model observations deliberately exclude public UI links, command cards,
// persistence proofs and internal artifact locations. Durable ToolResult stays
// unchanged for recovery, audit and completion checks.
func modelToolObservation(result ToolResult) string {
	value := map[string]any{"ok": result.OK, "summary": result.Summary}
	if result.Data != nil {
		value["data"] = result.Data
	}
	if len(result.ChangedTargets) > 0 {
		targets := []any{}
		for _, target := range result.ChangedTargets {
			targets = append(targets, map[string]any{"resource": target.Target(), "content_hash": target.Hash})
		}
		value["changed_targets"] = targets
	}
	if len(result.Issues) > 0 {
		value["issues"] = result.Issues
	}
	if !result.OK {
		value["code"], value["retryable"] = result.Code, result.Retryable
		value["next_action"] = model.ErrorDefinitionFor(result.Code).ModelMessage
	}
	raw, _ := json.Marshal(contextengine.ModelValue(value))
	return string(raw)
}

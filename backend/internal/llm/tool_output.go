package llm

import (
	"encoding/json"
	"fmt"
)

// NoReplyOutput declares a result-submission tool. An absent reply is not JSON null.
func NoReplyOutput(description string) map[string]any {
	return map[string]any{"description": description, "x-content-kind": "none"}
}

// ModelToolSchemas projects internal contracts onto ordinary function calling.
// OutputSchema describes the successful JSON text, with x-content-kind identifying
// json, image, json+image or none, and x-error-schema describing error JSON text.
// These annotations describe transport; they are never fields of a tool result.
// Keep this projection at the provider boundary so repeated requests cannot append
// the generated documentation twice or mutate the canonical tool definitions.
func ModelToolSchemas(tools []ToolSchema) ([]ToolSchema, error) {
	out := make([]ToolSchema, len(tools))
	for i, tool := range tools {
		out[i] = tool
		if len(tool.OutputSchema) == 0 {
			continue
		}
		raw, err := json.Marshal(tool.OutputSchema)
		if err != nil {
			return nil, fmt.Errorf("tool %s output schema: %w", tool.Name, err)
		}
		out[i].Description += "\n\nOutput contract (JSON Schema for returned text; x-content-kind identifies the successful response content, x-error-schema describes error replies; these annotations are not returned fields):\n" + string(raw)
		out[i].OutputSchema = nil
	}
	return out, nil
}

// EstimateToolTokens accounts for the generated description actually sent to a model.
func EstimateToolTokens(tools []ToolSchema) int {
	projected, err := ModelToolSchemas(tools)
	if err != nil {
		return EstimateValueTokens(tools)
	}
	return EstimateValueTokens(projected)
}

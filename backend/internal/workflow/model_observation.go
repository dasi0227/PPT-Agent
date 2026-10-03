package workflow

import (
	"encoding/json"
	"maps"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
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
	if len(result.ContentPrecheck) == 1 {
		value = maps.Clone(value)
		value["content_precheck"] = contentPrecheckObservation(result.ContentPrecheck[0])
	}
	raw, _ := json.Marshal(contextengine.ModelValue(value))
	return string(raw)
}

// Single-page tool results are already identified by their tool call. Keep the
// full assessment on ToolResult for events and audit, exposing only usable scores.
func contentPrecheckObservation(assessment model.ContentPrecheck) map[string]any {
	value := map[string]any{"status": assessment.Status}
	if assessment.Status == "completed" {
		scores := map[string]float64{}
		for _, rubric := range model.ContentPrecheckRubrics() {
			if score, ok := assessment.Scores[rubric.Dimension]; ok {
				scores[rubric.Dimension] = score.Score
			}
		}
		if len(scores) == len(model.ContentPrecheckRubrics()) {
			value["scores"] = scores
			return value
		}
		value["status"], value["reason"] = "unavailable", "invalid_answer"
		return value
	}
	if assessment.Status == "pending" {
		value["status"], value["reason"] = "unavailable", "assessment_incomplete"
		return value
	}
	value["reason"] = assessment.Reason
	return value
}

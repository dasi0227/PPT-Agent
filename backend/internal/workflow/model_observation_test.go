package workflow

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestModelObservationKeepsBusinessResultsAndHidesInternalMetadata(t *testing.T) {
	for _, success := range []bool{true, false} {
		result := SuccessfulToolResult("saved")
		result.OK = success
		result.Code = CodeContentConflict
		result.Data = map[string]any{"content": map[string]any{"title": "Deck", "ok": true, "content_hash": "private"}, "source_hash": "private"}
		result.ChangedTargets = []ChangedTarget{{Type: "deck", Part: "manifest", Hash: "private"}}
		raw := modelToolObservation(result)
		var value map[string]any
		if json.Unmarshal([]byte(raw), &value) != nil {
			t.Fatal(raw)
		}
		for _, forbidden := range []string{`"ok"`, `"content_hash"`, `"source_hash"`, `"changed_targets"`, `"data"`} {
			if strings.Contains(raw, forbidden) {
				t.Fatalf("metadata leaked: %s", raw)
			}
		}
		if !success && value["reason"] == nil {
			t.Fatal("error lost reason")
		}
	}
}

func TestContentPrecheckOutputContractsExposeTheScoringRubric(t *testing.T) {
	for _, name := range []string{"edit_html", "run_command"} {
		schema := toolOutputSchema(name)
		precheck := schema["properties"].(map[string]any)["content_precheck"].(map[string]any)
		completed := precheck["oneOf"].([]any)[0].(map[string]any)
		scores := completed["properties"].(map[string]any)["scores"].(map[string]any)["properties"].(map[string]any)
		projected, err := llm.ModelToolSchemas([]llm.ToolSchema{{Name: name, OutputSchema: schema}})
		if err != nil {
			t.Fatal(err)
		}
		for _, rubric := range model.ContentPrecheckRubrics() {
			field := scores[rubric.Dimension].(map[string]any)
			if field["type"] != "number" || field["minimum"] != 0 || field["maximum"] != 3 {
				t.Fatal("score contract does not accept the expected fractional 0–3 scores")
			}
			for _, meaning := range append([]string{rubric.Description}, rubric.Criteria[:]...) {
				if !strings.Contains(projected[0].Description, meaning) {
					t.Fatalf("%s output contract hid the rubric meaning: %s", name, meaning)
				}
			}
		}
	}
}

func TestContentPrecheckObservationOmitsUnusableScores(t *testing.T) {
	scores := map[string]model.ContentScore{model.ContentCoverage: {Score: 2, MaxScore: 3, Confidence: 1}}
	for _, status := range []string{"unavailable", "skipped", "stale", "pending", "completed"} {
		assessment := contentPrecheckObservation(model.ContentPrecheck{Status: status, Reason: "material_changed", Scores: scores})
		if assessment["scores"] != nil || assessment["reason"] == nil || assessment["status"] == "pending" || assessment["status"] == "completed" {
			t.Fatalf("incomplete assessment exposed usable scores: %+v", assessment)
		}
	}
}

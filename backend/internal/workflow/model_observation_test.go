package workflow

import (
	"encoding/json"
	"strings"
	"testing"
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

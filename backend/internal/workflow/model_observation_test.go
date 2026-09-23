package workflow

import (
	"encoding/json"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestModelObservationDistinguishesWriteTokenFromArtifactHash(t *testing.T) {
	raw := []byte(`{"title":"Deck", "updated_at":42}`)
	writeHash := spec.ResourceBytesHash(raw)
	artifactHash := hashBytes(raw)
	result := SuccessfulToolResult("PPT mutation applied")
	result.Data = map[string]any{"hashes": map[string]string{"manifest": writeHash}}
	result.ChangedTargets = []ChangedTarget{{Type: "deck", Part: "manifest", Hash: artifactHash}}
	var observation struct {
		Data struct {
			Hashes map[string]string `json:"hashes"`
		} `json:"data"`
		Targets []map[string]any `json:"changed_targets"`
	}
	if err := json.Unmarshal([]byte(modelToolObservation(result)), &observation); err != nil {
		t.Fatal(err)
	}
	if observation.Data.Hashes["manifest"] != writeHash || len(observation.Targets) != 1 {
		t.Fatalf("mutation write token lost: %+v", observation)
	}
	if _, ambiguous := observation.Targets[0]["content_hash"]; ambiguous || observation.Targets[0]["artifact_hash"] != artifactHash {
		t.Fatalf("artifact bytes advertised as a content write token: %+v", observation.Targets)
	}
}

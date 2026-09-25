package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestSpecCollectionScopeChangesRecoveryAndFrozenReferences(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	other := "sli_other"
	initial := map[string]spec.SlideSpec{generationSlide: pack.GenerationInputs[generationSlide].Spec, other: {KeyMessage: "Other", Elements: []spec.Element{}}}
	raw, _ := json.Marshal(initial)
	writeGenerationFile(t, dir, model.SpecCollectionPath, raw)
	session, err := NewRunSession(dir, "collection")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	next := []byte(`{"key_message":"Changed","elements":[]}`)
	if _, err := session.Write(specSlideRef(generationSlide), "mutate_ppt", next); err != nil {
		t.Fatal(err)
	}
	changes := session.ChangeSet().All()
	if len(changes) != 1 || changes[0].Artifact.ID != generationSlide || changes[0].Artifact.Kind != ArtifactSlideSpec {
		t.Fatalf("collection leaked physical change: %+v", changes)
	}
	if session.HasChange(specSlideRef(other)) {
		t.Fatal("untouched page marked changed")
	}
	value, err := session.Read(specSlideRef(other))
	if err != nil || spec.ResourceBytesHash(value) != spec.ResourceHash(initial[other]) {
		t.Fatalf("untouched page lost: %s %v", value, err)
	}
	frozen := pack.GenerationInputs[generationSlide].Clone()
	frozen.Spec.KeyMessage = "Earlier observed other page"
	pack.GenerationInputs[other] = frozen
	updated := session.generationContext(pack)
	if updated.GenerationInputs[other].Spec.KeyMessage != frozen.Spec.KeyMessage {
		t.Fatal("collection imported an unobserved reference into frozen context")
	}
	if _, err := session.StageGenerationInputs(updated); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("metadata failure")
	if _, err := session.CommitOperation(context.Background(), "fail", "", func(context.Context, CommitContext) error { return failure }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	disk, _ := os.ReadFile(filepath.Join(dir, model.SpecCollectionPath))
	if string(disk) != string(raw) {
		t.Fatal("rollback did not restore entire collection")
	}
	if _, err := session.Write(specSlideRef(other), "run_command", next); err != nil {
		t.Fatal(err)
	}
	if _, err := session.StageGenerationInputs(pack); !errors.Is(err, ErrTargetOutOfScope) {
		t.Fatalf("shared file bypassed page scope: %v", err)
	}
	session.RollbackOperation()
	if _, err := session.Write(specSlideRef(generationSlide), "mutate_ppt", next); err != nil {
		t.Fatal(err)
	}
	changeset := session.ChangeSet()
	if _, err := session.CommitOperation(context.Background(), "ok", "", nil); err != nil {
		t.Fatal(err)
	}
	recovered, err := ReconcileDirectWrites(context.Background(), dir, RuntimeCheckpoint{RunID: "collection", Changes: changeset})
	if err != nil || len(recovered.Results) != 1 || recovered.Results[0].Status != ReconcileClean {
		t.Fatalf("per-page recovery: %+v %v", recovered, err)
	}
}

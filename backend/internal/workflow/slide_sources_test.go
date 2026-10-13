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

func TestPageSpecScopeChangesAndRecovery(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	other := "sli_other"
	initial := map[string]spec.SlideSpec{generationSlide: spec.SlideSpec{Core: "Message", Elements: []spec.Element{}}, other: {Core: "Other", Elements: []spec.Element{}}}
	for id, slide := range initial {
		raw, _ := json.Marshal(slide)
		writeGenerationFile(t, dir, model.SlideSpecPath(id), raw)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, model.SlideSpecPath(generationSlide)))
	session, err := NewRunSession(dir, "page-files")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	next := []byte(`{"core":"Changed","elements":[]}`)
	if _, err := session.Write(specSlideRef(generationSlide), "edit_spec", next); err != nil {
		t.Fatal(err)
	}
	changes := session.ChangeSet().All()
	if len(changes) != 1 || changes[0].Artifact.ID != generationSlide || changes[0].Artifact.Kind != ArtifactSlideSpec {
		t.Fatalf("incorrect page change: %+v", changes)
	}
	if session.HasChange(specSlideRef(other)) {
		t.Fatal("untouched page marked changed")
	}
	value, err := session.Read(specSlideRef(other))
	if err != nil || spec.ResourceBytesHash(value) != spec.ResourceHash(initial[other]) {
		t.Fatalf("untouched page lost: %s %v", value, err)
	}
	if err := session.ValidateStagedArtifacts(pack); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("metadata failure")
	if _, err := session.CommitOperation(context.Background(), "fail", "", func(context.Context, CommitContext) error { return failure }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	disk, _ := os.ReadFile(filepath.Join(dir, model.SlideSpecPath(generationSlide)))
	if string(disk) != string(raw) {
		t.Fatal("rollback did not restore page source")
	}
	if _, err := session.Write(specSlideRef(other), "run_command", next); err != nil {
		t.Fatal(err)
	}
	if err := session.ValidateStagedArtifacts(pack); !errors.Is(err, ErrTargetOutOfScope) {
		t.Fatalf("page source bypassed page scope: %v", err)
	}
	session.RollbackOperation()
	if _, err := session.Write(specSlideRef(generationSlide), "edit_spec", next); err != nil {
		t.Fatal(err)
	}
	changeset := session.ChangeSet()
	if _, err := session.CommitOperation(context.Background(), "ok", "", nil); err != nil {
		t.Fatal(err)
	}
	recovered, err := ReconcileDirectWrites(context.Background(), dir, RuntimeCheckpoint{RunID: "page-files", Changes: changeset})
	if err != nil || len(recovered.Results) != 1 || recovered.Results[0].Status != ReconcileClean {
		t.Fatalf("per-page recovery: %+v %v", recovered, err)
	}
}

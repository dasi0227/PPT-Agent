package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestReconcileDirectWritesClassifiesArtifactState(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, value string) string {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
		return hashBytes([]byte(value))
	}
	write(model.SpecCollectionPath, `{"sli_1":{"core":"after","elements":[]}}`)
	after := hashBytes([]byte(`{"core":"after","elements":[]}`))
	external := write(model.SlideHTMLPath("s1"), "external")
	checkpoint := RuntimeCheckpoint{RunID: "r", Changes: ChangeSet{Updated: []ArtifactChange{
		{Artifact: ArtifactRef{Kind: ArtifactSlideSpec, ID: "sli_1"}, AfterHash: after},
		{Artifact: ArtifactRef{Kind: ArtifactSlideHTML, ID: "s1"}, BeforeHash: "before", AfterHash: "expected"},
		{Artifact: ArtifactRef{Kind: ArtifactDesign, ID: "deck"}, AfterHash: "missing"},
	}}}
	if external == "expected" {
		t.Fatal("test setup invalid")
	}
	snapshot, err := ReconcileDirectWrites(context.Background(), dir, checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[ArtifactKind]string{}
	for _, result := range snapshot.Results {
		statuses[result.Artifact.Kind] = result.Status
	}
	if statuses[ArtifactSlideSpec] != ReconcileClean ||
		statuses[ArtifactSlideHTML] != ReconcileExternalModified ||
		statuses[ArtifactDesign] != ReconcileMissingArtifact {
		t.Fatalf("statuses=%+v", statuses)
	}
}

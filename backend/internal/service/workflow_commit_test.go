package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

func TestWorkflowCommitKeepsMetadataWithoutVersionFiles(t *testing.T) {
	f := newBriefingFixture(t)
	ctx := context.Background()
	c := workflowCommitter{store: f.store, project: f.project, runID: "run"}
	changes := workflow.EmptyChangeSet()
	for _, ref := range []workflow.ArtifactRef{
		{Kind: workflow.ArtifactManifest, ID: f.project.ID},
		{Kind: workflow.ArtifactOutline, ID: f.project.ID},
		{Kind: workflow.ArtifactDesign, ID: f.project.ID},
		{Kind: workflow.ArtifactSlideSpec, ID: "sli_aaaaaa"},
		{Kind: workflow.ArtifactSlideHTML, ID: "sli_aaaaaa"},
	} {
		changes.Updated = append(changes.Updated, workflow.ArtifactChange{Artifact: ref})
	}
	first := workflow.CommitContext{OperationID: "call1", RequestHash: "hash1", Changes: changes}
	if err := c.Commit(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.OperationID, second.RequestHash = "call2", "hash2"
	if err := c.Commit(ctx, second); err != nil {
		t.Fatal(err)
	}
	// A late replay must neither increment nor rewind current metadata.
	if err := c.Commit(ctx, first); err != nil {
		t.Fatal(err)
	}
	slide, err := f.store.GetSlide(ctx, "sli_aaaaaa")
	if err != nil || slide.CurrentVersion != 3 {
		t.Fatalf("slide=%+v err=%v", slide, err)
	}
	if _, err := os.Stat(filepath.Join(f.project.WorkDir, "versions")); !os.IsNotExist(err) {
		t.Fatalf("version files created: %v", err)
	}
	for _, name := range []string{"manifest.json", "outline.json", "design.json", model.SlideSpecPath(slide.ID)} {
		var content struct {
			Revision int `json:"revision"`
		}
		if err := readJSON(filepath.Join(f.project.WorkDir, name), &content); err != nil || content.Revision != 1 {
			t.Fatalf("content revision %s: %+v %v", name, content, err)
		}
	}
	// Removing an unrendered page still proves it belonged to this project.
	if err := f.store.InsertSlide(ctx, model.Slide{ID: "pending", ProjectID: f.project.ID}); err != nil {
		t.Fatal(err)
	}
	if err := f.store.ReplaceSlides(ctx, f.project.ID, []model.Slide{slide}); err != nil {
		t.Fatal(err)
	}
	selections := []model.DOMSelection{{SlideID: "pending", Status: model.DOMSelectionPageDeleted}, {SlideID: "foreign", Status: model.DOMSelectionPageDeleted}}
	svc := &RunService{store: f.store}
	deleted, err := svc.deletedSelectionSlideIDs(ctx, f.project.ID, spec.ProjectContentSnapshot{}, selections)
	if err != nil || !deleted["pending"] || deleted["foreign"] {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}
	foreign, err := svc.deletedSelectionSlideIDs(ctx, "other-project", spec.ProjectContentSnapshot{}, selections[:1])
	if err != nil || foreign["pending"] {
		t.Fatalf("cross-project selection accepted: %v %v", foreign, err)
	}
}

package service

import (
	"context"
	"encoding/json"
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
	if err := f.store.CreateRun(ctx, model.Run{ID: "run", ThreadID: f.thread.ID, ProjectID: f.project.ID,
		Command: model.RunCommand{Scope: model.NewRunScope(model.ScopeAllPages, "sli_aaaaaa"), Mode: model.ModeExecute, Instruction: "验证创作提交"},
		Status:  model.RunRunning, CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
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
	var outline spec.Outline
	if err := readJSON(filepath.Join(f.project.WorkDir, ".outline.json"), &outline); err != nil {
		t.Fatal(err)
	}
	outline.Sections[0].Slides = append(outline.Sections[0].Slides, spec.SlideNode{ID: "sli_bbbbbb", Title: "Later page"})
	raw, _ := json.Marshal(outline)
	if err := os.WriteFile(filepath.Join(f.project.WorkDir, ".outline.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := c.Commit(ctx, second); err != nil {
		t.Fatal(err)
	}
	// A late replay must never rewind current metadata.
	if err := c.Commit(ctx, first); err != nil {
		t.Fatal(err)
	}
	slide, err := f.store.GetSlide(ctx, "sli_aaaaaa")
	if err != nil || slide.ProjectID != f.project.ID {
		t.Fatalf("slide=%+v err=%v", slide, err)
	}
	if slides, err := f.store.ListSlides(ctx, f.project.ID); err != nil || len(slides) != 2 {
		t.Fatalf("late replay rewound page membership: %+v %v", slides, err)
	}
	if err := c.Commit(ctx, workflow.CommitContext{OperationID: "reference_only", RequestHash: "hash3"}); err != nil {
		t.Fatal(err)
	}
	unchanged, err := f.store.GetSlide(ctx, slide.ID)
	if err != nil || unchanged != slide {
		t.Fatalf("non-HTML commit changed page identity: %+v %v", unchanged, err)
	}
	conflicting := first
	conflicting.RequestHash = "conflict"
	if err := c.Commit(ctx, conflicting); err == nil {
		t.Fatal("conflicting commit receipt accepted")
	}
	if _, err := os.Stat(filepath.Join(f.project.WorkDir, "versions")); !os.IsNotExist(err) {
		t.Fatalf("version files created: %v", err)
	}
	for _, name := range []string{".manifest.json", ".outline.json", ".design.json", model.SlideSpecPath("sli_aaaaaa")} {
		var content map[string]any
		if err := readJSON(filepath.Join(f.project.WorkDir, name), &content); err != nil {
			t.Fatal(err)
		}
		if _, exists := content["revision"]; exists {
			t.Fatalf("unexpected revision in %s", name)
		}
	}
}

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
	var baseline spec.GenerationInputs
	for path, value := range map[string]any{".manifest.json": &baseline.Manifest, ".design.json": &baseline.Design} {
		if err := readJSON(filepath.Join(f.project.WorkDir, path), value); err != nil {
			t.Fatal(err)
		}
	}
	rawSpec, err := spec.ReadSlideSpec(func(path string) ([]byte, error) { return os.ReadFile(filepath.Join(f.project.WorkDir, path)) }, "sli_aaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rawSpec, &baseline.Spec); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := json.Marshal(baseline)
	first := workflow.CommitContext{GenerationInputs: map[string]json.RawMessage{"sli_aaaaaa": snapshot}, OperationID: "call1", RequestHash: "hash1", Changes: changes}
	if err := c.Commit(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.OperationID, second.RequestHash = "call2", "hash2"
	baseline.Design.Direction = "Later generation"
	nextSnapshot, _ := json.Marshal(baseline)
	second.GenerationInputs = map[string]json.RawMessage{"sli_aaaaaa": nextSnapshot}
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
	if slide.GenerationInputsJSON == nil || *slide.GenerationInputsJSON != string(nextSnapshot) {
		t.Fatal("late replay overwrote generation inputs")
	}
	if err := c.Commit(ctx, workflow.CommitContext{OperationID: "reference_only", RequestHash: "hash3"}); err != nil {
		t.Fatal(err)
	}
	unchanged, err := f.store.GetSlide(ctx, slide.ID)
	if err != nil || unchanged.GenerationInputsJSON == nil || *unchanged.GenerationInputsJSON != string(nextSnapshot) {
		t.Fatal("non-HTML commit changed snapshot")
	}
	invalid := workflow.CommitContext{OperationID: "invalid_snapshot", RequestHash: "hash4", Changes: changes, GenerationInputs: map[string]json.RawMessage{slide.ID: json.RawMessage(`{}`)}}
	if err := c.Commit(ctx, invalid); err == nil {
		t.Fatal("invalid snapshot committed")
	}
	if _, err := f.store.GetIdempotency(ctx, "artifact_commit", "run", "invalid_snapshot"); err == nil {
		t.Fatal("failed operation has receipt")
	}
	if _, err := os.Stat(filepath.Join(f.project.WorkDir, "versions")); !os.IsNotExist(err) {
		t.Fatalf("version files created: %v", err)
	}
	for _, name := range []string{".manifest.json", ".outline.json", ".design.json", model.SpecCollectionPath} {
		var content map[string]any
		if err := readJSON(filepath.Join(f.project.WorkDir, name), &content); err != nil {
			t.Fatal(err)
		}
		if _, exists := content["revision"]; exists {
			t.Fatalf("unexpected revision in %s", name)
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

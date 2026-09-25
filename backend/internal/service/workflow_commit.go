package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type workflowCommitter struct {
	store   store.Store
	project model.Project
	runID   string
}

func (c workflowCommitter) Commit(ctx context.Context, commitContext workflow.CommitContext) error {
	// Check before inspecting current files: late replays must not depend on or
	// replace the current page membership and generation snapshots.
	if commitContext.OperationID != "" {
		receipt, err := c.store.GetIdempotency(ctx, "artifact_commit", c.runID, commitContext.OperationID)
		if err == nil {
			if receipt.RequestHash != commitContext.RequestHash || receipt.Status != "completed" {
				return fmt.Errorf("artifact commit receipt conflicts with operation %s", commitContext.OperationID)
			}
			return nil
		}
		if !errors.Is(err, run.ErrRunNotFound) {
			return err
		}
	}
	var outline spec.Outline
	if err := readJSON(filepath.Join(c.project.WorkDir, "outline.json"), &outline); err != nil {
		return err
	}
	existing, err := c.store.ListSlides(ctx, c.project.ID)
	if err != nil {
		return err
	}
	byID := map[string]model.Slide{}
	for _, slide := range existing {
		byID[slide.ID] = slide
	}
	changedHTML := map[string]bool{}
	for _, change := range append(commitContext.Changes.Created, commitContext.Changes.Updated...) {
		if change.Artifact.Kind == workflow.ArtifactSlideHTML {
			changedHTML[change.Artifact.ID] = true
		}
	}
	nextSlides := []model.Slide{}
	inDeck := map[string]bool{}
	for _, loc := range spec.FlattenOutline(outline) {
		id := loc.Slide.SlideID
		inDeck[id] = true
		meta := byID[id]
		meta.ID, meta.ProjectID = id, c.project.ID
		if raw, exists := commitContext.GenerationInputs[id]; exists {
			if !changedHTML[id] {
				return fmt.Errorf("generation snapshot requires an HTML change for %s", id)
			}
			if string(raw) == "null" {
				meta.GenerationInputsJSON = nil
			} else {
				if spec.ParseGenerationInputs(raw) == nil {
					return fmt.Errorf("invalid generation snapshot for %s", id)
				}
				value := string(raw)
				meta.GenerationInputsJSON = &value
			}
		}
		nextSlides = append(nextSlides, meta)
	}
	for id := range commitContext.GenerationInputs {
		if !inDeck[id] {
			return fmt.Errorf("generation snapshot targets missing page %s", id)
		}
	}
	deleted := []string{}
	for id := range byID {
		if !inDeck[id] {
			deleted = append(deleted, id)
		}
	}
	return c.store.CommitWorkflow(ctx, model.ArtifactCommit{
		ProjectID: c.project.ID, RunID: c.runID, OperationID: commitContext.OperationID,
		RequestHash: commitContext.RequestHash, ToolResultJSON: commitContext.ToolResultJSON,
		Slides: nextSlides, DeletedSlideIDs: deleted,
	})
}

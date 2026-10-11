package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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
	// replace the current page membership.
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
	outline := spec.Outline{Sections: []spec.Section{}}
	if err := readJSON(filepath.Join(c.project.WorkDir, ".outline.json"), &outline); err != nil && !errors.Is(err, fs.ErrNotExist) {
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
	nextSlides := []model.Slide{}
	inDeck := map[string]bool{}
	for _, loc := range spec.FlattenOutline(outline) {
		id := loc.Slide.ID
		inDeck[id] = true
		meta := byID[id]
		meta.ID, meta.ProjectID = id, c.project.ID
		nextSlides = append(nextSlides, meta)
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

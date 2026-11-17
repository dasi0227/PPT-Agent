package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
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
	if commitContext.OperationID == "" || commitContext.RequestHash == "" {
		return errors.New("artifact commit requires operation id and request hash")
	}
	err := c.commit(ctx, commitContext)
	if err == nil {
		return nil
	}
	// A canceled request or a lost COMMIT acknowledgement does not establish
	// failure. Query the original identity after the database call has returned.
	lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	receiptHash, found, lookupErr := artifactReceiptLookup(c.store)(lookupCtx, c.runID, commitContext.OperationID)
	if lookupErr != nil {
		return errors.Join(pptmutation.ErrRecoveryRequired, err, fmt.Errorf("check artifact commit receipt: %w", lookupErr))
	}
	if found {
		if receiptHash != commitContext.RequestHash {
			return errors.Join(pptmutation.ErrRecoveryRequired, err, fmt.Errorf("artifact commit receipt conflicts with operation %s", commitContext.OperationID))
		}
		return nil
	}
	return err
}

func artifactReceiptLookup(st workflow.IdempotencyStore) workflow.MutationReceiptLookup {
	return func(ctx context.Context, runID, operationID string) (string, bool, error) {
		receipt, err := st.GetIdempotency(ctx, "artifact_commit", runID, operationID)
		if errors.Is(err, run.ErrRunNotFound) {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}
		if receipt.Status != "completed" {
			return "", false, fmt.Errorf("artifact commit receipt for operation %s has status %q", operationID, receipt.Status)
		}
		return receipt.RequestHash, true, nil
	}
}

func (c workflowCommitter) commit(ctx context.Context, commitContext workflow.CommitContext) error {
	// Check before inspecting current files: late replays must not depend on or
	// replace the current page membership.
	if commitContext.OperationID != "" {
		receiptHash, found, err := artifactReceiptLookup(c.store)(ctx, c.runID, commitContext.OperationID)
		if err != nil {
			return err
		}
		if found {
			if receiptHash != commitContext.RequestHash {
				return fmt.Errorf("artifact commit receipt conflicts with operation %s", commitContext.OperationID)
			}
			return nil
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

package service

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type workflowCommitter struct {
	store   store.Store
	project model.Project
	runID   string
}

func (c workflowCommitter) Commit(ctx context.Context, changes workflow.ChangeSet) error {
	var deck blueprint.Deck
	if err := readJSON(filepath.Join(c.project.WorkDir, "deck.json"), &deck); err != nil {
		return err
	}
	var design blueprint.DesignSpec
	if err := readJSON(filepath.Join(c.project.WorkDir, "design", "design-spec.json"), &design); err != nil {
		return err
	}
	existing, err := c.store.ListSlides(ctx, c.project.ID)
	if err != nil {
		return err
	}
	byID := make(map[string]model.Slide, len(existing))
	for _, slide := range existing {
		byID[slide.ID] = slide
	}
	changed := map[string]workflow.ArtifactChange{}
	for _, change := range append(append(changes.Created, changes.Updated...), changes.Deleted...) {
		changed[change.Artifact.Key()] = change
	}
	versions := []model.Version{}
	snapshotPaths := []string{}
	cleanup := func() {
		for _, path := range snapshotPaths {
			_ = os.Remove(filepath.Join(c.project.WorkDir, filepath.FromSlash(path)))
		}
	}
	addVersion := func(targetType, targetID, path string, content []byte) (int, error) {
		number, err := c.store.NextVersionNo(ctx, targetType, targetID)
		if err != nil {
			return 0, err
		}
		if err := atomicWrite(filepath.Join(c.project.WorkDir, filepath.FromSlash(path)), content); err != nil {
			return 0, err
		}
		snapshotPaths = append(snapshotPaths, path)
		versions = append(versions, model.Version{
			ID: uuid.NewString(), TargetType: targetType, TargetID: targetID,
			VersionNo: number, SnapshotPath: path, RunID: c.runID, CreatedAt: time.Now().Unix(),
		})
		return number, nil
	}
	if _, ok := changed[(workflow.ArtifactRef{Kind: workflow.ArtifactDeck, ID: c.project.ID}).Key()]; ok {
		if _, err := addVersion("blueprint_deck", model.BlueprintDeckVersionTarget(c.project.ID),
			model.BlueprintDeckVersionSnapshot(deck.Revision), mustJSON(deck)); err != nil {
			cleanup()
			return err
		}
	}
	if _, ok := changed[(workflow.ArtifactRef{Kind: workflow.ArtifactDesign, ID: c.project.ID}).Key()]; ok {
		if _, err := addVersion("design", model.DesignVersionTarget(c.project.ID),
			model.DesignVersionSnapshot(design.Revision), mustJSON(design)); err != nil {
			cleanup()
			return err
		}
	}
	nextSlides := make([]model.Slide, 0, len(deck.SlideOrder))
	inDeck := map[string]bool{}
	for position, id := range deck.SlideOrder {
		inDeck[id] = true
		var semantic blueprint.Slide
		if err := readJSON(filepath.Join(c.project.WorkDir, filepath.FromSlash(model.SlideJSONPath(id))), &semantic); err != nil {
			cleanup()
			return err
		}
		meta := byID[id]
		meta.ID, meta.ProjectID = id, c.project.ID
		meta.Position = position
		meta.Layout, meta.Title = semantic.VisualIntent.Archetype, semantic.Title
		meta.JSONPath, meta.HTMLPath = model.SlideJSONPath(id), model.SlideHTMLPath(id)
		meta.BlueprintRevision = semantic.Revision
		if _, ok := changed[(workflow.ArtifactRef{Kind: workflow.ArtifactSlide, ID: id}).Key()]; ok {
			target := model.BlueprintSlideVersionTarget(c.project.ID, id)
			path := model.BlueprintSlideVersionSnapshot(id, semantic.Revision)
			if _, err := addVersion("blueprint_slide", target, path, mustJSON(semantic)); err != nil {
				cleanup()
				return err
			}
		}
		if _, ok := changed[(workflow.ArtifactRef{Kind: workflow.ArtifactPresentation, ID: id}).Key()]; ok {
			raw, readErr := os.ReadFile(filepath.Join(c.project.WorkDir, filepath.FromSlash(model.SlideHTMLPath(id))))
			if readErr != nil {
				cleanup()
				return readErr
			}
			meta.PresentationRevision++
			meta.SourceDeckRevision, meta.SourceBlueprintRevision, meta.SourceDesignRevision =
				deck.Revision, semantic.Revision, design.Revision
			target := model.PresentationSlideVersionTarget(c.project.ID, id)
			number, versionErr := c.store.NextVersionNo(ctx, "presentation_slide", target)
			if versionErr != nil {
				cleanup()
				return versionErr
			}
			path := model.SlideVersionSnapshot(id, number)
			if err := atomicWrite(filepath.Join(c.project.WorkDir, filepath.FromSlash(path)), raw); err != nil {
				cleanup()
				return err
			}
			snapshotPaths = append(snapshotPaths, path)
			versions = append(versions, model.Version{
				ID: uuid.NewString(), TargetType: "presentation_slide", TargetID: target,
				VersionNo: number, SnapshotPath: path, RunID: c.runID, CreatedAt: time.Now().Unix(),
			})
			meta.CurrentVersion = number
		}
		nextSlides = append(nextSlides, meta)
	}
	deleted := []string{}
	for id := range byID {
		if !inDeck[id] {
			deleted = append(deleted, id)
		}
	}
	commit := model.ArtifactCommit{
		ProjectID: c.project.ID, DeckRevision: deck.Revision, DesignRevision: design.Revision,
		Slides: nextSlides, DeletedSlideIDs: deleted, Versions: versions,
	}
	if err := c.store.CommitWorkflow(ctx, commit); err != nil {
		cleanup()
		return err
	}
	return nil
}

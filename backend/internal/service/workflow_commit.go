package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
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
	changes := commitContext.Changes
	var deck spec.Outline
	if err := readJSON(filepath.Join(c.project.WorkDir, "outline.json"), &deck); err != nil {
		return err
	}
	var design spec.Design
	if err := readJSON(filepath.Join(c.project.WorkDir, "design.json"), &design); err != nil {
		return err
	}
	designRaw, err := os.ReadFile(filepath.Join(c.project.WorkDir, "design.json"))
	if err != nil {
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
	proofs := make(map[string]workflow.MaterializationProof, len(commitContext.MaterializationProofs))
	for _, proof := range commitContext.MaterializationProofs {
		if proof.SlideID == "" {
			return fmt.Errorf("materialization proof has no slide_id")
		}
		if _, duplicate := proofs[proof.SlideID]; duplicate {
			return fmt.Errorf("duplicate materialization proof for %s", proof.SlideID)
		}
		proofs[proof.SlideID] = proof
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
	if _, ok := changed[(workflow.ArtifactRef{Kind: workflow.ArtifactOutline, ID: c.project.ID}).Key()]; ok {
		if _, err := addVersion("outline", model.OutlineVersionTarget(c.project.ID),
			model.OutlineVersionSnapshot(deck.Revision), mustJSON(deck)); err != nil {
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
		var semantic spec.SlideSpec
		specPath := filepath.Join(c.project.WorkDir, filepath.FromSlash(model.SlideSpecPath(id)))
		if err := readJSON(specPath, &semantic); err != nil {
			cleanup()
			return err
		}
		specRaw, err := os.ReadFile(specPath)
		if err != nil {
			cleanup()
			return err
		}
		meta := byID[id]
		meta.ID, meta.ProjectID = id, c.project.ID
		meta.Position = position
		meta.Layout, meta.Title = semantic.VisualIntent.Archetype, semantic.Title
		meta.SpecPath, meta.HTMLPath = model.SlideSpecPath(id), model.SlideHTMLPath(id)
		meta.SpecRevision = semantic.Revision
		if _, ok := changed[(workflow.ArtifactRef{Kind: workflow.ArtifactSlideSpec, ID: id}).Key()]; ok {
			target := model.SlideSpecVersionTarget(c.project.ID, id)
			path := model.SlideSpecVersionSnapshot(id, semantic.Revision)
			if _, err := addVersion("slide_spec", target, path, mustJSON(semantic)); err != nil {
				cleanup()
				return err
			}
		}
		_, htmlChanged := changed[(workflow.ArtifactRef{Kind: workflow.ArtifactSlideHTML, ID: id}).Key()]
		proof, hasProof := proofs[id]
		var htmlRaw []byte
		if htmlChanged || hasProof {
			raw, readErr := os.ReadFile(filepath.Join(c.project.WorkDir, filepath.FromSlash(model.SlideHTMLPath(id))))
			if readErr != nil {
				cleanup()
				return readErr
			}
			htmlRaw = raw
		}
		expectedHTMLRevision := meta.HTMLRevision
		if htmlChanged {
			expectedHTMLRevision++
		}
		if htmlChanged && !hasProof {
			cleanup()
			return fmt.Errorf("latest materialization proof is required for changed HTML %s", id)
		}
		if hasProof {
			currentHash := workflow.MaterializationSourceHash(id, designRaw, specRaw, htmlRaw)
			if proof.SourceHash != currentHash ||
				proof.HTMLRevision != expectedHTMLRevision ||
				proof.SourceOutlineRevision != deck.Revision ||
				proof.SourceSpecRevision != semantic.Revision ||
				proof.SourceDesignRevision != design.Revision {
				cleanup()
				return fmt.Errorf("stale materialization proof for %s", id)
			}
		}
		if htmlChanged {
			meta.HTMLRevision++
			target := model.SlideHTMLVersionTarget(c.project.ID, id)
			number, versionErr := c.store.NextVersionNo(ctx, "slide_html", target)
			if versionErr != nil {
				cleanup()
				return versionErr
			}
			path := model.SlideHTMLVersionSnapshot(id, number)
			if err := atomicWrite(filepath.Join(c.project.WorkDir, filepath.FromSlash(path)), htmlRaw); err != nil {
				cleanup()
				return err
			}
			snapshotPaths = append(snapshotPaths, path)
			versions = append(versions, model.Version{
				ID: uuid.NewString(), TargetType: "slide_html", TargetID: target,
				VersionNo: number, SnapshotPath: path, RunID: c.runID, CreatedAt: time.Now().Unix(),
			})
			meta.CurrentVersion = number
		}
		if hasProof {
			meta.SourceOutlineRevision = proof.SourceOutlineRevision
			meta.SourceSpecRevision = proof.SourceSpecRevision
			meta.SourceDesignRevision = proof.SourceDesignRevision
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
		ProjectID: c.project.ID, OutlineRevision: deck.Revision, DesignRevision: design.Revision,
		Slides: nextSlides, DeletedSlideIDs: deleted, Versions: versions,
	}
	if err := c.store.CommitWorkflow(ctx, commit); err != nil {
		cleanup()
		return err
	}
	return nil
}

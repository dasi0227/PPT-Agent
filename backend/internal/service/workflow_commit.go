package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
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
	var manifest spec.Manifest
	if err := readJSON(filepath.Join(c.project.WorkDir, "manifest.json"), &manifest); err != nil {
		return err
	}
	manifestRaw, err := os.ReadFile(filepath.Join(c.project.WorkDir, "manifest.json"))
	if err != nil {
		return err
	}
	var outline spec.Outline
	if err := readJSON(filepath.Join(c.project.WorkDir, "outline.json"), &outline); err != nil {
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
	flat := spec.FlattenOutline(outline)
	nextSlides := make([]model.Slide, 0, len(flat))
	inDeck := map[string]bool{}
	for _, location := range flat {
		id := location.Slide.SlideID
		inDeck[id] = true
		meta := byID[id]
		meta.ID, meta.ProjectID = id, c.project.ID
		var semantic spec.SlideSpec
		specPath := filepath.Join(c.project.WorkDir, filepath.FromSlash(model.SlideSpecPath(id)))
		if err := readJSON(specPath, &semantic); os.IsNotExist(err) {
			if _, hasProof := proofs[id]; hasProof {
				return fmt.Errorf("materialization proof requires slide spec %s", id)
			}
			nextSlides = append(nextSlides, meta)
			continue
		} else if err != nil {
			return err
		}
		specRaw, err := os.ReadFile(specPath)
		if err != nil {
			return err
		}
		_, htmlChanged := changed[(workflow.ArtifactRef{Kind: workflow.ArtifactSlideHTML, ID: id}).Key()]
		proof, hasProof := proofs[id]
		var htmlRaw []byte
		if htmlChanged || hasProof {
			raw, readErr := os.ReadFile(filepath.Join(c.project.WorkDir, filepath.FromSlash(model.SlideHTMLPath(id))))
			if readErr != nil {
				return readErr
			}
			htmlRaw = raw
		}
		materializationPath := filepath.Join(
			c.project.WorkDir,
			filepath.FromSlash(model.SlideMaterializationPath(id)),
		)
		if hasProof {
			appearance, appearanceErr := runtimeassets.ProjectAppearance(c.project.WorkDir, design.Theme)
			if appearanceErr != nil {
				return appearanceErr
			}
			artifactHash := spec.ContentHash(htmlRaw)[len("sha256:"):]
			nodeHash := spec.SemanticSlideNodeHash(outline, id)
			sourceHash := spec.SourceHash(manifestRaw, nodeHash, specRaw, designRaw)
			if proof.ArtifactHash != artifactHash ||
				proof.SourceHash != sourceHash ||
				proof.ManifestHash != spec.ResourceHash(manifest) || proof.OutlineNodeHash != nodeHash ||
				proof.SpecHash != spec.ResourceHash(semantic) || proof.DesignContentHash != spec.DesignContentHash(design) || proof.FrameContextHash != spec.FrameContextHash(manifest, outline, design, id, appearance) {
				return fmt.Errorf("stale materialization proof for %s", id)
			}
		}
		if hasProof {
			record, err := spec.ReadMaterialization(materializationPath)
			if err != nil {
				return err
			}
			if record.Artifact.Hash != "sha256:"+proof.ArtifactHash ||
				record.Source.ManifestHash != proof.ManifestHash || record.Source.OutlineNodeHash != proof.OutlineNodeHash ||
				record.Source.SpecHash != proof.SpecHash || record.Source.DesignContentHash != proof.DesignContentHash ||
				record.Source.Hash != proof.SourceHash || record.Frame.ContextHash != proof.FrameContextHash {
				return fmt.Errorf("materialization record does not match proof for %s", id)
			}
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
		ProjectID: c.project.ID, RunID: c.runID, OperationID: commitContext.OperationID,
		RequestHash: commitContext.RequestHash, ToolResultJSON: commitContext.ToolResultJSON,
		Slides: nextSlides, DeletedSlideIDs: deleted,
	}
	if err := c.store.CommitWorkflow(ctx, commit); err != nil {
		return err
	}
	return nil
}

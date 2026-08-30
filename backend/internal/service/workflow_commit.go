package service

import (
	"context"
	"encoding/json"
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
	versions := []model.Version{}
	snapshotPaths := []string{}
	type materializationBackup struct {
		path    string
		raw     []byte
		existed bool
	}
	materializationBackups := []materializationBackup{}
	restoreMaterializations := func() {
		for index := len(materializationBackups) - 1; index >= 0; index-- {
			backup := materializationBackups[index]
			if backup.existed {
				_ = atomicWrite(backup.path, backup.raw)
			} else {
				_ = os.Remove(backup.path)
			}
		}
	}
	cleanup := func() {
		for _, path := range snapshotPaths {
			_ = os.Remove(filepath.Join(c.project.WorkDir, filepath.FromSlash(path)))
		}
		restoreMaterializations()
	}
	findRunVersion := func(targetType, targetID string) (model.Version, bool, error) {
		versionID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(c.runID+"|"+targetType+"|"+targetID)).String()
		existing, err := c.store.ListVersions(ctx, targetType, targetID)
		if err != nil {
			return model.Version{}, false, err
		}
		for _, version := range existing {
			if version.ID == versionID {
				return version, true, nil
			}
		}
		return model.Version{ID: versionID}, false, nil
	}
	addVersion := func(targetType, targetID, path string, content []byte) (int, error) {
		version, exists, err := findRunVersion(targetType, targetID)
		if err != nil {
			return 0, err
		}
		if exists {
			return version.VersionNo, nil
		}
		number, err := c.store.NextVersionNo(ctx, targetType, targetID)
		if err != nil {
			return 0, err
		}
		if err := atomicWrite(filepath.Join(c.project.WorkDir, filepath.FromSlash(path)), content); err != nil {
			return 0, err
		}
		snapshotPaths = append(snapshotPaths, path)
		versions = append(versions, model.Version{
			ID:         version.ID,
			TargetType: targetType, TargetID: targetID,
			VersionNo: number, SnapshotPath: path, RunID: c.runID, CreatedAt: time.Now().Unix(),
		})
		return number, nil
	}
	if _, ok := changed[(workflow.ArtifactRef{Kind: workflow.ArtifactOutline, ID: c.project.ID}).Key()]; ok {
		if _, err := addVersion("outline", model.OutlineVersionTarget(c.project.ID),
			model.OutlineVersionSnapshot(outline.Revision), mustJSON(outline)); err != nil {
			cleanup()
			return err
		}
	}
	if _, ok := changed[(workflow.ArtifactRef{Kind: workflow.ArtifactManifest, ID: c.project.ID}).Key()]; ok {
		if _, err := addVersion("manifest", model.ManifestVersionTarget(c.project.ID), model.ManifestVersionSnapshot(manifest.Revision), mustJSON(manifest)); err != nil {
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
	flat := spec.FlattenOutline(outline)
	nextSlides := make([]model.Slide, 0, len(flat))
	inDeck := map[string]bool{}
	for _, location := range flat {
		id := location.Slide.SlideID
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
		materializationPath := filepath.Join(
			c.project.WorkDir,
			filepath.FromSlash(model.SlideMaterializationPath(id)),
		)
		currentMaterialization, materializationErr := spec.ReadMaterialization(materializationPath)
		expectedHTMLRevision := 0
		if materializationErr == nil {
			expectedHTMLRevision = currentMaterialization.Artifact.Revision
		}
		if htmlChanged {
			expectedHTMLRevision++
		} else if expectedHTMLRevision == 0 && hasProof {
			expectedHTMLRevision = 1
		}
		if htmlChanged && !hasProof {
			cleanup()
			return fmt.Errorf("latest materialization proof is required for changed HTML %s", id)
		}
		if hasProof {
			artifactHash := spec.ContentHash(htmlRaw)[len("sha256:"):]
			nodeHash := spec.SemanticSlideNodeHash(outline, id)
			sourceHash := spec.SourceHash(manifestRaw, nodeHash, specRaw, designRaw)
			if proof.ArtifactHash != artifactHash ||
				proof.SourceHash != sourceHash ||
				proof.HTMLRevision != expectedHTMLRevision ||
				proof.ManifestRevision != manifest.Revision || proof.OutlineNodeHash != nodeHash ||
				proof.SpecRevision != semantic.Revision || proof.DesignContentHash != spec.DesignContentHash(design) || proof.FrameContextHash != spec.FrameContextHash(manifest, outline, design, id) {
				cleanup()
				return fmt.Errorf("stale materialization proof for %s", id)
			}
		}
		if htmlChanged {
			target := model.SlideHTMLVersionTarget(c.project.ID, id)
			version, exists, versionErr := findRunVersion("slide_html", target)
			if versionErr != nil {
				cleanup()
				return versionErr
			}
			number := version.VersionNo
			if !exists {
				number, versionErr = c.store.NextVersionNo(ctx, "slide_html", target)
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
					ID:         version.ID,
					TargetType: "slide_html", TargetID: target,
					VersionNo: number, SnapshotPath: path, RunID: c.runID, CreatedAt: time.Now().Unix(),
				})
			}
			meta.CurrentVersion = number
		}
		if hasProof {
			record := spec.MaterializationRecord{
				SchemaVersion: spec.SchemaVersion,
				Artifact: spec.MaterializationArtifact{
					Revision: proof.HTMLRevision,
					Hash:     "sha256:" + proof.ArtifactHash,
				},
				Source: spec.MaterializationSource{
					ManifestRevision: proof.ManifestRevision, OutlineNodeHash: proof.OutlineNodeHash,
					SpecRevision: proof.SpecRevision, DesignContentHash: proof.DesignContentHash, Hash: proof.SourceHash,
				},
				Frame:      spec.MaterializationFrame{ContextHash: proof.FrameContextHash},
				RenderedAt: time.Now().Unix(),
			}
			if err := spec.ValidateMaterialization(record); err != nil {
				cleanup()
				return err
			}
			recordRaw, err := json.MarshalIndent(record, "", "  ")
			if err != nil {
				cleanup()
				return err
			}
			before, readErr := os.ReadFile(materializationPath)
			materializationBackups = append(materializationBackups, materializationBackup{
				path: materializationPath, raw: before, existed: readErr == nil,
			})
			if err := atomicWrite(materializationPath, recordRaw); err != nil {
				cleanup()
				return err
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
		ProjectID: c.project.ID,
		Slides:    nextSlides, DeletedSlideIDs: deleted, Versions: versions,
	}
	if err := c.store.CommitWorkflow(ctx, commit); err != nil {
		cleanup()
		return err
	}
	materializationBackups = nil
	return nil
}

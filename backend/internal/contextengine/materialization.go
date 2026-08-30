package contextengine

import (
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func loadMaterializationState(workDir, slideID string, deck pptspec.Manifest, outline pptspec.Outline, slide pptspec.SlideSpec, design pptspec.Design) (string, model.MaterializationRevisions) {
	htmlRaw, htmlErr := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(model.SlideHTMLPath(slideID))))
	if htmlErr != nil {
		return string(model.MaterializationNotMaterialized), model.MaterializationRevisions{}
	}
	record, err := pptspec.ReadMaterialization(filepath.Join(workDir, filepath.FromSlash(model.SlideMaterializationPath(slideID))))
	if err != nil {
		return string(model.MaterializationUnknown), model.MaterializationRevisions{}
	}
	revisions := model.MaterializationRevisions{SlideHTML: record.Artifact.Revision, Outline: outline.Revision, SlideSpec: record.Source.SpecRevision, Design: design.Revision}
	deckRaw, deckErr := os.ReadFile(filepath.Join(workDir, "manifest.json"))
	specRaw, specErr := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(model.SlideSpecPath(slideID))))
	designRaw, designErr := os.ReadFile(filepath.Join(workDir, "design.json"))
	if deckErr != nil || specErr != nil || designErr != nil {
		return string(model.MaterializationUnknown), revisions
	}
	nodeHash := pptspec.SemanticSlideNodeHash(outline, slideID)
	return pptspec.DeriveMaterializationState(true, &record, deck.Revision, nodeHash, slide.Revision, pptspec.DesignContentHash(design),
		pptspec.ContentHash(htmlRaw), pptspec.SourceHash(deckRaw, nodeHash, specRaw, designRaw), pptspec.FrameContextHash(deck, outline, design, slideID)), revisions
}

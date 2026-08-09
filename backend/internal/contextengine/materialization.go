package contextengine

import (
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func loadMaterializationState(
	workDir, slideID string,
	outlineRevision, specRevision, designRevision int,
) (string, model.MaterializationRevisions) {
	htmlRaw, htmlErr := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(model.SlideHTMLPath(slideID))))
	if htmlErr != nil {
		return string(model.MaterializationNotMaterialized), model.MaterializationRevisions{}
	}
	record, recordErr := pptspec.ReadMaterialization(
		filepath.Join(workDir, filepath.FromSlash(model.SlideMaterializationPath(slideID))),
	)
	if recordErr != nil {
		return string(model.MaterializationUnknown), model.MaterializationRevisions{}
	}
	revisions := model.MaterializationRevisions{
		SlideHTML: record.Artifact.Revision,
		Outline:   record.Source.Outline,
		SlideSpec: record.Source.Spec,
		Design:    record.Source.Design,
	}
	outlineRaw, outlineErr := os.ReadFile(filepath.Join(workDir, "outline.json"))
	specRaw, specErr := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(model.SlideSpecPath(slideID))))
	designRaw, designErr := os.ReadFile(filepath.Join(workDir, "design.json"))
	if outlineErr != nil || specErr != nil || designErr != nil {
		return string(model.MaterializationUnknown), revisions
	}
	state := pptspec.DeriveMaterializationState(
		true,
		&record,
		outlineRevision,
		specRevision,
		designRevision,
		pptspec.ContentHash(htmlRaw),
		pptspec.SourceHash(outlineRaw, specRaw, designRaw),
	)
	return state, revisions
}

package contextengine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type ProjectLoader struct{}

func (ProjectLoader) Load(project model.Project) ProjectContext {
	return ProjectContext{ID: project.ID, Title: project.Title}
}

type OutlineLoader struct{}

func (OutlineLoader) Load(workDir string) (pptspec.Outline, error) {
	var outline pptspec.Outline
	return outline, readSourceJSON(filepath.Join(workDir, "outline.json"), &outline)
}

type ManifestLoader struct{}

func (ManifestLoader) Load(workDir string) (pptspec.Manifest, error) {
	var manifest pptspec.Manifest
	return manifest, readSourceJSON(filepath.Join(workDir, "manifest.json"), &manifest)
}

type SlideSpecLoader struct{}

func (SlideSpecLoader) LoadAll(workDir string, ids []string) (map[string]pptspec.SlideSpec, error) {
	out := make(map[string]pptspec.SlideSpec, len(ids))
	for _, id := range ids {
		var slide pptspec.SlideSpec
		if err := readSourceJSON(filepath.Join(workDir, filepath.FromSlash(model.SlideSpecPath(id))), &slide); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("slide %s: %w", id, err)
		}
		out[id] = slide
	}
	return out, nil
}

type RelatedSlideLoader struct{}

func (RelatedSlideLoader) Load(deck pptspec.Outline, slides map[string]pptspec.SlideSpec, target pptspec.SlideSpec) []SlideSummary {
	return relatedSummaries(deck, slides, target)
}

type DesignLoader struct{}

func (DesignLoader) Load(workDir string) (pptspec.Design, error) {
	var design pptspec.Design
	return design, readSourceJSON(filepath.Join(workDir, "design.json"), &design)
}

type SlideHTMLSummaryLoader struct{}

func (SlideHTMLSummaryLoader) Load(path string) (HTMLSummary, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return HTMLSummary{}, nil, err
	}
	summary, err := SummarizeHTML(raw)
	return summary, raw, err
}

type AssetCandidateLoader struct{}

func (AssetCandidateLoader) Select(assets []model.Asset, target *pptspec.SlideSpec) []AssetCandidate {
	return selectAssets(assets, target)
}

type ThreadMemoryLoader struct{ Store ThreadMemoryStore }

func (l ThreadMemoryLoader) Load(workDir, threadID string) (ThreadMemory, []string, error) {
	return l.Store.Load(workDir, threadID)
}

type RevisionLoader struct{}

func (RevisionLoader) From(manifest pptspec.Manifest, outline pptspec.Outline, design pptspec.Design, slides map[string]pptspec.SlideSpec, memory ThreadMemory) RevisionRefs {
	r := RevisionRefs{
		Manifest: manifest.Revision, Outline: outline.Revision, Design: design.Revision,
		SlideSpecs: map[string]int{}, SlideHTML: map[string]int{}, ThreadMemory: memory.Revision,
	}
	for id, slide := range slides {
		r.SlideSpecs[id] = slide.Revision
	}
	return r
}

func readSourceJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrSourceInvalid, path, err)
	}
	return nil
}

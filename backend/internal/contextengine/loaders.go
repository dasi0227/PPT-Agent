package contextengine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type ProjectLoader struct{}

func (ProjectLoader) Load(project model.Project) ProjectContext {
	return ProjectContext{ID: project.ID, Title: project.Title}
}

type DeckLoader struct{}

func (DeckLoader) Load(workDir string) (blueprint.Deck, error) {
	var deck blueprint.Deck
	return deck, readSourceJSON(filepath.Join(workDir, "deck.json"), &deck)
}

type SlideBlueprintLoader struct{}

func (SlideBlueprintLoader) LoadAll(workDir string, ids []string) (map[string]blueprint.Slide, error) {
	out := make(map[string]blueprint.Slide, len(ids))
	for _, id := range ids {
		var slide blueprint.Slide
		if err := readSourceJSON(filepath.Join(workDir, "slides", id, "slide.json"), &slide); err != nil {
			return nil, fmt.Errorf("slide %s: %w", id, err)
		}
		out[id] = slide
	}
	return out, nil
}

type RelatedSlideLoader struct{}

func (RelatedSlideLoader) Load(deck blueprint.Deck, slides map[string]blueprint.Slide, target blueprint.Slide) []SlideSummary {
	return relatedSummaries(deck, slides, target)
}

type DesignSpecLoader struct{}

func (DesignSpecLoader) Load(workDir string) (blueprint.DesignSpec, error) {
	var spec blueprint.DesignSpec
	return spec, readSourceJSON(filepath.Join(workDir, "design", "design-spec.json"), &spec)
}

type PresentationSummaryLoader struct{}

func (PresentationSummaryLoader) Load(path string) (HTMLSummary, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return HTMLSummary{}, nil, err
	}
	summary, err := SummarizeHTML(raw)
	return summary, raw, err
}

type AssetCandidateLoader struct{}

func (AssetCandidateLoader) Select(assets []model.Asset, target *blueprint.Slide) []AssetCandidate {
	return selectAssets(assets, target)
}

type ThreadMemoryLoader struct{ Store ThreadMemoryStore }

func (l ThreadMemoryLoader) Load(workDir, threadID string) (ThreadMemory, []string, error) {
	return l.Store.Load(workDir, threadID)
}

type RevisionLoader struct{}

func (RevisionLoader) From(deck blueprint.Deck, design blueprint.DesignSpec, slides map[string]blueprint.Slide, memory ThreadMemory) RevisionRefs {
	r := RevisionRefs{Deck: deck.Revision, Design: design.Revision, Slides: map[string]int{}, Presentations: map[string]int{}, ThreadMemory: memory.Revision}
	for id, slide := range slides {
		r.Slides[id] = slide.Revision
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

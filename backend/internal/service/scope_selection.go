package service

import (
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func resolveRunScope(snapshot spec.ProjectContentSnapshot, input model.CreateRunScopeInput) (model.RunScope, error) {
	locations := spec.FlattenOutline(snapshot.Outline)
	ordered := make([]string, 0, len(locations))
	knownSlides := make(map[string]bool, len(locations))
	for _, location := range locations {
		id := location.Slide.SlideID
		ordered = append(ordered, id)
		knownSlides[id] = true
	}
	knownSections := make(map[string]spec.Section, len(snapshot.Outline.Sections))
	for _, section := range snapshot.Outline.Sections {
		knownSections[section.ID] = section
	}

	if input.Object == model.ScopeObjectGlobal {
		input.Selection = model.ScopeSelectionInput{Kind: model.ScopeAllPages}
	}
	result := model.RunScope{Object: input.Object, Revision: 1}
	switch input.Selection.Kind {
	case model.ScopeCurrentPage:
		id := strings.TrimSpace(input.Selection.CurrentSlideID)
		if !knownSlides[id] {
			return model.RunScope{}, fmt.Errorf("%w: current slide %q not found", model.ErrInvalidRunCommand, id)
		}
		result.SlideIDs = []string{id}
		result.Source = model.ScopeSource{Kind: model.ScopeCurrentPage}
	case model.ScopeAllPages:
		result.SlideIDs = append([]string{}, ordered...)
		result.Source = model.ScopeSource{Kind: model.ScopeAllPages}
		result.IncludeRunCreatedSlides = true
	case model.ScopeCustomPages:
		selected, err := normalizeSelectedSlides(ordered, knownSlides, input.Selection.SlideIDs)
		if err != nil {
			return model.RunScope{}, err
		}
		result.SlideIDs = selected
		result.Source = model.ScopeSource{Kind: model.ScopeCustomPages}
	case model.ScopeCustomSections:
		if len(input.Selection.SectionIDs) == 0 {
			return model.RunScope{}, fmt.Errorf("%w: custom_sections requires section_ids", model.ErrInvalidRunCommand)
		}
		selectedSections := make(map[string]bool, len(input.Selection.SectionIDs))
		for _, rawID := range input.Selection.SectionIDs {
			id := strings.TrimSpace(rawID)
			if _, ok := knownSections[id]; !ok {
				return model.RunScope{}, fmt.Errorf("%w: section %q not found", model.ErrInvalidRunCommand, id)
			}
			selectedSections[id] = true
		}
		selectedSlides := make(map[string]bool)
		sectionIDs := make([]string, 0, len(selectedSections))
		for _, section := range snapshot.Outline.Sections {
			if !selectedSections[section.ID] {
				continue
			}
			sectionIDs = append(sectionIDs, section.ID)
			for _, slide := range section.Slides {
				selectedSlides[slide.SlideID] = true
			}
			for _, subsection := range section.Subsections {
				for _, slide := range subsection.Slides {
					selectedSlides[slide.SlideID] = true
				}
			}
		}
		for _, id := range ordered {
			if selectedSlides[id] {
				result.SlideIDs = append(result.SlideIDs, id)
			}
		}
		if len(result.SlideIDs) == 0 {
			return model.RunScope{}, fmt.Errorf("%w: selected sections contain no slides", model.ErrInvalidRunCommand)
		}
		result.Source = model.ScopeSource{Kind: model.ScopeCustomSections, SectionIDs: sectionIDs}
	default:
		return model.RunScope{}, fmt.Errorf("%w: unsupported scope selection %q", model.ErrInvalidRunCommand, input.Selection.Kind)
	}
	if err := result.Validate(); err != nil {
		return model.RunScope{}, fmt.Errorf("%w: %v", model.ErrInvalidRunCommand, err)
	}
	return result, nil
}

func normalizeSelectedSlides(ordered []string, known map[string]bool, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: custom_pages requires slide_ids", model.ErrInvalidRunCommand)
	}
	selected := make(map[string]bool, len(ids))
	for _, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if !known[id] {
			return nil, fmt.Errorf("%w: slide %q not found", model.ErrInvalidRunCommand, id)
		}
		selected[id] = true
	}
	out := make([]string, 0, len(selected))
	for _, id := range ordered {
		if selected[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

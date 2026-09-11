package service

import (
	"errors"
	"fmt"
	"slices"
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

func validateSelectionProject(snapshot spec.ProjectContentSnapshot, selections []model.DOMSelection, deletedSlideIDs map[string]bool) error {
	known := map[string]bool{}
	for _, location := range spec.FlattenOutline(snapshot.Outline) {
		known[location.Slide.SlideID] = true
	}
	for _, selection := range selections {
		if !known[selection.SlideID] && (selection.Status != model.DOMSelectionPageDeleted || !deletedSlideIDs[selection.SlideID]) {
			return model.ErrDOMSelectionInvalid
		}
		if known[selection.SlideID] && selection.Status == model.DOMSelectionPageDeleted {
			return model.ErrDOMSelectionInvalid
		}
	}
	return nil
}

func reconcileSelectionMaterialization(snapshot spec.ProjectContentSnapshot, selections []model.DOMSelection) {
	for index := range selections {
		content, exists := snapshot.SlidesByID[selections[index].SlideID]
		if !exists || content.Materialization == nil {
			continue
		}
		artifact := content.Materialization.Artifact
		if artifact.Revision != selections[index].HTMLRevision || artifact.Hash != selections[index].HTMLHash {
			continue
		}
		selections[index].Status = model.DOMSelectionActive
		for targetIndex := range selections[index].DOMTargets {
			selections[index].DOMTargets[targetIndex].Status = model.DOMSelectionActive
		}
	}
}

func mergeSelectionScope(scope model.RunScope, snapshot spec.ProjectContentSnapshot, selections []model.DOMSelection) model.RunScope {
	if len(selections) == 0 {
		return scope
	}
	original := scope
	ordered := spec.FlattenOutline(snapshot.Outline)
	selected := map[string]bool{}
	for _, id := range scope.SlideIDs {
		selected[id] = true
	}
	hasChrome := false
	for _, selection := range selections {
		if selection.Status != model.DOMSelectionPageDeleted {
			selected[selection.SlideID] = true
		}
		if len(selection.ChromeTargets) > 0 {
			hasChrome = true
		}
	}
	if hasChrome {
		scope.Object = model.ScopeObjectGlobal
		scope.Source = model.ScopeSource{Kind: model.ScopeAllPages}
		scope.SlideIDs = scope.SlideIDs[:0]
		for _, location := range ordered {
			scope.SlideIDs = append(scope.SlideIDs, location.Slide.SlideID)
		}
		scope.IncludeRunCreatedSlides = true
	} else {
		if scope.Object == model.ScopeObjectSpec {
			scope.Object = model.ScopeObjectPresentation
		}
		next := make([]string, 0, len(selected))
		for _, location := range ordered {
			if selected[location.Slide.SlideID] {
				next = append(next, location.Slide.SlideID)
			}
		}
		if !slices.Equal(next, scope.SlideIDs) {
			scope.SlideIDs = next
			if scope.Source.Kind != model.ScopeAllPages {
				scope.Source = model.ScopeSource{Kind: model.ScopeCustomPages}
				scope.IncludeRunCreatedSlides = false
			}
		}
	}
	semanticEqual := original.Object == scope.Object && original.Source.Kind == scope.Source.Kind && slices.Equal(original.Source.SectionIDs, scope.Source.SectionIDs) && slices.Equal(original.SlideIDs, scope.SlideIDs) && original.IncludeRunCreatedSlides == scope.IncludeRunCreatedSlides
	if !semanticEqual {
		scope.Revision = original.Revision + 1
	}
	return scope
}

func domSelectionAgentError(operation string, err error) error {
	code := "DOM_SELECTION_INVALID"
	switch {
	case errors.Is(err, model.ErrDOMSelectionLimit):
		code = "DOM_SELECTION_LIMIT"
	case errors.Is(err, model.ErrDOMSelectionTooLarge):
		code = "DOM_SELECTION_TOO_LARGE"
	case errors.Is(err, model.ErrDOMSelectionCommentTooLong):
		code = "DOM_SELECTION_COMMENT_TOO_LONG"
	case errors.Is(err, model.ErrReferenceOrderInvalid):
		code = "REFERENCE_ORDER_INVALID"
	}
	return model.NewAgentError(code, operation, err)
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

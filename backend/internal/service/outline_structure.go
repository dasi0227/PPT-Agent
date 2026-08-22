package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

// Outline structure editing: manual add / remove of sections and subsections.
//
// These operations keep the strict two-level invariant (a section is either
// "direct": no subsection, pages mount on the section; or "grouped": has
// subsections, pages mount on a subsection). Slide order is never touched here,
// so callers do not reorder; only placement (section_id / subsection_id on each
// slide spec) and the outline sections change.

const (
	defaultSectionTitle    = "新章节"
	defaultSectionPurpose  = "待补充本章节目标"
	defaultSubsectionTitle = "新子节"
	maxDirectoryTitleRunes = 60
)

func normalizeDirectoryTitle(title string) (string, error) {
	normalized := strings.TrimSpace(title)
	if normalized == "" || utf8.RuneCountInString(normalized) > maxDirectoryTitleRunes {
		return "", validationError("title length must be 1..%d", maxDirectoryTitleRunes)
	}
	return normalized, nil
}

// AddSection appends a new empty section (direct form, no pages) to the outline.
func (svc *SlideService) AddSection(ctx context.Context, projectID, title string) (StructureSnapshot, error) {
	return svc.editOutlineStructure(ctx, projectID, func(outline *spec.Outline, _ map[string]spec.SlideSpec) (map[string]spec.SlideSpec, error) {
		outline.Sections = append(outline.Sections, spec.Section{
			ID:          model.MustShortID("sec"),
			Title:       firstNonEmpty(title, defaultSectionTitle),
			Purpose:     defaultSectionPurpose,
			Subsections: []spec.Subsection{},
		})
		return nil, nil
	})
}

// AddSubsection adds a subsection to a section. If the section was in direct
// form and already owns pages, those direct pages migrate into the new
// subsection so the section becomes grouped without losing data.
func (svc *SlideService) AddSubsection(ctx context.Context, projectID, sectionID, title string) (StructureSnapshot, error) {
	return svc.editOutlineStructure(ctx, projectID, func(outline *spec.Outline, specs map[string]spec.SlideSpec) (map[string]spec.SlideSpec, error) {
		section := findSection(outline, sectionID)
		if section == nil {
			return nil, validationError("unknown section_id %s", sectionID)
		}
		newSub := spec.Subsection{ID: model.MustShortID("sub"), Title: firstNonEmpty(title, defaultSubsectionTitle)}
		wasDirect := len(section.Subsections) == 0
		section.Subsections = append(section.Subsections, newSub)
		if !wasDirect {
			return nil, nil
		}
		changed := map[string]spec.SlideSpec{}
		for id, s := range specs {
			if s.SectionID == sectionID && s.SubsectionID == "" {
				s.SubsectionID = newSub.ID
				specs[id] = s
				changed[id] = s
			}
		}
		return changed, nil
	})
}

// RenameSection updates only the user-facing section title and returns the
// authoritative project snapshot.
func (svc *SlideService) RenameSection(ctx context.Context, projectID, sectionID, title string) (StructureSnapshot, error) {
	normalized, err := normalizeDirectoryTitle(title)
	if err != nil {
		return StructureSnapshot{}, err
	}
	return svc.editOutlineStructure(ctx, projectID, func(outline *spec.Outline, _ map[string]spec.SlideSpec) (map[string]spec.SlideSpec, error) {
		section := findSection(outline, sectionID)
		if section == nil {
			return nil, validationError("unknown section_id %s", sectionID)
		}
		section.Title = normalized
		return nil, nil
	})
}

// RenameSubsection updates only the user-facing subsection title and returns
// the authoritative project snapshot.
func (svc *SlideService) RenameSubsection(ctx context.Context, projectID, sectionID, subsectionID, title string) (StructureSnapshot, error) {
	normalized, err := normalizeDirectoryTitle(title)
	if err != nil {
		return StructureSnapshot{}, err
	}
	return svc.editOutlineStructure(ctx, projectID, func(outline *spec.Outline, _ map[string]spec.SlideSpec) (map[string]spec.SlideSpec, error) {
		section := findSection(outline, sectionID)
		if section == nil {
			return nil, validationError("unknown section_id %s", sectionID)
		}
		subsection := findSubsection(section, subsectionID)
		if subsection == nil {
			return nil, validationError("subsection_id %s does not belong to section_id %s", subsectionID, sectionID)
		}
		subsection.Title = normalized
		return nil, nil
	})
}

// RenameSlide updates the title in the slide spec, leaving placement and page
// order untouched, then returns the authoritative project snapshot.
func (svc *SlideService) RenameSlide(ctx context.Context, projectID, slideID, title string) (StructureSnapshot, error) {
	normalized, err := normalizeDirectoryTitle(title)
	if err != nil {
		return StructureSnapshot{}, err
	}
	meta, err := svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return StructureSnapshot{}, err
	}
	if meta.ProjectID != projectID {
		return StructureSnapshot{}, ErrSlideTargetNotFound
	}
	if active, err := svc.store.HasActiveRun(ctx, meta.ProjectID); err != nil {
		return StructureSnapshot{}, err
	} else if active {
		return StructureSnapshot{}, ErrRunActive
	}
	specService := NewSpecService(svc.store)
	current, _, err := specService.GetSlide(ctx, slideID)
	if err != nil {
		return StructureSnapshot{}, err
	}
	next := current
	next.Title = normalized
	if _, err := specService.PatchSlide(ctx, slideID, current.Revision, next); err != nil {
		return StructureSnapshot{}, err
	}
	return svc.readStructureSnapshot(ctx, meta.ProjectID)
}

// RemoveSection deletes a section. It is rejected while the section still owns
// any page; the user must move those pages away first.
func (svc *SlideService) RemoveSection(ctx context.Context, projectID, sectionID string) (StructureSnapshot, error) {
	return svc.editOutlineStructure(ctx, projectID, func(outline *spec.Outline, specs map[string]spec.SlideSpec) (map[string]spec.SlideSpec, error) {
		if findSection(outline, sectionID) == nil {
			return nil, validationError("unknown section_id %s", sectionID)
		}
		if sectionOwnsPage(specs, sectionID) {
			return nil, validationError("section_id %s still owns pages; move them out first", sectionID)
		}
		outline.Sections = removeSection(outline.Sections, sectionID)
		return nil, nil
	})
}

// RemoveSubsection deletes a subsection. If it is empty it is removed directly.
// If it owns pages and is the section's only subsection, those pages fall back
// to direct pages (the section returns to direct form). If it owns pages while
// other subsections remain, the delete is rejected (page destination ambiguous).
func (svc *SlideService) RemoveSubsection(ctx context.Context, projectID, sectionID, subsectionID string) (StructureSnapshot, error) {
	return svc.editOutlineStructure(ctx, projectID, func(outline *spec.Outline, specs map[string]spec.SlideSpec) (map[string]spec.SlideSpec, error) {
		section := findSection(outline, sectionID)
		if section == nil {
			return nil, validationError("unknown section_id %s", sectionID)
		}
		if !subsectionBelongs(section, subsectionID) {
			return nil, validationError("subsection_id %s does not belong to section_id %s", subsectionID, sectionID)
		}
		ownsPage := subsectionOwnsPage(specs, subsectionID)
		if ownsPage && len(section.Subsections) > 1 {
			return nil, validationError("subsection_id %s still owns pages; move them out first", subsectionID)
		}
		section.Subsections = removeSubsection(section.Subsections, subsectionID)
		if !ownsPage {
			return nil, nil
		}
		// Only subsection removed while owning pages: pages fall back to direct.
		changed := map[string]spec.SlideSpec{}
		for id, s := range specs {
			if s.SubsectionID == subsectionID {
				s.SubsectionID = ""
				specs[id] = s
				changed[id] = s
			}
		}
		return changed, nil
	})
}

// editOutlineStructure runs a mutation against a working copy of the outline and
// slide specs, then validates and persists the result via the shared strict
// two-level path, returning the authoritative snapshot.
func (svc *SlideService) editOutlineStructure(
	ctx context.Context,
	projectID string,
	mutate func(outline *spec.Outline, specs map[string]spec.SlideSpec) (map[string]spec.SlideSpec, error),
) (StructureSnapshot, error) {
	if active, err := svc.store.HasActiveRun(ctx, projectID); err != nil {
		return StructureSnapshot{}, err
	} else if active {
		return StructureSnapshot{}, ErrRunActive
	}
	project, err := svc.store.GetProject(ctx, projectID)
	if err != nil {
		return StructureSnapshot{}, err
	}
	view, err := NewSpecService(svc.store).EnsureProject(ctx, projectID)
	if err != nil {
		return StructureSnapshot{}, err
	}

	nextOutline := view.Outline
	nextOutline.Sections = cloneSections(view.Outline.Sections)
	nextOutline.Revision = view.Outline.Revision + 1
	nextSpecs := make(map[string]spec.SlideSpec, len(view.SlideSpecs))
	for id, slide := range view.SlideSpecs {
		nextSpecs[id] = slide
	}

	changed, err := mutate(&nextOutline, nextSpecs)
	if err != nil {
		return StructureSnapshot{}, err
	}
	now := svc.clock()
	for id, slide := range changed {
		slide.Revision++
		slide.UpdatedAt = now
		if err := spec.ValidateSlideSpec(slide); err != nil {
			return StructureSnapshot{}, validationError("%v", err)
		}
		changed[id] = slide
		nextSpecs[id] = slide
	}
	if err := spec.ValidateOutline(nextOutline, nextSpecs); err != nil {
		return StructureSnapshot{}, validationError("%v", err)
	}
	if err := svc.persistStructureChange(ctx, project, projectID, view.Outline.Revision, nextOutline, changed); err != nil {
		return StructureSnapshot{}, err
	}
	return svc.readStructureSnapshot(ctx, projectID)
}

func findSection(outline *spec.Outline, sectionID string) *spec.Section {
	for i := range outline.Sections {
		if outline.Sections[i].ID == sectionID {
			return &outline.Sections[i]
		}
	}
	return nil
}

func findSubsection(section *spec.Section, subsectionID string) *spec.Subsection {
	for i := range section.Subsections {
		if section.Subsections[i].ID == subsectionID {
			return &section.Subsections[i]
		}
	}
	return nil
}

func subsectionBelongs(section *spec.Section, subsectionID string) bool {
	for _, sub := range section.Subsections {
		if sub.ID == subsectionID {
			return true
		}
	}
	return false
}

func sectionOwnsPage(specs map[string]spec.SlideSpec, sectionID string) bool {
	for _, s := range specs {
		if s.SectionID == sectionID {
			return true
		}
	}
	return false
}

func subsectionOwnsPage(specs map[string]spec.SlideSpec, subsectionID string) bool {
	for _, s := range specs {
		if s.SubsectionID == subsectionID {
			return true
		}
	}
	return false
}

func cloneSections(sections []spec.Section) []spec.Section {
	out := make([]spec.Section, len(sections))
	for i, section := range sections {
		out[i] = section
		out[i].Subsections = append([]spec.Subsection{}, section.Subsections...)
	}
	return out
}

func removeSection(sections []spec.Section, sectionID string) []spec.Section {
	out := make([]spec.Section, 0, len(sections))
	for _, section := range sections {
		if section.ID != sectionID {
			out = append(out, section)
		}
	}
	return out
}

func removeSubsection(subsections []spec.Subsection, subsectionID string) []spec.Subsection {
	out := make([]spec.Subsection, 0, len(subsections))
	for _, sub := range subsections {
		if sub.ID != subsectionID {
			out = append(out, sub)
		}
	}
	return out
}

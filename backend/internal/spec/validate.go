package spec

import (
	"encoding/json"
	"errors"
	"fmt"

	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
)

var (
	ErrInvalid         = errors.New("spec invalid")
	ErrReferenceBroken = errors.New("spec reference broken")
)

func ValidateOutline(d Outline, slides map[string]SlideSpec) error {
	if err := validateSchema(pptschema.OutlineName, d); err != nil {
		return err
	}
	index := SectionIndex{
		Sections:         map[string]bool{},
		SubsectionOwners: map[string]string{},
		SectionHasSub:    map[string]bool{},
	}
	for _, section := range d.Sections {
		if section.ID == "" || section.Title == "" || section.Purpose == "" || index.Sections[section.ID] {
			return fmt.Errorf("%w: invalid or duplicate section", ErrInvalid)
		}
		index.Sections[section.ID] = true
		for _, subsection := range section.Subsections {
			if subsection.ID == "" || subsection.Title == "" || index.SubsectionOwners[subsection.ID] != "" {
				return fmt.Errorf("%w: invalid or duplicate subsection", ErrInvalid)
			}
			index.SubsectionOwners[subsection.ID] = section.ID
			index.SectionHasSub[section.ID] = true
		}
	}
	seen := map[string]bool{}
	for _, id := range d.SlideOrder {
		if seen[id] {
			return fmt.Errorf("%w: duplicate slide_id %s", ErrInvalid, id)
		}
		seen[id] = true
		s, ok := slides[id]
		if !ok {
			return fmt.Errorf("%w: missing slide %s", ErrReferenceBroken, id)
		}
		if err := ValidateSlideSpec(s); err != nil {
			return err
		}
		if s.ProjectID != d.ProjectID {
			return fmt.Errorf("%w: slide %s belongs to another project", ErrReferenceBroken, id)
		}
		if s.SlideID != id {
			return fmt.Errorf("%w: slide key %s does not match slide_id %s", ErrReferenceBroken, id, s.SlideID)
		}
		if err := index.ValidatePlacement(s.SectionID, s.SubsectionID); err != nil {
			return fmt.Errorf("%w: slide %s %v", ErrReferenceBroken, id, err)
		}
	}
	return nil
}

// SectionIndex captures the strict two-level structure derived from an outline:
// which sections exist, which section each subsection belongs to, and whether a
// section is in grouped form (has at least one subsection).
type SectionIndex struct {
	Sections         map[string]bool
	SubsectionOwners map[string]string
	SectionHasSub    map[string]bool
}

// BuildSectionIndex derives the strict two-level lookup tables from an outline.
func BuildSectionIndex(sections []Section) SectionIndex {
	index := SectionIndex{
		Sections:         map[string]bool{},
		SubsectionOwners: map[string]string{},
		SectionHasSub:    map[string]bool{},
	}
	for _, section := range sections {
		index.Sections[section.ID] = true
		for _, subsection := range section.Subsections {
			index.SubsectionOwners[subsection.ID] = section.ID
			index.SectionHasSub[section.ID] = true
		}
	}
	return index
}

// ValidatePlacement enforces the strict two-level mounting rule for one slide:
//   - the section must exist;
//   - grouped section (has subsections)  ⟹ slide must carry a subsection_id owned by that section;
//   - direct section (no subsections)    ⟹ slide must not carry any subsection_id.
func (index SectionIndex) ValidatePlacement(sectionID, subsectionID string) error {
	if !index.Sections[sectionID] {
		return fmt.Errorf("references unknown section_id %s", sectionID)
	}
	if index.SectionHasSub[sectionID] {
		if subsectionID == "" {
			return fmt.Errorf("section_id %s is grouped and requires a subsection_id", sectionID)
		}
		if index.SubsectionOwners[subsectionID] != sectionID {
			return fmt.Errorf("subsection_id %s does not belong to section_id %s", subsectionID, sectionID)
		}
		return nil
	}
	if subsectionID != "" {
		return fmt.Errorf("section_id %s is direct and must not carry a subsection_id", sectionID)
	}
	return nil
}

func ValidateSlideSpec(s SlideSpec) error {
	return validateSchema(pptschema.SlideSpecName, s)
}

func ValidateDesign(d Design) error {
	return validateSchema(pptschema.DesignName, d)
}

func ValidateMaterialization(value MaterializationRecord) error {
	return validateSchema(pptschema.MaterializationName, value)
}

func validateSchema(name string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if err := pptschema.Validate(name, decoded); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return nil
}

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

var validRoles = map[string]bool{
	"cover": true, "agenda": true, "section-divider": true, "context": true,
	"problem": true, "insight": true, "evidence": true, "comparison": true,
	"solution": true, "process": true, "case-study": true, "summary": true,
	"cta": true, "closing": true,
}

func ValidateOutline(d Outline, slides map[string]SlideSpec) error {
	if err := validateSchema(pptschema.OutlineName, d); err != nil {
		return err
	}
	sections, subsectionOwners := map[string]bool{}, map[string]string{}
	for _, section := range d.Sections {
		if section.ID == "" || section.Title == "" || section.Purpose == "" || sections[section.ID] {
			return fmt.Errorf("%w: invalid or duplicate section", ErrInvalid)
		}
		sections[section.ID] = true
		for _, subsection := range section.Subsections {
			if subsection.ID == "" || subsection.Title == "" || subsectionOwners[subsection.ID] != "" {
				return fmt.Errorf("%w: invalid or duplicate subsection", ErrInvalid)
			}
			subsectionOwners[subsection.ID] = section.ID
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
		if !sections[s.SectionID] || (s.SubsectionID != "" && subsectionOwners[s.SubsectionID] != s.SectionID) {
			return fmt.Errorf("%w: slide %s references an unknown section", ErrReferenceBroken, id)
		}
	}
	return nil
}

func ValidateSlideSpec(s SlideSpec) error {
	if !validRoles[s.Role] {
		return fmt.Errorf("%w: unsupported role %q", ErrInvalid, s.Role)
	}
	return validateSchema(pptschema.SlideSpecName, s)
}

func ValidateDesign(d Design) error {
	return validateSchema(pptschema.DesignName, d)
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

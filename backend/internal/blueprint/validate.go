package blueprint

import (
	"errors"
	"fmt"
)

var (
	ErrInvalid         = errors.New("blueprint invalid")
	ErrReferenceBroken = errors.New("blueprint reference broken")
)

var validRoles = map[string]bool{
	"cover": true, "agenda": true, "section-divider": true, "context": true,
	"problem": true, "insight": true, "evidence": true, "comparison": true,
	"solution": true, "process": true, "case-study": true, "summary": true,
	"cta": true, "closing": true,
}

func ValidateDeck(d Deck, slides map[string]Slide) error {
	if d.SchemaVersion != SchemaVersion || d.Revision < 1 || d.ProjectID == "" || d.Title == "" {
		return fmt.Errorf("%w: invalid deck header", ErrInvalid)
	}
	sections, subsections := map[string]bool{}, map[string]bool{}
	for _, section := range d.Sections {
		if section.ID == "" || section.Number == "" || section.Title == "" || sections[section.ID] {
			return fmt.Errorf("%w: invalid or duplicate section", ErrInvalid)
		}
		sections[section.ID] = true
		for _, subsection := range section.Subsections {
			if subsection.ID == "" || subsection.Number == "" || subsection.Title == "" || subsections[subsection.ID] {
				return fmt.Errorf("%w: invalid or duplicate subsection", ErrInvalid)
			}
			subsections[subsection.ID] = true
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
		if err := ValidateSlide(s); err != nil {
			return err
		}
		if !sections[s.SectionID] || (s.SubsectionID != "" && !subsections[s.SubsectionID]) {
			return fmt.Errorf("%w: slide %s references an unknown section", ErrReferenceBroken, id)
		}
	}
	return nil
}

func ValidateSlide(s Slide) error {
	if s.SchemaVersion != SchemaVersion || s.Revision < 1 || s.SlideID == "" ||
		s.SectionID == "" || s.Title == "" || s.KeyMessage == "" ||
		s.Content.Summary == "" || s.VisualIntent.Archetype == "" || s.VisualIntent.Description == "" {
		return fmt.Errorf("%w: invalid slide %s", ErrInvalid, s.SlideID)
	}
	if !validRoles[s.Role] {
		return fmt.Errorf("%w: unsupported role %q", ErrInvalid, s.Role)
	}
	if len(s.Content.Points) > 6 {
		return fmt.Errorf("%w: at most 6 content points", ErrInvalid)
	}
	return nil
}

func ValidateDesignSpec(d DesignSpec) error {
	if d.SchemaVersion != SchemaVersion || d.Revision < 1 || len(d.Canvas) == 0 ||
		len(d.Palette) == 0 || len(d.Typography) == 0 || len(d.LayoutSystem) == 0 ||
		d.Signature == "" {
		return fmt.Errorf("%w: invalid design spec", ErrInvalid)
	}
	return nil
}

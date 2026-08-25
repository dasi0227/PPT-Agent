package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
)

var ErrInvalid = errors.New("spec invalid")
var ErrReferenceBroken = errors.New("spec reference broken")

type SlideLocation struct {
	Slide      SlideNode
	Section    *Section
	Subsection *Subsection
	Ordinal    int
}

func ValidateDeck(d Deck) error { return validateSchema(pptschema.DeckName, d) }
func ValidateOutline(d Outline) error {
	if err := validateSchema(pptschema.OutlineName, d); err != nil {
		return err
	}
	seen := map[string]bool{}
	for i := range d.Sections {
		section := &d.Sections[i]
		if seen[section.ID] {
			return fmt.Errorf("%w: duplicate node id %s", ErrInvalid, section.ID)
		}
		seen[section.ID] = true
		if len(section.Slides) > 0 && len(section.Subsections) > 0 {
			return fmt.Errorf("%w: section %s mixes direct slides and subsections", ErrInvalid, section.ID)
		}
		for _, slide := range section.Slides {
			if seen[slide.SlideID] {
				return fmt.Errorf("%w: duplicate node id %s", ErrInvalid, slide.SlideID)
			}
			seen[slide.SlideID] = true
		}
		for j := range section.Subsections {
			sub := &section.Subsections[j]
			if seen[sub.ID] {
				return fmt.Errorf("%w: duplicate node id %s", ErrInvalid, sub.ID)
			}
			seen[sub.ID] = true
			for _, slide := range sub.Slides {
				if seen[slide.SlideID] {
					return fmt.Errorf("%w: duplicate node id %s", ErrInvalid, slide.SlideID)
				}
				seen[slide.SlideID] = true
			}
		}
	}
	return nil
}

func FlattenOutline(outline Outline) []SlideLocation {
	out := []SlideLocation{}
	for si := range outline.Sections {
		section := &outline.Sections[si]
		for _, slide := range section.Slides {
			out = append(out, SlideLocation{Slide: slide, Section: section, Ordinal: len(out) + 1})
		}
		for subi := range section.Subsections {
			sub := &section.Subsections[subi]
			for _, slide := range sub.Slides {
				out = append(out, SlideLocation{Slide: slide, Section: section, Subsection: sub, Ordinal: len(out) + 1})
			}
		}
	}
	return out
}
func FindSlide(outline Outline, id string) (SlideLocation, bool) {
	for _, loc := range FlattenOutline(outline) {
		if loc.Slide.SlideID == id {
			return loc, true
		}
	}
	return SlideLocation{}, false
}
func ResolveSlideOrdinal(outline Outline, id string) (int, bool) {
	loc, ok := FindSlide(outline, id)
	return loc.Ordinal, ok
}
func SemanticSlideNodeHash(outline Outline, id string) string {
	loc, ok := FindSlide(outline, id)
	if !ok {
		return ""
	}
	value := map[string]any{"slide": loc.Slide, "section": map[string]any{"id": loc.Section.ID, "title": loc.Section.Title, "purpose": loc.Section.Purpose}}
	if loc.Subsection != nil {
		value["subsection"] = map[string]any{"id": loc.Subsection.ID, "title": loc.Subsection.Title}
	}
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func ValidateSlideSpec(s SlideSpec) error { return validateSchema(pptschema.SlideSpecName, s) }
func ValidateDesign(d Design) error       { return validateSchema(pptschema.DesignName, d) }
func ValidateMaterialization(v MaterializationRecord) error {
	return validateSchema(pptschema.MaterializationName, v)
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

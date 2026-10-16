package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/decoration"
	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
)

func FrameContextHash(manifest Manifest, outline Outline, positions decoration.Positions, logo *decoration.Image, slideID string, slide SlideSpec, appearance *designsystem.Appearance) string {
	frame, ok := BuildRuntimeFrame(manifest, outline, positions, logo, slideID, slide, appearance)
	if !ok {
		return ""
	}
	return RuntimeFrameHash(frame)
}

func RuntimeFrameHash(frame RuntimeFrameContext) string {
	// Only rendered appearance invalidates pixels. Source evidence is tracked separately.
	content := map[string]string{"slide_pagination": fmt.Sprint(frame.Ordinal), "deck_title": frame.DeckTitle, "section_title": frame.Section.Title}
	positions := map[string]string{"slide_pagination": frame.Decorations.SlidePagination, "deck_title": frame.Decorations.DeckTitle, "section_title": frame.Decorations.SectionTitle, "logo": frame.Decorations.Logo}
	visible := map[string]any{}
	for key, position := range positions {
		if position == "none" {
			continue
		}
		text := content[key]
		if key == "logo" {
			if frame.Logo == nil {
				continue
			}
			text = frame.Logo.Hash
		} else if strings.TrimSpace(text) == "" {
			continue
		}
		visible[key] = []string{position, text}
	}
	appearanceHash := ""
	if frame.Appearance != nil {
		appearanceHash = frame.Appearance.Hash
	}
	raw, _ := json.Marshal(struct {
		Canvas      RuntimeCanvas
		Appearance  string
		Decorations map[string]any
	}{frame.Canvas, appearanceHash, visible})
	return ContentHash(raw)
}

func BuildRuntimeFrame(manifest Manifest, outline Outline, positions decoration.Positions, logo *decoration.Image, slideID string, slide SlideSpec, appearance *designsystem.Appearance) (RuntimeFrameContext, bool) {
	loc, ok := FindSlide(outline, slideID)
	if !ok {
		return RuntimeFrameContext{}, false
	}
	sectionIndex := 0
	for index := range outline.Sections {
		if outline.Sections[index].ID == loc.Section.ID {
			sectionIndex = index + 1
			break
		}
	}
	var subsection *RuntimeFrameAncestor
	if loc.Subsection != nil {
		index := 0
		for candidate := range loc.Section.Subsections {
			if loc.Section.Subsections[candidate].ID == loc.Subsection.ID {
				index = candidate + 1
				break
			}
		}
		subsection = &RuntimeFrameAncestor{ID: loc.Subsection.ID, Title: loc.Subsection.Title, Index: index}
	}
	themeID := ""
	if appearance != nil {
		themeID = appearance.ThemeID
	}
	return RuntimeFrameContext{
		Logo: logo, Appearance: appearance, SlideID: slideID, Canvas: CanonicalCanvas(), ThemeID: themeID, DeckTitle: manifest.Title, Ordinal: loc.Ordinal, Total: len(FlattenOutline(outline)), Purpose: string(slide.Purpose), Approach: string(slide.Approach),
		Section:     RuntimeFrameAncestor{ID: loc.Section.ID, Title: loc.Section.Title, Index: sectionIndex},
		Subsection:  subsection,
		Decorations: positions,
	}, true
}

func ContentHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ResourceHash identifies authoring content, independently of JSON formatting and field order.
func ResourceHash(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return ResourceBytesHash(raw)
}
func ResourceBytesHash(raw []byte) string {
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil || value == nil {
		return ""
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return ContentHash(canonical)
}

func SourceHash(manifestRaw []byte, outlineNodeHash string, specRaw, designRaw []byte) string {
	var design Design
	_ = json.Unmarshal(designRaw, &design)
	designRaw, _ = json.Marshal(design)
	size := len(manifestRaw) + len(outlineNodeHash) + len(specRaw) + len(designRaw) + 48
	combined := make([]byte, 0, size)
	for _, item := range []struct {
		name string
		raw  []byte
	}{
		{name: "manifest", raw: []byte(ResourceBytesHash(manifestRaw))},
		{name: "outline_node", raw: []byte(outlineNodeHash)},
		{name: "spec", raw: []byte(ResourceBytesHash(specRaw))},
		{name: "design", raw: designRaw},
	} {
		combined = append(combined, item.name...)
		combined = append(combined, 0)
		combined = append(combined, item.raw...)
		combined = append(combined, 0)
	}
	return ContentHash(combined)
}

func DesignContentHash(design Design) string {
	return ResourceHash(design)
}

package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
)

func FrameContextHash(deck Deck, outline Outline, design Design, slideID string) string {
	frame, ok := BuildRuntimeFrame(deck, outline, design, slideID)
	if !ok {
		return ""
	}
	raw, _ := json.Marshal(frame)
	return ContentHash(raw)
}

func BuildRuntimeFrame(deck Deck, outline Outline, design Design, slideID string) (RuntimeFrameContext, bool) {
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
	visible := deck.Numbering.Enabled
	for _, role := range deck.Numbering.HiddenRoles {
		if role == loc.Slide.Role {
			visible = false
			break
		}
	}
	return RuntimeFrameContext{
		SlideID: slideID, DeckTitle: deck.Title, Ordinal: loc.Ordinal, Total: len(FlattenOutline(outline)), Role: loc.Slide.Role,
		Section:    RuntimeFrameAncestor{ID: loc.Section.ID, Title: loc.Section.Title, Index: sectionIndex},
		Subsection: subsection, Numbering: RuntimeFrameNumbering{Visible: visible, Format: deck.Numbering.Format},
		Chrome: append([]ChromeItem(nil), design.Chrome...),
	}, true
}

func ContentHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func SourceHash(deckRaw []byte, outlineNodeHash string, specRaw, designRaw []byte) string {
	size := len(deckRaw) + len(outlineNodeHash) + len(specRaw) + len(designRaw) + 48
	combined := make([]byte, 0, size)
	for _, item := range []struct {
		name string
		raw  []byte
	}{
		{name: "deck", raw: deckRaw},
		{name: "outline_node", raw: []byte(outlineNodeHash)},
		{name: "spec", raw: specRaw},
		{name: "design", raw: designRaw},
	} {
		combined = append(combined, item.name...)
		combined = append(combined, 0)
		combined = append(combined, item.raw...)
		combined = append(combined, 0)
	}
	return ContentHash(combined)
}

func ReadMaterialization(path string) (MaterializationRecord, error) {
	var value MaterializationRecord
	raw, err := os.ReadFile(path)
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return MaterializationRecord{}, err
	}
	if err := ValidateMaterialization(value); err != nil {
		return MaterializationRecord{}, err
	}
	return value, nil
}

func DeriveMaterializationState(
	hasHTML bool,
	record *MaterializationRecord,
	currentDeck int, currentOutlineNodeHash string, currentSpec, currentDesign int,
	artifactHash, sourceHash, frameHash string,
) string {
	if !hasHTML {
		return "not_materialized"
	}
	if record == nil {
		return "unknown"
	}
	if record.Artifact.Hash != artifactHash ||
		record.Source.DeckRevision > currentDeck ||
		record.Source.SpecRevision > currentSpec ||
		record.Source.DesignRevision > currentDesign {
		return "unknown"
	}
	if record.Source.DeckRevision < currentDeck || record.Source.OutlineNodeHash != currentOutlineNodeHash || record.Source.SpecRevision < currentSpec {
		return "spec_stale"
	}
	if record.Source.DesignRevision < currentDesign {
		return "design_stale"
	}
	if record.Source.Hash != sourceHash {
		return "unknown"
	}
	if record.Frame.ContextHash != frameHash {
		return "frame_stale"
	}
	return "fresh"
}

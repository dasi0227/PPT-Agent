package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
)

func FrameContextHash(manifest Manifest, outline Outline, design Design, slideID string) string {
	frame, ok := BuildRuntimeFrame(manifest, outline, design, slideID)
	if !ok {
		return ""
	}
	raw, _ := json.Marshal(frame)
	return ContentHash(raw)
}

func BuildRuntimeFrame(manifest Manifest, outline Outline, design Design, slideID string) (RuntimeFrameContext, bool) {
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
	visible := manifest.Numbering.Enabled
	for _, role := range manifest.Numbering.HiddenRoles {
		if role == string(loc.Slide.Role) {
			visible = false
			break
		}
	}
	return RuntimeFrameContext{
		SlideID: slideID, Canvas: CanonicalCanvas(), ThemeID: design.Theme, DeckTitle: manifest.Title, Ordinal: loc.Ordinal, Total: len(FlattenOutline(outline)), Role: string(loc.Slide.Role),
		Section:    RuntimeFrameAncestor{ID: loc.Section.ID, Title: loc.Section.Title, Index: sectionIndex},
		Subsection: subsection, Numbering: RuntimeFrameNumbering{Visible: visible, Format: manifest.Numbering.Format},
		Chrome: append([]ChromeItem(nil), design.Chrome...),
	}, true
}

func ContentHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func SourceHash(manifestRaw []byte, outlineNodeHash string, specRaw, designRaw []byte) string {
	var design Design
	_ = json.Unmarshal(designRaw, &design)
	designRaw = designContentBytes(design)
	size := len(manifestRaw) + len(outlineNodeHash) + len(specRaw) + len(designRaw) + 48
	combined := make([]byte, 0, size)
	for _, item := range []struct {
		name string
		raw  []byte
	}{
		{name: "manifest", raw: manifestRaw},
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

func DesignContentHash(design Design) string {
	return ContentHash(designContentBytes(design))
}

func designContentBytes(design Design) []byte {
	raw, _ := json.Marshal(struct {
		Direction string       `json:"direction"`
		Density   string       `json:"density"`
		Chrome    []ChromeItem `json:"chrome"`
	}{
		Direction: design.Direction, Density: design.Density, Chrome: design.Chrome,
	})
	return raw
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
	currentManifest int, currentOutlineNodeHash string, currentSpec int, currentDesignHash string,
	artifactHash, sourceHash, frameHash string,
) string {
	if !hasHTML {
		return "not_materialized"
	}
	if record == nil {
		return "unknown"
	}
	if record.Artifact.Hash != artifactHash ||
		record.Source.ManifestRevision > currentManifest ||
		record.Source.SpecRevision > currentSpec {
		return "unknown"
	}
	if record.Source.ManifestRevision < currentManifest || record.Source.OutlineNodeHash != currentOutlineNodeHash || record.Source.SpecRevision < currentSpec {
		return "spec_stale"
	}
	if record.Source.DesignContentHash != currentDesignHash {
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

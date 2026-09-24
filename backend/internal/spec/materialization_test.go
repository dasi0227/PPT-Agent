package spec

import (
	"encoding/json"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
)

func TestRuntimeFrameAndMaterializationFreshnessAreIndependent(t *testing.T) {
	deck, outline := validDeck(), validOutline()
	design := Design{SchemaVersion: SchemaVersion, ProjectID: deck.ProjectID, Direction: "minimal", LayoutPreferences: []string{}, Decorations: Decorations{PageNumber: "bottom-right", DeckTitle: "none", SectionTitle: "none", KeyMessage: "none"}, CreatedAt: 1, UpdatedAt: 1}
	cover, ok := BuildRuntimeFrame(deck, outline, design, "sli_aaaaaa", "Opening message", nil)
	if !ok || cover.Canvas != CanonicalCanvas() || cover.Ordinal != 1 || cover.KeyMessage != "Opening message" {
		t.Fatalf("unexpected cover frame: %#v", cover)
	}
	second, _ := BuildRuntimeFrame(deck, outline, design, "sli_bbbbbb", "", nil)
	if second.Canvas != CanonicalCanvas() || second.Ordinal != 2 || second.Total != 3 {
		t.Fatalf("unexpected second frame: %#v", second)
	}

	artifactHash, sourceHash := ContentHash([]byte("html")), ContentHash([]byte("source"))
	record := &MaterializationRecord{SchemaVersion: SchemaVersion, Artifact: MaterializationArtifact{Hash: artifactHash}, Source: MaterializationSource{ManifestHash: ResourceHash(deck), OutlineNodeHash: SemanticSlideNodeHash(outline, "sli_bbbbbb"), SpecHash: ContentHash([]byte("spec")), DesignContentHash: DesignContentHash(design), Hash: sourceHash}, Frame: MaterializationFrame{ContextHash: FrameContextHash(deck, outline, design, "sli_bbbbbb", "", nil)}, RenderedAt: 1}
	if state := DeriveMaterializationState(true, record, record.Source.ManifestHash, record.Source.OutlineNodeHash, record.Source.SpecHash, DesignContentHash(design), artifactHash, sourceHash, record.Frame.ContextHash); state != "fresh" {
		t.Fatalf("state=%s", state)
	}
	reordered := outline
	reordered.Sections[0].Slides = []SlideNode{outline.Sections[0].Slides[1], outline.Sections[0].Slides[0]}
	if SemanticSlideNodeHash(reordered, "sli_bbbbbb") != record.Source.OutlineNodeHash {
		t.Fatal("reorder changed semantic node hash")
	}
	if state := DeriveMaterializationState(true, record, record.Source.ManifestHash, record.Source.OutlineNodeHash, record.Source.SpecHash, DesignContentHash(design), artifactHash, sourceHash, FrameContextHash(deck, reordered, design, "sli_bbbbbb", "", nil)); state != "frame_stale" {
		t.Fatalf("state=%s", state)
	}
}

func TestDesignContentHashIncludesLayoutPreferences(t *testing.T) {
	left := Design{Direction: "clear", LayoutPreferences: []string{}, Decorations: DefaultDecorations()}
	right := left
	right.UpdatedAt = 99
	if DesignContentHash(left) != DesignContentHash(right) {
		t.Fatal("timestamp changed the HTML freshness hash")
	}
	right.LayoutPreferences = []string{"Use fewer cards"}
	if DesignContentHash(left) == DesignContentHash(right) {
		t.Fatal("layout preference did not change the freshness hash")
	}
}

func TestResourceHashIgnoresFormattingAndTimestamps(t *testing.T) {
	first := []byte(`{"key_message":"same","elements":[{"type":"text","intent":"explain"}],"created_at":1,"updated_at":2}`)
	same := []byte(`{ "updated_at":99,"elements": [ { "intent": "explain", "type": "text" } ], "key_message":"same", "created_at":3 }`)
	if ResourceBytesHash(first) != ResourceBytesHash(same) {
		t.Fatal("metadata or formatting changed content identity")
	}
	changed := []byte(`{"key_message":"changed","elements":[{"type":"text","intent":"explain"}],"created_at":1,"updated_at":2}`)
	if ResourceBytesHash(first) == ResourceBytesHash(changed) {
		t.Fatal("business content change was ignored")
	}
	manifest := []byte(`{"title":"Deck","updated_at":1}`)
	design, _ := json.Marshal(Design{Direction: "clear", LayoutPreferences: []string{}, Decorations: DefaultDecorations()})
	if SourceHash(manifest, "node", first, design) != SourceHash([]byte(`{"updated_at":5,"title":"Deck"}`), "node", same, design) {
		t.Fatal("render source changed for metadata-only updates")
	}
	if SourceHash(manifest, "node", first, design) == SourceHash(manifest, "node", changed, design) {
		t.Fatal("render source missed content change")
	}
}

func TestAppearanceChangeInvalidatesFrameWithoutChangingSource(t *testing.T) {
	deck, outline := validDeck(), validOutline()
	design := Design{Direction: "clear", LayoutPreferences: []string{}, Decorations: DefaultDecorations()}
	a := runtimeassets.Appearance("editorial-serif", []byte(":root{--color-bg:#fff;}"))
	b := runtimeassets.Appearance("editorial-serif", []byte(":root{--color-bg:#eee;}"))
	oldFrame := FrameContextHash(deck, outline, design, "sli_bbbbbb", "", a)
	newFrame := FrameContextHash(deck, outline, design, "sli_bbbbbb", "", b)
	record := &MaterializationRecord{Artifact: MaterializationArtifact{Hash: "html"}, Source: MaterializationSource{ManifestHash: "m", OutlineNodeHash: "o", SpecHash: "s", DesignContentHash: "d", Hash: "source"}, Frame: MaterializationFrame{ContextHash: oldFrame}}
	if state := DeriveMaterializationState(true, record, "m", "o", "s", "d", "html", "source", newFrame); state != "frame_stale" {
		t.Fatalf("state=%s", state)
	}
	c := runtimeassets.Appearance("blueprint", []byte(":root{--color-bg:#fff;}"))
	if FrameContextHash(deck, outline, design, "sli_bbbbbb", "", c) == oldFrame {
		t.Fatal("changing the project theme did not change the runtime frame")
	}
}

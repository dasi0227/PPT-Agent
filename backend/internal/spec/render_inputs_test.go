package spec

import (
	"encoding/json"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
)

func TestRuntimeFrameChangesWithOrderWithoutChangingSemanticNode(t *testing.T) {
	deck, outline := validDeck(), validOutline()
	design := Design{Direction: "minimal", LayoutPreferences: []string{}, Decorations: Decorations{PageNumber: "bottom-right", DeckTitle: "none", SectionTitle: "none", KeyMessage: "none"}}
	cover, ok := BuildRuntimeFrame(deck, outline, design, "sli_aaaaaa", SlideSpec{Role: SlideRoleCover, KeyMessage: "Opening message"}, nil)
	if !ok || cover.Canvas != CanonicalCanvas() || cover.Ordinal != 1 || cover.KeyMessage != "Opening message" || cover.Role != "cover" {
		t.Fatalf("unexpected cover frame: %#v", cover)
	}
	second, _ := BuildRuntimeFrame(deck, outline, design, "sli_bbbbbb", SlideSpec{}, nil)
	if second.Canvas != CanonicalCanvas() || second.Ordinal != 2 || second.Total != 3 || second.Role != "" {
		t.Fatalf("unexpected second frame: %#v", second)
	}

	oldNode := SemanticSlideNodeHash(outline, "sli_bbbbbb")
	oldFrame := FrameContextHash(deck, outline, design, "sli_bbbbbb", SlideSpec{}, nil)
	reordered := outline
	reordered.Sections[0].Slides = []SlideNode{outline.Sections[0].Slides[1], outline.Sections[0].Slides[0]}
	if SemanticSlideNodeHash(reordered, "sli_bbbbbb") != oldNode {
		t.Fatal("reorder changed semantic node hash")
	}
	if oldFrame == FrameContextHash(deck, reordered, design, "sli_bbbbbb", SlideSpec{}, nil) {
		t.Fatal("reorder did not invalidate the rendered frame")
	}
}

func TestDesignContentHashIncludesLayoutPreferences(t *testing.T) {
	left := Design{Direction: "clear", LayoutPreferences: []string{}, Decorations: DefaultDecorations()}
	right := left
	right.LayoutPreferences = []string{"Use fewer cards"}
	if DesignContentHash(left) == DesignContentHash(right) {
		t.Fatal("layout preference did not change the freshness hash")
	}
}

func TestResourceHashIgnoresFormattingAndFieldOrder(t *testing.T) {
	first := []byte(`{"key_message":"same","elements":[{"type":"text","intent":"explain"}]}`)
	same := []byte(`{ "elements": [ { "intent": "explain", "type": "text" } ], "key_message":"same" }`)
	if ResourceBytesHash(first) != ResourceBytesHash(same) {
		t.Fatal("field order or formatting changed content identity")
	}
	changed := []byte(`{"key_message":"changed","elements":[{"type":"text","intent":"explain"}]}`)
	if ResourceBytesHash(first) == ResourceBytesHash(changed) {
		t.Fatal("business content change was ignored")
	}
	manifest := []byte(`{"title":"Deck"}`)
	design, _ := json.Marshal(Design{Direction: "clear", LayoutPreferences: []string{}, Decorations: DefaultDecorations()})
	if SourceHash(manifest, "node", first, design) != SourceHash([]byte(`{ "title": "Deck" }`), "node", same, design) {
		t.Fatal("render source changed for formatting-only updates")
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
	oldFrame := FrameContextHash(deck, outline, design, "sli_bbbbbb", SlideSpec{}, a)
	newFrame := FrameContextHash(deck, outline, design, "sli_bbbbbb", SlideSpec{}, b)
	if oldFrame == newFrame {
		t.Fatal("appearance change did not invalidate the rendered frame")
	}
	c := runtimeassets.Appearance("blueprint", []byte(":root{--color-bg:#fff;}"))
	if FrameContextHash(deck, outline, design, "sli_bbbbbb", SlideSpec{}, c) == oldFrame {
		t.Fatal("changing the project theme did not change the runtime frame")
	}
}

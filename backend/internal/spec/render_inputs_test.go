package spec

import (
	"encoding/json"
	"github.com/dasi0227/PPT-Agent/backend/internal/decoration"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
)

func TestRuntimeFrameChangesWithOrderWithoutChangingSemanticNode(t *testing.T) {
	deck, outline := validDeck(), validOutline()
	positions := decoration.DefaultPositions()
	cover, ok := BuildRuntimeFrame(deck, outline, positions, nil, "sli_aaaaaa", SlideSpec{Purpose: "cover", Core: "Opening message"}, nil)
	if !ok || cover.Canvas != CanonicalCanvas() || cover.Ordinal != 1 || cover.Logo != nil || cover.Purpose != "cover" {
		t.Fatalf("unexpected cover frame: %#v", cover)
	}
	second, _ := BuildRuntimeFrame(deck, outline, positions, nil, "sli_bbbbbb", SlideSpec{}, nil)
	if second.Canvas != CanonicalCanvas() || second.Ordinal != 2 || second.Total != 3 || second.Purpose != "" {
		t.Fatalf("unexpected second frame: %#v", second)
	}

	oldNode := SemanticSlideNodeHash(outline, "sli_bbbbbb")
	oldFrame := FrameContextHash(deck, outline, decoration.DefaultPositions(), nil, "sli_bbbbbb", SlideSpec{}, nil)
	reordered := outline
	reordered.Sections[0].Slides = []SlideNode{outline.Sections[0].Slides[1], outline.Sections[0].Slides[0]}
	if SemanticSlideNodeHash(reordered, "sli_bbbbbb") != oldNode {
		t.Fatal("reorder changed semantic node hash")
	}
	if oldFrame == FrameContextHash(deck, reordered, positions, nil, "sli_bbbbbb", SlideSpec{}, nil) {
		t.Fatal("reorder did not invalidate the rendered frame")
	}
}

func TestDesignContentHashIncludesRequirements(t *testing.T) {
	left := Design{Requirements: []string{"clear"}, Prohibitions: []string{}}
	right := left
	right.Requirements = []string{"Use fewer cards"}
	if DesignContentHash(left) == DesignContentHash(right) {
		t.Fatal("design requirement did not change the freshness hash")
	}
}

func TestResourceHashIgnoresFormattingAndFieldOrder(t *testing.T) {
	first := []byte(`{"core":"same","elements":[{"type":"text","intent":"explain"}]}`)
	same := []byte(`{ "elements": [ { "intent": "explain", "type": "text" } ], "core":"same" }`)
	if ResourceBytesHash(first) != ResourceBytesHash(same) {
		t.Fatal("field order or formatting changed content identity")
	}
	changed := []byte(`{"core":"changed","elements":[{"type":"text","intent":"explain"}]}`)
	if ResourceBytesHash(first) == ResourceBytesHash(changed) {
		t.Fatal("business content change was ignored")
	}
	manifest := []byte(`{"title":"Deck"}`)
	design, _ := json.Marshal(Design{Requirements: []string{"clear"}, Prohibitions: []string{}})
	if SourceHash(manifest, "node", first, design) != SourceHash([]byte(`{ "title": "Deck" }`), "node", same, design) {
		t.Fatal("render source changed for formatting-only updates")
	}
	if SourceHash(manifest, "node", first, design) == SourceHash(manifest, "node", changed, design) {
		t.Fatal("render source missed content change")
	}
}

func TestAppearanceChangeInvalidatesFrameWithoutChangingSource(t *testing.T) {
	deck, outline := validDeck(), validOutline()
	a := runtimeassets.Appearance("editorial-serif", []byte(":root{--color-bg:#fff;}"))
	b := runtimeassets.Appearance("editorial-serif", []byte(":root{--color-bg:#eee;}"))
	oldFrame := FrameContextHash(deck, outline, decoration.DefaultPositions(), nil, "sli_bbbbbb", SlideSpec{}, a)
	newFrame := FrameContextHash(deck, outline, decoration.DefaultPositions(), nil, "sli_bbbbbb", SlideSpec{}, b)
	if oldFrame == newFrame {
		t.Fatal("appearance change did not invalidate the rendered frame")
	}
	c := runtimeassets.Appearance("blueprint", []byte(":root{--color-bg:#fff;}"))
	if FrameContextHash(deck, outline, decoration.DefaultPositions(), nil, "sli_bbbbbb", SlideSpec{}, c) == oldFrame {
		t.Fatal("changing the project theme did not change the runtime frame")
	}
}

func TestFrameHashUsesOnlyVisibleDecorationContent(t *testing.T) {
	deck, outline := validDeck(), validOutline()
	positions := decoration.DefaultPositions()
	frame, _ := BuildRuntimeFrame(deck, outline, positions, nil, "sli_aaaaaa", SlideSpec{Core: "First"}, nil)
	baseline := RuntimeFrameHash(frame)
	frame.DeckTitle = "Hidden title changed"
	frame.Purpose = "conclusion"
	if RuntimeFrameHash(frame) != baseline {
		t.Fatal("nonvisual content invalidated pixels")
	}
	frame.Decorations.DeckTitle = "top-center"
	if RuntimeFrameHash(frame) == baseline {
		t.Fatal("visible title was ignored")
	}
	frame.Decorations.DeckTitle = "none"
	frame.Decorations.Logo = "bottom-center"
	frame.Logo = &decoration.Image{Src: "/api/logo", Hash: "first-logo"}
	withLogo := RuntimeFrameHash(frame)
	frame.Logo.Src = "data:image/png;base64,frozen"
	if RuntimeFrameHash(frame) != withLogo {
		t.Fatal("transport URL changed image identity")
	}
	frame.Logo.Hash = "second-logo"
	if RuntimeFrameHash(frame) == withLogo {
		t.Fatal("logo replacement was ignored")
	}
}

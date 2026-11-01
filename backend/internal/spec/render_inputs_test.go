package spec

import (
	"encoding/json"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/decoration"
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

func TestSourceHashIncludesBothConstraintDomains(t *testing.T) {
	left := DefaultConstraints()
	leftRaw, _ := json.Marshal(left)
	for _, domain := range []string{"content", "visual"} {
		right := left
		if domain == "content" {
			right.Content.Requirements = []string{"Include examples"}
		} else {
			right.Visual.Requirements = []string{"Use fewer cards"}
		}
		rightRaw, _ := json.Marshal(right)
		if SourceHash(nil, "node", nil, leftRaw) == SourceHash(nil, "node", nil, rightRaw) {
			t.Fatalf("%s requirement did not change the freshness hash", domain)
		}
	}
}

func TestResourceHashIgnoresFormattingAndFieldOrder(t *testing.T) {
	first := []byte(`{"purpose":"content_explanation","layout":"Title above content","core":"same","elements":[{"type":"text","intent":"explain"}]}`)
	same := []byte(`{"purpose":"content_explanation","layout":"Title above content", "elements": [ { "intent": "explain", "type": "text" } ], "core":"same" }`)
	if ResourceBytesHash(first) != ResourceBytesHash(same) {
		t.Fatal("field order or formatting changed content identity")
	}
	changed := []byte(`{"purpose":"content_explanation","layout":"Title above content","core":"changed","elements":[{"type":"text","intent":"explain"}]}`)
	if ResourceBytesHash(first) == ResourceBytesHash(changed) {
		t.Fatal("business content change was ignored")
	}
	meta := []byte(`{"title":"Deck"}`)
	constraints, _ := json.Marshal(Constraints{Content: ConstraintSet{Requirements: []string{}, Prohibitions: []string{}}, Visual: ConstraintSet{Requirements: []string{"clear"}, Prohibitions: []string{}}})
	if SourceHash(meta, "node", first, constraints) != SourceHash([]byte(`{ "title": "Deck" }`), "node", same, constraints) {
		t.Fatal("render source changed for formatting-only updates")
	}
	if SourceHash(meta, "node", first, constraints) == SourceHash(meta, "node", changed, constraints) {
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

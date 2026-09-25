package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func validDeck() Manifest {
	return Manifest{Title: "Manifest", Goal: "Explain", Audience: "Builders", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}}
}
func validOutline() Outline {
	return Outline{Sections: []Section{
		{ID: "sec_aaaaaa", Title: "Direct", Purpose: "Open", Slides: []SlideNode{{SlideID: "sli_aaaaaa", Title: "Cover", Role: "cover"}, {SlideID: "sli_bbbbbb", Title: "Agenda", Role: "agenda"}}, Subsections: []Subsection{}},
		{ID: "sec_bbbbbb", Title: "Grouped", Purpose: "Explain", Slides: []SlideNode{}, Subsections: []Subsection{{ID: "sub_aaaaaa", Title: "Part", Purpose: "Develop the argument", Slides: []SlideNode{{SlideID: "sli_cccccc", Title: "Body", Role: "content"}}}}},
	}}
}

func TestDeckAndTreeOutlineValidation(t *testing.T) {
	if err := ValidateManifest(validDeck()); err != nil {
		t.Fatal(err)
	}
	outline := validOutline()
	if err := ValidateOutline(outline); err != nil {
		t.Fatal(err)
	}
	got := FlattenOutline(outline)
	if len(got) != 3 || got[0].Slide.SlideID != "sli_aaaaaa" || got[2].Ordinal != 3 || got[2].Subsection == nil {
		t.Fatalf("unexpected flatten: %#v", got)
	}
	if ordinal, ok := ResolveSlideOrdinal(outline, "sli_bbbbbb"); !ok || ordinal != 2 {
		t.Fatalf("ordinal=%d ok=%v", ordinal, ok)
	}
	outline.Sections[0].Subsections = []Subsection{{ID: "sub_mixed1", Title: "Invalid", Slides: []SlideNode{}}}
	if err := ValidateOutline(outline); err == nil {
		t.Fatal("mixed direct/grouped section must fail")
	}
}

func TestOutlineRejectsUnknownSlideRole(t *testing.T) {
	outline := validOutline()
	outline.Sections[0].Slides[0].Role = "ending"

	if err := ValidateOutline(outline); err == nil {
		t.Fatal("unknown slide role was accepted")
	}
}

func TestSlideSpecHasNoPlacementContract(t *testing.T) {
	valid := SlideSpec{KeyMessage: "Message", Elements: []Element{{Type: "text", Intent: "Explain"}}, Layout: "hero"}
	if err := ValidateSlideSpec(valid); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(valid)
	for _, forbidden := range []string{"project_id", "slide_id", "section" + "_id", "subsection" + "_id", `"role"`, `"title"`, `"version"`, `"created_at"`, `"updated_at"`} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("persisted spec contains %s", forbidden)
		}
	}
}

func TestDesignDecorationsRequireFixedSlotsAndVisiblePageNumber(t *testing.T) {
	design := Design{Direction: "", LayoutPreferences: []string{}, Decorations: DefaultDecorations()}
	if err := ValidateDesign(design); err != nil {
		t.Fatal(err)
	}
	design.Decorations.PageNumber = "none"
	if err := ValidateDesign(design); err == nil {
		t.Fatal("page number cannot be hidden")
	}
	design.Decorations = DefaultDecorations()
	design.Decorations.SectionTitle = ""
	if err := ValidateDesign(design); err == nil {
		t.Fatal("decoration slots require an explicit placement")
	}
	design.Decorations = DefaultDecorations()
	design.Decorations.KeyMessage = "bottom-center"
	if err := ValidateDesign(design); err != nil {
		t.Fatal(err)
	}
	design.LayoutPreferences = []string{"Use fewer cards", "Prefer spacious alignment"}
	if err := ValidateDesign(design); err != nil {
		t.Fatal(err)
	}
}

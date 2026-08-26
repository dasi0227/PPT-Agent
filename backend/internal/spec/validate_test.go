package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func validDeck() Deck {
	return Deck{SchemaVersion: SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", Title: "Deck", Goal: "Explain", Audience: "Builders", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}, Canvas: CanvasSettings{AspectRatio: "16:9"}, Numbering: NumberingPolicy{Enabled: true, HiddenRoles: []string{"cover"}, Format: "number"}, CreatedAt: 1, UpdatedAt: 1}
}
func validOutline() Outline {
	return Outline{SchemaVersion: SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", CreatedAt: 1, UpdatedAt: 1, Sections: []Section{
		{ID: "sec_aaaaaa", Title: "Direct", Purpose: "Open", Slides: []SlideNode{{SlideID: "sli_aaaaaa", Label: "Cover", Role: "cover"}, {SlideID: "sli_bbbbbb", Label: "Agenda", Role: "agenda"}}, Subsections: []Subsection{}},
		{ID: "sec_bbbbbb", Title: "Grouped", Purpose: "Explain", Slides: []SlideNode{}, Subsections: []Subsection{{ID: "sub_aaaaaa", Title: "Part", Slides: []SlideNode{{SlideID: "sli_cccccc", Label: "Body", Role: "content"}}}}},
	}}
}

func TestDeckAndTreeOutlineValidation(t *testing.T) {
	if err := ValidateDeck(validDeck()); err != nil {
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

func TestSlideSpecHasNoPlacementContract(t *testing.T) {
	valid := SlideSpec{SchemaVersion: SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", SlideID: "sli_aaaaaa", Title: "Title", KeyMessage: "Message", Elements: []Element{{Type: "text", Intent: "Explain"}}, Layout: "hero", CreatedAt: 1, UpdatedAt: 1}
	if err := ValidateSlideSpec(valid); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(valid)
	for _, forbidden := range []string{"section" + "_id", "subsection" + "_id", `"role"`} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("persisted spec contains %s", forbidden)
		}
	}
}

package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func validDeck() Manifest {
	return Manifest{Title: "Manifest", Goal: "Explain", Audience: "Builders", Language: "zh-CN", Pages: "待明确", Requirements: []string{}, Prohibitions: []string{}}
}
func validOutline() Outline {
	return Outline{Sections: []Section{
		{ID: "sec_aaaaaa", Title: "Direct", Purpose: "Open", Slides: []SlideNode{{ID: "sli_aaaaaa", Title: "Cover"}, {ID: "sli_bbbbbb", Title: "Agenda"}}, Subsections: []Subsection{}},
		{ID: "sec_bbbbbb", Title: "Grouped", Purpose: "Explain", Slides: []SlideNode{}, Subsections: []Subsection{{ID: "sub_aaaaaa", Title: "Part", Purpose: "Develop the argument", Slides: []SlideNode{{ID: "sli_cccccc", Title: "Body"}}}}},
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
	if len(got) != 3 || got[0].Slide.ID != "sli_aaaaaa" || got[2].Ordinal != 3 || got[2].Subsection == nil {
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

func TestSlideSpecPurposeIsOptionalAndValidated(t *testing.T) {
	slide := SlideSpec{Core: "Message", Elements: []Element{}}
	if err := ValidateSlideSpec(slide); err != nil {
		t.Fatal(err)
	}
	for _, purpose := range []SlidePurpose{"cover", "introduction", "transition", "content", "conclusion", "other"} {
		slide.Purpose = purpose
		if err := ValidateSlideSpec(slide); err != nil {
			t.Fatalf("purpose %q: %v", purpose, err)
		}
	}
	slide.Purpose = "ending"
	if err := ValidateSlideSpec(slide); err == nil {
		t.Fatal("unknown slide purpose was accepted")
	}
}

func TestSlideSpecHasNoPlacementContract(t *testing.T) {
	valid := SlideSpec{Core: "Message", Elements: []Element{{Type: "text", Intent: "Explain"}}, Layout: "hero"}
	if err := ValidateSlideSpec(valid); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(valid)
	for _, forbidden := range []string{"project_id", "slide_id", "section" + "_id", "subsection" + "_id", `"title"`, `"version"`, `"created_at"`, `"updated_at"`} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("persisted spec contains %s", forbidden)
		}
	}
}

func TestDesignContainsOnlyVisualRequirementsAndProhibitions(t *testing.T) {
	design := DefaultDesign()
	design.Requirements = []string{"Prefer spacious alignment"}
	design.Prohibitions = []string{"Avoid gradients"}
	if err := ValidateDesign(design); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(design)
	if _, err := ParseStrictSourceJSON(raw, "design"); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"decorations", "direction", "layout_preferences"} {
		var source map[string]any
		_ = json.Unmarshal(raw, &source)
		source[field] = "unsupported field"
		invalid, _ := json.Marshal(source)
		if _, err := ParseStrictSourceJSON(invalid, "design"); err == nil {
			t.Fatalf("unsupported field %s was accepted", field)
		}
	}
	design.Prohibitions = nil
	if err := ValidateDesign(design); err == nil {
		t.Fatal("a missing list must not be accepted")
	}
}

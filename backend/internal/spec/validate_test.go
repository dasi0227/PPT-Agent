package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func validDeck() Meta {
	return Meta{Title: "Meta", Goal: "Explain", Audience: "Builders", Language: "zh-CN", Count: "待明确"}
}
func validOutline() Outline {
	return Outline{Sections: []Section{
		{ID: "sec_aaaaaa", Title: "Direct", Slides: []SlideNode{{ID: "sli_aaaaaa", Title: "Cover"}, {ID: "sli_bbbbbb", Title: "Agenda"}}, Subsections: []Subsection{}},
		{ID: "sec_bbbbbb", Title: "Grouped", Slides: []SlideNode{}, Subsections: []Subsection{{ID: "sec_cccccc", Title: "Part", Slides: []SlideNode{{ID: "sli_cccccc", Title: "Body"}}}}},
	}}
}

func TestDeckAndTreeOutlineValidation(t *testing.T) {
	if err := ValidateMeta(validDeck()); err != nil {
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
	outline.Sections[0].Subsections = []Subsection{{ID: "sec_mixed1", Title: "Invalid", Slides: []SlideNode{}}}
	if err := ValidateOutline(outline); err == nil {
		t.Fatal("mixed direct/grouped section must fail")
	}
}

func TestSlideSpecCoreAndPurposeConstraints(t *testing.T) {
	slide := SlideSpec{Core: "Message", Purpose: "content_explanation", Layout: "Title above content", Elements: []Element{}}
	if err := ValidateSlideSpec(slide); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"core", "purpose", "layout"} {
		for _, value := range []any{nil, ""} {
			raw, _ := json.Marshal(slide)
			var source map[string]any
			_ = json.Unmarshal(raw, &source)
			source[field] = value
			raw, _ = json.Marshal(source)
			if _, err := ParseStrictSourceJSON(raw, "spec"); err == nil {
				t.Fatalf("accepted %s=%v", field, value)
			}
			delete(source, field)
			raw, _ = json.Marshal(source)
			if _, err := ParseStrictSourceJSON(raw, "spec"); err == nil {
				t.Fatalf("accepted missing %s", field)
			}
		}
	}
	for _, purpose := range []SlidePurpose{"cover", "introduction", "transition", "content_explanation", "content_comparison", "content_example", "content_guidance", "content_other", "conclusion", "other"} {
		slide.Purpose = purpose
		if err := ValidateSlideSpec(slide); err != nil {
			t.Fatalf("purpose %q: %v", purpose, err)
		}
	}
	for _, purpose := range []SlidePurpose{"ending", "content", "content_else"} {
		slide.Purpose = purpose
		if err := ValidateSlideSpec(slide); err == nil {
			t.Fatalf("unsupported slide purpose %q was accepted", purpose)
		}
	}
	for _, source := range []string{
		`{"core":"Message","layout":"Title above content","elements":[],"purpose":"content","approach":"explanation"}`,
		`{"core":"Message","layout":"Title above content","elements":[],"purpose":"content_explanation","approach":"explanation"}`,
	} {
		if _, err := ParseStrictSourceJSON([]byte(source), "spec"); err == nil {
			t.Fatalf("removed approach field was accepted: %s", source)
		}
	}
}

func TestSlideSpecHasNoPlacementContract(t *testing.T) {
	valid := SlideSpec{Purpose: "content_explanation", Core: "Message", Elements: []Element{{Type: "text", Intent: "Explain"}}, Layout: "hero"}
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

func TestConstraintsContainContentAndVisualRequirementsAndProhibitions(t *testing.T) {
	constraints := DefaultConstraints()
	constraints.Visual.Requirements = []string{"Prefer spacious alignment"}
	constraints.Visual.Prohibitions = []string{"Avoid gradients"}
	raw, _ := json.Marshal(constraints)
	if _, err := ParseStrictSourceJSON(raw, "constraints"); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"decorations", "direction", "layout_preferences"} {
		var source map[string]any
		_ = json.Unmarshal(raw, &source)
		source[field] = "unsupported field"
		invalid, _ := json.Marshal(source)
		if _, err := ParseStrictSourceJSON(invalid, "constraints"); err == nil {
			t.Fatalf("unsupported field %s was accepted", field)
		}
	}
	constraints.Visual.Prohibitions = nil
	raw, _ = json.Marshal(constraints)
	if _, err := ParseStrictSourceJSON(raw, "constraints"); err == nil {
		t.Fatal("a missing list must not be accepted")
	}
}

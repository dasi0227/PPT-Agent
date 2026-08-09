package spec

import "testing"

func validSlide(id string) SlideSpec {
	return SlideSpec{
		SchemaVersion: SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", SlideID: id,
		SectionID: "sec_aaaaaa",
		Role:      "evidence", Title: "Title", KeyMessage: "Message",
		Elements: []Element{{Type: "chart", Intent: "Show growth"}},
		Layout:   "two-column", CreatedAt: 1, UpdatedAt: 1,
	}
}

func TestValidateMaterialization(t *testing.T) {
	record := MaterializationRecord{
		SchemaVersion: SchemaVersion,
		Artifact: MaterializationArtifact{
			Revision: 1,
			Hash:     "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		Source: MaterializationSource{
			Outline: 1,
			Spec:    1,
			Design:  1,
			Hash:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
		RenderedAt: 1,
	}
	if err := ValidateMaterialization(record); err != nil {
		t.Fatalf("valid materialization rejected: %v", err)
	}
	record.Artifact.Hash = "bad"
	if err := ValidateMaterialization(record); err == nil {
		t.Fatal("invalid materialization hash should fail")
	}
}

func TestValidateOutlineReferences(t *testing.T) {
	outline := Outline{
		SchemaVersion: SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", Title: "Deck",
		Goal: "goal", Audience: "audience", Language: "en-US",
		Constraints: Constraints{MustInclude: []string{}, MustAvoid: []string{}, StyleLimits: []string{}, ContentLimits: []string{}},
		CreatedAt:   1, UpdatedAt: 1,
		Sections:   []Section{{ID: "sec_aaaaaa", Title: "Main", Purpose: "Introduce the main section", Subsections: []Subsection{}}},
		SlideOrder: []string{"sli_aaaaaa"},
	}
	if err := ValidateOutline(outline, map[string]SlideSpec{"sli_aaaaaa": validSlide("sli_aaaaaa")}); err != nil {
		t.Fatalf("valid outline rejected: %v", err)
	}
	if err := ValidateOutline(outline, map[string]SlideSpec{}); err == nil {
		t.Fatal("missing slide reference should fail")
	}
	outline.SlideOrder = []string{"sli_aaaaaa", "sli_aaaaaa"}
	if err := ValidateOutline(outline, map[string]SlideSpec{"sli_aaaaaa": validSlide("sli_aaaaaa")}); err == nil {
		t.Fatal("duplicate stable id should fail")
	}
}

func TestValidateOutlineRejectsSubsectionFromAnotherSection(t *testing.T) {
	outline := Outline{
		SchemaVersion: SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", Title: "Deck",
		Goal: "goal", Audience: "audience", Language: "en-US",
		Constraints: Constraints{MustInclude: []string{}, MustAvoid: []string{}, StyleLimits: []string{}, ContentLimits: []string{}},
		CreatedAt:   1, UpdatedAt: 1,
		Sections: []Section{
			{ID: "sec_aaaaaa", Title: "One", Purpose: "First section", Subsections: []Subsection{{ID: "sub_aaaaaa", Title: "Sub"}}},
			{ID: "sec_bbbbbb", Title: "Two", Purpose: "Second section", Subsections: []Subsection{}},
		},
		SlideOrder: []string{"sli_aaaaaa"},
	}
	slide := validSlide("sli_aaaaaa")
	slide.SectionID = "sec_bbbbbb"
	slide.SubsectionID = "sub_aaaaaa"
	if err := ValidateOutline(outline, map[string]SlideSpec{"sli_aaaaaa": slide}); err == nil {
		t.Fatal("subsection from another section should fail")
	}
}

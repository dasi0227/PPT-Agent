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
		Requirements: []string{}, Prohibitions: []string{},
		CreatedAt: 1, UpdatedAt: 1,
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
		Requirements: []string{}, Prohibitions: []string{},
		CreatedAt: 1, UpdatedAt: 1,
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

// TestValidatePlacementStrictTwoLevel exercises the strict two-level mounting
// rule for one slide against a fixed outline where sec_grp is grouped (owns
// sub_x) and sec_dir is direct (no subsections).
func TestValidatePlacementStrictTwoLevel(t *testing.T) {
	index := BuildSectionIndex([]Section{
		{ID: "sec_grp", Title: "Grouped", Purpose: "p", Subsections: []Subsection{{ID: "sub_x", Title: "X"}}},
		{ID: "sec_dir", Title: "Direct", Purpose: "p", Subsections: []Subsection{}},
	})
	cases := []struct {
		name       string
		sectionID  string
		subID      string
		wantReject bool
	}{
		{"grouped page with owned subsection", "sec_grp", "sub_x", false},
		{"grouped page without subsection", "sec_grp", "", true},
		{"grouped page with foreign subsection", "sec_grp", "sub_missing", true},
		{"direct page without subsection", "sec_dir", "", false},
		{"direct page with subsection", "sec_dir", "sub_x", true},
		{"unknown section", "sec_none", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := index.ValidatePlacement(tc.sectionID, tc.subID)
			if tc.wantReject && err == nil {
				t.Fatalf("expected rejection for %s/%s", tc.sectionID, tc.subID)
			}
			if !tc.wantReject && err != nil {
				t.Fatalf("unexpected rejection for %s/%s: %v", tc.sectionID, tc.subID, err)
			}
		})
	}
}

// TestValidateOutlineRejectsGroupedSectionDirectPage guards the outline-level
// mirror of the rule: a grouped section may not hold a page that omits its
// subsection_id.
func TestValidateOutlineRejectsGroupedSectionDirectPage(t *testing.T) {
	outline := Outline{
		SchemaVersion: SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", Title: "Deck",
		Goal: "goal", Audience: "audience", Language: "en-US",
		Requirements: []string{}, Prohibitions: []string{},
		CreatedAt: 1, UpdatedAt: 1,
		Sections:   []Section{{ID: "sec_aaaaaa", Title: "One", Purpose: "First section", Subsections: []Subsection{{ID: "sub_aaaaaa", Title: "Sub"}}}},
		SlideOrder: []string{"sli_aaaaaa"},
	}
	slide := validSlide("sli_aaaaaa") // SectionID sec_aaaaaa, no subsection_id
	if err := ValidateOutline(outline, map[string]SlideSpec{"sli_aaaaaa": slide}); err == nil {
		t.Fatal("grouped section with a direct page should fail")
	}
}

package spec

import "testing"

func validSlide(id string) SlideSpec {
	return SlideSpec{
		SchemaVersion: SchemaVersion, Revision: 1, ProjectID: "p", SlideID: id,
		SourceOutlineRevision: 1, SectionID: "s",
		Role: "evidence", Title: "Title", KeyMessage: "Message",
		Content:      Content{Summary: "Summary", Points: []string{}},
		VisualIntent: VisualIntent{Archetype: "data-story", Description: "Chart", AssetQueries: []string{}},
		SpeakerNotes: "", CreatedAt: 1, UpdatedAt: 1,
	}
}

func TestValidateOutlineReferences(t *testing.T) {
	outline := Outline{
		SchemaVersion: SchemaVersion, Revision: 1, ProjectID: "p", Title: "Deck",
		Goal: "goal", Audience: "audience", Language: "en-US", CoreThesis: "thesis",
		NarrativeArc: "arc", CreatedAt: 1, UpdatedAt: 1,
		Sections:   []Section{{ID: "s", Number: "01", Title: "Main", Subsections: []Subsection{}}},
		SlideOrder: []string{"stable"},
	}
	if err := ValidateOutline(outline, map[string]SlideSpec{"stable": validSlide("stable")}); err != nil {
		t.Fatalf("valid outline rejected: %v", err)
	}
	if err := ValidateOutline(outline, map[string]SlideSpec{}); err == nil {
		t.Fatal("missing slide reference should fail")
	}
	outline.SlideOrder = []string{"stable", "stable"}
	if err := ValidateOutline(outline, map[string]SlideSpec{"stable": validSlide("stable")}); err == nil {
		t.Fatal("duplicate stable id should fail")
	}
}

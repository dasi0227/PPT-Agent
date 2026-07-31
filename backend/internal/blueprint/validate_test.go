package blueprint

import "testing"

func validSlide(id string) Slide {
	return Slide{
		SchemaVersion: SchemaVersion, Revision: 1, SlideID: id, SectionID: "s",
		Role: "evidence", Title: "Title", KeyMessage: "Message",
		Content:      Content{Summary: "Summary", Points: []string{}},
		VisualIntent: VisualIntent{Archetype: "data-story", Description: "Chart", AssetQueries: []string{}},
	}
}

func TestValidateDeckReferences(t *testing.T) {
	deck := Deck{
		SchemaVersion: SchemaVersion, Revision: 1, ProjectID: "p", Title: "Deck",
		Sections:   []Section{{ID: "s", Number: "01", Title: "Main", Subsections: []Subsection{}}},
		SlideOrder: []string{"stable"},
	}
	if err := ValidateDeck(deck, map[string]Slide{"stable": validSlide("stable")}); err != nil {
		t.Fatalf("valid deck rejected: %v", err)
	}
	if err := ValidateDeck(deck, map[string]Slide{}); err == nil {
		t.Fatal("missing slide reference should fail")
	}
	deck.SlideOrder = []string{"stable", "stable"}
	if err := ValidateDeck(deck, map[string]Slide{"stable": validSlide("stable")}); err == nil {
		t.Fatal("duplicate stable id should fail")
	}
}

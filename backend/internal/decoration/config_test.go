package decoration

import "testing"

func TestHumanPositionsEnforceSlotsAndAllowAtomicSwap(t *testing.T) {
	positions := DefaultPositions()
	if err := positions.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := positions.Merge(map[string]string{"slide_pagination": "none"}); err == nil {
		t.Fatal("page number was hidden")
	}
	if _, err := positions.Merge(map[string]string{"slide_pagination": "top-left"}); err == nil {
		t.Fatal("occupied position was accepted")
	}
	if _, err := positions.Merge(map[string]string{"logo": ""}); err == nil {
		t.Fatal("empty placement was accepted")
	}
	merged, err := positions.Merge(map[string]string{"slide_pagination": "top-left", "section_title": "bottom-right"})
	if err != nil {
		t.Fatal(err)
	}
	if merged.SlidePagination != "top-left" || merged.SectionTitle != "bottom-right" || merged.Logo != "none" {
		t.Fatalf("unexpected positions: %+v", merged)
	}
	if positions != DefaultPositions() {
		t.Fatal("merging changed the original configuration")
	}
}

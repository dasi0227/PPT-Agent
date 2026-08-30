package spec

import "testing"

func TestRuntimeFrameAndMaterializationFreshnessAreIndependent(t *testing.T) {
	deck, outline := validDeck(), validOutline()
	design := Design{SchemaVersion: SchemaVersion, Revision: 1, ProjectID: deck.ProjectID, Theme: "clean", Direction: "minimal", Density: "medium", Chrome: []ChromeItem{{Type: "page_number", Placement: "bottom-right", Style: "tiny muted mono"}}, CreatedAt: 1, UpdatedAt: 1}
	cover, ok := BuildRuntimeFrame(deck, outline, design, "sli_aaaaaa")
	if !ok || cover.Numbering.Visible || cover.Ordinal != 1 {
		t.Fatalf("unexpected cover frame: %#v", cover)
	}
	second, _ := BuildRuntimeFrame(deck, outline, design, "sli_bbbbbb")
	if !second.Numbering.Visible || second.Ordinal != 2 || second.Total != 3 {
		t.Fatalf("unexpected second frame: %#v", second)
	}

	artifactHash, sourceHash := ContentHash([]byte("html")), ContentHash([]byte("source"))
	record := &MaterializationRecord{SchemaVersion: SchemaVersion, Artifact: MaterializationArtifact{Revision: 1, Hash: artifactHash}, Source: MaterializationSource{ManifestRevision: 1, OutlineNodeHash: SemanticSlideNodeHash(outline, "sli_bbbbbb"), SpecRevision: 1, DesignContentHash: DesignContentHash(design), Hash: sourceHash}, Frame: MaterializationFrame{ContextHash: FrameContextHash(deck, outline, design, "sli_bbbbbb")}, RenderedAt: 1}
	if state := DeriveMaterializationState(true, record, 1, record.Source.OutlineNodeHash, 1, DesignContentHash(design), artifactHash, sourceHash, record.Frame.ContextHash); state != "fresh" {
		t.Fatalf("state=%s", state)
	}
	reordered := outline
	reordered.Sections[0].Slides = []SlideNode{outline.Sections[0].Slides[1], outline.Sections[0].Slides[0]}
	if SemanticSlideNodeHash(reordered, "sli_bbbbbb") != record.Source.OutlineNodeHash {
		t.Fatal("reorder changed semantic node hash")
	}
	if state := DeriveMaterializationState(true, record, 1, record.Source.OutlineNodeHash, 1, DesignContentHash(design), artifactHash, sourceHash, FrameContextHash(deck, reordered, design, "sli_bbbbbb")); state != "frame_stale" {
		t.Fatalf("state=%s", state)
	}
}

func TestDesignContentHashExcludesTheme(t *testing.T) {
	left := Design{Theme: "swiss-modern", Direction: "clear", Density: "medium", Chrome: []ChromeItem{}}
	right := left
	right.Theme = "tokyo-night"
	right.Revision = 99
	if DesignContentHash(left) != DesignContentHash(right) {
		t.Fatal("theme or revision changed the HTML freshness hash")
	}
	right.Direction = "editorial"
	if DesignContentHash(left) == DesignContentHash(right) {
		t.Fatal("agent-owned design content did not change the freshness hash")
	}
}

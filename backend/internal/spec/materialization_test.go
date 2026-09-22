package spec

import "testing"

func TestRuntimeFrameAndMaterializationFreshnessAreIndependent(t *testing.T) {
	deck, outline := validDeck(), validOutline()
	design := Design{SchemaVersion: SchemaVersion, ProjectID: deck.ProjectID, Theme: "clean", Direction: "minimal", Chrome: []ChromeItem{{Type: "page_number", Placement: "bottom-right", Style: "tiny muted mono"}}, CreatedAt: 1, UpdatedAt: 1}
	cover, ok := BuildRuntimeFrame(deck, outline, design, "sli_aaaaaa")
	if !ok || cover.Canvas != CanonicalCanvas() || cover.Numbering.Visible || cover.Ordinal != 1 {
		t.Fatalf("unexpected cover frame: %#v", cover)
	}
	second, _ := BuildRuntimeFrame(deck, outline, design, "sli_bbbbbb")
	if !second.Numbering.Visible || second.Ordinal != 2 || second.Total != 3 {
		t.Fatalf("unexpected second frame: %#v", second)
	}

	artifactHash, sourceHash := ContentHash([]byte("html")), ContentHash([]byte("source"))
	record := &MaterializationRecord{SchemaVersion: SchemaVersion, Artifact: MaterializationArtifact{Hash: artifactHash}, Source: MaterializationSource{ManifestHash: ResourceHash(deck), OutlineNodeHash: SemanticSlideNodeHash(outline, "sli_bbbbbb"), SpecHash: ContentHash([]byte("spec")), DesignContentHash: DesignContentHash(design), Hash: sourceHash}, Frame: MaterializationFrame{ContextHash: FrameContextHash(deck, outline, design, "sli_bbbbbb")}, RenderedAt: 1}
	if state := DeriveMaterializationState(true, record, record.Source.ManifestHash, record.Source.OutlineNodeHash, record.Source.SpecHash, DesignContentHash(design), artifactHash, sourceHash, record.Frame.ContextHash); state != "fresh" {
		t.Fatalf("state=%s", state)
	}
	reordered := outline
	reordered.Sections[0].Slides = []SlideNode{outline.Sections[0].Slides[1], outline.Sections[0].Slides[0]}
	if SemanticSlideNodeHash(reordered, "sli_bbbbbb") != record.Source.OutlineNodeHash {
		t.Fatal("reorder changed semantic node hash")
	}
	if state := DeriveMaterializationState(true, record, record.Source.ManifestHash, record.Source.OutlineNodeHash, record.Source.SpecHash, DesignContentHash(design), artifactHash, sourceHash, FrameContextHash(deck, reordered, design, "sli_bbbbbb")); state != "frame_stale" {
		t.Fatalf("state=%s", state)
	}
}

func TestDesignContentHashExcludesTheme(t *testing.T) {
	left := Design{Theme: "swiss-modern", Direction: "clear", Chrome: []ChromeItem{}}
	right := left
	right.Theme = "tokyo-night"
	right.UpdatedAt = 99
	if DesignContentHash(left) != DesignContentHash(right) {
		t.Fatal("theme or timestamp changed the HTML freshness hash")
	}
	right.Direction = "editorial"
	if DesignContentHash(left) == DesignContentHash(right) {
		t.Fatal("agent-owned design content did not change the freshness hash")
	}
}

func TestResourceHashIgnoresFormattingAndTimestamps(t *testing.T) {
	first := []byte(`{"key_message":"same","elements":[{"type":"text","intent":"explain"}],"created_at":1,"updated_at":2}`)
	same := []byte(`{ "updated_at":99,"elements": [ { "intent": "explain", "type": "text" } ], "key_message":"same", "created_at":3 }`)
	if ResourceBytesHash(first) != ResourceBytesHash(same) {
		t.Fatal("metadata or formatting changed content identity")
	}
	changed := []byte(`{"key_message":"changed","elements":[{"type":"text","intent":"explain"}],"created_at":1,"updated_at":2}`)
	if ResourceBytesHash(first) == ResourceBytesHash(changed) {
		t.Fatal("business content change was ignored")
	}
	manifest := []byte(`{"title":"Deck","updated_at":1}`)
	design := []byte(`{"direction":"clear","chrome":[]}`)
	if SourceHash(manifest, "node", first, design) != SourceHash([]byte(`{"updated_at":5,"title":"Deck"}`), "node", same, design) {
		t.Fatal("render source changed for metadata-only updates")
	}
	if SourceHash(manifest, "node", first, design) == SourceHash(manifest, "node", changed, design) {
		t.Fatal("render source missed content change")
	}
}

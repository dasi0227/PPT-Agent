package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func generationFixture() *GenerationInputs {
	return &GenerationInputs{Manifest: validDeck(), Design: Design{Demands: []string{"A", "grid", "space"}, Decorations: DefaultDecorations()}, Spec: SlideSpec{Core: "Complete message", Elements: []Element{}}}
}

func TestGenerationInputsNetFieldChanges(t *testing.T) {
	before := generationFixture()
	after := before.Clone()
	after.Manifest.Goal = "New goal with full text"
	after.Design.Decorations.SectionTitle = "none"
	after.Design.Demands = []string{"space", "grid"}
	after.Spec.Layout = "two-column"
	after.Spec.Purpose = SlidePurposeContent
	after.Spec.ContentType = SlideContentTypeExplanation
	diff := DiffGenerationInputs(before, after)
	raw, _ := json.Marshal(diff)
	for _, want := range []string{`"/goal":{"op":"replace","old":"Explain","new":"New goal with full text"}`, `"/decorations/section_title":{"op":"replace","old":"top-left","new":"none"}`, `"/demands":{"op":"replace","old":["A","grid","space"],"new":["space","grid"]}`, `"/layout":{"op":"add","new":"two-column"}`, `"/content_type":{"op":"add","new":"explanation"}`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("missing %s in %s", want, raw)
		}
	}
	removed := DiffGenerationInputs(after, before)
	if purpose := diff.Spec["/purpose"]; purpose.Op != "add" || string(purpose.New) != `"content"` {
		t.Fatalf("purpose addition missing: %+v", purpose)
	}
	if purpose := removed.Spec["/purpose"]; purpose.Op != "remove" || string(purpose.Old) != `"content"` {
		t.Fatalf("purpose removal missing: %+v", purpose)
	}
	changedPurpose := after.Clone()
	changedPurpose.Spec.Purpose = SlidePurposeConclusion
	changedPurpose.Spec.ContentType = ""
	if purpose := DiffGenerationInputs(after, changedPurpose).Spec["/purpose"]; purpose.Op != "replace" || string(purpose.Old) != `"content"` || string(purpose.New) != `"conclusion"` {
		t.Fatalf("purpose replacement missing: %+v", purpose)
	}
	change := removed.Spec["/layout"]
	if change.Op != "remove" || string(change.Old) != `"two-column"` || change.New != nil {
		t.Fatalf("remove: %+v", change)
	}
	// Reverting requirements to A clears accumulated net differences without an acknowledgement.
	if DiffGenerationInputs(before, before.Clone()) != nil {
		t.Fatal("A -> B -> A retained a difference")
	}
	escaped := diffReference(map[string]any{"a/b~c": "old"}, map[string]any{"a/b~c": "new"})
	if escaped["/a~1b~0c"].Op != "replace" {
		t.Fatalf("invalid JSON pointer: %+v", escaped)
	}
}

func TestGenerationInputsFormattingAndUnknownBaseline(t *testing.T) {
	before := generationFixture()
	raw, _ := json.MarshalIndent(before, "", "  ")
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	reordered := []byte(`{"spec":` + string(fields["spec"]) + `,"design":` + string(fields["design"]) + `,"manifest":` + string(fields["manifest"]) + `}`)
	if parsed := ParseGenerationInputs(reordered); parsed == nil || DiffGenerationInputs(before, parsed) != nil {
		t.Fatal("format/order changed requirements")
	}
	for _, raw := range []string{"null", "{}", `{"manifest":{},"design":{},"spec":{}}`, strings.Replace(string(reordered), `"demands": [`, `"unknown_demands": [`, 1), strings.TrimSuffix(string(reordered), "}") + `,"outline":{}}`} {
		if ParseGenerationInputs([]byte(raw)) != nil {
			t.Fatalf("invalid snapshot accepted: %s", raw)
		}
	}
	for _, baseline := range []*GenerationInputs{nil, {}} {
		diff := DiffGenerationInputs(baseline, before)
		raw, _ := json.Marshal(diff)
		if string(raw) != `{"baseline":"unknown"}` {
			t.Fatalf("unknown fabricated old values: %s", raw)
		}
	}
}

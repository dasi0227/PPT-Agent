package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func generationFixture() *GenerationInputs {
	return &GenerationInputs{Manifest: validDeck(), Design: Design{Direction: "A", LayoutPreferences: []string{"grid", "space"}, Decorations: DefaultDecorations()}, Spec: SlideSpec{KeyMessage: "Complete message", Elements: []Element{}}}
}

func TestGenerationInputsNetFieldChanges(t *testing.T) {
	before := generationFixture()
	after := before.Clone()
	after.Manifest.Goal = "New goal with full text"
	after.Design.Decorations.SectionTitle = "none"
	after.Design.LayoutPreferences = []string{"space", "grid"}
	after.Spec.Layout = "two-column"
	after.Spec.Role = SlideRoleEvidence
	diff := DiffGenerationInputs(before, after)
	raw, _ := json.Marshal(diff)
	for _, want := range []string{`"/goal":{"op":"replace","old":"Explain","new":"New goal with full text"}`, `"/decorations/section_title":{"op":"replace","old":"top-left","new":"none"}`, `"/layout_preferences":{"op":"replace","old":["grid","space"],"new":["space","grid"]}`, `"/layout":{"op":"add","new":"two-column"}`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("missing %s in %s", want, raw)
		}
	}
	removed := DiffGenerationInputs(after, before)
	if role := diff.Spec["/role"]; role.Op != "add" || string(role.New) != `"evidence"` {
		t.Fatalf("role addition missing: %+v", role)
	}
	if role := removed.Spec["/role"]; role.Op != "remove" || string(role.Old) != `"evidence"` {
		t.Fatalf("role removal missing: %+v", role)
	}
	changedRole := after.Clone()
	changedRole.Spec.Role = SlideRoleConclusion
	if role := DiffGenerationInputs(after, changedRole).Spec["/role"]; role.Op != "replace" || string(role.Old) != `"evidence"` || string(role.New) != `"conclusion"` {
		t.Fatalf("role replacement missing: %+v", role)
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
	for _, raw := range []string{"null", "{}", `{"manifest":{},"design":{},"spec":{}}`, strings.Replace(string(reordered), `"direction": "A",`, "", 1), strings.TrimSuffix(string(reordered), "}") + `,"outline":{}}`} {
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

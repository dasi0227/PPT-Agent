package spec

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func generationFixture() *GenerationInputs {
	return &GenerationInputs{Manifest: validDeck(), Design: Design{Demands: []string{"A", "grid", "space"}, Decorations: DefaultDecorations()}, Spec: SlideSpec{Core: "Complete message", Elements: []Element{}}}
}

func TestParseGenerationInputs(t *testing.T) {
	before := generationFixture()
	raw, _ := json.MarshalIndent(before, "", "  ")
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	reordered := []byte(`{"spec":` + string(fields["spec"]) + `,"design":` + string(fields["design"]) + `,"manifest":` + string(fields["manifest"]) + `}`)
	if parsed := ParseGenerationInputs(reordered); !reflect.DeepEqual(before, parsed) {
		t.Fatal("format/order changed requirements")
	}
	for _, raw := range []string{"null", "{}", `{"manifest":{},"design":{},"spec":{}}`, strings.Replace(string(reordered), `"demands": [`, `"unknown_demands": [`, 1), strings.TrimSuffix(string(reordered), "}") + `,"outline":{}}`} {
		if ParseGenerationInputs([]byte(raw)) != nil {
			t.Fatalf("invalid snapshot accepted: %s", raw)
		}
	}
}

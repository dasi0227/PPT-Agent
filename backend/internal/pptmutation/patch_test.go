package pptmutation

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestRestrictedPatchSupportsRFCArrayAndNestedOperations(t *testing.T) {
	raw := []byte(`{"requirements":[],"elements":[{"type":"text","intent":"old"}]}`)
	next, err := applyPatch(raw, []Patch{
		{Op: "add", Path: "/requirements/-", Value: "first"},
		{Op: "add", Path: "/requirements/0", Value: "zero"},
		{Op: "replace", Path: "/requirements/1", Value: "updated"},
		{Op: "remove", Path: "/requirements/0"},
	}, "deck.patch")
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Requirements []string `json:"requirements"`
	}
	if err := json.Unmarshal(next, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Requirements) != 1 || decoded.Requirements[0] != "updated" {
		t.Fatalf("requirements=%v", decoded.Requirements)
	}

	next, err = applyPatch(raw, []Patch{{Op: "replace", Path: "/elements/0/intent", Value: "new"}}, "slide.spec.patch")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Elements []struct {
			Intent string `json:"intent"`
		} `json:"elements"`
	}
	if err := json.Unmarshal(next, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Elements[0].Intent != "new" {
		t.Fatalf("elements=%v", spec.Elements)
	}
}

func TestRestrictedPatchRejectsDeniedAndNonStandardPaths(t *testing.T) {
	raw := []byte(`{"requirements":[]}`)
	cases := []Patch{
		{Op: "replace", Path: "/revision", Value: 2},
		{Op: "add", Path: "/requirements/-1", Value: "negative"},
		{Op: "add", Path: "/requirements/~2", Value: "bad escape"},
		{Op: "remove", Path: "/requirements/0", Value: "unexpected"},
		{Op: "replace", Path: "/requirements/0"},
	}
	for _, patch := range cases {
		if _, err := applyPatch(raw, []Patch{patch}, "deck.patch"); err == nil {
			t.Fatalf("patch unexpectedly accepted: %+v", patch)
		}
	}
	if _, err := applyPatch(raw, []Patch{{Op: "replace", Path: "/revision", Value: 2}}, "deck.patch"); !errors.Is(err, ErrPatchPathDenied) {
		t.Fatalf("error=%v", err)
	}
}

func TestPatchDecodeDistinguishesMissingAndExplicitNull(t *testing.T) {
	var explicitNull Patch
	if err := json.Unmarshal([]byte(`{"op":"replace","path":"/layout","value":null}`), &explicitNull); err != nil {
		t.Fatal(err)
	}
	if !explicitNull.ValueSet || explicitNull.Value != nil {
		t.Fatalf("patch=%+v", explicitNull)
	}
	if err := validatePatchOperation("slide.spec.patch", explicitNull); err != nil {
		t.Fatalf("explicit null is a syntactically valid patch value: %v", err)
	}

	var missing Patch
	if err := json.Unmarshal([]byte(`{"op":"replace","path":"/layout"}`), &missing); err != nil {
		t.Fatal(err)
	}
	if err := validatePatchOperation("slide.spec.patch", missing); !errors.Is(err, ErrPatchInvalid) {
		t.Fatalf("error=%v", err)
	}
}

func TestPatchSequenceIsAtomicOnFailure(t *testing.T) {
	raw := []byte(`{"requirements":["original"]}`)
	_, err := applyPatch(raw, []Patch{
		{Op: "replace", Path: "/requirements/0", Value: "changed"},
		{Op: "remove", Path: "/requirements/5"},
	}, "deck.patch")
	if !errors.Is(err, ErrPatchInvalid) {
		t.Fatalf("error=%v", err)
	}
	if string(raw) != `{"requirements":["original"]}` {
		t.Fatalf("input mutated: %s", raw)
	}
}

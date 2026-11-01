package pptmutation

import (
	"encoding/json"
	"errors"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"testing"
)

func TestRestrictedPatchSupportsRFCArrayAndNestedOperations(t *testing.T) {
	raw := []byte(`{"content":{"requirements":[],"prohibitions":[]},"visual":{"requirements":[],"prohibitions":[]}}`)
	next, err := applyPatch(raw, []Patch{
		{Op: "add", Path: "/content/requirements/-", Value: "first"},
		{Op: "add", Path: "/content/requirements/0", Value: "zero"},
		{Op: "replace", Path: "/content/requirements/1", Value: "updated"},
		{Op: "remove", Path: "/content/requirements/0"},
	}, "constraints.patch")
	if err != nil {
		t.Fatal(err)
	}
	var decoded spec.Constraints
	if err := json.Unmarshal(next, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Content.Requirements) != 1 || decoded.Content.Requirements[0] != "updated" {
		t.Fatalf("requirements=%v", decoded.Content.Requirements)
	}

	next, err = applyPatch([]byte(`{"elements":[{"type":"text","intent":"old"}]}`), []Patch{{Op: "replace", Path: "/elements/0/intent", Value: "new"}}, "slide.spec.patch")
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
	raw := []byte(`{"content":{"requirements":[],"prohibitions":[]},"visual":{"requirements":[],"prohibitions":[]}}`)
	cases := []Patch{
		{Op: "replace", Path: "/revision", Value: 2},
		{Op: "add", Path: "/positioning", Value: "redundant positioning"},
		{Op: "add", Path: "/content/requirements/-1", Value: "negative"},
		{Op: "add", Path: "/content/requirements/~2", Value: "bad escape"},
		{Op: "remove", Path: "/content/requirements/0", Value: "unexpected"},
		{Op: "replace", Path: "/content/requirements/0"},
	}
	for _, patch := range cases {
		if _, err := applyPatch(raw, []Patch{patch}, "constraints.patch"); err == nil {
			t.Fatalf("patch unexpectedly accepted: %+v", patch)
		}
	}
	if _, err := applyPatch(raw, []Patch{{Op: "replace", Path: "/revision", Value: 2}}, "constraints.patch"); !errors.Is(err, ErrPatchPathDenied) {
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

func TestMetaPatchRejectsRuntimeSettings(t *testing.T) {
	for _, path := range []string{"/canvas", "/canvas/aspect_ratio", "/numbering", "/numbering/enabled", "/numbering/hidden_roles", "/numbering/hidden_roles/-", "/numbering/format"} {
		t.Run(path, func(t *testing.T) {
			_, err := applyPatch([]byte(`{"title":"Deck"}`), []Patch{{Op: "add", Path: path, Value: true}}, "meta.patch")
			if !errors.Is(err, ErrPatchPathDenied) {
				t.Fatalf("runtime setting was not denied: %v", err)
			}
		})
	}
}

func TestPatchSequenceIsAtomicOnFailure(t *testing.T) {
	raw := []byte(`{"content":{"requirements":["original"],"prohibitions":[]},"visual":{"requirements":[],"prohibitions":[]}}`)
	_, err := applyPatch(raw, []Patch{
		{Op: "replace", Path: "/content/requirements/0", Value: "changed"},
		{Op: "remove", Path: "/content/requirements/5"},
	}, "constraints.patch")
	if !errors.Is(err, ErrPatchInvalid) {
		t.Fatalf("error=%v", err)
	}
	if string(raw) != `{"content":{"requirements":["original"],"prohibitions":[]},"visual":{"requirements":[],"prohibitions":[]}}` {
		t.Fatalf("input mutated: %s", raw)
	}
}

func TestConstraintsPatchOnlyChangesGuidanceLists(t *testing.T) {
	raw := []byte(`{"content":{"requirements":[],"prohibitions":[]},"visual":{"requirements":[],"prohibitions":[]}}`)
	next, err := applyPatch(raw, []Patch{{Op: "add", Path: "/content/requirements/-", Value: "Open grids"}, {Op: "add", Path: "/content/prohibitions/-", Value: "No gradients"}}, "constraints.patch")
	if err != nil {
		t.Fatal(err)
	}
	var value spec.Constraints
	if err := json.Unmarshal(next, &value); err != nil {
		t.Fatal(err)
	}
	if len(value.Content.Requirements) != 1 || len(value.Content.Prohibitions) != 1 {
		t.Fatalf("unexpected guidance: %+v", value)
	}
	for _, path := range []string{"/decorations/logo", "/theme", "/logo"} {
		if _, err := applyPatch(raw, []Patch{{Op: "add", Path: path, Value: "top-right"}}, "constraints.patch"); !errors.Is(err, ErrPatchPathDenied) {
			t.Fatalf("human setting accepted: %s (%v)", path, err)
		}
	}
}

func TestConstraintsPatchMaintainsRequirementsIndividually(t *testing.T) {
	raw := []byte(`{"content":{"requirements":[],"prohibitions":[]},"visual":{"requirements":["Use fewer cards","Keep open space"],"prohibitions":[]}}`)
	next, err := applyPatch(raw, []Patch{
		{Op: "replace", Path: "/visual/requirements/0", Value: "Use diagrams"},
		{Op: "remove", Path: "/visual/requirements/1"},
	}, "constraints.patch")
	if err != nil {
		t.Fatal(err)
	}
	var value spec.Constraints
	if err := json.Unmarshal(next, &value); err != nil || len(value.Visual.Requirements) != 1 || value.Visual.Requirements[0] != "Use diagrams" {
		t.Fatalf("requirements=%v, err=%v", value.Visual.Requirements, err)
	}
}

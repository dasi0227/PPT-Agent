package spec

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"strings"
)

// GenerationInputs is a historical authoring snapshot, not proof of compliance.
type GenerationInputs struct {
	Manifest Manifest  `json:"manifest"`
	Design   Design    `json:"design"`
	Spec     SlideSpec `json:"spec"`
}

func (v GenerationInputs) Valid() bool {
	return ValidateManifest(v.Manifest) == nil && ValidateDesign(v.Design) == nil && ValidateSlideSpec(v.Spec) == nil
}

func (v GenerationInputs) Clone() *GenerationInputs {
	raw, _ := json.Marshal(v)
	var out GenerationInputs
	_ = json.Unmarshal(raw, &out)
	return &out
}

func ParseGenerationInputs(raw []byte) *GenerationInputs {
	var fields map[string]json.RawMessage
	if ValidateJSONSource(raw) != nil || json.Unmarshal(raw, &fields) != nil || len(fields) != 3 {
		return nil
	}
	for _, kind := range []string{"manifest", "design", "spec"} {
		if _, err := ParseStrictSourceJSON(fields[kind], kind); err != nil {
			return nil
		}
	}
	var value *GenerationInputs
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || value == nil || decoder.Decode(&struct{}{}) != io.EOF || !value.Valid() {
		return nil
	}
	return value
}

type ReferenceChange struct {
	Op  string          `json:"op"`
	Old json.RawMessage `json:"old,omitempty"`
	New json.RawMessage `json:"new,omitempty"`
}

type HTMLReferenceChanges struct {
	Baseline string                     `json:"baseline,omitempty"`
	Manifest map[string]ReferenceChange `json:"manifest,omitempty"`
	Design   map[string]ReferenceChange `json:"design,omitempty"`
	Spec     map[string]ReferenceChange `json:"spec,omitempty"`
}

// DiffGenerationInputs reports net changes; arrays are ordered atomic values.
func DiffGenerationInputs(baseline, current *GenerationInputs) *HTMLReferenceChanges {
	if baseline == nil || !baseline.Valid() {
		return &HTMLReferenceChanges{Baseline: "unknown"}
	}
	if current == nil || !current.Valid() {
		return nil // Missing current requirements are handled by ordinary context validation.
	}
	out := &HTMLReferenceChanges{
		Manifest: diffReference(baseline.Manifest, current.Manifest),
		Design:   diffReference(baseline.Design, current.Design),
		Spec:     diffReference(baseline.Spec, current.Spec),
	}
	if len(out.Manifest)+len(out.Design)+len(out.Spec) == 0 {
		return nil
	}
	return out
}

func diffReference(before, after any) map[string]ReferenceChange {
	decode := func(value any) any {
		raw, _ := json.Marshal(value)
		var out any
		_ = json.Unmarshal(raw, &out)
		return out
	}
	out := map[string]ReferenceChange{}
	var visit func(string, any, any, bool, bool)
	visit = func(path string, old, next any, oldExists, nextExists bool) {
		if oldExists && nextExists && reflect.DeepEqual(old, next) {
			return
		}
		left, leftObject := old.(map[string]any)
		right, rightObject := next.(map[string]any)
		if oldExists && nextExists && leftObject && rightObject {
			keys := map[string]bool{}
			for key := range left {
				keys[key] = true
			}
			for key := range right {
				keys[key] = true
			}
			for key := range keys {
				a, aok := left[key]
				b, bok := right[key]
				escaped := strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
				visit(path+"/"+escaped, a, b, aok, bok)
			}
			return
		}
		change := ReferenceChange{Op: "replace"}
		if oldExists {
			change.Old, _ = json.Marshal(old)
		} else {
			change.Op = "add"
		}
		if nextExists {
			change.New, _ = json.Marshal(next)
		} else {
			change.Op = "remove"
		}
		out[path] = change
	}
	visit("", decode(before), decode(after), true, true)
	return out
}

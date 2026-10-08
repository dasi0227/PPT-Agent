package spec

import (
	"bytes"
	"encoding/json"
	"io"
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

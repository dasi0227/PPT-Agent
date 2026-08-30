package pptmutation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

const (
	maxPatchOperations = 32
	maxPatchPathBytes  = 512
)

var (
	ErrPatchInvalid    = errors.New("mutation patch invalid")
	ErrPatchPathDenied = errors.New("mutation patch path denied")
)

// Patch is the supported RFC 6902 operation subset. ValueSet preserves the
// important distinction between an omitted value and an explicit JSON null.
type Patch struct {
	Op       string `json:"op"`
	Path     string `json:"path"`
	Value    any    `json:"value,omitempty"`
	ValueSet bool   `json:"-"`
}

func (p *Patch) UnmarshalJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil {
		return fmt.Errorf("%w: %v", ErrPatchInvalid, err)
	}
	for key := range fields {
		if key != "op" && key != "path" && key != "value" {
			return fmt.Errorf("%w: unknown field %q", ErrPatchInvalid, key)
		}
	}
	if err := json.Unmarshal(fields["op"], &p.Op); err != nil || p.Op == "" {
		return fmt.Errorf("%w: op is required", ErrPatchInvalid)
	}
	if err := json.Unmarshal(fields["path"], &p.Path); err != nil {
		return fmt.Errorf("%w: path is required", ErrPatchInvalid)
	}
	value, exists := fields["value"]
	p.ValueSet = exists
	if exists {
		valueDecoder := json.NewDecoder(bytes.NewReader(value))
		valueDecoder.UseNumber()
		if err := valueDecoder.Decode(&p.Value); err != nil {
			return fmt.Errorf("%w: invalid value: %v", ErrPatchInvalid, err)
		}
	}
	return nil
}

func (p Patch) MarshalJSON() ([]byte, error) {
	valueSet := p.ValueSet || p.Value != nil
	fields := map[string]any{"op": p.Op, "path": p.Path}
	if valueSet {
		fields["value"] = p.Value
	}
	return json.Marshal(fields)
}

// PatchPathRule is the single source of truth for both Tool Schema disclosure
// and runtime path authorization. Pattern is anchored and matches the encoded
// RFC 6901 JSON Pointer, not a decoded filesystem-like path.
type PatchPathRule struct {
	Pattern     string
	Description string
}

var patchPathRules = map[string]map[string][]PatchPathRule{
	"manifest.patch": {
		"add": {
			{Pattern: `^/(?:title|goal|audience|language|positioning|requirements|prohibitions)$`, Description: "deck author fields"},
			{Pattern: `^/(?:canvas/aspect_ratio|numbering/(?:enabled|hidden_roles|format))$`, Description: "deck settings"},
			{Pattern: `^/(?:requirements|prohibitions|numbering/hidden_roles)/(?:-|0|[1-9][0-9]*)$`, Description: "deck list item or append position"},
		},
		"remove": {
			{Pattern: `^/(?:title|goal|audience|language|positioning|requirements|prohibitions)$`, Description: "deck author fields"},
			{Pattern: `^/(?:canvas/aspect_ratio|numbering/(?:enabled|hidden_roles|format))$`, Description: "deck settings"},
			{Pattern: `^/(?:requirements|prohibitions|numbering/hidden_roles)/(?:0|[1-9][0-9]*)$`, Description: "existing deck list item"},
		},
		"replace": {
			{Pattern: `^/(?:title|goal|audience|language|positioning|requirements|prohibitions)$`, Description: "deck author fields"},
			{Pattern: `^/(?:canvas/aspect_ratio|numbering/(?:enabled|hidden_roles|format))$`, Description: "deck settings"},
			{Pattern: `^/(?:requirements|prohibitions|numbering/hidden_roles)/(?:0|[1-9][0-9]*)$`, Description: "existing deck list item"},
		},
	},
	"design.patch": {
		"add": {
			{Pattern: `^/(?:theme|direction|density|chrome)$`, Description: "design author fields"},
			{Pattern: `^/chrome/(?:-|0|[1-9][0-9]*)$`, Description: "chrome item or append position"},
			{Pattern: `^/chrome/(?:0|[1-9][0-9]*)/(?:type|placement|style)$`, Description: "chrome item fields"},
		},
		"remove": {
			{Pattern: `^/(?:theme|direction|density|chrome)$`, Description: "design author fields"},
			{Pattern: `^/chrome/(?:0|[1-9][0-9]*)$`, Description: "existing chrome item"},
			{Pattern: `^/chrome/(?:0|[1-9][0-9]*)/(?:type|placement|style)$`, Description: "chrome item fields"},
		},
		"replace": {
			{Pattern: `^/(?:theme|direction|density|chrome)$`, Description: "design author fields"},
			{Pattern: `^/chrome/(?:0|[1-9][0-9]*)$`, Description: "existing chrome item"},
			{Pattern: `^/chrome/(?:0|[1-9][0-9]*)/(?:type|placement|style)$`, Description: "chrome item fields"},
		},
	},
	"slide.spec.patch": {
		"add": {
			{Pattern: `^/(?:key_message|elements|layout)$`, Description: "slide spec author fields"},
			{Pattern: `^/elements/(?:-|0|[1-9][0-9]*)$`, Description: "slide element or append position"},
			{Pattern: `^/elements/(?:0|[1-9][0-9]*)/(?:type|intent)$`, Description: "slide element fields"},
		},
		"remove": {
			{Pattern: `^/(?:key_message|elements|layout)$`, Description: "slide spec author fields"},
			{Pattern: `^/elements/(?:0|[1-9][0-9]*)$`, Description: "existing slide element"},
			{Pattern: `^/elements/(?:0|[1-9][0-9]*)/(?:type|intent)$`, Description: "slide element fields"},
		},
		"replace": {
			{Pattern: `^/(?:key_message|elements|layout)$`, Description: "slide spec author fields"},
			{Pattern: `^/elements/(?:0|[1-9][0-9]*)$`, Description: "existing slide element"},
			{Pattern: `^/elements/(?:0|[1-9][0-9]*)/(?:type|intent)$`, Description: "slide element fields"},
		},
	},
}

func PatchPathRules(operation, patchOp string) []PatchPathRule {
	rules := patchPathRules[operation][patchOp]
	return append([]PatchPathRule(nil), rules...)
}

func applyPatch(raw []byte, patches []Patch, operation string) ([]byte, error) {
	if len(patches) == 0 || len(patches) > maxPatchOperations {
		return nil, fmt.Errorf("%w: patch must contain 1-%d operations", ErrPatchInvalid, maxPatchOperations)
	}
	for index, patch := range patches {
		if err := validatePatchOperation(operation, patch); err != nil {
			return nil, fmt.Errorf("patch[%d]: %w", index, err)
		}
	}
	patchRaw, err := json.Marshal(patches)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPatchInvalid, err)
	}
	decoded, err := jsonpatch.DecodePatch(patchRaw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPatchInvalid, err)
	}
	options := jsonpatch.NewApplyOptions()
	options.SupportNegativeIndices = false
	options.AllowMissingPathOnRemove = false
	options.EnsurePathExistsOnAdd = false
	next, err := decoded.ApplyWithOptions(raw, options)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPatchInvalid, err)
	}
	return next, nil
}

func validatePatchOperation(operation string, patch Patch) error {
	if patch.Op != "add" && patch.Op != "remove" && patch.Op != "replace" {
		return fmt.Errorf("%w: op must be add, remove, or replace", ErrPatchInvalid)
	}
	valueSet := patch.ValueSet || patch.Value != nil
	if patch.Op == "remove" && valueSet {
		return fmt.Errorf("%w: remove must not include value", ErrPatchInvalid)
	}
	if patch.Op != "remove" && !valueSet {
		return fmt.Errorf("%w: %s requires value", ErrPatchInvalid, patch.Op)
	}
	if err := validateJSONPointer(patch.Path); err != nil {
		return err
	}
	for _, rule := range PatchPathRules(operation, patch.Op) {
		if regexp.MustCompile(rule.Pattern).MatchString(patch.Path) {
			return nil
		}
	}
	return fmt.Errorf("%w: path %s is not writable for %s", ErrPatchPathDenied, patch.Path, operation)
}

func validateJSONPointer(path string) error {
	if path == "" || len(path) > maxPatchPathBytes || !strings.HasPrefix(path, "/") {
		return fmt.Errorf("%w: path must be a non-empty JSON Pointer", ErrPatchInvalid)
	}
	parts := strings.Split(path[1:], "/")
	if len(parts) > 32 {
		return fmt.Errorf("%w: path is too deep", ErrPatchInvalid)
	}
	for _, part := range parts {
		for index := 0; index < len(part); index++ {
			if part[index] != '~' {
				continue
			}
			if index+1 >= len(part) || (part[index+1] != '0' && part[index+1] != '1') {
				return fmt.Errorf("%w: invalid JSON Pointer escape", ErrPatchInvalid)
			}
			index++
		}
	}
	return nil
}

package spec

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ParseCollection rejects damaged data rather than silently replacing it with
// an empty collection during a later read-modify-write.
func ParseCollection(raw []byte) (map[string]json.RawMessage, error) {
	if err := ValidateJSONSource(raw); err != nil {
		return nil, err
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	if entries == nil {
		return nil, fmt.Errorf("spec collection must be an object")
	}
	for id, value := range entries {
		if !slideIDPattern.MatchString(id) {
			return nil, fmt.Errorf("invalid spec key %q", id)
		}
		if _, err := ParseStrictSourceJSON(value, "spec"); err != nil {
			return nil, fmt.Errorf("spec %s: %w", id, err)
		}
	}
	return entries, nil
}

var slideIDPattern = regexp.MustCompile(`^sli_[A-Za-z0-9_-]+$`)

func ReadCollection(read func(string) ([]byte, error)) (map[string]json.RawMessage, error) {
	raw, err := read(model.SpecCollectionPath)
	if err != nil {
		return nil, err
	}
	return ParseCollection(raw)
}

func CollectionEntry(raw []byte, id string) ([]byte, error) {
	entries, err := ParseCollection(raw)
	if err != nil {
		return nil, err
	}
	return SpecEntry(entries, id)
}

// SpecEntry reads a canonical per-page value from an already validated map.
func SpecEntry(entries map[string]json.RawMessage, id string) ([]byte, error) {
	value, ok := entries[id]
	if !ok {
		return nil, fs.ErrNotExist
	}
	// Canonical bytes keep per-page evidence independent of collection formatting.
	var valueJSON any
	if err := json.Unmarshal(value, &valueJSON); err != nil {
		return nil, err
	}
	return json.Marshal(valueJSON)
}

func ReadSlideSpec(read func(string) ([]byte, error), id string) ([]byte, error) {
	raw, err := read(model.SpecCollectionPath)
	if err != nil {
		return nil, err
	}
	return CollectionEntry(raw, id)
}

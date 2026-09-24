package spec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
	nethtml "golang.org/x/net/html"
)

var ErrSlideStageMissing = errors.New("slide HTML must contain a slide-stage element")
var ErrStaticPageNumber = errors.New("page numbers belong to the runtime frame")

// ParseStrictSourceJSON validates the submitted source before typed decoding,
// preserving required-field and null checks that decoding alone would erase.
func ParseStrictSourceJSON(raw []byte, kind string) (any, error) {
	if err := ValidateJSONSource(raw); err != nil {
		return nil, err
	}
	var name string
	var value any
	switch kind {
	case "spec":
		name, value = pptschema.SlideSpecName, &SlideSpec{}
	case "manifest":
		name, value = pptschema.ManifestName, &Manifest{}
	case "design":
		name, value = pptschema.DesignName, &Design{}
	default:
		return nil, errors.New("unsupported JSON source kind")
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	if err := pptschema.Validate(name, decoded); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, value); err != nil {
		return nil, err
	}
	return value, nil
}

func ValidateJSONSource(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := scanJSONValue(decoder, ""); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("JSON contains more than one root value")
		}
		return err
	}
	return nil
}

type SourceSystemMetadata struct {
	SchemaVersion string
	ProjectID     string
	SlideID       string
	CreatedAt     int64
	UpdatedAt     int64
}

// SourceSystemFields permits repair of invalid business fields while proving
// the immutable identity and metadata of an existing source file.
func SourceSystemFields(raw []byte, kind string) (SourceSystemMetadata, error) {
	var metadata SourceSystemMetadata
	if err := ValidateJSONSource(raw); err != nil {
		return metadata, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return metadata, errors.New("source must be a JSON object")
	}
	targets := []struct {
		name   string
		target any
	}{
		{"version", &metadata.SchemaVersion}, {"project_id", &metadata.ProjectID},
		{"created_at", &metadata.CreatedAt}, {"updated_at", &metadata.UpdatedAt},
	}
	if kind == "spec" {
		targets = append(targets, struct {
			name   string
			target any
		}{"slide_id", &metadata.SlideID})
	}
	for _, item := range targets {
		if bytes.Equal(bytes.TrimSpace(fields[item.name]), []byte("null")) {
			return metadata, fmt.Errorf("invalid system field %s", item.name)
		}
		if err := json.Unmarshal(fields[item.name], item.target); err != nil {
			return metadata, fmt.Errorf("invalid system field %s: %w", item.name, err)
		}
	}
	if metadata.SchemaVersion != SchemaVersion || metadata.ProjectID == "" || (kind == "spec" && metadata.SlideID == "") || metadata.CreatedAt < 0 || metadata.UpdatedAt < 0 {
		return metadata, errors.New("source system fields are invalid")
	}
	return metadata, nil
}

func scanJSONValue(decoder *json.Decoder, pointer string) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("JSON object key is invalid")
			}
			child := pointer + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
			if seen[key] {
				return fmt.Errorf("duplicate JSON key at %s", child)
			}
			seen[key] = true
			if err := scanJSONValue(decoder, child); err != nil {
				return err
			}
		}
	case '[':
		index := 0
		for decoder.More() {
			if err := scanJSONValue(decoder, fmt.Sprintf("%s/%d", pointer, index)); err != nil {
				return err
			}
			index++
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	_, err = decoder.Token()
	return err
}

// ValidateSlideHTML checks the authoring structure, not browser rendering.
func ValidateSlideHTML(raw []byte) error {
	if strings.Contains(string(raw), "data-page-number") || strings.Contains(string(raw), "data-runtime-page-number") {
		return ErrStaticPageNumber
	}
	root, err := nethtml.Parse(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	found := false
	var visit func(*nethtml.Node)
	visit = func(node *nethtml.Node) {
		if node.Type == nethtml.ElementNode {
			for _, attr := range node.Attr {
				if attr.Key == "class" {
					for _, class := range strings.Fields(attr.Val) {
						if class == "slide-stage" {
							found = true
						}
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	if err != nil {
		return err
	}
	if !found {
		return ErrSlideStageMissing
	}
	return nil
}

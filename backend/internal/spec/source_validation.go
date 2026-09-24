package spec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	nethtml "golang.org/x/net/html"
)

var ErrSlideStageMissing = errors.New("slide HTML must contain a slide-stage element")
var ErrStaticPageNumber = errors.New("page numbers belong to the runtime frame")

// ParseStrictSlideSpec rejects ambiguous source before decoding into the
// authoring schema. In particular, encoding/json.Unmarshal accepts duplicate
// keys and would silently discard an earlier value.
func ParseStrictSlideSpec(raw []byte) (SlideSpec, error) {
	var slide SlideSpec
	if err := ValidateJSONSource(raw); err != nil {
		return slide, err
	}
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&slide); err != nil {
		return slide, err
	}
	if err := ValidateSlideSpec(slide); err != nil {
		return slide, err
	}
	return slide, nil
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

// SlideSpecSystemFields permits repair of invalid business fields while still
// proving which file and immutable metadata the source belongs to.
func SlideSpecSystemFields(raw []byte) (SlideSpec, error) {
	var slide SlideSpec
	if err := ValidateJSONSource(raw); err != nil {
		return slide, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return slide, errors.New("spec must be a JSON object")
	}
	for _, item := range []struct {
		name   string
		target any
	}{
		{"version", &slide.SchemaVersion}, {"project_id", &slide.ProjectID}, {"slide_id", &slide.SlideID},
		{"created_at", &slide.CreatedAt}, {"updated_at", &slide.UpdatedAt},
	} {
		if err := json.Unmarshal(fields[item.name], item.target); err != nil {
			return slide, fmt.Errorf("invalid system field %s: %w", item.name, err)
		}
	}
	if slide.SchemaVersion != SchemaVersion || slide.ProjectID == "" || slide.SlideID == "" || slide.CreatedAt < 0 || slide.UpdatedAt < 0 {
		return slide, errors.New("spec system fields are invalid")
	}
	return slide, nil
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

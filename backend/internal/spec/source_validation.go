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
	case "outline":
		name, value = pptschema.OutlineName, &Outline{}
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

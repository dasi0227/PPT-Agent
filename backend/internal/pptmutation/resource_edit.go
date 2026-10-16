package pptmutation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

// ResourceEdit is the business input shared by the focused authoring tools.
// The UI's node operations remain internal to its own management API.
type ResourceEdit struct {
	Resource     string
	ProjectTitle string
	SlideID      string
	ExpectedHash string
	Fields       map[string]any
	Content      string
	Edits        []Edit
}

// ResourceSourceError identifies an existing resource read or validation failure,
// separately from validation of a proposed edit.
type ResourceSourceError struct {
	Resource string
	Err      error
}

func (e *ResourceSourceError) Error() string { return e.Resource + " source: " + e.Err.Error() }
func (e *ResourceSourceError) Unwrap() error { return e.Err }

var ErrOutlineNotExist = errors.New("outline source .outline.json does not exist")
var ErrSlideNotFound = errors.New("slide_id is not in outline")

type TextEditMatchError struct {
	Index, Matches int
}

func (e *TextEditMatchError) Error() string {
	return fmt.Sprintf("edits[%d].old_text must match exactly once, got %d", e.Index, e.Matches)
}

type SourceFieldError struct {
	Field, Message string
}

func (e *SourceFieldError) Error() string { return e.Field + ": " + e.Message }

func ApplyTextEdits(raw []byte, edits []Edit) ([]byte, error) {
	if len(edits) == 0 {
		return nil, invalid(errors.New("edits must not be empty"))
	}
	text := string(raw)
	for i, edit := range edits {
		if edit.OldText == "" {
			return nil, invalid(fmt.Errorf("edits[%d].old_text must not be empty", i))
		}
		count := strings.Count(text, edit.OldText)
		if count != 1 {
			return nil, invalid(&TextEditMatchError{Index: i, Matches: count})
		}
		text = strings.Replace(text, edit.OldText, edit.NewText, 1)
	}
	return []byte(text), nil
}

func (s Service) EditResource(req ResourceEdit) error {
	if req.Resource == "outline" {
		return s.editOutlineSource(req)
	}
	var path string
	var before []byte
	var err error
	if req.Resource == "spec" {
		outlineRaw, e := s.Workspace.Read(".outline.json")
		if e != nil {
			return &ResourceSourceError{Resource: "outline", Err: e}
		}
		parsed, e := spec.ParseStrictSourceJSON(outlineRaw, "outline")
		if e != nil {
			return &ResourceSourceError{Resource: "outline", Err: e}
		}
		if _, ok := spec.FindSlide(*parsed.(*spec.Outline), req.SlideID); !ok {
			return invalid(ErrSlideNotFound)
		}
		path = model.SlideSpecPath(req.SlideID)
		before, err = spec.ReadSlideSpec(s.Workspace.Read, req.SlideID)
	} else if req.Resource == "manifest" || req.Resource == "design" {
		path = "." + req.Resource + ".json"
		before, err = s.Workspace.Read(path)
		if err == nil {
			_, err = spec.ParseStrictSourceJSON(before, req.Resource)
		}
	} else {
		return invalid(errors.New("unsupported editable resource"))
	}
	missing := errors.Is(err, fs.ErrNotExist)
	if err != nil && !(missing && (req.Resource == "spec" || req.Resource == "manifest" || req.Resource == "design")) {
		return &ResourceSourceError{Resource: req.Resource, Err: err}
	}
	if req.ExpectedHash != "" && (err != nil || checkHash(req.ExpectedHash, spec.ResourceBytesHash(before)) != nil) {
		return ErrContentConflict
	}
	current := map[string]any{}
	if err == nil {
		if e := json.Unmarshal(before, &current); e != nil {
			return e
		}
		if current == nil {
			return errors.New("validated resource must be an object")
		}
	} else if req.Resource == "manifest" || req.Resource == "design" {
		var defaults any = spec.DefaultDesign()
		if req.Resource == "manifest" {
			defaults = spec.DefaultManifest(req.ProjectTitle)
		}
		defaultRaw, _ := json.Marshal(defaults)
		if e := json.Unmarshal(defaultRaw, &current); e != nil {
			return e
		}
	}
	if len(req.Fields) == 0 {
		return invalid(errors.New("provide at least one editable field"))
	}
	if req.Resource == "spec" && missing {
		if _, provided := req.Fields["elements"]; !provided {
			current["elements"] = []any{}
		}
	}
	changed := false
	for key, value := range req.Fields {
		old, exists := current[key]
		if value == nil {
			if req.Resource != "spec" || (key != "core" && key != "purpose" && key != "approach" && key != "layout") {
				return invalid(fmt.Errorf("%s cannot be null", key))
			}
			if exists {
				delete(current, key)
				changed = true
			}
			continue
		}

		if !exists || !reflect.DeepEqual(old, value) {
			changed = true
		}
		current[key] = value
	}
	if req.Resource == "spec" {
		_, purposeSet := req.Fields["purpose"]
		_, approachSet := req.Fields["approach"]
		if purposeSet && current["purpose"] != string(spec.SlidePurposeContent) && !approachSet {
			if _, exists := current["approach"]; exists {
				delete(current, "approach")
				changed = true
			}
		}
	}
	raw, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if _, err = spec.ParseStrictSourceJSON(raw, req.Resource); err != nil {
		return invalid(err)
	}
	if !changed && !missing {
		return nil
	}
	return s.Workspace.Write(path, raw)
}

func (s Service) editOutlineSource(req ResourceEdit) error {
	hasContent := req.Content != ""
	if hasContent == (len(req.Edits) > 0) {
		return invalid(errors.New("provide exactly one of content or edits"))
	}
	if s.NewID == nil {
		s.NewID = func(prefix string) string { return model.MustShortID(prefix) }
	}
	before, err := s.Workspace.Read(".outline.json")
	missing := errors.Is(err, fs.ErrNotExist)
	if err != nil && !missing {
		return err
	}
	if !hasContent && missing {
		return invalid(ErrOutlineNotExist)
	}
	if req.ExpectedHash != "" && (missing || checkHash(req.ExpectedHash, spec.ContentHash(before)) != nil) {
		return ErrContentConflict
	}
	old := spec.Outline{Sections: []spec.Section{}}
	known := map[string]string{}
	if !missing {
		parsed, err := spec.ParseStrictSourceJSON(before, "outline")
		if err != nil {
			return err
		}
		old = *parsed.(*spec.Outline)
		for _, section := range old.Sections {
			known[section.ID] = "section"
			for _, slide := range section.Slides {
				known[slide.ID] = "slide"
			}
			for _, sub := range section.Subsections {
				known[sub.ID] = "subsection"
				for _, slide := range sub.Slides {
					known[slide.ID] = "slide"
				}
			}
		}
	}
	candidate := []byte(req.Content)
	if !hasContent {
		candidate, err = ApplyTextEdits(before, req.Edits)
		if err != nil {
			return err
		}
	}
	if err = spec.ValidateJSONSource(candidate); err != nil {
		return invalid(err)
	}
	var root map[string]any
	if err = json.Unmarshal(candidate, &root); err != nil || root == nil {
		return invalid(errors.New("content must be a JSON outline object"))
	}
	allocated := map[string]bool{}
	for id := range known {
		allocated[id] = true
	}
	var visit func(any, string, string) error
	visit = func(list any, kind, path string) error {
		nodes, ok := list.([]any)
		if !ok {
			return invalid(&SourceFieldError{Field: path, Message: "required outline children must be JSON arrays; use [] when empty"})
		}
		for index, value := range nodes {
			nodePath := fmt.Sprintf("%s/%d", path, index)
			node, ok := value.(map[string]any)
			if !ok {
				return invalid(&SourceFieldError{Field: nodePath, Message: "outline nodes must be objects"})
			}
			id, ok := node["id"].(string)
			if !ok || id == "" || id != "new" && known[id] != kind {
				return invalid(&SourceFieldError{Field: nodePath + "/id", Message: fmt.Sprintf("must be \"new\" for a new node or an existing %s identity; preserve existing IDs when editing or moving", kind)})
			}
			newNode := id == "new"
			if newNode {
				prefix := "sec"
				if kind == "slide" {
					prefix = "sli"
				}
				id = s.NewID(prefix)
				if allocated[id] {
					return errors.New("generated outline identity already exists")
				}
				allocated[id] = true
				node["id"] = id
			}
			if newNode && kind != "slide" {
				if _, present := node["slides"]; !present {
					node["slides"] = []any{}
				}
			}
			if newNode && kind == "section" {
				if _, present := node["subsections"]; !present {
					node["subsections"] = []any{}
				}
			}
			if kind != "slide" {
				if err := visit(node["slides"], "slide", nodePath+"/slides"); err != nil {
					return err
				}
			}
			if kind == "section" {
				if err := visit(node["subsections"], "subsection", nodePath+"/subsections"); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err = visit(root["sections"], "section", "/sections"); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	parsed, err := spec.ParseStrictSourceJSON(raw, "outline")
	if err != nil {
		return invalid(err)
	}
	next := parsed.(*spec.Outline)
	if !missing && reflect.DeepEqual(old, *next) {
		return nil
	}
	remaining := map[string]bool{}
	for _, loc := range spec.FlattenOutline(*next) {
		remaining[loc.Slide.ID] = true
	}
	removed := []string{}
	for _, loc := range spec.FlattenOutline(old) {
		if !remaining[loc.Slide.ID] {
			removed = append(removed, loc.Slide.ID)
		}
	}
	for _, id := range removed {
		if err := s.deleteSlideSources(id); err != nil {
			return err
		}
	}
	return s.Workspace.Write(".outline.json", raw)
}

package pptmutation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

// ResourceEdit is the business input shared by the focused authoring tools.
// The UI's node operations remain internal to its own management API.
type ResourceEdit struct {
	Resource     string
	SlideID      string
	ExpectedHash string
	Fields       map[string]any
	Content      string
	Edits        []Edit
	Initialize   bool
}

type ResourceEditResult struct {
	Content       []byte
	ChangedFields []string
}

var ErrOutlineExists = errors.New("outline already exists; use arrange_outline")
var ErrOutlineNotInitialized = errors.New("outline is not initialized; use init_outline")
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

func (s Service) EditResource(req ResourceEdit) (ResourceEditResult, error) {
	if req.Resource == "outline" {
		return s.editOutlineSource(req)
	}
	path := "." + req.Resource + ".json"
	var before []byte
	var err error
	if req.Resource == "spec" {
		outline, e := s.currentOutline()
		if e != nil {
			return ResourceEditResult{}, e
		}
		if _, ok := spec.FindSlide(outline, req.SlideID); !ok {
			return ResourceEditResult{}, invalid(ErrSlideNotFound)
		}
		before, err = spec.ReadSlideSpec(s.Workspace.Read, req.SlideID)
	} else if req.Resource == "manifest" || req.Resource == "design" {
		before, err = s.Workspace.Read(path)
	} else {
		return ResourceEditResult{}, invalid(errors.New("unsupported editable resource"))
	}
	if err != nil && !(req.Resource == "spec" && errors.Is(err, fs.ErrNotExist)) {
		return ResourceEditResult{}, err
	}
	if req.ExpectedHash != "" && (err != nil || checkHash(req.ExpectedHash, spec.ResourceBytesHash(before)) != nil) {
		return ResourceEditResult{}, ErrContentConflict
	}
	current := map[string]any{}
	if err == nil {
		if e := json.Unmarshal(before, &current); e != nil {
			return ResourceEditResult{}, e
		}
		if current == nil {
			return ResourceEditResult{}, invalid(errors.New("resource must be an object"))
		}
	}
	if len(req.Fields) == 0 {
		return ResourceEditResult{}, invalid(errors.New("provide at least one editable field"))
	}
	changed := []string{}
	for key, value := range req.Fields {
		old, exists := current[key]
		if value == nil {
			if req.Resource != "spec" || (key != "role" && key != "layout") {
				return ResourceEditResult{}, invalid(fmt.Errorf("%s cannot be null", key))
			}
			if exists {
				delete(current, key)
				changed = append(changed, key)
			}
			continue
		}
		if req.Resource == "design" && key == "decorations" {
			updates, ok := value.(map[string]any)
			if !ok || len(updates) == 0 {
				return ResourceEditResult{}, invalid(errors.New("decorations must contain at least one position"))
			}
			merged := map[string]any{}
			if previous, ok := old.(map[string]any); ok {
				for k, v := range previous {
					merged[k] = v
				}
			}
			for k, v := range updates {
				merged[k] = v
			}
			value = merged
		}
		if !exists || !reflect.DeepEqual(old, value) {
			changed = append(changed, key)
		}
		current[key] = value
	}
	raw, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return ResourceEditResult{}, err
	}
	if _, err = spec.ParseStrictSourceJSON(raw, req.Resource); err != nil {
		return ResourceEditResult{}, invalid(err)
	}
	sort.Strings(changed)
	if len(changed) == 0 {
		return ResourceEditResult{Content: before, ChangedFields: changed}, nil
	}
	if req.Resource == "spec" {
		entries, e := spec.ReadCollection(s.Workspace.Read)
		if e != nil {
			return ResourceEditResult{}, e
		}
		entries[req.SlideID] = json.RawMessage(raw)
		err = s.writeJSON(model.SpecCollectionPath, entries)
	} else {
		err = s.Workspace.Write(path, raw)
	}
	return ResourceEditResult{Content: raw, ChangedFields: changed}, err
}

func (s Service) editOutlineSource(req ResourceEdit) (ResourceEditResult, error) {
	if s.NewID == nil {
		s.NewID = func(prefix string) string { return model.MustShortID(prefix) }
	}
	before, err := s.Workspace.Read(".outline.json")
	missing := errors.Is(err, fs.ErrNotExist)
	if err != nil && !missing {
		return ResourceEditResult{}, err
	}
	if req.Initialize && !missing {
		return ResourceEditResult{}, invalid(ErrOutlineExists)
	}
	if !req.Initialize && missing {
		return ResourceEditResult{}, invalid(ErrOutlineNotInitialized)
	}
	if req.ExpectedHash != "" && (missing || checkHash(req.ExpectedHash, spec.ContentHash(before)) != nil) {
		return ResourceEditResult{}, ErrContentConflict
	}
	old := spec.Outline{Sections: []spec.Section{}}
	known := map[string]string{}
	if !missing {
		if _, err = spec.ParseStrictSourceJSON(before, "outline"); err != nil {
			return ResourceEditResult{}, err
		}
		if err = json.Unmarshal(before, &old); err != nil {
			return ResourceEditResult{}, err
		}
		if err = spec.ValidateOutline(old); err != nil {
			return ResourceEditResult{}, err
		}
		for _, section := range old.Sections {
			known[section.ID] = "sec"
			for _, slide := range section.Slides {
				known[slide.SlideID] = "sli"
			}
			for _, sub := range section.Subsections {
				known[sub.ID] = "sub"
				for _, slide := range sub.Slides {
					known[slide.SlideID] = "sli"
				}
			}
		}
	}
	candidate := []byte(req.Content)
	if !req.Initialize {
		candidate, err = ApplyTextEdits(before, req.Edits)
		if err != nil {
			return ResourceEditResult{}, err
		}
	}
	if err = spec.ValidateJSONSource(candidate); err != nil {
		return ResourceEditResult{}, invalid(err)
	}
	var root map[string]any
	if err = json.Unmarshal(candidate, &root); err != nil || root == nil {
		return ResourceEditResult{}, invalid(errors.New("content must be a JSON outline object"))
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
			key := "id"
			if kind == "sli" {
				key = "slide_id"
			}
			if v, present := node[key]; present {
				id, ok := v.(string)
				if !ok || id == "" || known[id] != kind {
					return invalid(&SourceFieldError{Field: nodePath + "/" + key, Message: fmt.Sprintf("must be an existing %s identity; omit it for new nodes", kind)})
				}
			} else {
				node[key] = s.NewID(kind)
			}
			if kind != "sli" {
				if err := visit(node["slides"], "sli", nodePath+"/slides"); err != nil {
					return err
				}
			}
			if kind == "sec" {
				if err := visit(node["subsections"], "sub", nodePath+"/subsections"); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err = visit(root["sections"], "sec", "/sections"); err != nil {
		return ResourceEditResult{}, err
	}
	raw, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return ResourceEditResult{}, err
	}
	parsed, err := spec.ParseStrictSourceJSON(raw, "outline")
	if err != nil {
		return ResourceEditResult{}, invalid(err)
	}
	next := parsed.(*spec.Outline)
	if err = spec.ValidateOutline(*next); err != nil {
		return ResourceEditResult{}, invalid(err)
	}
	if !missing && reflect.DeepEqual(old, *next) {
		return ResourceEditResult{Content: before, ChangedFields: []string{}}, nil
	}
	remaining := map[string]bool{}
	for _, loc := range spec.FlattenOutline(*next) {
		remaining[loc.Slide.SlideID] = true
	}
	removed := []string{}
	for _, loc := range spec.FlattenOutline(old) {
		if !remaining[loc.Slide.SlideID] {
			removed = append(removed, loc.Slide.SlideID)
		}
	}
	if len(removed) > 0 {
		entries, e := spec.ReadCollection(s.Workspace.Read)
		if e != nil {
			return ResourceEditResult{}, e
		}
		for _, id := range removed {
			delete(entries, id)
			if e := s.Workspace.Delete(model.SlideHTMLPath(id)); e != nil && !errors.Is(e, fs.ErrNotExist) {
				return ResourceEditResult{}, e
			}
		}
		if err = s.writeJSON(model.SpecCollectionPath, entries); err != nil {
			return ResourceEditResult{}, err
		}
	}
	err = s.Workspace.Write(".outline.json", raw)
	return ResourceEditResult{Content: raw, ChangedFields: []string{"sections"}}, err
}

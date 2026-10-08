package pptmutation

import (
	"encoding/json"
	"errors"
	"io/fs"
	"reflect"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

// ReplaceApprovedResource stages a complete, validated user-approved source.
// Callers decide whether the staged buffer is committed to durable project files.
func (s Service) ReplaceApprovedResource(resource string, candidate []byte) (bool, error) {
	if resource != "manifest" && resource != "design" && resource != "outline" {
		return false, invalid(errors.New("unsupported approval resource"))
	}
	parsed, err := spec.ParseStrictSourceJSON(candidate, resource)
	if err != nil {
		return false, invalid(err)
	}
	path := "." + resource + ".json"
	before, err := s.Workspace.Read(path)
	missing := errors.Is(err, fs.ErrNotExist)
	if err != nil && !missing {
		return false, err
	}
	var normalized []byte
	switch resource {
	case "manifest":
		next := parsed.(*spec.Manifest)
		if err = spec.ValidateManifest(*next); err != nil {
			return false, invalid(err)
		}
		normalized, err = json.MarshalIndent(next, "", "  ")
		if err == nil && !missing && spec.ResourceBytesHash(before) == spec.ResourceHash(next) {
			return false, nil
		}
	case "design":
		next := parsed.(*spec.Design)
		if err = spec.ValidateDesign(*next); err != nil {
			return false, invalid(err)
		}
		normalized, err = json.MarshalIndent(next, "", "  ")
		if err == nil && !missing && spec.ResourceBytesHash(before) == spec.ResourceHash(next) {
			return false, nil
		}
	case "outline":
		next := parsed.(*spec.Outline)
		if err = spec.ValidateOutline(*next); err != nil {
			return false, invalid(err)
		}
		old := spec.Outline{Sections: []spec.Section{}}
		if !missing {
			if _, err = spec.ParseStrictSourceJSON(before, "outline"); err != nil {
				return false, err
			}
			if err = json.Unmarshal(before, &old); err != nil {
				return false, err
			}
			if err = spec.ValidateOutline(old); err != nil {
				return false, err
			}
			if reflect.DeepEqual(old, *next) {
				return false, nil
			}
		}
		if err = validateApprovedOutlineIdentities(old, *next); err != nil {
			return false, invalid(err)
		}
		remaining := map[string]bool{}
		for _, loc := range spec.FlattenOutline(*next) {
			remaining[loc.Slide.ID] = true
		}
		entries, readErr := spec.ReadCollection(s.Workspace.Read)
		if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
			return false, readErr
		}
		removed := false
		for _, loc := range spec.FlattenOutline(old) {
			if remaining[loc.Slide.ID] {
				continue
			}
			removed = true
			delete(entries, loc.Slide.ID)
			if deleteErr := s.Workspace.Delete(model.SlideHTMLPath(loc.Slide.ID)); deleteErr != nil && !errors.Is(deleteErr, fs.ErrNotExist) {
				return false, deleteErr
			}
		}
		if removed {
			if err = s.writeJSON(model.SpecCollectionPath, entries); err != nil {
				return false, err
			}
		}
		normalized, err = json.MarshalIndent(next, "", "  ")
	}
	if err != nil {
		return false, err
	}
	normalized = append(normalized, '\n')
	return true, s.Workspace.Write(path, normalized)
}

func validateApprovedOutlineIdentities(before, after spec.Outline) error {
	kinds := map[string]string{}
	for _, section := range before.Sections {
		kinds[section.ID] = "section"
		for _, slide := range section.Slides {
			kinds[slide.ID] = "slide"
		}
		for _, sub := range section.Subsections {
			kinds[sub.ID] = "subsection"
			for _, slide := range sub.Slides {
				kinds[slide.ID] = "slide"
			}
		}
	}
	check := func(id, kind string) error {
		if oldKind := kinds[id]; oldKind != "" && oldKind != kind {
			return errors.New("outline identity cannot change node kind")
		}
		return nil
	}
	for _, section := range after.Sections {
		if err := check(section.ID, "section"); err != nil {
			return err
		}
		for _, slide := range section.Slides {
			if err := check(slide.ID, "slide"); err != nil {
				return err
			}
		}
		for _, sub := range section.Subsections {
			if err := check(sub.ID, "subsection"); err != nil {
				return err
			}
			for _, slide := range sub.Slides {
				if err := check(slide.ID, "slide"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

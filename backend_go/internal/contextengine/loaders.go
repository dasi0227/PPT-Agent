package contextengine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type ProjectLoader struct{}

func (ProjectLoader) Load(project model.Project) ProjectContext {
	return ProjectContext{ID: project.ID, Title: project.Title, ThemeID: project.Theme, Decorations: project.Decorations, Logo: project.Logo}
}

type OutlineLoader struct{}

func (OutlineLoader) Load(workDir string) (pptspec.Outline, error) {
	parsed, err := readSourceJSON(filepath.Join(workDir, ".outline.json"), "outline")
	if errors.Is(err, fs.ErrNotExist) {
		return pptspec.Outline{Sections: []pptspec.Section{}}, nil
	}
	if err != nil {
		return pptspec.Outline{}, err
	}
	return *parsed.(*pptspec.Outline), nil
}

type MetaLoader struct{}

func (MetaLoader) Load(workDir string) (pptspec.Meta, error) {
	parsed, err := readSourceJSON(filepath.Join(workDir, ".meta.json"), "meta")
	if err != nil {
		return pptspec.Meta{}, err
	}
	return *parsed.(*pptspec.Meta), nil
}

type SlideSpecLoader struct{}

func (SlideSpecLoader) LoadAll(workDir string, ids []string) (map[string]pptspec.SlideSpec, error) {
	out := make(map[string]pptspec.SlideSpec, len(ids))
	for _, id := range ids {
		raw, err := pptspec.ReadSlideSpec(func(path string) ([]byte, error) { return os.ReadFile(filepath.Join(workDir, path)) }, id)
		// This is summary context, not a source read: omit only this page
		// when unavailable. read_resource reports its exact missing/invalid error.
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, pptspec.ErrInvalid) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrSourceInvalid, id, err)
		}
		var slide pptspec.SlideSpec
		if err := json.Unmarshal(raw, &slide); err != nil {
			return nil, err
		}
		out[id] = slide
	}
	return out, nil
}

type ConstraintsLoader struct{}

func (ConstraintsLoader) Load(workDir string) (pptspec.Constraints, error) {
	parsed, err := readSourceJSON(filepath.Join(workDir, ".constraints.json"), "constraints")
	if err != nil {
		return pptspec.Constraints{}, err
	}
	return *parsed.(*pptspec.Constraints), nil
}

func readSourceJSON(path, kind string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	parsed, err := pptspec.ParseStrictSourceJSON(raw, kind)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrSourceInvalid, path, err)
	}
	return parsed, nil
}

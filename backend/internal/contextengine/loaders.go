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
	var outline pptspec.Outline
	raw, err := os.ReadFile(filepath.Join(workDir, ".outline.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return pptspec.Outline{Sections: []pptspec.Section{}}, nil
	}
	if err != nil {
		return outline, err
	}
	parsed, err := pptspec.ParseStrictSourceJSON(raw, "outline")
	if err != nil {
		return outline, fmt.Errorf("%w: %v", ErrSourceInvalid, err)
	}
	return *parsed.(*pptspec.Outline), nil
}

type ManifestLoader struct{}

func (ManifestLoader) Load(workDir string) (pptspec.Manifest, error) {
	var manifest pptspec.Manifest
	return manifest, readSourceJSON(filepath.Join(workDir, ".manifest.json"), &manifest)
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

type DesignLoader struct{}

func (DesignLoader) Load(workDir string) (pptspec.Design, error) {
	var design pptspec.Design
	return design, readSourceJSON(filepath.Join(workDir, ".design.json"), &design)
}

func readSourceJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrSourceInvalid, path, err)
	}
	return nil
}

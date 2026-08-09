package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

var ErrSpecRevisionConflict = errors.New("spec revision conflict")

type SpecService struct {
	store store.Store
	clock func() int64
}

func NewSpecService(s store.Store) *SpecService {
	return &SpecService{store: s, clock: func() int64 { return time.Now().Unix() }}
}

func (s *SpecService) EnsureProject(ctx context.Context, projectID string) (spec.ProjectView, error) {
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return spec.ProjectView{}, err
	}
	slides, err := s.store.ListSlides(ctx, projectID)
	if err != nil {
		return spec.ProjectView{}, err
	}
	return s.read(project, slides)
}

func (s *SpecService) GetSlide(ctx context.Context, slideID string) (spec.SlideSpec, spec.Materialization, error) {
	meta, err := s.store.GetSlide(ctx, slideID)
	if err != nil {
		return spec.SlideSpec{}, spec.Materialization{}, err
	}
	view, err := s.EnsureProject(ctx, meta.ProjectID)
	if err != nil {
		return spec.SlideSpec{}, spec.Materialization{}, err
	}
	return view.SlideSpecs[slideID], view.States[slideID], nil
}

func (s *SpecService) PatchSlide(ctx context.Context, slideID string, expected int, next spec.SlideSpec) (spec.SlideSpec, error) {
	meta, err := s.store.GetSlide(ctx, slideID)
	if err != nil {
		return spec.SlideSpec{}, err
	}
	view, err := s.EnsureProject(ctx, meta.ProjectID)
	if err != nil {
		return spec.SlideSpec{}, err
	}
	current := view.SlideSpecs[slideID]
	if expected != current.Revision {
		return spec.SlideSpec{}, ErrSpecRevisionConflict
	}
	next.SchemaVersion, next.ProjectID, next.SlideID = spec.SchemaVersion, meta.ProjectID, slideID
	next.SectionID, next.SubsectionID = current.SectionID, current.SubsectionID
	next.Revision, next.CreatedAt, next.UpdatedAt = current.Revision+1, current.CreatedAt, s.clock()
	if err := spec.ValidateSlideSpec(next); err != nil {
		return spec.SlideSpec{}, err
	}
	raw, _ := json.MarshalIndent(next, "", "  ")
	project, err := s.store.GetProject(ctx, meta.ProjectID)
	if err != nil {
		return spec.SlideSpec{}, err
	}
	path := filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideSpecPath(slideID)))
	if err := atomicWrite(path, raw); err != nil {
		return spec.SlideSpec{}, err
	}
	return next, nil
}

func (s *SpecService) ReplaceOutline(ctx context.Context, projectID string, expected int, next spec.Outline) (spec.Outline, error) {
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return spec.Outline{}, err
	}
	view, err := s.EnsureProject(ctx, projectID)
	if err != nil {
		return spec.Outline{}, err
	}
	if expected != view.Outline.Revision {
		return spec.Outline{}, ErrSpecRevisionConflict
	}
	next.SchemaVersion, next.ProjectID = spec.SchemaVersion, projectID
	next.Revision, next.CreatedAt, next.UpdatedAt = view.Outline.Revision+1, view.Outline.CreatedAt, s.clock()
	if err := spec.ValidateOutline(next, view.SlideSpecs); err != nil {
		return spec.Outline{}, err
	}
	path := filepath.Join(project.WorkDir, "outline.json")
	if err := atomicWrite(path, mustJSON(next)); err != nil {
		return spec.Outline{}, err
	}
	return next, nil
}

func (s *SpecService) ReplaceDesign(ctx context.Context, projectID string, expected int, next spec.Design) (spec.Design, error) {
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return spec.Design{}, err
	}
	view, err := s.EnsureProject(ctx, projectID)
	if err != nil {
		return spec.Design{}, err
	}
	if expected != view.Design.Revision {
		return spec.Design{}, ErrSpecRevisionConflict
	}
	next.SchemaVersion, next.ProjectID = spec.SchemaVersion, projectID
	next.Revision = view.Design.Revision + 1
	next.CreatedAt, next.UpdatedAt = view.Design.CreatedAt, s.clock()
	if next.CreatedAt == 0 {
		next.CreatedAt = next.UpdatedAt
	}
	if err := spec.ValidateDesign(next); err != nil {
		return spec.Design{}, err
	}
	path := filepath.Join(project.WorkDir, "design.json")
	if err := atomicWrite(path, mustJSON(next)); err != nil {
		return spec.Design{}, err
	}
	return next, nil
}

func (s *SpecService) read(project model.Project, metas []model.Slide) (spec.ProjectView, error) {
	outlineRaw, err := os.ReadFile(filepath.Join(project.WorkDir, "outline.json"))
	if err != nil {
		return spec.ProjectView{}, err
	}
	var outline spec.Outline
	if err := json.Unmarshal(outlineRaw, &outline); err != nil {
		return spec.ProjectView{}, err
	}
	designRaw, err := os.ReadFile(filepath.Join(project.WorkDir, "design.json"))
	if err != nil {
		return spec.ProjectView{}, err
	}
	var design spec.Design
	if err := json.Unmarshal(designRaw, &design); err != nil {
		return spec.ProjectView{}, err
	}
	out := spec.ProjectView{
		Outline: outline, Design: design,
		SlideSpecs: map[string]spec.SlideSpec{}, States: map[string]spec.Materialization{},
	}
	for _, meta := range metas {
		specPath := filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideSpecPath(meta.ID)))
		specRaw, err := os.ReadFile(specPath)
		if err != nil {
			return spec.ProjectView{}, err
		}
		var slide spec.SlideSpec
		if err := json.Unmarshal(specRaw, &slide); err != nil {
			return spec.ProjectView{}, err
		}
		out.SlideSpecs[meta.ID] = slide
		htmlRaw, htmlErr := os.ReadFile(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideHTMLPath(meta.ID))))
		materialization, materializationErr := spec.ReadMaterialization(
			filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideMaterializationPath(meta.ID))),
		)
		var record *spec.MaterializationRecord
		revs := model.MaterializationRevisions{}
		if materializationErr == nil {
			record = &materialization
			revs = model.MaterializationRevisions{
				SlideHTML: materialization.Artifact.Revision,
				Outline:   materialization.Source.Outline,
				SlideSpec: materialization.Source.Spec,
				Design:    materialization.Source.Design,
			}
		}
		out.States[meta.ID] = spec.Materialization{
			State: spec.DeriveMaterializationState(
				htmlErr == nil,
				record,
				outline.Revision,
				slide.Revision,
				design.Revision,
				spec.ContentHash(htmlRaw),
				spec.SourceHash(outlineRaw, specRaw, designRaw),
			),
			Revisions: revs,
		}
	}
	if err := spec.ValidateOutline(outline, out.SlideSpecs); err != nil {
		return spec.ProjectView{}, err
	}
	return out, nil
}

func defaultDesign(projectID string, now int64) spec.Design {
	return spec.Design{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID,
		Theme:     "swiss-modern",
		Direction: "清晰、克制、结构化的通用商务演示",
		Density:   "medium",
		Chrome: []spec.ChromeItem{
			{Type: "page_number", Placement: "bottom-right", Style: "tiny muted mono counter"},
			{Type: "section_marker", Placement: "top-left", Style: "compact section label"},
		},
		CreatedAt: now, UpdatedAt: now,
	}
}

func atomicWrite(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".artifact-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(raw); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

func readJSON(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func mustJSON(v any) []byte {
	raw, _ := json.MarshalIndent(v, "", "  ")
	return raw
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

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
	next.SourceOutlineRevision = view.Outline.Revision
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
	if err := s.store.UpdateSlideMeta(ctx, slideID, next.Title, next.VisualIntent.Archetype); err != nil {
		_ = atomicWrite(path, mustJSON(current))
		return spec.SlideSpec{}, err
	}
	if err := s.store.UpdateSlideRevisions(ctx, slideID, next.Revision, meta.HTMLRevision, meta.SourceOutlineRevision, meta.SourceSpecRevision, meta.SourceDesignRevision); err != nil {
		_ = atomicWrite(path, mustJSON(current))
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
	if err := s.store.UpdateProjectRevisions(ctx, projectID, next.Revision, view.Design.Revision); err != nil {
		_ = atomicWrite(path, mustJSON(view.Outline))
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
	if err := s.store.UpdateProjectRevisions(ctx, projectID, view.Outline.Revision, next.Revision); err != nil {
		_ = atomicWrite(path, mustJSON(view.Design))
		return spec.Design{}, err
	}
	return next, nil
}

func (s *SpecService) read(project model.Project, metas []model.Slide) (spec.ProjectView, error) {
	var outline spec.Outline
	if err := readJSON(filepath.Join(project.WorkDir, "outline.json"), &outline); err != nil {
		return spec.ProjectView{}, err
	}
	var design spec.Design
	if err := readJSON(filepath.Join(project.WorkDir, "design.json"), &design); err != nil {
		return spec.ProjectView{}, err
	}
	out := spec.ProjectView{
		Outline: outline, Design: design,
		SlideSpecs: map[string]spec.SlideSpec{}, States: map[string]spec.Materialization{},
	}
	for _, meta := range metas {
		var slide spec.SlideSpec
		if err := readJSON(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideSpecPath(meta.ID))), &slide); err != nil {
			return spec.ProjectView{}, err
		}
		out.SlideSpecs[meta.ID] = slide
		_, statErr := os.Stat(filepath.Join(project.WorkDir, filepath.FromSlash(meta.HTMLPath)))
		revs := model.MaterializationRevisions{
			SlideHTML: meta.HTMLRevision, Outline: meta.SourceOutlineRevision,
			SlideSpec: meta.SourceSpecRevision, Design: meta.SourceDesignRevision,
		}
		out.States[meta.ID] = spec.Materialization{
			State:     string(model.DeriveMaterializationState(statErr == nil, outline.Revision, slide.Revision, design.Revision, revs)),
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
		Canvas:  spec.CanvasSpec{Width: 1600, Height: 900, Ratio: "16:9"},
		Palette: []string{"#111827", "#ffffff", "#2563eb"},
		Typography: spec.TypographySpec{
			Display: spec.FontSpec{Family: "Inter, sans-serif", Weight: 700},
			Body:    spec.FontSpec{Family: "Inter, sans-serif", Weight: 400},
			Utility: spec.FontSpec{Family: "Inter, sans-serif", Weight: 500},
		},
		Spacing: spec.SpacingSpec{Unit: 8}, Radius: spec.RadiusSpec{Card: 16},
		Shadows:      spec.ShadowSpec{Card: "0 12px 36px rgba(15,23,42,.12)"},
		LayoutSystem: spec.LayoutSystem{Grid: "12-col", Rhythm: "generous", Density: "medium"},
		Signature:    "minimal geometric accent", Motion: spec.MotionSpec{Policy: "restrained"},
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

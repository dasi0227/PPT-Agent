package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

var ErrSlideHTMLMissing = errors.New("service: slide html not rendered")

func projectSlidesFromFiles(ctx context.Context, st store.Store, projectID string, slides []model.Slide) ([]model.Slide, error) {
	project, err := st.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	byID := map[string]model.Slide{}
	for _, slide := range slides {
		byID[slide.ID] = slide
	}
	var outline spec.Outline
	if err = readJSON(filepath.Join(project.WorkDir, "outline.json"), &outline); err != nil {
		return nil, err
	}
	out := make([]model.Slide, 0, len(spec.FlattenOutline(outline)))
	for index, loc := range spec.FlattenOutline(outline) {
		id := loc.Slide.SlideID
		meta := byID[id]
		meta.ID = id
		meta.ProjectID = projectID
		meta.Position = index
		meta.Title = loc.Slide.Label
		meta.SpecPath = model.SlideSpecPath(id)
		meta.HTMLPath = model.SlideHTMLPath(id)
		var content spec.SlideSpec
		if readJSON(filepath.Join(project.WorkDir, filepath.FromSlash(meta.SpecPath)), &content) == nil {
			meta.Title = content.Title
			meta.Layout = content.Layout
			meta.SpecRevision = content.Revision
		}
		if record, readErr := spec.ReadMaterialization(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideMaterializationPath(id)))); readErr == nil {
			meta.HTMLRevision = record.Artifact.Revision
			meta.SourceOutlineRevision = outline.Revision
			meta.SourceSpecRevision = record.Source.SpecRevision
			meta.SourceDesignRevision = record.Source.DesignRevision
		}
		out = append(out, meta)
	}
	return out, nil
}
func (svc *SlideService) ReadHTML(ctx context.Context, slideID string) ([]byte, error) {
	slide, err := svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return nil, err
	}
	project, err := svc.store.GetProject(ctx, slide.ProjectID)
	if err != nil {
		return nil, err
	}
	sandbox, err := artifactfs.NewSandbox(project.WorkDir)
	if err != nil {
		return nil, err
	}
	raw, err := sandbox.Read(model.SlideHTMLPath(slideID))
	if os.IsNotExist(err) {
		return nil, ErrSlideHTMLMissing
	}
	return raw, err
}
func (svc *SlideService) ReadSpec(ctx context.Context, slideID string) (spec.SlideSpec, spec.Materialization, error) {
	return NewSpecService(svc.store).GetSlide(ctx, slideID)
}
func (svc *SlideService) ReadContent(ctx context.Context, slideID string) (spec.SlideSpec, error) {
	slide, err := svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return spec.SlideSpec{}, err
	}
	project, err := svc.store.GetProject(ctx, slide.ProjectID)
	if err != nil {
		return spec.SlideSpec{}, err
	}
	var content spec.SlideSpec
	if err = readJSON(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideSpecPath(slideID))), &content); err != nil {
		return spec.SlideSpec{}, err
	}
	return content, spec.ValidateSlideSpec(content)
}

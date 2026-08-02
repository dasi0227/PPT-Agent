package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

var ErrSlideHTMLMissing = errors.New("service: slide html not rendered")

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
	if err := readJSON(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideSpecPath(slideID))), &content); err != nil {
		return spec.SlideSpec{}, err
	}
	if err := spec.ValidateSlideSpec(content); err != nil {
		return spec.SlideSpec{}, err
	}
	return content, nil
}

func (svc *SlideService) AddSlide(ctx context.Context, projectID, afterSlideID, layout string) (model.Slide, error) {
	if active, err := svc.store.HasActiveRun(ctx, projectID); err != nil {
		return model.Slide{}, err
	} else if active {
		return model.Slide{}, ErrRunActive
	}
	project, err := svc.store.GetProject(ctx, projectID)
	if err != nil {
		return model.Slide{}, err
	}
	view, err := NewSpecService(svc.store).EnsureProject(ctx, projectID)
	if err != nil {
		return model.Slide{}, err
	}
	sectionID := "section-main"
	nextOutline := view.Outline
	if len(nextOutline.Sections) == 0 {
		nextOutline.Sections = []spec.Section{{
			ID: sectionID, Number: "01", Title: "正文", Subsections: []spec.Subsection{},
		}}
	} else {
		sectionID = nextOutline.Sections[0].ID
	}
	id := svc.newID()
	now := svc.clock()
	if layout == "" {
		layout = "content"
	}
	content := spec.SlideSpec{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, SlideID: id,
		SourceOutlineRevision: nextOutline.Revision + 1,
		SectionID:             sectionID, Role: "context", Title: "未命名页面",
		KeyMessage: "待补充本页核心信息",
		Content:    spec.Content{Summary: "待补充本页内容", Points: []string{}},
		VisualIntent: spec.VisualIntent{
			Archetype: layout, Description: "使用清晰的信息层级表达本页核心信息", AssetQueries: []string{},
		},
		SpeakerNotes: "", CreatedAt: now, UpdatedAt: now,
	}
	order := insertAfter(nextOutline.SlideOrder, afterSlideID, id)
	nextOutline.SlideOrder = order
	if err := atomicWrite(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideSpecPath(id))), mustJSON(content)); err != nil {
		return model.Slide{}, err
	}
	meta := model.Slide{
		ID: id, ProjectID: projectID, Position: len(order) - 1,
		Layout: layout, Title: content.Title, SpecPath: model.SlideSpecPath(id),
		HTMLPath: model.SlideHTMLPath(id), SpecRevision: 1,
	}
	if err := svc.store.InsertSlide(ctx, meta); err != nil {
		_ = os.RemoveAll(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideDir(id))))
		return model.Slide{}, err
	}
	if _, err := NewSpecService(svc.store).ReplaceOutline(ctx, projectID, view.Outline.Revision, nextOutline); err != nil {
		_ = svc.store.DeleteSlideByID(ctx, id)
		_ = os.RemoveAll(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideDir(id))))
		return model.Slide{}, err
	}
	return meta, nil
}

func (svc *SlideService) DeleteSlide(ctx context.Context, slideID string) error {
	meta, err := svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return err
	}
	if active, err := svc.store.HasActiveRun(ctx, meta.ProjectID); err != nil {
		return err
	} else if active {
		return ErrRunActive
	}
	project, err := svc.store.GetProject(ctx, meta.ProjectID)
	if err != nil {
		return err
	}
	view, err := NewSpecService(svc.store).EnsureProject(ctx, meta.ProjectID)
	if err != nil {
		return err
	}
	nextOutline := view.Outline
	nextOutline.SlideOrder = removeID(nextOutline.SlideOrder, slideID)
	if _, err := NewSpecService(svc.store).ReplaceOutline(ctx, meta.ProjectID, view.Outline.Revision, nextOutline); err != nil {
		return err
	}
	if err := svc.store.DeleteSlideByID(ctx, slideID); err != nil {
		_ = atomicWrite(filepath.Join(project.WorkDir, "outline.json"), mustJSON(view.Outline))
		return err
	}
	return os.RemoveAll(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideDir(slideID))))
}

func (svc *SlideService) ReorderSlides(ctx context.Context, projectID string, orderedIDs []string) error {
	if active, err := svc.store.HasActiveRun(ctx, projectID); err != nil {
		return err
	} else if active {
		return ErrRunActive
	}
	view, err := NewSpecService(svc.store).EnsureProject(ctx, projectID)
	if err != nil {
		return err
	}
	if !sameIDs(view.Outline.SlideOrder, orderedIDs) {
		return validationError("ordered_ids must contain every slide exactly once")
	}
	nextOutline := view.Outline
	nextOutline.SlideOrder = append([]string{}, orderedIDs...)
	if _, err := NewSpecService(svc.store).ReplaceOutline(ctx, projectID, view.Outline.Revision, nextOutline); err != nil {
		return err
	}
	positions := make(map[string]int, len(orderedIDs))
	for index, id := range orderedIDs {
		positions[id] = index
	}
	return svc.store.SetSlidesOrder(ctx, projectID, positions)
}

func insertAfter(values []string, after, id string) []string {
	if after == "" {
		return append(append([]string{}, values...), id)
	}
	out := make([]string, 0, len(values)+1)
	inserted := false
	for _, value := range values {
		out = append(out, value)
		if value == after {
			out = append(out, id)
			inserted = true
		}
	}
	if !inserted {
		out = append(out, id)
	}
	return out
}

func removeID(values []string, id string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != id {
			out = append(out, value)
		}
	}
	return out
}

func sameIDs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := map[string]int{}
	for _, value := range left {
		seen[value]++
	}
	for _, value := range right {
		seen[value]--
		if seen[value] < 0 {
			return false
		}
	}
	return true
}

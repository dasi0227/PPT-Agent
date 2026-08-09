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

type SlidePlacement struct {
	SlideID      string `json:"slide_id"`
	SectionID    string `json:"section_id"`
	SubsectionID string `json:"subsection_id,omitempty"`
}

// projectSlidesFromFiles projects the file-owned content fields (order, title,
// layout) onto DB slide metadata. Order comes from outline.json's slide_order;
// title/layout come from each spec.json. Slides not present in the outline are
// appended after ordered ones so nothing silently disappears from the API.
func projectSlidesFromFiles(ctx context.Context, st store.Store, projectID string, slides []model.Slide) ([]model.Slide, error) {
	project, err := st.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]model.Slide, len(slides))
	for _, slide := range slides {
		byID[slide.ID] = slide
	}
	var outline spec.Outline
	if err := readJSON(filepath.Join(project.WorkDir, "outline.json"), &outline); err != nil {
		return nil, err
	}
	position := 0
	fill := func(meta model.Slide) model.Slide {
		var content spec.SlideSpec
		if readErr := readJSON(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideSpecPath(meta.ID))), &content); readErr == nil {
			meta.Title = content.Title
			meta.Layout = content.Layout
			meta.SpecRevision = content.Revision
		}
		if materialization, readErr := spec.ReadMaterialization(
			filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideMaterializationPath(meta.ID))),
		); readErr == nil {
			meta.HTMLRevision = materialization.Artifact.Revision
			meta.SourceOutlineRevision = materialization.Source.Outline
			meta.SourceSpecRevision = materialization.Source.Spec
			meta.SourceDesignRevision = materialization.Source.Design
		}
		meta.Position = position
		position++
		return meta
	}
	out := make([]model.Slide, 0, len(slides))
	seen := make(map[string]bool, len(slides))
	for _, id := range outline.SlideOrder {
		if meta, ok := byID[id]; ok {
			out = append(out, fill(meta))
			seen[id] = true
		}
	}
	for _, slide := range slides {
		if !seen[slide.ID] {
			out = append(out, fill(slide))
		}
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
	sectionID := model.MustShortID("sec")
	nextOutline := view.Outline
	if len(nextOutline.Sections) == 0 {
		nextOutline.Sections = []spec.Section{{
			ID: sectionID, Title: "正文", Purpose: "承载主要内容页面", Subsections: []spec.Subsection{},
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
		SectionID: sectionID, Role: "context", Title: "未命名页面",
		KeyMessage: "待补充本页核心信息",
		Elements: []spec.Element{{
			Type: "text", Intent: "补充支持本页核心信息的正文内容",
		}},
		Layout: layout, CreatedAt: now, UpdatedAt: now,
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

func (svc *SlideService) RestructureSlides(ctx context.Context, projectID string, orderedIDs []string, placements []SlidePlacement) error {
	if active, err := svc.store.HasActiveRun(ctx, projectID); err != nil {
		return err
	} else if active {
		return ErrRunActive
	}
	project, err := svc.store.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	view, err := NewSpecService(svc.store).EnsureProject(ctx, projectID)
	if err != nil {
		return err
	}
	if !sameIDs(view.Outline.SlideOrder, orderedIDs) {
		return validationError("ordered_ids must contain every slide exactly once")
	}
	if len(placements) != len(orderedIDs) {
		return validationError("placements must contain every slide exactly once")
	}

	sectionIDs := make(map[string]bool, len(view.Outline.Sections))
	subsectionOwners := map[string]string{}
	for _, section := range view.Outline.Sections {
		sectionIDs[section.ID] = true
		for _, subsection := range section.Subsections {
			subsectionOwners[subsection.ID] = section.ID
		}
	}

	seen := map[string]bool{}
	nextSpecs := make(map[string]spec.SlideSpec, len(view.SlideSpecs))
	for id, slide := range view.SlideSpecs {
		nextSpecs[id] = slide
	}

	nextOutline := view.Outline
	nextOutline.SlideOrder = append([]string{}, orderedIDs...)
	nextOutline.Revision = view.Outline.Revision + 1
	now := svc.clock()
	changed := map[string]spec.SlideSpec{}
	for _, placement := range placements {
		if seen[placement.SlideID] {
			return validationError("placements must not contain duplicate slide_id %s", placement.SlideID)
		}
		seen[placement.SlideID] = true
		current, ok := view.SlideSpecs[placement.SlideID]
		if !ok {
			return validationError("placement references unknown slide_id %s", placement.SlideID)
		}
		if !sectionIDs[placement.SectionID] {
			return validationError("placement references unknown section_id %s", placement.SectionID)
		}
		if placement.SubsectionID != "" && subsectionOwners[placement.SubsectionID] != placement.SectionID {
			return validationError("placement subsection_id %s does not belong to section_id %s", placement.SubsectionID, placement.SectionID)
		}
		next := current
		if next.SectionID != placement.SectionID || next.SubsectionID != placement.SubsectionID {
			next.SectionID = placement.SectionID
			next.SubsectionID = placement.SubsectionID
			next.Revision++
			next.UpdatedAt = now
			if err := spec.ValidateSlideSpec(next); err != nil {
				return validationError("%v", err)
			}
			changed[placement.SlideID] = next
		}
		nextSpecs[placement.SlideID] = next
	}
	for _, id := range orderedIDs {
		if !seen[id] {
			return validationError("placements missing slide_id %s", id)
		}
	}
	if err := spec.ValidateOutline(nextOutline, nextSpecs); err != nil {
		return validationError("%v", err)
	}

	rollbackSpecs := map[string][]byte{}
	for id, next := range changed {
		path := filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideSpecPath(id)))
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rollbackSpecs[id] = raw
		if err := atomicWrite(path, mustJSON(next)); err != nil {
			for rollbackID, rollbackRaw := range rollbackSpecs {
				_ = atomicWrite(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideSpecPath(rollbackID))), rollbackRaw)
			}
			return err
		}
	}
	if _, err := NewSpecService(svc.store).ReplaceOutline(ctx, projectID, view.Outline.Revision, nextOutline); err != nil {
		for id, raw := range rollbackSpecs {
			_ = atomicWrite(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideSpecPath(id))), raw)
		}
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

package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
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

func (svc *SlideService) ReadBlueprint(ctx context.Context, slideID string) (blueprint.Slide, blueprint.Materialization, error) {
	return NewBlueprintService(svc.store).GetSlide(ctx, slideID)
}

func (svc *SlideService) ReadContent(ctx context.Context, slideID string) (blueprint.Slide, error) {
	slide, err := svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return blueprint.Slide{}, err
	}
	project, err := svc.store.GetProject(ctx, slide.ProjectID)
	if err != nil {
		return blueprint.Slide{}, err
	}
	var content blueprint.Slide
	if err := readJSON(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideJSONPath(slideID))), &content); err != nil {
		return blueprint.Slide{}, err
	}
	if err := blueprint.ValidateSlide(content); err != nil {
		return blueprint.Slide{}, err
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
	view, err := NewBlueprintService(svc.store).EnsureProject(ctx, projectID)
	if err != nil {
		return model.Slide{}, err
	}
	sectionID := "section-main"
	nextDeck := view.Deck
	if len(nextDeck.Sections) == 0 {
		nextDeck.Sections = []blueprint.Section{{
			ID: sectionID, Number: "01", Title: "正文", Subsections: []blueprint.Subsection{},
		}}
	} else {
		sectionID = nextDeck.Sections[0].ID
	}
	id := svc.newID()
	now := svc.clock()
	if layout == "" {
		layout = "content"
	}
	content := blueprint.Slide{
		SchemaVersion: blueprint.SchemaVersion, Revision: 1, SlideID: id,
		SectionID: sectionID, Role: "context", Title: "未命名页面",
		KeyMessage: "待补充本页核心信息",
		Content:    blueprint.Content{Summary: "待补充本页内容", Points: []string{}},
		VisualIntent: blueprint.VisualIntent{
			Archetype: layout, Description: "使用清晰的信息层级表达本页核心信息", AssetQueries: []string{},
		},
		SpeakerNotes: "", CreatedAt: now, UpdatedAt: now,
	}
	order := insertAfter(nextDeck.SlideOrder, afterSlideID, id)
	nextDeck.SlideOrder = order
	if err := atomicWrite(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideJSONPath(id))), mustJSON(content)); err != nil {
		return model.Slide{}, err
	}
	meta := model.Slide{
		ID: id, ProjectID: projectID, Position: len(order) - 1,
		Layout: layout, Title: content.Title, JSONPath: model.SlideJSONPath(id),
		HTMLPath: model.SlideHTMLPath(id), BlueprintRevision: 1,
	}
	if err := svc.store.InsertSlide(ctx, meta); err != nil {
		_ = os.RemoveAll(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideDir(id))))
		return model.Slide{}, err
	}
	if _, err := NewBlueprintService(svc.store).ReplaceDeck(ctx, projectID, view.Deck.Revision, nextDeck); err != nil {
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
	view, err := NewBlueprintService(svc.store).EnsureProject(ctx, meta.ProjectID)
	if err != nil {
		return err
	}
	nextDeck := view.Deck
	nextDeck.SlideOrder = removeID(nextDeck.SlideOrder, slideID)
	if _, err := NewBlueprintService(svc.store).ReplaceDeck(ctx, meta.ProjectID, view.Deck.Revision, nextDeck); err != nil {
		return err
	}
	if err := svc.store.DeleteSlideByID(ctx, slideID); err != nil {
		_ = atomicWrite(filepath.Join(project.WorkDir, "deck.json"), mustJSON(view.Deck))
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
	view, err := NewBlueprintService(svc.store).EnsureProject(ctx, projectID)
	if err != nil {
		return err
	}
	if !sameIDs(view.Deck.SlideOrder, orderedIDs) {
		return validationError("ordered_ids must contain every slide exactly once")
	}
	nextDeck := view.Deck
	nextDeck.SlideOrder = append([]string{}, orderedIDs...)
	if _, err := NewBlueprintService(svc.store).ReplaceDeck(ctx, projectID, view.Deck.Revision, nextDeck); err != nil {
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

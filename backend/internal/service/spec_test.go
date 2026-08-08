package service_test

import (
	"context"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

func TestCanonicalSpecLifecycleUsesStableSlideIDs(t *testing.T) {
	root := t.TempDir()
	db, cleanup, err := sqlitestore.Open(&config.Config{DBPath: filepath.Join(root, "canonical.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	store, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	project, err := service.NewProjectService(store, service.WorkRoot(root)).CreateProject(context.Background(), service.CreateProjectParams{
		Topic: "Canonical deck", Brief: "Explain the plan", Language: "zh-CN",
	})
	if err != nil {
		t.Fatal(err)
	}
	slides := service.NewSlideService(store)
	first, err := slides.AddSlide(context.Background(), project.ID, "", "content")
	if err != nil {
		t.Fatal(err)
	}
	second, err := slides.AddSlide(context.Background(), project.ID, first.ID, "comparison")
	if err != nil {
		t.Fatal(err)
	}
	if err := slides.ReorderSlides(context.Background(), project.ID, []string{second.ID, first.ID}); err != nil {
		t.Fatal(err)
	}
	view, err := service.NewSpecService(store).EnsureProject(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Outline.SlideOrder) != 2 || view.Outline.SlideOrder[0] != second.ID {
		t.Fatalf("deck order=%v", view.Outline.SlideOrder)
	}
	if view.SlideSpecs[first.ID].SchemaVersion != spec.SchemaVersion || view.SlideSpecs[second.ID].SlideID != second.ID {
		t.Fatalf("canonical slides=%+v", view.SlideSpecs)
	}
	if got := view.States[first.ID].State; got != string(model.MaterializationNotMaterialized) {
		t.Fatalf("state=%s", got)
	}
	if err := slides.DeleteSlide(context.Background(), first.ID); err != nil {
		t.Fatal(err)
	}
	after, err := service.NewSpecService(store).EnsureProject(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Outline.SlideOrder) != 1 || after.Outline.SlideOrder[0] != second.ID {
		t.Fatalf("after delete=%v", after.Outline.SlideOrder)
	}
}

func TestRestructureSlidesUpdatesOrderAndPlacement(t *testing.T) {
	root := t.TempDir()
	db, cleanup, err := sqlitestore.Open(&config.Config{DBPath: filepath.Join(root, "restructure.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	store, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	project, err := service.NewProjectService(store, service.WorkRoot(root)).CreateProject(context.Background(), service.CreateProjectParams{
		Topic: "Restructure deck", Brief: "Explain structure", Language: "zh-CN",
	})
	if err != nil {
		t.Fatal(err)
	}
	slides := service.NewSlideService(store)
	first, err := slides.AddSlide(context.Background(), project.ID, "", "content")
	if err != nil {
		t.Fatal(err)
	}
	second, err := slides.AddSlide(context.Background(), project.ID, first.ID, "content")
	if err != nil {
		t.Fatal(err)
	}
	specSvc := service.NewSpecService(store)
	view, err := specSvc.EnsureProject(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	outline := view.Outline
	mainSectionID := view.SlideSpecs[first.ID].SectionID
	outline.Sections = []spec.Section{
		{ID: mainSectionID, Title: "Main", Purpose: "Main section", Subsections: []spec.Subsection{}},
		{ID: "section-two", Title: "Two", Purpose: "Second section", Subsections: []spec.Subsection{{ID: "sub-two", Title: "Sub"}}},
	}
	if _, err := specSvc.ReplaceOutline(context.Background(), project.ID, view.Outline.Revision, outline); err != nil {
		t.Fatal(err)
	}
	if err := slides.RestructureSlides(context.Background(), project.ID, []string{second.ID, first.ID}, []service.SlidePlacement{
		{SlideID: second.ID, SectionID: "section-two", SubsectionID: "sub-two"},
		{SlideID: first.ID, SectionID: mainSectionID},
	}); err != nil {
		t.Fatal(err)
	}
	after, err := specSvc.EnsureProject(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.Outline.SlideOrder; len(got) != 2 || got[0] != second.ID || got[1] != first.ID {
		t.Fatalf("order=%v", got)
	}
	moved := after.SlideSpecs[second.ID]
	if moved.SectionID != "section-two" || moved.SubsectionID != "sub-two" {
		t.Fatalf("placement=%+v", moved)
	}
	if moved.Revision != view.SlideSpecs[second.ID].Revision+1 {
		t.Fatalf("moved revision=%d", moved.Revision)
	}
}

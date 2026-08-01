package service_test

import (
	"context"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

func TestCanonicalBlueprintLifecycleUsesStableSlideIDs(t *testing.T) {
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
	view, err := service.NewBlueprintService(store).EnsureProject(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Deck.SlideOrder) != 2 || view.Deck.SlideOrder[0] != second.ID {
		t.Fatalf("deck order=%v", view.Deck.SlideOrder)
	}
	if view.Slides[first.ID].SchemaVersion != "2.0" || view.Slides[second.ID].SlideID != second.ID {
		t.Fatalf("canonical slides=%+v", view.Slides)
	}
	if got := view.States[first.ID].State; got != string(model.MaterializationNotMaterialized) {
		t.Fatalf("state=%s", got)
	}
	if err := slides.DeleteSlide(context.Background(), first.ID); err != nil {
		t.Fatal(err)
	}
	after, err := service.NewBlueprintService(store).EnsureProject(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Deck.SlideOrder) != 1 || after.Deck.SlideOrder[0] != second.ID {
		t.Fatalf("after delete=%v", after.Deck.SlideOrder)
	}
}

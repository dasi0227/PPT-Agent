package service_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

func TestBlueprintMigrationIsIdempotentAndKeepsStableIDs(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(root, "blueprint.db")}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	store, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	workDir := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(workDir, "slides", "stable-id"), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	project := model.Project{ID: "p", Title: "Legacy", WorkDir: workDir, Theme: "default", Status: "draft", CreatedAt: now, UpdatedAt: now}
	if err := store.CreateProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	slide := model.Slide{ID: "stable-id", ProjectID: "p", Idx: 0, Order: 0, Layout: "bullets", Title: "Legacy title", JSONPath: model.SlideJSONPath("stable-id"), HTMLPath: model.SlideHTMLPath("stable-id")}
	if err := store.InsertSlide(context.Background(), slide); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(slidejson.SlideJSON{ID: slide.ID, Layout: "bullets", Title: slide.Title, Bullets: []string{"one"}})
	if err := os.WriteFile(filepath.Join(workDir, filepath.FromSlash(slide.JSONPath)), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	svc := service.NewBlueprintService(store)
	first, err := svc.EnsureProject(context.Background(), "p")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.EnsureProject(context.Background(), "p")
	if err != nil {
		t.Fatal(err)
	}
	if first.Deck.Revision != second.Deck.Revision || second.Deck.SlideOrder[0] != "stable-id" || second.Slides["stable-id"].Revision != 1 {
		t.Fatalf("migration was not idempotent: first=%+v second=%+v", first.Deck, second.Deck)
	}
	if _, err := os.Stat(filepath.Join(workDir, "slides", "stable-id", "slide.legacy.json")); err != nil {
		t.Fatalf("legacy source backup missing: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workDir, filepath.FromSlash(slide.HTMLPath)), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateSlideRevisions(context.Background(), slide.ID, 1, 1, 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	nextDesign := second.DesignSpec
	nextDesign.Signature = "editorial contrast"
	if _, err := svc.ReplaceDesignSpec(context.Background(), project.ID, nextDesign.Revision, nextDesign); err != nil {
		t.Fatal(err)
	}
	afterDesign, err := svc.EnsureProject(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := afterDesign.States[slide.ID].State; got != string(model.MaterializationDesignStale) {
		t.Fatalf("design revision should derive design_stale, got %s", got)
	}

	slideSvc := service.NewSlideService(store)
	added, err := slideSvc.AddSlide(context.Background(), project.ID, slide.ID, "bullets")
	if err != nil {
		t.Fatal(err)
	}
	title := "新增蓝图页"
	if _, err := slideSvc.PatchContent(context.Background(), added.ID, service.SlidePatch{Title: &title}); err != nil {
		t.Fatal(err)
	}
	withAdded, err := svc.EnsureProject(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if withAdded.Slides[added.ID].Title != title || withAdded.Slides[added.ID].SchemaVersion != "2.0" {
		t.Fatalf("legacy slide endpoint downgraded blueprint: %+v", withAdded.Slides[added.ID])
	}
	if err := slideSvc.ReorderSlides(context.Background(), project.ID, []string{added.ID, slide.ID}); err != nil {
		t.Fatal(err)
	}
	reordered, err := svc.EnsureProject(context.Background(), project.ID)
	if err != nil || reordered.Deck.SlideOrder[0] != added.ID {
		t.Fatalf("deck order did not follow stable IDs: %+v, %v", reordered.Deck.SlideOrder, err)
	}
	if err := slideSvc.DeleteSlide(context.Background(), added.ID); err != nil {
		t.Fatal(err)
	}
	afterDelete, err := svc.EnsureProject(context.Background(), project.ID)
	if err != nil || len(afterDelete.Deck.SlideOrder) != 1 || afterDelete.Deck.SlideOrder[0] != slide.ID {
		t.Fatalf("deck aggregate not updated after delete: %+v, %v", afterDelete.Deck.SlideOrder, err)
	}
}
